package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// FileHandler handles file management requests.
type FileHandler struct {
	repo    *repository.Repository
	dataDir string
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler(repo *repository.Repository, dataDir string) *FileHandler {
	return &FileHandler{repo: repo, dataDir: dataDir}
}

// Upload handles multipart file upload with deduplication.
func (h *FileHandler) Upload(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "File upload required",
			Code:    http.StatusBadRequest,
			Details: "A file must be uploaded via multipart form",
		})
	}

	src, err := file.Open()
	if err != nil {
		logrus.WithError(err).Error("Failed to open uploaded file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process uploaded file",
			Code:  http.StatusInternalServerError,
		})
	}
	defer src.Close()

	// Read file content and compute SHA-256
	content, err := io.ReadAll(src)
	if err != nil {
		logrus.WithError(err).Error("Failed to read uploaded file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to read uploaded file",
			Code:  http.StatusInternalServerError,
		})
	}

	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Check for deduplication
	existing, err := h.repo.GetFileByHash(hashStr)
	if err == nil && existing != nil {
		logrus.WithField("sha256", hashStr).Info("Duplicate file detected, returning existing record")
		return c.JSON(http.StatusOK, existing)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(h.dataDir, 0755); err != nil {
		logrus.WithError(err).Error("Failed to create data directory")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to store file",
			Code:  http.StatusInternalServerError,
		})
	}

	// Save file to dataDir
	fileID := uuid.New().String()
	ext := filepath.Ext(file.Filename)
	storageName := fileID + ext
	storagePath := filepath.Join(h.dataDir, storageName)

	dst, err := os.Create(storagePath)
	if err != nil {
		logrus.WithError(err).Error("Failed to create destination file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to store file",
			Code:  http.StatusInternalServerError,
		})
	}
	defer dst.Close()

	if _, err := dst.Write(content); err != nil {
		logrus.WithError(err).Error("Failed to write file content")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to store file",
			Code:  http.StatusInternalServerError,
		})
	}

	// Detect MIME type
	mimeType := http.DetectContentType(content)

	managedFile := &models.ManagedFile{
		ID:           fileID,
		SHA256:       hashStr,
		OriginalName: file.Filename,
		MimeType:     mimeType,
		SizeBytes:    file.Size,
		StoragePath:  storagePath,
		CreatedAt:    time.Now(),
	}

	if err := h.repo.CreateFile(managedFile); err != nil {
		logrus.WithError(err).Error("Failed to create managed file record")
		// Clean up the file
		os.Remove(storagePath)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to record file",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"original_name": file.Filename,
		"size_bytes":    file.Size,
		"sha256":        hashStr,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "upload_file",
		EntityType: "managed_file",
		EntityID:   fileID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"file_id": fileID,
		"sha256":  hashStr,
	}).Info("File uploaded")

	return c.JSON(http.StatusCreated, managedFile)
}

// Download streams a file by ID.
func (h *FileHandler) Download(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "File ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	managedFile, err := h.repo.GetFileByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "File not found",
			Code:    http.StatusNotFound,
			Details: "No file found with the given ID",
		})
	}

	// Check if file exists on disk
	if _, err := os.Stat(managedFile.StoragePath); os.IsNotExist(err) {
		logrus.WithField("file_id", id).Error("File record exists but file is missing from disk")
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "File not found on disk",
			Code:    http.StatusNotFound,
			Details: "The file record exists but the physical file is missing",
		})
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", managedFile.OriginalName))
	c.Response().Header().Set("Content-Type", managedFile.MimeType)

	return c.File(managedFile.StoragePath)
}

// ExportZip creates a ZIP bundle from a list of file IDs.
func (h *FileHandler) ExportZip(c echo.Context) error {
	var req struct {
		FileIDs []string `json:"file_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if len(req.FileIDs) == 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "At least one file ID is required",
		})
	}

	files, err := h.repo.GetFilesByIDs(req.FileIDs)
	if err != nil {
		logrus.WithError(err).Error("Failed to get files for ZIP export")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve files",
			Code:  http.StatusInternalServerError,
		})
	}

	if len(files) == 0 {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "No files found",
			Code:    http.StatusNotFound,
			Details: "None of the specified file IDs were found",
		})
	}

	// Create ZIP in memory
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for _, f := range files {
		fileData, err := os.ReadFile(f.StoragePath)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"file_id":      f.ID,
				"storage_path": f.StoragePath,
			}).WithError(err).Warn("Skipping file that could not be read")
			continue
		}

		w, err := zipWriter.Create(f.OriginalName)
		if err != nil {
			logrus.WithError(err).Warn("Failed to create ZIP entry")
			continue
		}

		if _, err := w.Write(fileData); err != nil {
			logrus.WithError(err).Warn("Failed to write file to ZIP")
			continue
		}
	}

	if err := zipWriter.Close(); err != nil {
		logrus.WithError(err).Error("Failed to finalize ZIP archive")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create ZIP archive",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"file_ids": req.FileIDs,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "export_zip",
		EntityType: "managed_file",
		EntityID:   "bulk",
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":    userID,
		"file_count": len(files),
	}).Info("ZIP export created")

	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"export.zip\"")

	return c.Blob(http.StatusOK, "application/zip", buf.Bytes())
}

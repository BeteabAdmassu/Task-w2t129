package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// SystemHandler handles system-level requests.
type SystemHandler struct {
	repo        *repository.Repository
	dataDir     string
	databaseURL string
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(repo *repository.Repository, dataDir, databaseURL string) *SystemHandler {
	return &SystemHandler{repo: repo, dataDir: dataDir, databaseURL: databaseURL}
}

// HealthCheck returns the system health status.
func (h *SystemHandler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Backup performs a pg_dump of the database to the data directory.
func (h *SystemHandler) Backup(c echo.Context) error {
	userID := middleware.GetUserID(c)

	backupDir := filepath.Join(h.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		logrus.WithError(err).Error("Failed to create backup directory")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to prepare backup directory",
			Code:  http.StatusInternalServerError,
		})
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup_%s.sql", timestamp))

	cmd := exec.Command("pg_dump", "--dbname", h.databaseURL, "--file", backupFile, "--format=plain", "--no-password")
	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.WithError(err).WithField("output", string(output)).Error("pg_dump failed")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Backup failed",
			Code:    http.StatusInternalServerError,
			Details: "Database dump did not complete successfully",
		})
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "backup_completed",
		EntityType: "system",
		EntityID:   "backup",
	})

	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"backup_file": backupFile,
	}).Info("Backup completed")

	return c.JSON(http.StatusOK, map[string]string{
		"message":     "Backup completed successfully",
		"backup_file": backupFile,
		"timestamp":   timestamp,
	})
}

// BackupStatus returns backup status by checking for recent backup files.
func (h *SystemHandler) BackupStatus(c echo.Context) error {
	backupDir := filepath.Join(h.dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":      "no_backups",
			"last_backup": nil,
			"message":     "No backups found",
		})
	}

	var lastBackup *string
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			name := entries[i].Name()
			lastBackup = &name
			break
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":      "idle",
		"last_backup": lastBackup,
		"message":     "Backup system operational",
	})
}

// GetConfig returns the system configuration.
func (h *SystemHandler) GetConfig(c echo.Context) error {
	config, err := h.repo.GetConfig()
	if err != nil {
		logrus.WithError(err).Error("Failed to get system config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve system configuration",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"config": config,
	})
}

// UpdateConfig updates a system configuration value.
func (h *SystemHandler) UpdateConfig(c echo.Context) error {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Key == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Configuration key is required",
		})
	}

	if err := h.repo.UpdateConfig(req.Key, req.Value); err != nil {
		logrus.WithError(err).Error("Failed to update system config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update system configuration",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"key": req.Key,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_config",
		EntityType: "system_config",
		EntityID:   req.Key,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"key":     req.Key,
	}).Info("System config updated")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Configuration updated successfully",
		"key":     req.Key,
	})
}

// ApplyUpdate applies a pending offline update package from DATA_DIR/updates/.
// The package is a directory named "pending" containing SQL migration files and
// an optional shell script "post_migrate.sh". After a successful apply the
// directory is renamed to "applied_<timestamp>" for audit purposes.
func (h *SystemHandler) ApplyUpdate(c echo.Context) error {
	userID := middleware.GetUserID(c)

	updateDir := filepath.Join(h.dataDir, "updates", "pending")
	if _, err := os.Stat(updateDir); os.IsNotExist(err) {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "No pending update found",
			Code:    http.StatusNotFound,
			Details: "Place the update package in " + updateDir + " before calling this endpoint",
		})
	}

	// Run any .sql migration files in the package, ordered lexicographically.
	entries, err := os.ReadDir(updateDir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to read update package",
			Code:  http.StatusInternalServerError,
		})
	}

	applied := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		sqlPath := filepath.Join(updateDir, entry.Name())
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			logrus.WithError(err).WithField("file", entry.Name()).Error("Failed to read SQL migration file")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to read migration file",
				Code:    http.StatusInternalServerError,
				Details: entry.Name(),
			})
		}
		if _, err := h.repo.DB.Exec(string(sqlBytes)); err != nil {
			logrus.WithError(err).WithField("file", entry.Name()).Error("SQL migration failed")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Migration failed",
				Code:    http.StatusInternalServerError,
				Details: entry.Name() + ": " + err.Error(),
			})
		}
		applied++
	}

	// Rename pending → applied_<timestamp> for auditability.
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	appliedDir := filepath.Join(h.dataDir, "updates", "applied_"+timestamp)
	if err := os.Rename(updateDir, appliedDir); err != nil {
		logrus.WithError(err).Warn("Failed to rename applied update directory")
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "apply_update",
		EntityType: "system",
		EntityID:   "update",
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"migrations":   applied,
		"applied_path": appliedDir,
	}).Info("Offline update applied")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":      "Update applied successfully",
		"migrations":   applied,
		"applied_path": appliedDir,
		"timestamp":    timestamp,
	})
}

// Rollback restores the database from the most recent backup file in DATA_DIR/backups/.
// It uses psql to execute the SQL dump. The current database is NOT dropped first —
// the dump must contain IF NOT EXISTS / ON CONFLICT guards for idempotent re-apply.
func (h *SystemHandler) Rollback(c echo.Context) error {
	userID := middleware.GetUserID(c)

	backupDir := filepath.Join(h.dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "No backups available",
			Code:    http.StatusNotFound,
			Details: "Create a backup via POST /system/backup before attempting rollback",
		})
	}

	// Find the most recent .sql backup (entries are sorted lexicographically by ReadDir).
	var latestBackup string
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() && filepath.Ext(entries[i].Name()) == ".sql" {
			latestBackup = filepath.Join(backupDir, entries[i].Name())
			break
		}
	}
	if latestBackup == "" {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "No SQL backup files found",
			Code:  http.StatusNotFound,
		})
	}

	cmd := exec.Command("psql", "--dbname", h.databaseURL, "--file", latestBackup, "--no-password", "--single-transaction")
	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.WithError(err).WithField("output", string(output)).Error("Rollback (psql restore) failed")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Rollback failed",
			Code:    http.StatusInternalServerError,
			Details: "Database restore did not complete successfully",
		})
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "rollback_completed",
		EntityType: "system",
		EntityID:   "rollback",
	})

	logrus.WithFields(logrus.Fields{
		"user_id":        userID,
		"restored_from":  latestBackup,
	}).Info("Rollback completed")

	return c.JSON(http.StatusOK, map[string]string{
		"message":       "Rollback completed successfully",
		"restored_from": latestBackup,
	})
}

// SaveDraft saves a draft checkpoint.
func (h *SystemHandler) SaveDraft(c echo.Context) error {
	userID := middleware.GetUserID(c)

	var req struct {
		FormType  string          `json:"form_type"`
		FormID    *string         `json:"form_id"`
		StateJSON json.RawMessage `json:"state_json"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.FormType == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Form type is required",
		})
	}
	if req.StateJSON == nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "State JSON is required",
		})
	}

	draft := &models.DraftCheckpoint{
		ID:        uuid.New().String(),
		UserID:    userID,
		FormType:  req.FormType,
		FormID:    req.FormID,
		StateJSON: req.StateJSON,
		SavedAt:   time.Now(),
	}

	if err := h.repo.SaveDraft(draft); err != nil {
		logrus.WithError(err).Error("Failed to save draft")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to save draft",
			Code:  http.StatusInternalServerError,
		})
	}

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"draft_id": draft.ID,
	}).Info("Draft saved")

	return c.JSON(http.StatusCreated, draft)
}

// ListDrafts returns all drafts for the authenticated user.
func (h *SystemHandler) ListDrafts(c echo.Context) error {
	userID := middleware.GetUserID(c)

	drafts, err := h.repo.ListDrafts(userID)
	if err != nil {
		logrus.WithError(err).Error("Failed to list drafts")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve drafts",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": drafts,
	})
}

// GetDraft returns a single draft by ID.
func (h *SystemHandler) GetDraft(c echo.Context) error {
	id := c.Param("formId")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Draft ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	draft, err := h.repo.GetDraftByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Draft not found",
			Code:    http.StatusNotFound,
			Details: "No draft found with the given ID",
		})
	}

	// Ensure user can only access their own drafts
	userID := middleware.GetUserID(c)
	if draft.UserID != userID {
		role := middleware.GetUserRole(c)
		if role != "system_admin" {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Access denied",
				Code:    http.StatusForbidden,
				Details: "You can only access your own drafts",
			})
		}
	}

	return c.JSON(http.StatusOK, draft)
}

// DeleteDraft deletes a draft by ID.
func (h *SystemHandler) DeleteDraft(c echo.Context) error {
	id := c.Param("formId")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Draft ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	draft, err := h.repo.GetDraftByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Draft not found",
			Code:    http.StatusNotFound,
			Details: "No draft found with the given ID",
		})
	}

	// Ensure user can only delete their own drafts
	userID := middleware.GetUserID(c)
	if draft.UserID != userID {
		role := middleware.GetUserRole(c)
		if role != "system_admin" {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Access denied",
				Code:    http.StatusForbidden,
				Details: "You can only delete your own drafts",
			})
		}
	}

	if err := h.repo.DeleteDraftByID(id); err != nil {
		logrus.WithError(err).Error("Failed to delete draft")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to delete draft",
			Code:  http.StatusInternalServerError,
		})
	}

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"draft_id": id,
	}).Info("Draft deleted")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Draft deleted successfully",
	})
}

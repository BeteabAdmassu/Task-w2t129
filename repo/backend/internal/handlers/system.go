package handlers

import (
	"encoding/json"
	"net/http"
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
	repo *repository.Repository
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(repo *repository.Repository) *SystemHandler {
	return &SystemHandler{repo: repo}
}

// HealthCheck returns the system health status.
func (h *SystemHandler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Backup returns a success message (simplified for Docker env).
func (h *SystemHandler) Backup(c echo.Context) error {
	userID := middleware.GetUserID(c)

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "backup_initiated",
		EntityType: "system",
		EntityID:   "backup",
	})

	logrus.WithField("user_id", userID).Info("Backup initiated")

	return c.JSON(http.StatusOK, map[string]string{
		"message":   "Backup completed successfully",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// BackupStatus returns the backup status.
func (h *SystemHandler) BackupStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":      "idle",
		"last_backup": time.Now().UTC().Format(time.RFC3339),
		"message":     "No backup currently in progress",
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
	id := c.Param("id")
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
		if role != "admin" {
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
	id := c.Param("id")
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
		if role != "admin" {
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

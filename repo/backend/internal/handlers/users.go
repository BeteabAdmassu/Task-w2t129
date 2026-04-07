package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// UserHandler handles user management requests (admin-only).
type UserHandler struct {
	repo *repository.Repository
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(repo *repository.Repository) *UserHandler {
	return &UserHandler{repo: repo}
}

// validRoles defines the set of allowed user roles.
var validRoles = map[string]bool{
	"admin":            true,
	"pharmacist":       true,
	"technician":       true,
	"maintenance_tech": true,
	"manager":          true,
	"staff":            true,
}

// ListUsers returns all users.
func (h *UserHandler) ListUsers(c echo.Context) error {
	users, err := h.repo.ListUsers()
	if err != nil {
		logrus.WithError(err).Error("Failed to list users")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve users",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": users,
	})
}

// CreateUser creates a new user account.
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req models.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	// Validate username
	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Username is required",
		})
	}

	// Validate password length
	if len(req.Password) < 12 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Password must be at least 12 characters long",
		})
	}

	// Validate role
	if !validRoles[req.Role] {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Invalid role. Must be one of: admin, pharmacist, technician, maintenance_tech, manager, staff",
		})
	}

	// Check if username already exists
	existing, _ := h.repo.GetUserByUsername(req.Username)
	if existing != nil {
		return c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Username already exists",
			Code:    http.StatusConflict,
			Details: "A user with this username already exists",
		})
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logrus.WithError(err).Error("Failed to hash password")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create user",
			Code:  http.StatusInternalServerError,
		})
	}

	now := time.Now()
	user := &models.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.repo.CreateUser(user); err != nil {
		logrus.WithError(err).Error("Failed to create user in database")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create user",
			Code:  http.StatusInternalServerError,
		})
	}

	adminID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"username": user.Username,
		"role":     user.Role,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     adminID,
		Action:     "create_user",
		EntityType: "user",
		EntityID:   user.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"admin_id": adminID,
		"new_user": user.ID,
		"role":     user.Role,
	}).Info("User created")

	return c.JSON(http.StatusCreated, user)
}

// UpdateUser updates a user's role and/or active status.
func (h *UserHandler) UpdateUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "User ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	user, err := h.repo.GetUserByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Code:    http.StatusNotFound,
			Details: "No user found with the given ID",
		})
	}

	var body struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if body.Role != nil {
		if !validRoles[*body.Role] {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Invalid role",
			})
		}
		user.Role = *body.Role
	}

	if body.IsActive != nil {
		user.IsActive = *body.IsActive
	}

	user.UpdatedAt = time.Now()
	if err := h.repo.UpdateUser(user); err != nil {
		logrus.WithError(err).Error("Failed to update user")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update user",
			Code:  http.StatusInternalServerError,
		})
	}

	adminID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"target_user": id,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     adminID,
		Action:     "update_user",
		EntityType: "user",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"admin_id":    adminID,
		"target_user": id,
	}).Info("User updated")

	return c.JSON(http.StatusOK, user)
}

// DeleteUser soft-deletes a user by setting is_active to false.
func (h *UserHandler) DeleteUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "User ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	user, err := h.repo.GetUserByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Code:    http.StatusNotFound,
			Details: "No user found with the given ID",
		})
	}

	// Prevent self-deletion
	adminID := middleware.GetUserID(c)
	if adminID == id {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Cannot delete yourself",
			Code:    http.StatusBadRequest,
			Details: "You cannot deactivate your own account",
		})
	}

	user.IsActive = false
	user.UpdatedAt = time.Now()
	if err := h.repo.UpdateUser(user); err != nil {
		logrus.WithError(err).Error("Failed to soft-delete user")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to deactivate user",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]string{
		"target_user": id,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     adminID,
		Action:     "delete_user",
		EntityType: "user",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"admin_id":    adminID,
		"target_user": id,
	}).Info("User soft-deleted")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "User deactivated successfully",
	})
}

// UnlockUser resets lockout for a user.
func (h *UserHandler) UnlockUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "User ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	// Verify user exists
	_, err := h.repo.GetUserByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Code:    http.StatusNotFound,
			Details: "No user found with the given ID",
		})
	}

	if err := h.repo.UnlockUser(id); err != nil {
		logrus.WithError(err).Error("Failed to unlock user")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to unlock user",
			Code:  http.StatusInternalServerError,
		})
	}

	adminID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"target_user": id,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     adminID,
		Action:     "unlock_user",
		EntityType: "user",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"admin_id":    adminID,
		"target_user": id,
	}).Info("User unlocked")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "User unlocked successfully",
	})
}

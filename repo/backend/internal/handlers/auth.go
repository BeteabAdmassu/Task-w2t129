package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	repo      *repository.Repository
	jwtSecret string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(repo *repository.Repository, jwtSecret string) *AuthHandler {
	return &AuthHandler{repo: repo, jwtSecret: jwtSecret}
}

// Login authenticates a user and returns a JWT token.
func (h *AuthHandler) Login(c echo.Context) error {
	var req models.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Username and password are required",
		})
	}

	user, err := h.repo.GetUserByUsername(req.Username)
	if err != nil {
		logrus.WithField("username", req.Username).Warn("Login attempt for unknown user")
		return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid credentials",
			Code:  http.StatusUnauthorized,
		})
	}

	if !user.IsActive {
		logrus.WithField("username", req.Username).Warn("Login attempt for inactive user")
		return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "Account disabled",
			Code:    http.StatusUnauthorized,
			Details: "This account has been deactivated",
		})
	}

	// Check lockout
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		logrus.WithField("username", req.Username).Warn("Login attempt for locked user")
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Account locked",
			Code:    http.StatusForbidden,
			Details: "Too many failed attempts. Please try again later.",
		})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Increment failed attempts
		if incErr := h.repo.IncrementFailedAttempts(user.ID); incErr != nil {
			logrus.WithError(incErr).Error("Failed to increment failed attempts")
		}

		// Lock account if >= 5 failed attempts (current attempt is already incremented)
		if user.FailedAttempts+1 >= 5 {
			if lockErr := h.repo.LockUser(user.ID, 15); lockErr != nil {
				logrus.WithError(lockErr).Error("Failed to lock user")
			}
			logrus.WithField("username", req.Username).Warn("Account locked due to failed attempts")
		}

		return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid credentials",
			Code:  http.StatusUnauthorized,
		})
	}

	// Reset failed attempts on success
	if err := h.repo.ResetFailedAttempts(user.ID); err != nil {
		logrus.WithError(err).Error("Failed to reset failed attempts")
	}

	// Generate JWT token
	token, err := middleware.GenerateToken(user.ID, user.Role, h.jwtSecret)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate JWT token")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate authentication token",
			Code:  http.StatusInternalServerError,
		})
	}

	// Audit log
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     user.ID,
		Action:     "login",
		EntityType: "user",
		EntityID:   user.ID,
	})

	logrus.WithField("user_id", user.ID).Info("User logged in successfully")

	return c.JSON(http.StatusOK, models.LoginResponse{
		Token: token,
		User:  *user,
	})
}

// Logout handles stateless JWT logout (no server-side action required).
func (h *AuthHandler) Logout(c echo.Context) error {
	userID := middleware.GetUserID(c)

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "logout",
		EntityType: "user",
		EntityID:   userID,
	})

	logrus.WithField("user_id", userID).Info("User logged out")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// GetMe returns the currently authenticated user's profile.
func (h *AuthHandler) GetMe(c echo.Context) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Not authenticated",
			Code:  http.StatusUnauthorized,
		})
	}

	user, err := h.repo.GetUserByID(userID)
	if err != nil {
		logrus.WithError(err).WithField("user_id", userID).Error("Failed to get user profile")
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
			Code:  http.StatusNotFound,
		})
	}

	return c.JSON(http.StatusOK, user)
}

// ChangePassword updates the authenticated user's password.
func (h *AuthHandler) ChangePassword(c echo.Context) error {
	userID := middleware.GetUserID(c)

	var req models.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Both old and new passwords are required",
		})
	}

	if len(req.NewPassword) < 12 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "New password must be at least 12 characters long",
		})
	}

	user, err := h.repo.GetUserByID(userID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get user for password change")
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
			Code:  http.StatusNotFound,
		})
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid old password",
			Code:    http.StatusBadRequest,
			Details: "The current password you entered is incorrect",
		})
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logrus.WithError(err).Error("Failed to hash new password")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update password",
			Code:  http.StatusInternalServerError,
		})
	}

	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now()
	if err := h.repo.UpdateUser(user); err != nil {
		logrus.WithError(err).Error("Failed to update user password")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update password",
			Code:  http.StatusInternalServerError,
		})
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "change_password",
		EntityType: "user",
		EntityID:   userID,
	})

	logrus.WithField("user_id", userID).Info("User changed password")

	details, _ := json.Marshal(map[string]string{"action": "password_changed"})
	_ = details

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Password updated successfully",
	})
}

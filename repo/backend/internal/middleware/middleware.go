package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/models"
)

// UserLookup is a function that retrieves a live user record by ID.
// JWTAuth accepts an optional UserLookup to re-check active/lock state
// on every request so deactivated or locked accounts are denied immediately
// rather than waiting for the token to expire.
type UserLookup func(id string) (*models.User, error)

// Context keys for user info
const (
	contextKeyUserID   = "user_id"
	contextKeyUserRole = "user_role"
)

// GetUserID extracts the user_id from the echo context.
func GetUserID(c echo.Context) string {
	val, _ := c.Get(contextKeyUserID).(string)
	return val
}

// GetUserRole extracts the user_role from the echo context.
func GetUserRole(c echo.Context) string {
	val, _ := c.Get(contextKeyUserRole).(string)
	return val
}

// GenerateToken creates a JWT token valid for 24 hours with the given user ID, role, and secret.
func GenerateToken(userID, role, secret string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// JWTAuth returns middleware that validates a Bearer token from the Authorization header.
// On success it sets "user_id" and "user_role" in the echo context.
//
// An optional UserLookup may be provided. When present, every authenticated request
// re-fetches the user record from the database and rejects the request if the account
// has been deactivated or locked since the token was issued. The live role from the DB
// is used (not the token claim) so role changes take effect immediately.
func JWTAuth(secret string, lookupUser ...UserLookup) echo.MiddlewareFunc {
	var lookup UserLookup
	if len(lookupUser) > 0 {
		lookup = lookupUser[0]
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
					Error: "Missing authorization header",
					Code:  http.StatusUnauthorized,
				})
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
					Error: "Invalid authorization header format",
					Code:  http.StatusUnauthorized,
				})
			}

			tokenString := parts[1]
			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
					Error:   "Invalid or expired token",
					Code:    http.StatusUnauthorized,
					Details: "Token validation failed",
				})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
					Error: "Invalid token claims",
					Code:  http.StatusUnauthorized,
				})
			}

			userID, _ := claims["user_id"].(string)
			role, _ := claims["role"].(string)

			if userID == "" {
				return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
					Error: "Token missing user_id claim",
					Code:  http.StatusUnauthorized,
				})
			}

			// Re-validate live user state when a lookup function is provided.
			// This ensures deactivated/locked accounts are rejected immediately
			// without waiting for the JWT to expire.
			if lookup != nil {
				user, lookupErr := lookup(userID)
				if lookupErr != nil {
					logrus.WithError(lookupErr).Warn("JWTAuth: user lookup failed")
					return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
						Error: "Authentication check failed",
						Code:  http.StatusUnauthorized,
					})
				}
				if user == nil || !user.IsActive {
					return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
						Error:   "Account deactivated",
						Code:    http.StatusUnauthorized,
						Details: "This account has been deactivated",
					})
				}
				if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
					return c.JSON(http.StatusUnauthorized, models.ErrorResponse{
						Error:   "Account locked",
						Code:    http.StatusUnauthorized,
						Details: "This account is temporarily locked",
					})
				}
				// Block all endpoints except password-change/logout/me when a
				// forced password change is pending. This prevents API clients
				// from bypassing the UI-level guard.
				if user.MustChangePassword {
					path := c.Request().URL.Path
					if !strings.HasSuffix(path, "/auth/password") &&
						!strings.HasSuffix(path, "/auth/logout") &&
						!strings.HasSuffix(path, "/auth/me") {
						return c.JSON(http.StatusForbidden, models.ErrorResponse{
							Error:   "Password change required",
							Code:    http.StatusForbidden,
							Details: "You must change your password before accessing this resource",
						})
					}
				}
				// Use live role from DB so role changes take effect immediately.
				role = user.Role
			}

			c.Set(contextKeyUserID, userID)
			c.Set(contextKeyUserRole, role)

			return next(c)
		}
	}
}

// RequireRole returns middleware that checks whether the authenticated user's role
// is among the allowed roles. Returns 403 Forbidden if not.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userRole := GetUserRole(c)
			if _, ok := allowed[userRole]; !ok {
				return c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "Insufficient permissions",
					Code:    http.StatusForbidden,
					Details: fmt.Sprintf("Role '%s' is not authorized for this resource", userRole),
				})
			}
			return next(c)
		}
	}
}

// RequestLogger returns middleware that logs each request using logrus with structured fields.
func RequestLogger() echo.MiddlewareFunc {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			latency := time.Since(start)
			req := c.Request()
			res := c.Response()

			fields := logrus.Fields{
				"method":     req.Method,
				"path":       req.URL.Path,
				"status":     res.Status,
				"latency_ms": latency.Milliseconds(),
				"remote_ip":  c.RealIP(),
			}

			if userID := GetUserID(c); userID != "" {
				fields["user_id"] = userID
			}

			entry := logger.WithFields(fields)

			status := res.Status
			switch {
			case status >= 500:
				entry.Error("Server error")
			case status >= 400:
				entry.Warn("Client error")
			default:
				entry.Info("Request handled")
			}

			return nil
		}
	}
}

// Recovery returns middleware that recovers from panics, logs the error, and returns a 500 response.
func Recovery() echo.MiddlewareFunc {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"panic":  fmt.Sprintf("%v", r),
						"method": c.Request().Method,
						"path":   c.Request().URL.Path,
					}).Error("Panic recovered")

					c.JSON(http.StatusInternalServerError, models.ErrorResponse{
						Error: "Internal server error",
						Code:  http.StatusInternalServerError,
					})
				}
			}()

			return next(c)
		}
	}
}

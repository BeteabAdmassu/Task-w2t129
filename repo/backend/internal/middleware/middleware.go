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
func JWTAuth(secret string) echo.MiddlewareFunc {
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

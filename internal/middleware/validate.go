package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

type Middleware struct {
	Logger *zap.Logger
}

func InitMiddleware(logger *zap.Logger) *Middleware {
	return &Middleware{
		Logger: logger,
	}
}

func (m Middleware) ValidateJWT() gin.HandlerFunc {

	return func(c *gin.Context) {

		var jwtSecret = config.GetEnv("JWT_SECRET_KEY", "")

		tokenString, err := c.Cookie("access_token")
		if err != nil {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			}

			m.Logger.Error("Missing access token cookie",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			}

			m.Logger.Error("Invalid token",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			errorResponse := response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}

			m.Logger.Error("Failed parse the claims",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse)
			return
		}

		role, hasRole := claims["role"].(string)
		userID, hasUserID := claims["user_id"].(string)
		organizationID, hasOrganizationID := claims["organization_id"].(string)
		mustChangePassword, _ := claims["must_change_password"].(bool)

		if !hasRole || !hasUserID || !hasOrganizationID || (role == "" && userID == "") {
			errorResponse := response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to perform this action",
			}

			m.Logger.Error("Access Denied",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse)
			return
		}

		if mustChangePassword && !isAllowedWhenPasswordChangeRequired(c.FullPath()) {
			errorResponse := response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Password change is required before accessing this resource",
			}

			m.Logger.Error("Access denied until password change is completed",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse)
			return
		}

		c.Set("role", role)
		c.Set("user_id", userID)
		c.Set("organization_id", organizationID)
		c.Set("must_change_password", mustChangePassword)

		c.Next()
	}
}

func isAllowedWhenPasswordChangeRequired(path string) bool {
	allowed := []string{
		"/auth/change-password",
		"/auth/logout",
		"/auth/refresh",
		"/organization/invitations/accept",
	}

	for _, allowedPath := range allowed {
		if path == allowedPath {
			return true
		}
	}
	return false
}

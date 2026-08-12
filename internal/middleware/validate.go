package middleware

import (
	"fmt"
	"net/http"
	"strings"

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

		var tokenString string

		authHeader := c.GetHeader("Authorization")

		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)

			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenString = parts[1]
			}
		}

		if tokenString == "" {
			cookieToken, err := c.Cookie("access_token")

			if err == nil && cookieToken != "" {
				tokenString = cookieToken
			}
		}

		if tokenString == "" {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			}

			m.Logger.Error("Missing access token",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

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

			m.Logger.Error("Failed to parse claims",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse)
			return
		}

		role, _ := claims["role"].(string)
		userID, hasUserID := claims["user_id"].(string)
		organizationID, _ := claims["organization_id"].(string)

		if !hasUserID || userID == "" {

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

		c.Set("role", role)
		c.Set("user_id", userID)
		c.Set("organization_id", organizationID)

		c.Next()
	}
}

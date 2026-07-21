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

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Enter the Token",
				Details: []response.Details{
					{
						Field:   "authorization",
						Message: "Missing token",
					},
				},
			}

			m.Logger.Error("Missing token in while validation",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Enter valid Token",
				Details: []response.Details{
					{
						Field:   "authorization",
						Message: "Unauthorized",
					},
				},
			}

			m.Logger.Error("Malformed authorization header",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Enter valid Token",
				Details: []response.Details{
					{
						Field:   "token",
						Message: "Unauthorized",
					},
				},
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
				Message:    "Failed parse the claims",
				Details: []response.Details{
					{
						Field:   "token",
						Message: "InternalServerError",
					},
				},
			}

			m.Logger.Error("Failed parse the claims",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse)
			return
		}

		role, roleOk := claims["role"].(string)
		userID, userIDOk := claims["user_id"].(string)
		organizationID, organizationIDOk := claims["Organization_id"].(string)

		if !roleOk || !userIDOk || !organizationIDOk || (role == "" && userID == "") {
			errorResponse := response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Access Denied",
				Details: []response.Details{
					{
						Field:   "Token",
						Message: "Forbidden",
					},
				},
			}

			m.Logger.Error("Access Denied",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse)
			return
		}

		c.Set("role", role)
		c.Set("user_id", userID)
		c.Set("Organization_id", organizationID)

		c.Next()
	}
}

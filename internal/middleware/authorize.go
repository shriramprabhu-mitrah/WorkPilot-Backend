package middleware

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

func (m Middleware) Authorize(rolesAllowed ...string) gin.HandlerFunc {

	return func(c *gin.Context) {

		roleVal, exists := c.Get("role")
		if !exists {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			}

			m.Logger.Error("Forbidden,Missing Authentication",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse)
			return
		}

		role, ok := roleVal.(string)
		if !ok || role == "" {
			errorResponse := response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to perform this action",
			}

			m.Logger.Error("Forbidden,Missing Authentication",
				zap.Error(fmt.Errorf("%v", errorResponse)))

			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse)
			return
		}

		if slices.Contains(rolesAllowed, role) {
			c.Next()
			return
		}

		errorResponse := response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}

		m.Logger.Error("Forbidden,Missing Authentication",
			zap.Error(fmt.Errorf("%v", errorResponse)))

		c.AbortWithStatusJSON(http.StatusForbidden, errorResponse)
	}
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
)

func writeErrorResponse(c *gin.Context, logger *zap.Logger, err response.Error, logMessage string) {
	if logger != nil && logMessage != "" {
		logger.Error(logMessage)
	}
	c.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: err})
}

func getRequiredContextValue(c *gin.Context, logger *zap.Logger, key, contextName string) (string, bool) {
	value, exists := c.Get(key)
	if !exists {
		writeErrorResponse(c, logger, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error: missing " + contextName + " context",
		}, "Missing context value: "+key)
		return "", false
	}

	strValue, ok := value.(string)
	if !ok {
		writeErrorResponse(c, logger, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error: invalid " + contextName + " context",
		}, "Invalid context value type: "+key)
		return "", false
	}

	return strValue, true
}

func getRequiredContextUUID(c *gin.Context, logger *zap.Logger, key, contextName string) (uuid.UUID, bool) {
	value, ok := getRequiredContextValue(c, logger, key, contextName)
	if !ok {
		return uuid.Nil, false
	}

	parsed, errResponse := utils.StringToUUID(value)
	if errResponse != nil {
		if logger != nil {
			logger.Error("Failed to convert the string into UUID")
		}
		c.JSON(errResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errResponse})
		return uuid.Nil, false
	}

	return parsed, true
}

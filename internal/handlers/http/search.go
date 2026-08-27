package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type SearchHandler interface {
	GlobalSearch(c *gin.Context)
}

type searchHandler struct {
	service services.SearchService
	logger  *zap.Logger
}

func InitSearchHandler(service services.SearchService, logger *zap.Logger) SearchHandler {
	return &searchHandler{
		service: service,
		logger:  logger,
	}
}

// GlobalSearch godoc
// @Summary Global Search
// @Description Search across all Tasks, User Stories, Projects, and Members in the current user's organization
// @Tags Search
// @Produce json
// @Param q query string true "Search query string"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Security BearerAuth
// @Router /search [get]
func (h *searchHandler) GlobalSearch(c *gin.Context) {
	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	orgUUID, ok := getRequiredContextUUID(c, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	query := c.Query("q")

	res, errResp := h.service.GlobalSearch(userUUID, orgUUID, query)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Search results retrieved successfully",
		Data:       res,
	})
}

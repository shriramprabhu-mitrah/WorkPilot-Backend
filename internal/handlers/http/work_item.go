package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type WorkItemHandler interface {
	GetWorkItemBySerialNumber(g *gin.Context)
}

type workItemHandler struct {
	service services.WorkItemService
	logger  *zap.Logger
}

func InitWorkItemHandler(service services.WorkItemService, logger *zap.Logger) WorkItemHandler {
	return &workItemHandler{
		service: service,
		logger:  logger,
	}
}

// GetWorkItemBySerialNumber godoc
// @Summary Get project work item by global serial ID
// @Description Retrieve a project work item (task or user story) using its global serial ID.
// @Tags WorkItem
// @Produce json
// @Param project_id path string true "Project ID (UUID) or Project Slug"
// @Param serial_id path int64 true "Global Serial ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/work-items/{serial_id} [get]
func (h *workItemHandler) GetWorkItemBySerialNumber(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")

	serialIDParam := g.Param("serial_id")
	serialID, err := strconv.ParseInt(serialIDParam, 10, 64)
	if err != nil || serialID <= 0 {
		g.JSON(http.StatusBadRequest, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid serial ID format",
		})
		return
	}

	res, errResp := h.service.GetWorkItemBySerialNumber(projectIDParam, serialID, userUUID)
	if errResp != nil {
		g.JSON(errResp.StatusCode, errResp)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Work item retrieved successfully",
		Data:       res,
	})
}

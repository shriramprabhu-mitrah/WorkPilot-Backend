package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type WorkItemHandler interface {
	GetWorkItemBySerialNumber(g *gin.Context)
	GetTaskByKey(g *gin.Context)
	GetUserStoryByKey(g *gin.Context)
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
// @Param serial_id path string true "Global Serial ID or Key (e.g. US-1, MP-1)"
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

	res, errResp := h.service.GetWorkItemBySerialNumber(projectIDParam, serialIDParam, userUUID)
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

// GetTaskByKey godoc
// @Summary Get project task by key
// @Description Retrieve a project task using its unique project-scoped key.
// @Tags WorkItem
// @Produce json
// @Param project_id path string true "Project ID (UUID) or Project Slug"
// @Param key path string true "Task Key (e.g. TF-101)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/work-items/task/{key} [get]
func (h *workItemHandler) GetTaskByKey(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	keyParam := g.Param("key")

	res, errResp := h.service.GetTaskByKey(projectIDParam, keyParam, userUUID)
	if errResp != nil {
		g.JSON(errResp.StatusCode, errResp)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Task retrieved successfully",
		Data:       res,
	})
}

// GetUserStoryByKey godoc
// @Summary Get project user story by key
// @Description Retrieve a project user story using its unique project-scoped key.
// @Tags WorkItem
// @Produce json
// @Param project_id path string true "Project ID (UUID) or Project Slug"
// @Param key path string true "User Story Key (e.g. US-1)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/work-items/us/{key} [get]
func (h *workItemHandler) GetUserStoryByKey(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	keyParam := g.Param("key")

	res, errResp := h.service.GetUserStoryByKey(projectIDParam, keyParam, userUUID)
	if errResp != nil {
		g.JSON(errResp.StatusCode, errResp)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "User Story retrieved successfully",
		Data:       res,
	})
}

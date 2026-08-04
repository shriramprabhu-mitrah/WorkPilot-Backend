package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type taskHandler struct {
	service services.TaskService
	logger  *zap.Logger
}

func InitTaskHandler(service services.TaskService, logger *zap.Logger) *taskHandler {
	return &taskHandler{
		service: service,
		logger:  logger,
	}
}

func (h *taskHandler) CreateTask(g *gin.Context) {
	var payload requestdto.CreateTaskRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the project string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectID
	payload.OrganizationID = organizationUUID

	_, err := h.service.CreateTask(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created Task",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) GetTaskByID(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskIDParam := g.Param("task_id")
	taskID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	taskRes, err := h.service.GetTaskByID(taskID, projectID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Task retrieved successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       taskRes,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) UpdateTask(g *gin.Context) {
	var payload requestdto.UpdateTaskRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskIDParam := g.Param("task_id")
	taskID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectID
	payload.TaskID = taskID
	payload.OrganizationID = organizationUUID

	taskRes, err := h.service.UpdateTask(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Updated Task",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"task_id": taskRes.ID,
		},
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) DeleteTask(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskIDParam := g.Param("task_id")
	taskID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.DeleteTask(taskID, projectID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Deleted Task",
		StatusCode: http.StatusOK,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) RestoreTask(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskIDParam := g.Param("task_id")
	taskID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.RestoreTask(taskID, projectID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Restored Task",
		StatusCode: http.StatusOK,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) CloneTask(g *gin.Context) {
	var payload requestdto.CloneTaskRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskIDParam := g.Param("task_id")
	taskID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectID
	payload.TaskID = taskID
	payload.OrganizationID = organizationUUID

	taskRes, err := h.service.CloneTask(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Cloned Task",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       taskRes,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *taskHandler) GetTasks(g *gin.Context) {
	var filter requestdto.TaskFilter

	if err := g.BindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid filter parameters",
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	tasks, pagination, err := h.service.GetTasks(projectID, userUUID, organizationUUID, filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Tasks retrieved successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       tasks,
		Meta:       &pagination,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

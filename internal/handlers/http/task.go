package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
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

// CreateTask godoc
// @Summary Create a new task
// @Description Create a new task in the specified project. The description field accepts HTML and is sanitized before storage.
// @Tags Task
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.CreateTaskRequest true "Create Task Request Body"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks [post]
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

	taskid, _, err := h.service.CreateTask(payload)
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
		Data: map[string]any{
			"task_id": taskid,
		},
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

// GetTaskByID godoc
// @Summary Get Task By ID
// @Description Retrieve details of a specific task by ID
// @Tags Task
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id} [get]
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

// UpdateTask godoc
// @Summary Update Task
// @Description Update the details of a specific task by ID. Description supports HTML content and is sanitized before storage.
// @Tags Task
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param request body requestdto.UpdateTaskRequest true "Update Task Request Body"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id} [patch]
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

// DeleteTasks godoc
// @Summary Bulk Delete Tasks
// @Description Soft delete multiple tasks in a project
// @Tags Task
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.BulkDeleteTasksRequest true "Bulk Delete Tasks Request Body"
// @Success 200 {object} response.SuccessResponse
// @Success 207 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks [delete]
func (h *taskHandler) DeleteTasks(g *gin.Context) {
	var payload requestdto.BulkDeleteTasksRequest

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.ProjectID = projectID
	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	res, err := h.service.BulkDeleteTasks(payload)
	if err != nil {
		errResp := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errResp)
		return
	}

	statusCode := http.StatusOK
	success := true
	message := "Successfully deleted tasks"
	if len(res.FailedTaskIDs) > 0 {
		if len(res.FailedTaskIDs) == len(payload.TaskIDs) {
			statusCode = http.StatusBadRequest
			success = false
			message = "Failed to delete all tasks"
		} else {
			statusCode = http.StatusMultiStatus // 207 Partial Success
			message = "Bulk deletion completed with some failures"
		}
	}

	successResponse := &response.SuccessResponse{
		Message:    message,
		StatusCode: statusCode,
		Success:    success,
		Data:       res,
	}

	g.JSON(statusCode, successResponse)
}

// RestoreTask godoc
// @Summary Restore Task
// @Description Restore a soft-deleted task by ID (within the retention period)
// @Tags Task
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/restore [post]
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

// CloneTask godoc
// @Summary Clone Task
// @Description Clone a task to create a copy of it
// @Tags Task
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param request body requestdto.CloneTaskRequest true "Clone Task Request Body"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/clone [post]
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

// GetTasks godoc
// @Summary Get Tasks
// @Description Retrieve tasks for a project with search, filter, sort and pagination options
// @Tags Task
// @Produce json
// @Param project_id path string true "Project ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param sort_by query string false "Sort by field" Enums(title,created_at,updated_at,priority,status)
// @Param sort_order query string false "Sort order" Enums(ASC,DESC)
// @Param status query string false "Task Status"
// @Param assignee_id query string false "Assignee User ID"
// @Param reporter_id query string false "Reporter User ID"
// @Param sprint_id query string false "Sprint ID"
// @Param user_story_id query string false "User Story ID"
// @Param type query string false "Task Type" Enums(bug,feature,task,chore,story)
// @Param priority query string false "Task Priority" Enums(low,medium,high,critical)
// @Param search query string false "Search query for title, description or key"
// @Param labels query string false "Comma-separated labels"
// @Param is_deleted query boolean false "Get soft deleted tasks"
// @Param match query string false "Match mode" Enums(any,all)
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks [get]
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

	var finalLabels []string
	rawLabels := g.Request.URL.Query()["labels"]
	for _, raw := range rawLabels {
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				finalLabels = append(finalLabels, trimmed)
			}
		}
	}
	filter.Labels = finalLabels

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

// BulkUpdateTasks godoc
// @Summary Bulk Update Tasks
// @Description Update status, sprint, or assignee of multiple tasks in a project
// @Tags Task
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.BulkUpdateTasksRequest true "Bulk Update Tasks Request Body"
// @Success 200 {object} response.SuccessResponse
// @Success 207 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/bulk [patch]
func (h *taskHandler) BulkUpdateTasks(g *gin.Context) {
	var payload requestdto.BulkUpdateTasksRequest

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectID
	payload.OrganizationID = organizationUUID

	res, err := h.service.BulkUpdateTasks(payload)
	if err != nil {
		errResp := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errResp)
		return
	}

	statusCode := http.StatusOK
	success := true
	message := "Successfully updated tasks"
	if len(res.FailedTaskIDs) > 0 {
		if len(res.FailedTaskIDs) == len(payload.Tasks) {
			statusCode = http.StatusBadRequest
			success = false
			message = "Failed to update all tasks"
		} else {
			statusCode = http.StatusMultiStatus // 207 Partial Success
			message = "Bulk update completed with some failures"
		}
	}

	successResponse := &response.SuccessResponse{
		Message:    message,
		StatusCode: statusCode,
		Success:    success,
		Data:       res,
	}

	g.JSON(statusCode, successResponse)
}

// AttachLabelToTask godoc
// @Summary Attach Label to Task
// @Description Attach a project label to a specific task
// @Tags Task
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param label_id path string true "Label ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/labels/{label_id} [put]
func (h *taskHandler) AttachLabelToTask(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	taskIDParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	labelIDParam := g.Param("label_id")
	labelUUID, errorResponse := utils.StringToUUID(labelIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.AttachLabelToTask(projectUUID, taskUUID, labelUUID, userUUID, organizationUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Label attached to task successfully",
	})
}

// RemoveLabelFromTask godoc
// @Summary Remove Label from Task
// @Description Remove a project label from a specific task
// @Tags Task
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param label_id path string true "Label ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/labels/{label_id} [delete]
func (h *taskHandler) RemoveLabelFromTask(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	taskIDParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	labelIDParam := g.Param("label_id")
	labelUUID, errorResponse := utils.StringToUUID(labelIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.RemoveLabelFromTask(projectUUID, taskUUID, labelUUID, userUUID, organizationUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Label removed from task successfully",
		Data: map[string]uuid.UUID{
			"Label_id": labelUUID},
	})
}

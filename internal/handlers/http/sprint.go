package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitSprintHandler(service services.SprintService, logger *zap.Logger) *sprintHandler {
	return &sprintHandler{
		service: service,
		logger:  logger,
	}
}

type sprintHandler struct {
	service services.SprintService
	logger  *zap.Logger
}

// CreateSprint godoc
// @Summary Create Sprints
// @Description Create one or more sprints under a project
// @Tags Sprint
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.CreateSprintRequest true "List of sprints to create"
// @Success 201 {object} response.SuccessResponse "Sprint(s) created successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid request payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Forbidden"
// @Failure 409 {object} response.ErrorResponse "Conflict"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /projects/{project_id}/sprint/ [post]
func (h *sprintHandler) CreateSprint(g *gin.Context) {

	var payload requestdto.CreateSprintRequest

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
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectUUID
	payload.OrganizationID = organizationUUID

	err := h.service.CreateSprint(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created Sprint",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

// DeleteSprint godoc
// @Summary Delete Sprint
// @Description Delete a sprint from a project
// @Tags Sprint
// @Produce json
// @Param project_id path string true "Project ID"
// @Param sprint_id path string true "Sprint ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint/{sprint_id} [delete]
func (h *sprintHandler) DeleteSprint(g *gin.Context) {

	var payload requestdto.DeleteSprint

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Internal server error: missing organization context")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	sprintID := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}
	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	payload.ProjectID = projectUUID
	payload.SprintID = sprintUUID
	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	err := h.service.DeleteSprint(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Sprint deleted successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"Sprint ID": sprintUUID,
		},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateSprint godoc
// @Summary Update Sprint
// @Description Update sprint details
// @Tags Sprint
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param sprint_id path string true "Sprint ID"
// @Param request body requestdto.UpdateSprintRequest true "Sprint Update Details"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint/{sprint_id} [patch]
func (h *sprintHandler) UpdateSprint(g *gin.Context) {

	var payload requestdto.UpdateSprintRequest

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

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Internal server error: missing organization context")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	sprintParam := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	payload.ProjectID = projectID
	payload.SprintID = sprintUUID
	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	err := h.service.UpdateSprint(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Sprint Updated successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"Sprint ID": sprintUUID,
		},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetSprints godoc
// @Summary Get Sprints
// @Description Retrieve all sprints for a project with pagination, search and status filter
// @Tags Sprint
// @Produce json
// @Param project_id path string true "Project ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param status query string false "Sprint Status" Enums(planning,active,on_hold,completed,cancelled,archived)
// @Param search query string false "Search sprint by name"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint [get]
func (h *sprintHandler) GetSprints(g *gin.Context) {

	var filter requestdto.SprintFilter

	if err := g.ShouldBindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid query parameters",
			}}
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	payload := requestdto.GetSprint{
		ProjectID:      projectID,
		OrganizationID: organizationUUID,
		UserID:         userUUID,
	}

	projects, pagination, err := h.service.GetSprints(payload, filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	sprintResponses := make([]responsedto.Sprint, 0, len(projects))
	for _, sprint := range projects {
		sprintResponses = append(sprintResponses, responsedto.SprintFromModel(sprint))
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprints retrieved successfully.",
		Data:       sprintResponses,
		Meta:       &pagination,
	})
}

// GetSprintByID godoc
// @Summary Get Sprint By ID
// @Description Retrieve a sprint by project ID and sprint ID
// @Tags Sprint
// @Produce json
// @Param project_id path string true "Project ID"
// @Param sprint_id path string true "Sprint ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint/{sprint_id} [get]
func (h *sprintHandler) GetSprintByID(g *gin.Context) {

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}
	sprintParam := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	payload := requestdto.GetSprint{
		ProjectID:      projectUUID,
		OrganizationID: organizationUUID,
		UserID:         userUUID,
		SprintID:       sprintUUID,
	}

	project, err := h.service.GetSprintByID(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	sprintResponse := responsedto.SprintFromModel(*project)

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprint retrieved successfully.",
		Data:       sprintResponse,
	})
}

func (h *sprintHandler) GetSprintBurndown(g *gin.Context) {
	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}
		h.logger.Error("Organization Id Invalid/Missing")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	sprintParam := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	burndown, err := h.service.GetSprintBurndown(sprintUUID, projectUUID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprint burndown data retrieved successfully.",
		Data:       burndown,
	})
}

func (h *sprintHandler) TriggerSnapshot(g *gin.Context) {
	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	err := h.service.TriggerDailySnapshots(projectUUID, userUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprint snapshots triggered successfully.",
	})
}

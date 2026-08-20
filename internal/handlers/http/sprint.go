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
// @Router /projects/{project_id}/sprint [post]
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

	sprintid, err := h.service.CreateSprint(payload)
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
		Data: map[string]any{
			"sprint_id": sprintid,
		},
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
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
// @Description Retrieve all sprints for a project with pagination, search, sorting and status filter
// @Tags Sprint
// @Produce json
// @Param project_id path string true "Project ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param status query string false "Sprint Status" Enums(planning,active,on_hold,completed,cancelled,archived)
// @Param search query string false "Search sprint by name"
// @Param sort_by query string false "Sort by field" Enums(name,created_at,updated_at,start_date,end_date,status)
// @Param sort_order query string false "Sort order" Enums(ASC,DESC)
// @Param fields query string false "Fields to return (comma separated)"
// @Param start_date query string false "Filter sprints from this start date" Format(date)
// @Param end_date query string false "Filter sprints up to this end date" Format(date)
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint [get]
func (h *sprintHandler) GetSprints(g *gin.Context) {

	var filter requestdto.SprintFilter

	// Bind query parameters
	if err := g.ShouldBindQuery(&filter); err != nil {
		h.logger.Error(
			"Failed to bind sprint filters",
			zap.Error(err),
		)

		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Invalid query parameters. Date format must be YYYY-MM-DD",
			},
		}

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	// Validate date range
	if !filter.StartDate.IsZero() &&
		!filter.EndDate.IsZero() &&
		filter.StartDate.After(filter.EndDate) {

		h.logger.Warn(
			"Invalid sprint date range",
			zap.Time("startDate", filter.StartDate),
			zap.Time("endDate", filter.EndDate),
		)

		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "start_date must be before or equal to end_date",
			},
		}

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	// Log dates to verify binding
	h.logger.Info(
		"Sprint date filter",
		zap.Time("startDate", filter.StartDate),
		zap.Time("endDate", filter.EndDate),
	)

	// Get organization ID
	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	// Get project ID
	projectIDParam := g.Param("project_id")

	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")

		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	// Get user ID
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	payload := requestdto.GetSprint{
		ProjectID:      projectID,
		OrganizationID: organizationUUID,
		UserID:         userUUID,
	}

	// Pass filter to service
	projects, pagination, err := h.service.GetSprints(payload, filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}

		g.JSON(err.StatusCode, errorResponse)
		return
	}

	// Convert model to response DTO
	sprintResponses := make([]responsedto.Sprint, 0, len(projects))

	for _, sprint := range projects {
		sprintResponses = append(sprintResponses, responsedto.SprintFromModel(sprint))
	}

	// Filter response fields
	var filteredData any = sprintResponses

	if filter.Fields != "" {
		var filterErr error

		filteredData, filterErr = utils.FilterFields(sprintResponses, filter.Fields)

		if filterErr != nil {
			h.logger.Error(
				"Failed to filter sprint fields",
				zap.Error(filterErr),
			)

			filteredData = sprintResponses
		}
	}

	// Response
	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprints retrieved successfully.",
		Data:       filteredData,
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
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

// GetSprintBurndown godoc
// @Summary Get Sprint Burndown
// @Description Retrieve burndown chart data for a specific sprint
// @Tags Sprints
// @Produce json
// @Param project_id path string true "Project ID (UUID)"
// @Param sprint_id path string true "Sprint ID (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint/{sprint_id}/burndown [get]
func (h *sprintHandler) GetSprintBurndown(g *gin.Context) {
	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
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

// TriggerSnapshot godoc
// @Summary Trigger Sprint Snapshots
// @Description Trigger daily snapshot calculation for active sprints in a project
// @Tags Sprints
// @Produce json
// @Param project_id path string true "Project ID (UUID)"
// @Param sprint_id path string true "Sprint ID (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/sprint/{sprint_id}/snapshot [post]
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.TriggerDailySnapshots(projectUUID, userUUID, organizationUUID)
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

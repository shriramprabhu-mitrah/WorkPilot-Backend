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

var _ = responsedto.UserStoryStatusResponse{}

type userStoryStatusHandler struct {
	service services.UserStoryStatusService
	logger  *zap.Logger
}

func InitUserStoryStatusHandler(service services.UserStoryStatusService, logger *zap.Logger) *userStoryStatusHandler {
	return &userStoryStatusHandler{
		service: service,
		logger:  logger,
	}
}

// CreateStatus godoc
// @Summary Create a User Story status
// @Description Create a new User Story custom status for a project
// @Tags User Story Statuses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param request body requestdto.CreateUserStoryStatusRequest true "Create User Story Status Request Body"
// @Success 201 {object} response.SuccessResponse{data=responsedto.UserStoryStatusResponse}
// @Failure 422 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-story-statuses [post]
func (h *userStoryStatusHandler) CreateStatus(g *gin.Context) {
	var payload requestdto.CreateUserStoryStatusRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    message,
		}, "Invalid request payload")
		return
	}

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectUUID
	payload.OrganizationID = organizationUUID

	// Validate color format
	if !requestdto.ValidateColor(payload.Color) {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    "Status color must be a valid hexadecimal color code (#RRGGBB)",
		}, "Invalid color code")
		return
	}

	res, err := h.service.CreateStatus(payload)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "User Story status created successfully",
		Data:       res,
	})
}

// GetStatuses godoc
// @Summary Get User Story statuses
// @Description Get all User Story statuses for a project
// @Tags User Story Statuses
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.UserStoryStatusResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-story-statuses [get]
func (h *userStoryStatusHandler) GetStatuses(g *gin.Context) {
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	res, err := h.service.GetStatuses(projectUUID, userUUID, organizationUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "User Story statuses retrieved successfully",
		Data:       res,
	})
}

// UpdateStatus godoc
// @Summary Update a User Story status
// @Description Update an existing User Story custom status
// @Tags User Story Statuses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param status_id path string true "Status ID"
// @Param request body requestdto.UpdateUserStoryStatusRequest true "Update User Story Status Request Body"
// @Success 200 {object} response.SuccessResponse{data=responsedto.UserStoryStatusResponse}
// @Failure 422 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-story-statuses/{status_id} [patch]
func (h *userStoryStatusHandler) UpdateStatus(g *gin.Context) {
	var payload requestdto.UpdateUserStoryStatusRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    message,
		}, "Invalid request payload")
		return
	}

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

	statusIDParam := g.Param("status_id")
	statusUUID, errorResponse := utils.StringToUUID(statusIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.ProjectID = projectUUID
	payload.StatusID = statusUUID
	payload.OrganizationID = organizationUUID

	if payload.Color != nil {
		if !requestdto.ValidateColor(*payload.Color) {
			writeErrorResponse(g, h.logger, response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Status color must be a valid hexadecimal color code (#RRGGBB)",
			}, "Invalid color code")
			return
		}
	}

	res, err := h.service.UpdateStatus(payload)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "User Story status updated successfully",
		Data:       res,
	})
}

// DeleteStatus godoc
// @Summary Delete a User Story status
// @Description Delete an existing User Story custom status
// @Tags User Story Statuses
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param status_id path string true "Status ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-story-statuses/{status_id} [delete]
func (h *userStoryStatusHandler) DeleteStatus(g *gin.Context) {
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

	statusIDParam := g.Param("status_id")
	statusUUID, errorResponse := utils.StringToUUID(statusIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.DeleteStatus(statusUUID, projectUUID, userUUID, organizationUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "User Story status deleted successfully",
		Data: map[string]uuid.UUID{
			"status_id": statusUUID},
	})
}

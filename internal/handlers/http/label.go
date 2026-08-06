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

type labelHandler struct {
	service services.LabelService
	logger  *zap.Logger
}

func InitLabelHandler(service services.LabelService, logger *zap.Logger) *labelHandler {
	return &labelHandler{
		service: service,
		logger:  logger,
	}
}

// CreateLabel godoc
// @Summary Create a label
// @Description Create a new label for a project
// @Tags Label
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.CreateLabelRequest true "Create Label Request Body"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/labels/ [post]
func (h *labelHandler) CreateLabel(g *gin.Context) {
	var payload requestdto.CreateLabelRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
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
			StatusCode: http.StatusBadRequest,
			Message:    "Label color must be a valid hexadecimal color code (#RRGGBB)",
		}, "Invalid color code")
		return
	}

	res, err := h.service.CreateLabel(payload)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Label created successfully",
		Data:       res,
	})
}

// GetLabels godoc
// @Summary Get labels
// @Description Get all labels for a project
// @Tags Label
// @Produce json
// @Param project_id path string true "Project ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/labels/ [get]
func (h *labelHandler) GetLabels(g *gin.Context) {
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

	res, err := h.service.GetLabels(projectUUID, userUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Labels retrieved successfully",
		Data:       res,
	})
}

// UpdateLabel godoc
// @Summary Update a label
// @Description Update an existing label
// @Tags Label
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param label_id path string true "Label ID"
// @Param request body requestdto.UpdateLabelRequest true "Update Label Request Body"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/labels/{label_id} [patch]
func (h *labelHandler) UpdateLabel(g *gin.Context) {
	var payload requestdto.UpdateLabelRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
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

	payload.UserID = userUUID
	payload.ProjectID = projectUUID
	payload.LabelID = labelUUID
	payload.OrganizationID = organizationUUID

	if payload.Color != nil {
		if !requestdto.ValidateColor(*payload.Color) {
			writeErrorResponse(g, h.logger, response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Label color must be a valid hexadecimal color code (#RRGGBB)",
			}, "Invalid color code")
			return
		}
	}

	res, err := h.service.UpdateLabel(payload)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Label updated successfully",
		Data:       res,
	})
}

// DeleteLabel godoc
// @Summary Delete a label
// @Description Delete an existing label
// @Tags Label
// @Produce json
// @Param project_id path string true "Project ID"
// @Param label_id path string true "Label ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/labels/{label_id} [delete]
func (h *labelHandler) DeleteLabel(g *gin.Context) {
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

	err := h.service.DeleteLabel(labelUUID, projectUUID, userUUID, organizationUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Label deleted successfully",
	})
}

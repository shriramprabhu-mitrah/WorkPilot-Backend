package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
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

func (h *sprintHandler) CreateSprint(g *gin.Context) {

	var payload dto.CreateSprintRequest

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

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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

	payload.UserID = userUUID
	payload.ProjectID = projectID
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

func (h *sprintHandler) DeleteSprint(g *gin.Context) {

	var payload dto.DeleteSprint

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

		h.logger.Error("Sprint Id Invalid/Missing ")
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

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	sprintID := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

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

func (h *sprintHandler) UpdateSprint(g *gin.Context) {

	var payload dto.UpdateSprintRequest

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

		h.logger.Error("Sprint Id Invalid/Missing ")
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

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
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

func (h *sprintHandler) GetSprints(g *gin.Context) {

	var filter dto.SprintFilter

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

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	payload := dto.GetSprint{
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

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprints retrieved successfully.",
		Data:       projects,
		Meta:       &pagination,
	})
}

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

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}
	sprintParam := g.Param("sprint_id")
	sprintUUID, errorResponse := utils.StringToUUID(sprintParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	payload := dto.GetSprint{
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

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Sprint retrieved successfully.",
		Data:       project,
	})
}

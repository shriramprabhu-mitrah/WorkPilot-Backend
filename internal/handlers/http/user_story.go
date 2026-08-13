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

type userStoryHandler struct {
	service services.UserStoryService
	logger  *zap.Logger
}

func InitUserStoryHandler(service services.UserStoryService, logger *zap.Logger) *userStoryHandler {
	return &userStoryHandler{
		service: service,
		logger:  logger,
	}
}

func (h *userStoryHandler) CreateUserStory(g *gin.Context) {
	var payload requestdto.CreateUserStoryRequest

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

	payload.ReporterID = userUUID
	payload.ProjectID = projectID
	payload.OrganizationID = organizationUUID

	storyRes, err := h.service.CreateUserStory(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created User Story",
		StatusCode: http.StatusCreated,
		Success:    true,
		Data:       storyRes,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *userStoryHandler) GetUserStoryByID(g *gin.Context) {
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

	storyIDParam := g.Param("user_story_id")
	storyID, errorResponse := utils.StringToUUID(storyIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	storyRes, err := h.service.GetUserStoryByID(storyID, projectID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "User Story retrieved successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       storyRes,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *userStoryHandler) UpdateUserStory(g *gin.Context) {
	var payload requestdto.UpdateUserStoryRequest

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

	storyIDParam := g.Param("user_story_id")
	storyID, errorResponse := utils.StringToUUID(storyIDParam)
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
	payload.UserStoryID = storyID
	payload.OrganizationID = organizationUUID

	storyRes, err := h.service.UpdateUserStory(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Updated User Story",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       storyRes,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *userStoryHandler) DeleteUserStory(g *gin.Context) {
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

	storyIDParam := g.Param("user_story_id")
	storyID, errorResponse := utils.StringToUUID(storyIDParam)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.DeleteUserStory(storyID, projectID, userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Deleted User Story",
		StatusCode: http.StatusOK,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

func (h *userStoryHandler) GetUserStories(g *gin.Context) {
	var filter requestdto.UserStoryFilter

	if err := g.BindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid filter parameters",
			},
		}
		h.logger.Error("Invalid request query parameters", zap.Error(err))
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

	stories, pagination, err := h.service.GetUserStories(projectID, userUUID, organizationUUID, filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "User Stories retrieved successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       stories,
		Meta:       &pagination,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

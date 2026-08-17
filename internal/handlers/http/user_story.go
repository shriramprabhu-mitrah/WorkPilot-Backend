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

// CreateUserStory godoc
// @Summary Create a new user story
// @Description Create a new user story in the specified project. The description field accepts HTML and is sanitized before storage.
// @Tags UserStory
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.CreateUserStoryRequest true "Create User Story Request Body"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories [post]
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

// GetUserStoryByID godoc
// @Summary Get User Story By ID
// @Description Retrieve details of a specific user story by ID
// @Tags UserStory
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id} [get]
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

// UpdateUserStory godoc
// @Summary Update User Story
// @Description Update fields of a specific user story by ID. Description supports HTML content and is sanitized before storage.
// @Tags UserStory
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Param request body requestdto.UpdateUserStoryRequest true "Update User Story Request Body"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id} [patch]
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

// DeleteUserStory godoc
// @Summary Delete User Story
// @Description Soft delete a specific user story by ID
// @Tags UserStory
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id} [delete]
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

// GetUserStories godoc
// @Summary Get User Stories
// @Description Retrieve a paginated and filtered list of user stories in a project
// @Tags UserStory
// @Produce json
// @Param project_id path string true "Project ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param sort_by query string false "Sort by field" Enums(title,created_at,updated_at,priority,status)
// @Param sort_order query string false "Sort order" Enums(ASC,DESC)
// @Param status query string false "User Story Status" Enums(todo,in_progress,in_review,testing,completed,blocked)
// @Param assignee_id query string false "Assignee User ID"
// @Param reporter_id query string false "Reporter User ID"
// @Param sprint_id query string false "Sprint ID"
// @Param priority query string false "User Story Priority" Enums(low,medium,high,critical)
// @Param search query string false "Search query for title or description"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories [get]
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

// ReorderUserStories godoc
// @Summary Reorder User Stories in the Product Backlog
// @Description Persist a new ordering for user stories in the project backlog
// @Tags UserStory
// @Accept json
// @Produce json
// @Param project_id path string true "Project ID"
// @Param request body requestdto.ReorderUserStoriesRequest true "Reorder User Stories Request Body"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/reorder [patch]
func (h *userStoryHandler) ReorderUserStories(g *gin.Context) {
	var payload requestdto.ReorderUserStoriesRequest

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

	err := h.service.ReorderUserStories(projectID, userUUID, organizationUUID, payload.StoryIDs)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "User Stories reordered successfully",
		StatusCode: http.StatusOK,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

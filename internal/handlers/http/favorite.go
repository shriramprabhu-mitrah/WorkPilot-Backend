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

type favoriteHandler struct {
	service services.FavoriteService
	logger  *zap.Logger
}

func InitFavoriteHandler(service services.FavoriteService, logger *zap.Logger) *favoriteHandler {
	return &favoriteHandler{
		service: service,
		logger:  logger,
	}
}

// AddFavorite godoc
// @Summary Add item to favorites
// @Description Add a user story or task to user favorites
// @Tags Favorite
// @Accept json
// @Produce json
// @Param request body requestdto.AddFavoriteRequest true "Add Favorite Request Body"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /favorites [post]
func (h *favoriteHandler) AddFavorite(g *gin.Context) {
	var payload requestdto.AddFavoriteRequest

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID
	res, err := h.service.AddFavorite(payload)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success: true,
		Data:    res,
	})
}

// RemoveFavorite godoc
// @Summary Remove item from favorites
// @Description Remove a favorite item by favorite ID or query params (item_type and item_id)
// @Tags Favorite
// @Produce json
// @Param favorite_id path string false "Favorite ID"
// @Param item_type query string false "Item Type (user_story or task)"
// @Param item_id query string false "Item ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /favorites [delete]
func (h *favoriteHandler) RemoveFavorite(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	itemType := g.Query("item_type")
	itemIDStr := g.Query("item_id")

	if itemType == "" || itemIDStr == "" {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Either favorite_id in URL path or both item_type and item_id in query parameters must be provided",
		}, "Missing remove favorite parameters")
		return
	}

	itemUUID, errorResponse := utils.StringToUUID(itemIDStr)
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	res, err := h.service.RemoveFavorite(userUUID, itemType, itemUUID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success: true,
		Message: "Favorite removed successfully",
		Data:    res,
	})
}

// GetFavorites godoc
// @Summary Get all favorites
// @Description Get all user stories and tasks added in favorites for the current user
// @Tags Favorite
// @Produce json
// @Param item_type query string false "Filter by item type (user_story or task)"
// @Param search query string false "Search keyword in title or description"
// @Param sort_by query string false "Sort by field (created_at, title, name)"
// @Param sort_order query string false "Sort order (ASC or DESC)"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /favorites [get]
func (h *favoriteHandler) GetFavorites(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	var filter requestdto.GetFavoritesFilter
	if err := g.ShouldBindQuery(&filter); err != nil {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid query parameters",
		}, "Failed to bind query parameters")
		return
	}

	res, pagination, err := h.service.GetFavorites(userUUID, filter)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Favorites retrieved successfully",
		Data:       res,
		Meta:       &pagination,
	})
}

// AddUserStoryFavorite godoc
// @Summary Favorite a user story
// @Description Add a user story to favorites under project path
// @Tags UserStory, Favorite
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/favorite [post]
func (h *favoriteHandler) AddUserStoryFavorite(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	userStoryID, errorResponse := utils.StringToUUID(g.Param("user_story_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	res, err := h.service.AddUserStoryFavorite(userUUID, projectID, userStoryID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success: true,
		Data:    res,
	})
}

// RemoveUserStoryFavorite godoc
// @Summary Unfavorite a user story
// @Description Remove a user story from favorites under project path
// @Tags UserStory, Favorite
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/favorite [delete]
func (h *favoriteHandler) RemoveUserStoryFavorite(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	userStoryID, errorResponse := utils.StringToUUID(g.Param("user_story_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	res, err := h.service.RemoveUserStoryFavorite(userUUID, projectID, userStoryID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success: true,
		Message: "User story removed from favorites",
		Data:    res,
	})
}

// AddTaskFavorite godoc
// @Summary Favorite a task
// @Description Add a task to favorites under project path
// @Tags Task, Favorite
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/favorite [post]
func (h *favoriteHandler) AddTaskFavorite(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	taskID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	res, err := h.service.AddTaskFavorite(userUUID, projectID, taskID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success: true,
		Data:    res,
	})
}

// RemoveTaskFavorite godoc
// @Summary Unfavorite a task
// @Description Remove a task from favorites under project path
// @Tags Task, Favorite
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/favorite [delete]
func (h *favoriteHandler) RemoveTaskFavorite(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	taskID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{Success: false, Error: *errorResponse})
		return
	}

	res, err := h.service.RemoveTaskFavorite(userUUID, projectID, taskID)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success: true,
		Message: "Task removed from favorites",
		Data:    res,
	})
}

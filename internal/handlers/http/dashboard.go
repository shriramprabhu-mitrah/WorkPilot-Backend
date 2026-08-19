package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitDashboardHandler(service services.DashboardService, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		service: service,
		logger:  logger,
	}
}

type DashboardHandler struct {
	service services.DashboardService
	logger  *zap.Logger
}

func (h *DashboardHandler) GetOverview(c *gin.Context) {
	// Get project ID from URL parameter
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	overview, serviceErr := h.service.GetOverview(projectID, userUUID)
	if serviceErr != nil {
		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// Return response
	successResponse := &response.SuccessResponse{
		Message:    "Task Overview fetched successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       overview,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}

func (h *DashboardHandler) GetTaskStatus(c *gin.Context) {

	// Get project ID from URL parameter
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}

	// Get user ID from context
	userUUID, ok := getRequiredContextUUID(
		c,
		h.logger,
		"user_id",
		"user",
	)
	if !ok {
		return
	}

	// Call service layer
	taskStatus, serviceErr := h.service.GetTaskStatus(
		projectID,
		userUUID,
	)

	if serviceErr != nil {
		c.JSON(
			serviceErr.StatusCode,
			serviceErr,
		)
		return
	}

	// Return successful response
	successResponse := &response.SuccessResponse{
		Message:    "Task status fetched successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       taskStatus,
	}

	c.JSON(
		successResponse.StatusCode,
		successResponse,
	)
}

func (h *DashboardHandler) GetTeamWorkload(c *gin.Context) {
	// Get project ID from URL parameter
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	teamWorkload, serviceErr := h.service.GetTeamWorkload(projectID, userUUID)
	if serviceErr != nil {
		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// Return successful response
	successResponse := &response.SuccessResponse{
		Message:    "TeamWorkLoad fetched successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       teamWorkload,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}
func (h *DashboardHandler) GetWeeklyProgress(c *gin.Context) {
	// Get project ID from URL parameter
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Get date parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "start_date and end_date are required",
		})
		return
	}

	// Parse start date
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		h.logger.Warn(
			"Invalid start date",
			zap.String("startDate", startDateStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Parse end date
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		h.logger.Warn(
			"Invalid end date",
			zap.String("endDate", endDateStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Call service layer
	weeklyProgress, serviceErr := h.service.GetWeeklyProgress(projectID, startDate, endDate, userUUID)
	if serviceErr != nil {
		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// Return successful response
	successResponse := &response.SuccessResponse{
		Message:    "Task status fetched successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       weeklyProgress,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}

func (h *DashboardHandler) GetSprintBurndown(c *gin.Context) {
	// Get project ID from URL parameter
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}

	// Get sprint ID from URL parameter
	sprintIDStr := c.Param("sprint_id")

	sprintID, err := uuid.FromString(sprintIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid sprint ID",
			zap.String("sprintID", sprintIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint ID",
		})
		return
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	sprintBurndown, serviceErr := h.service.GetSprintBurndown(projectID, sprintID, userUUID)
	if serviceErr != nil {
		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// Return successful response
	successResponse := &response.SuccessResponse{
		Message:    "SprintBurndown fetched successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       sprintBurndown,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}

// GetDashboardData handles the GET request to fetch all dashboard data

func (h *DashboardHandler) GetDashboardData(c *gin.Context) {

	// 1. Get project ID from URL
	projectIDStr := c.Param("project_id")

	projectID, err := uuid.FromString(projectIDStr)
	if err != nil {
		h.logger.Warn(
			"Invalid project ID",
			zap.String("projectID", projectIDStr),
			zap.Error(err),
		)

		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project ID",
		})
		return
	}
	// 2. sprint querey param
	sprintIDStr := c.Query("sprint_id")

	if sprintIDStr == "" {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "sprint_id is required",
		})
		return
	}

	sprintID, err := uuid.FromString(sprintIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint_id",
		})
		return
	}

	// 3. Get authenticated user ID
	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// 4. Call service
	result, serviceErr := h.service.GetDashboardData(projectID, sprintID, userUUID)

	if serviceErr != nil {
		h.logger.Error(
			"Failed to get dashboard data",
			zap.String("projectID", projectID.String()),
			zap.String("sprintID", sprintID.String()),
			zap.Error(fmt.Errorf("%v", serviceErr)),
		)

		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// 5. Return dashboard response
	successResponse := &response.SuccessResponse{
		Message:    "Successfully Got the Dashboard",
		StatusCode: http.StatusCreated,
		Success:    true,
		Data:       result,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}

package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var _ = responsedto.SprintBurndown{}

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

// GetOverview godoc
// @Summary Get Task Overview
// @Description Retrieve the task overview for a project
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param sprint_id query string false "Sprint ID"
// @Success 200 {object} response.SuccessResponse{data=responsedto.DashboardOverview}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/overview [get]
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

	// Get optional sprint ID from query parameter
	sprintIDStr := c.Query("sprint_id")
	if sprintIDStr == "" {
		sprintIDStr = c.Query("sprintid")
	}
	var sprintID uuid.UUID

	if sprintIDStr != "" {
		sprintID, err = uuid.FromString(sprintIDStr)
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
	} else {
		sprintID = uuid.Nil
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	overview, serviceErr := h.service.GetOverview(projectID, userUUID, sprintID)
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

// GetTaskStatus godoc
// @Summary Get Task Status
// @Description Retrieve the task status summary for a project
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param sprint_id query string false "Sprint ID"
// @Success 200 {object} response.SuccessResponse{data=map[string]any}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/task-status [get]
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

	// Get optional sprint ID from query parameter
	sprintIDStr := c.Query("sprint_id")
	if sprintIDStr == "" {
		sprintIDStr = c.Query("sprintid")
	}
	var sprintID uuid.UUID

	if sprintIDStr != "" {
		sprintID, err = uuid.FromString(sprintIDStr)
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
	} else {
		sprintID = uuid.Nil
	}

	// Get user ID from context
	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	taskStatus, serviceErr := h.service.GetTaskStatus(projectID, userUUID, sprintID)
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

// GetTeamWorkload godoc
// @Summary Get Team Workload
// @Description Retrieve the workload of team members for a project
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param sprint_id query string false "Sprint ID"
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.TeamWorkload}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/team-workload [get]
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

	// Get optional sprint ID from query parameter
	sprintIDStr := c.Query("sprint_id")
	if sprintIDStr == "" {
		sprintIDStr = c.Query("sprintid")
	}
	var sprintID uuid.UUID

	if sprintIDStr != "" {
		sprintID, err = uuid.FromString(sprintIDStr)
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
	} else {
		sprintID = uuid.Nil
	}

	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// Call service layer
	teamWorkload, serviceErr := h.service.GetTeamWorkload(projectID, userUUID, sprintID)
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

// GetWeeklyProgress godoc
// @Summary Get Weekly Progress
// @Description Retrieve weekly task progress for a project within the specified date range
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param start_date query string true "Start date" example(2026-08-01)
// @Param end_date query string true "End date" example(2026-08-07)
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.WeeklyProgress}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/weekly-progress [get]
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

// GetSprintBurndown godoc
// @Summary Get Sprint Burndown
// @Description Get sprint burndown chart data for dashboard. If sprint_id is provided, returns that sprint's burndown. If omitted, returns burndown for all active sprints of the project.
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param sprint_id query string false "Sprint ID"
// @Success 200 {object} response.SuccessResponse{data=responsedto.DashboardSprintBurndownResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/sprint-burndown [get]
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

	// Get optional sprint ID from query parameter
	sprintIDStr := c.Query("sprint_id")
	if sprintIDStr == "" {
		sprintIDStr = c.Query("sprintid")
	}
	var sprintID uuid.UUID

	if sprintIDStr != "" {
		sprintID, err = uuid.FromString(sprintIDStr)
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
	} else {
		sprintID = uuid.Nil
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

// GetDashboardData godoc
// @Summary Get Dashboard Data
// @Description Retrieve dashboard data for a project and sprint
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID"
// @Param sprint_id query string false "Sprint ID"
// @Success 200 {object} response.SuccessResponse{data=responsedto.DashboardResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /{project_id}/dashboard [get]
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

	// 2. Get optional sprint ID from query parameter
	sprintIDStr := c.Query("sprint_id")
	if sprintIDStr == "" {
		sprintIDStr = c.Query("sprintid")
	}
	var sprintID uuid.UUID

	if sprintIDStr != "" {
		sprintID, err = uuid.FromString(sprintIDStr)
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
	} else {
		sprintID = uuid.Nil
	}

	// 3. Get authenticated user ID
	userUUID, ok := getRequiredContextUUID(c, h.logger, "user_id", "user")
	if !ok {
		return
	}

	// 4. Call service
	result, serviceErr := h.service.GetDashboardData(projectID, userUUID, sprintID)

	if serviceErr != nil {
		h.logger.Error(
			"Failed to get dashboard data",
			zap.String("projectID", projectID.String()),
			zap.Error(fmt.Errorf("%v", serviceErr)),
		)

		c.JSON(serviceErr.StatusCode, serviceErr)
		return
	}

	// 5. Return dashboard response
	successResponse := &response.SuccessResponse{
		Message:    "Successfully Got the Dashboard",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       result,
	}

	c.JSON(successResponse.StatusCode, successResponse)
}

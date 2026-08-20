package dashboardrepo

import (
	"errors"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (r *dashboardDatabase) GetOverview(projectID uuid.UUID) (responsedto.DashboardOverview, *response.Error) {

	var result responsedto.DashboardOverview

	now := time.Now()
	fortyEightHoursLater := now.Add(48 * time.Hour)

	// Total tasks
	err := r.db.
		Model(&models.Task{}).
		Where("tasks.project_id = ?", projectID).
		Count(&result.TotalTasks).Error

	if err != nil {
		r.logger.Error("Failed to get total tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get total tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Completed tasks
	err = r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("custom_statuses.is_final = ?", true).
		Count(&result.Completed).Error

	if err != nil {
		r.logger.Error("Failed to get completed tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get completed tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Pending tasks
	err = r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("custom_statuses.is_final = ?", false).
		Count(&result.Pending).Error

	if err != nil {
		r.logger.Error("Failed to get pending tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get pending tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Overdue tasks
	err = r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("tasks.due_date < ?", now).
		Where("custom_statuses.is_final = ?", false).
		Count(&result.Overdue).Error

	if err != nil {
		r.logger.Error("Failed to get overdue tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get overdue tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Due soon - next 48 hours
	err = r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("tasks.due_date >= ?", now).
		Where("tasks.due_date <= ?", fortyEightHoursLater).
		Where("custom_statuses.is_final = ?", false).
		Count(&result.Duesoon).Error

	if err != nil {
		r.logger.Error("Failed to get due soon tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get due soon tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	return result, nil
}

func (r *dashboardDatabase) GetTaskStatus(projectID uuid.UUID) (map[string]int64, *response.Error) {

	var result []struct {
		Status string
		Count  int64
	}

	err := r.db.
		Model(&models.Task{}).
		Select(`
			CASE
				WHEN custom_statuses.is_final = true THEN 'completed'
				ELSE tasks.status
			END AS status,
			COUNT(*) AS count
		`).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Group(`
			CASE
				WHEN custom_statuses.is_final = true THEN 'completed'
				ELSE tasks.status
			END
		`).
		Order("status").
		Scan(&result).Error

	if err != nil {
		r.logger.Error(
			"Failed to get task status",
			zap.String("projectID", projectID.String()),
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get task status",
			StatusCode: http.StatusInternalServerError,
		}
	}

	taskStatus := make(map[string]int64)

	for _, item := range result {
		taskStatus[item.Status] = item.Count
	}

	return taskStatus, nil
}

func (r *dashboardDatabase) GetTeamWorkload(projectID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error) {

	var result []responsedto.TeamWorkload

	err := r.db.
		Table("tasks").
		Select(`
			users.id AS user_id,
			users.username AS user_name,
			COALESCE(users.full_name, '') AS full_name,
			COALESCE(users.avatar_url, '') AS avatar_url,
			COUNT(tasks.id) AS task_count,
			COALESCE(SUM(tasks.story_points), 0) AS points
		`).
		Joins(`
			INNER JOIN users
			ON users.id = tasks.assignee_id
		`).
		Where("tasks.project_id = ?", projectID).
		Group(`
			users.id,
			users.username,
			users.full_name,
			users.avatar_url
		`).
		Order("users.username").
		Scan(&result).Error

	if err != nil {
		r.logger.Error("Failed to get team workload", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get team workload",
			StatusCode: http.StatusInternalServerError,
		}
	}

	return result, nil
}

func (r *dashboardDatabase) GetWeeklyProgress(projectID uuid.UUID, startDate time.Time, endDate time.Time) ([]responsedto.WeeklyProgress, *response.Error) {

	var tasks []models.Task

	err := r.db.
		Where("project_id = ?", projectID).
		Where(
			"(due_date >= ? AND due_date < ?) OR (created_at >= ? AND created_at < ?)",
			startDate,
			endDate,
			startDate,
			endDate,
		).
		Find(&tasks).Error

	if err != nil {
		r.logger.Error("Failed to get weekly progress", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get weekly progress",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Store planned and completed counts by date.
	planned := make(map[string]int64)
	completed := make(map[string]int64)

	for _, task := range tasks {

		// Planned tasks
		if task.DueDate != nil {
			date := task.DueDate.Format("2006-01-02")
			planned[date]++
		}

		// Completed tasks
		if task.DueDate != nil && task.Status == "completed" {
			date := task.DueDate.Format("2006-01-02")
			completed[date]++
		}
	}

	var result []responsedto.WeeklyProgress

	// Create one result for each day.
	for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {

		dateKey := date.Format("2006-01-02")

		result = append(result, responsedto.WeeklyProgress{
			Day:       date.Format("Mon"),
			Planned:   planned[dateKey],
			Completed: int(completed[dateKey]),
		})
	}

	return result, nil
}

func (r *dashboardDatabase) GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.SprintBurndown, *response.Error) {

	// 1. Fetch sprint
	var sprint models.Sprint

	err := r.db.
		Where("id = ?", sprintID).
		Where("project_id = ?", projectID).
		First(&sprint).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(
				"Sprint not found",
				zap.String("projectID", projectID.String()),
				zap.String("sprintID", sprintID.String()),
			)

			return nil, &response.Error{
				Code:       response.ErrNotFound,
				Message:    "Sprint not found",
				StatusCode: http.StatusNotFound,
			}
		}

		r.logger.Error(
			"Failed to get sprint",
			zap.String("projectID", projectID.String()),
			zap.String("sprintID", sprintID.String()),
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get sprint",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 2. Validate sprint dates
	if sprint.EndDate.Before(sprint.StartDate) {
		r.logger.Error(
			"Invalid sprint dates",
			zap.String("sprintID", sprintID.String()),
			zap.Time("startDate", sprint.StartDate),
			zap.Time("endDate", sprint.EndDate),
		)

		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Sprint end date cannot be before start date",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 3. Fetch sprint tasks
	var tasks []models.Task

	err = r.db.
		Where("project_id = ?", projectID).
		Where("sprint_id = ?", sprintID).
		Find(&tasks).Error

	if err != nil {
		r.logger.Error(
			"Failed to get sprint tasks",
			zap.String("projectID", projectID.String()),
			zap.String("sprintID", sprintID.String()),
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get sprint tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 4. Calculate total estimated hours and actual hours
	var totalEstimatedHours float64
	var totalActualHours float64

	for _, task := range tasks {

		if task.EstimatedHours != nil {
			totalEstimatedHours += *task.EstimatedHours
		}

		if task.ActualHours != nil {
			totalActualHours += *task.ActualHours
		}
	}

	// 5. Calculate total sprint days
	startDate := sprint.StartDate.Truncate(24 * time.Hour)
	endDate := sprint.EndDate.Truncate(24 * time.Hour)

	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1

	if totalDays <= 0 {
		r.logger.Error(
			"Invalid sprint duration",
			zap.String("sprintID", sprintID.String()),
			zap.Int("totalDays", totalDays),
		)

		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Invalid sprint duration",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 6. Prepare result
	result := make(
		[]responsedto.SprintBurndown,
		0,
		totalDays,
	)

	// 7. Calculate burndown for each day
	for day := 0; day < totalDays; day++ {

		currentDate := startDate.AddDate(0, 0, day)

		// Calculate ideal hours.
		// Day 1 = total estimated hours
		// Last day = 0 hours
		var idealHours float64

		if totalDays == 1 {
			idealHours = 0
		} else {
			idealHours = totalEstimatedHours -
				(totalEstimatedHours/float64(totalDays-1))*float64(day)
		}

		if idealHours < 0 {
			idealHours = 0
		}

		// Append daily burndown data.
		result = append(
			result,
			responsedto.SprintBurndown{
				Day:         day + 1,
				Date:        currentDate.Format("2006-01-02"),
				IdealHours:  idealHours,
				ActualHours: totalActualHours,
			},
		)
	}

	return result, nil
}

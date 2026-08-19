package dashboardrepo

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

func (r *dashboardDatabase) GetOverview(projectID uuid.UUID) (responsedto.DashboardOverview, *response.Error) {

	var result responsedto.DashboardOverview

	// Total tasks
	err := r.db.
		Model(&models.Task{}).
		Where("project_id = ?", projectID).
		Count(&result.TotalTasks).Error
	if err != nil {
		r.logger.Error("Failed to get total tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get total tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Completed
	err = r.db.
		Model(&models.Task{}).
		Where("project_id = ?", projectID).
		Where("status = ?", "completed").
		Count(&result.Completed).Error
	if err != nil {
		r.logger.Error("Failed to get completed tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get completed tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Pending
	err = r.db.
		Model(&models.Task{}).
		Where("project_id = ?", projectID).
		Where("status != ?", "completed").
		Count(&result.Pending).Error
	if err != nil {
		r.logger.Error("Failed to get pending tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get pending tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Overdue
	err = r.db.
		Model(&models.Task{}).
		Where("project_id = ?", projectID).
		Where("due_date < ?", time.Now()).
		Where("status != ?", "completed").
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
	fortyEightHoursLater := time.Now().Add(48 * time.Hour)

	err = r.db.
		Model(&models.Task{}).
		Where("project_id = ?", projectID).
		Where("due_date >= ?", time.Now()).
		Where("due_date <= ?", fortyEightHoursLater).
		Where("status != ?", "completed").
		Count(&result.Duesoon).Error

	if err != nil {
		r.logger.Error("Failed to get Duesoon tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get Duesoon tasks",
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
			status,
			COUNT(*) AS count
		`).
		Where("project_id = ?", projectID).
		Group("status").
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

func (r *dashboardDatabase) GetSprintBurndown(projectID uuid.UUID,sprintID uuid.UUID,) ([]responsedto.SprintBurndown, *response.Error) {

	// 1. Fetch sprint
	var sprint models.Sprint

	err := r.db.
		Where("id = ?", sprintID).
		Where("project_id = ?", projectID).
		First(&sprint).Error

	if err != nil {
		r.logger.Error(
			"Failed to get sprint",
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get sprint",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 2. Fetch sprint tasks
	var tasks []models.Task

	err = r.db.
		Where("project_id = ?", projectID).
		Where("sprint_id = ?", sprintID).
		Find(&tasks).Error

	if err != nil {
		r.logger.Error(
			"Failed to get sprint tasks",
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get sprint tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 3. Calculate total estimated hours
	var totalEstimatedHours float64

	for _, task := range tasks {
		if task.EstimatedHours != nil {
			totalEstimatedHours += *task.EstimatedHours
		}
	}

	// 4. Calculate sprint days
	totalDays := int(
		sprint.EndDate.Sub(sprint.StartDate).Hours()/24,
	) + 1

	if totalDays <= 0 {
		r.logger.Error(
			"Invalid sprint days",
			zap.Int("totalDays", totalDays),
		)

		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Invalid sprint days",
			StatusCode: http.StatusBadRequest,
		}
	}

	result := make([]responsedto.SprintBurndown, 0, totalDays)

	// 5. Calculate burndown
	for day := 0; day < totalDays; day++ {

		currentDate := sprint.StartDate.AddDate(0, 0, day)

		// Ideal hours
		idealHours := totalEstimatedHours -
			(totalEstimatedHours/float64(totalDays))*float64(day)

		if idealHours < 0 {
			idealHours = 0
		}

		// Calculate actual hours
		var actualHours float64

		for _, task := range tasks {
			if task.ActualHours != nil {
				actualHours += *task.ActualHours
			}
		}

		result = append(
			result,
			responsedto.SprintBurndown{
				Day:        day + 1,
				Date:       currentDate.Format("2006-01-02"),
				IdealHours: idealHours,
				ActualHours: actualHours,
			},
		)
	}

	return result, nil
}

package dashboardrepo

import (
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (r *dashboardDatabase) GetOverview(projectID uuid.UUID, sprintID uuid.UUID) (responsedto.DashboardOverview, *response.Error) {

	var result responsedto.DashboardOverview

	now := time.Now()
	fortyEightHoursLater := now.Add(48 * time.Hour)

	// Total tasks
	qTotal := r.db.
		Model(&models.Task{}).
		Where("tasks.project_id = ?", projectID)

	// Debug logging of all tasks for this project
	var allTasks []models.Task
	if errDb := r.db.Where("project_id = ?", projectID).Find(&allTasks).Error; errDb == nil {
		for _, t := range allTasks {
			sprintStr := "nil"
			if t.SprintID != nil {
				sprintStr = t.SprintID.String()
			}
			r.logger.Info("DEBUG_TASK_INFO",
				zap.String("task_id", t.ID.String()),
				zap.String("key", t.Key),
				zap.String("title", t.Title),
				zap.String("sprint_id", sprintStr),
			)
		}
	}

	if sprintID != uuid.Nil {
		qTotal = qTotal.Where("tasks.sprint_id = ?", sprintID)
	}
	err := qTotal.Count(&result.TotalTasks).Error

	if err != nil {
		r.logger.Error("Failed to get total tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get total tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Completed tasks
	qCompleted := r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("custom_statuses.is_final = ?", true)
	if sprintID != uuid.Nil {
		qCompleted = qCompleted.Where("tasks.sprint_id = ?", sprintID)
	}
	err = qCompleted.Count(&result.Completed).Error

	if err != nil {
		r.logger.Error("Failed to get completed tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get completed tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Pending tasks
	qPending := r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("custom_statuses.is_final = ?", false)
	if sprintID != uuid.Nil {
		qPending = qPending.Where("tasks.sprint_id = ?", sprintID)
	}
	err = qPending.Count(&result.Pending).Error

	if err != nil {
		r.logger.Error("Failed to get pending tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get pending tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Overdue tasks
	qOverdue := r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("tasks.due_date < ?", now).
		Where("custom_statuses.is_final = ?", false)
	if sprintID != uuid.Nil {
		qOverdue = qOverdue.Where("tasks.sprint_id = ?", sprintID)
	}
	err = qOverdue.Count(&result.Overdue).Error

	if err != nil {
		r.logger.Error("Failed to get overdue tasks", zap.Error(err))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get overdue tasks",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Due soon - next 48 hours
	qDueSoon := r.db.
		Model(&models.Task{}).
		Joins(`
			JOIN custom_statuses
			ON custom_statuses.id = tasks.status_id
		`).
		Where("tasks.project_id = ?", projectID).
		Where("tasks.due_date >= ?", now).
		Where("tasks.due_date <= ?", fortyEightHoursLater).
		Where("custom_statuses.is_final = ?", false)
	if sprintID != uuid.Nil {
		qDueSoon = qDueSoon.Where("tasks.sprint_id = ?", sprintID)
	}
	err = qDueSoon.Count(&result.Duesoon).Error

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

func (r *dashboardDatabase) GetTaskStatus(projectID uuid.UUID, sprintID uuid.UUID) (map[string]any, *response.Error) {

	// 1. Fetch all custom statuses for the project to include zero values for unused statuses
	var customStatuses []models.CustomStatus
	if err := r.db.Where("project_id = ?", projectID).Find(&customStatuses).Error; err != nil {
		r.logger.Error(
			"Failed to get custom statuses for project",
			zap.String("projectID", projectID.String()),
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get custom statuses",
			StatusCode: http.StatusInternalServerError,
		}
	}

	taskStatus := make(map[string]any)
	for _, cs := range customStatuses {
		statusName := cs.Name
		if cs.IsFinal {
			statusName = "completed"
		}
		taskStatus[statusName] = map[string]any{
			"count": 0,
			"color": cs.Color,
		}
	}

	// 2. Fetch the task counts grouped by status
	var result []struct {
		Status string
		Count  int64
	}

	query := r.db.
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
		Where("tasks.project_id = ?", projectID)

	if sprintID != uuid.Nil {
		query = query.Where("tasks.sprint_id = ?", sprintID)
	}

	err := query.
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
			"Failed to get task status counts",
			zap.String("projectID", projectID.String()),
			zap.Error(err),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get task status",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 3. Update the taskStatus map with the counts obtained
	for _, item := range result {
		if statusMap, ok := taskStatus[item.Status].(map[string]any); ok {
			statusMap["count"] = item.Count
		} else {
			taskStatus[item.Status] = map[string]any{
				"count": item.Count,
				"color": "",
			}
		}
	}

	return taskStatus, nil
}

func (r *dashboardDatabase) GetTeamWorkload(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error) {

	var result []responsedto.TeamWorkload

	query := r.db.
		Table("tasks").
		Select(`
			users.id AS user_id,
			users.username AS user_name,
			COALESCE(users.full_name, '') AS full_name,
			COALESCE(users.avatar_url, '') AS avatar_url,
			COALESCE(users.color, '') AS color,
			COUNT(tasks.id) AS task_count,
			COALESCE(SUM(tasks.story_points), 0) AS points
		`).
		Joins(`
			INNER JOIN users
			ON users.id = tasks.assignee_id
		`).
		Where("tasks.project_id = ?", projectID)

	if sprintID != uuid.Nil {
		query = query.Where("tasks.sprint_id = ?", sprintID)
	}

	err := query.
		Group(`
			users.id,
			users.username,
			users.full_name,
			users.avatar_url,
			users.color
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

func (r *dashboardDatabase) GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.SprintBurndown, float64, float64, *response.Error) {

	// 1. Fetch sprint
	var sprint models.Sprint

	err := r.db.
		Where("id = ?", sprintID).
		Where("project_id = ?", projectID).
		Find(&sprint).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Warn(
				"Sprint not found",
				zap.String("projectID", projectID.String()),
				zap.String("sprintID", sprintID.String()),
			)

			return nil, 0, 0, &response.Error{
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

		return nil, 0, 0, &response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Failed to get sprint",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// 2. Validate sprint dates
	if sprint.StartDate == nil || sprint.EndDate == nil {
		r.logger.Error(
			"Sprint dates are nil",
			zap.String("sprintID", sprintID.String()),
		)

		return nil, 0, 0, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Sprint start date and end date must be set",
			StatusCode: http.StatusBadRequest,
		}
	}

	if sprint.EndDate.Before(*sprint.StartDate) {
		r.logger.Error(
			"Invalid sprint dates",
			zap.String("sprintID", sprintID.String()),
			zap.Time("startDate", *sprint.StartDate),
			zap.Time("endDate", *sprint.EndDate),
		)

		return nil, 0, 0, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Sprint end date cannot be before start date",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 3. Fetch sprint tasks
	var tasks []models.Task

	err = r.db.
		Model(&models.Task{}).
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

		return nil, 0, 0, &response.Error{
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

	// Round total hours to 2 decimal places.
	totalEstimatedHours = math.Round(totalEstimatedHours*100) / 100
	totalActualHours = math.Round(totalActualHours*100) / 100

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

		return nil, 0, 0, &response.Error{
			Code:       response.ErrBadRequest,
			Message:    "Invalid sprint duration",
			StatusCode: http.StatusBadRequest,
		}
	}

	// 6. Prepare result
	result := make([]responsedto.SprintBurndown, 0, totalDays)

	// 7. Calculate burndown for each day
	for day := 0; day < totalDays; day++ {

		currentDate := startDate.AddDate(0, 0, day)

		// Calculate ideal hours.
		// Day 1 = total estimated hours
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

		// Round ideal hours to 2 decimal places.
		idealHours = math.Round(idealHours*100) / 100

		// Round actual hours to 2 decimal places.
		actualHours := math.Round(totalActualHours*100) / 100

		// Append daily burndown data.
		result = append(
			result,
			responsedto.SprintBurndown{
				Day:         day + 1,
				Date:        currentDate.Format("2006-01-02"),
				IdealHours:  idealHours,
				ActualHours: actualHours,
			},
		)
	}

	return result, totalEstimatedHours, totalActualHours, nil
}

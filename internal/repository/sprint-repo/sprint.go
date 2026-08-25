package sprintrepo

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *sprintDatabase) CreateSprint(row *models.Sprint) *response.Error {
	if err := d.db.Create(row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict",
				zap.Error(err))
			return utils.ParseSprintDuplicateError(err)
		}

		d.logger.Error("Database error occurred",
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return nil
}

func (r *sprintDatabase) StartSprint(sprintID uuid.UUID, startDate time.Time, endDate time.Time) *response.Error {

	result := r.db.
		Model(&models.Sprint{}).
		Where("id = ?", sprintID).
		Where("status = ?", "planned").
		Updates(map[string]interface{}{
			"status":     "active",
			"start_date": startDate,
			"end_date":   endDate,
		})

	if result.Error != nil {
		r.logger.Error(
			"Failed to start sprint",
			zap.String("sprintID", sprintID.String()),
			zap.Error(result.Error),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to start sprint",
		}
	}

	if result.RowsAffected == 0 {
		r.logger.Warn(
			"Sprint cannot be started",
			zap.String("sprintID", sprintID.String()),
		)

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Only planned sprints can be started",
		}
	}

	return nil
}

func (r *sprintDatabase) CompleteSprint(sprintID uuid.UUID, projectID uuid.UUID, actualEndDate time.Time, velocity int) *response.Error {

	result := r.db.
		Model(&models.Sprint{}).
		Where("id = ?", sprintID).
		Where("project_id = ?", projectID).
		Where("status = ?", "active").
		Updates(map[string]interface{}{
			"status":          "completed",
			"actual_end_date": actualEndDate,
			"velocity":        velocity,
		})

	if result.Error != nil {
		r.logger.Error(
			"Failed to complete sprint",
			zap.String("sprintID", sprintID.String()),
			zap.String("projectID", projectID.String()),
			zap.Error(result.Error),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to complete sprint",
		}
	}

	if result.RowsAffected == 0 {
		r.logger.Warn(
			"Sprint cannot be completed",
			zap.String("sprintID", sprintID.String()),
			zap.String("projectID", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Only active sprints can be completed",
		}
	}

	return nil
}

func (d *sprintDatabase) UpdateSprint(projectID, sprintID uuid.UUID, updates map[string]interface{}) *response.Error {

	result := d.db.
		Model(&models.Sprint{}).
		Where("id = ? AND project_id = ?", sprintID, projectID).
		Updates(updates)

	if result.Error != nil {
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating sprint",
				zap.Error(result.Error))
			return utils.ParseSprintDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating sprint",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Sprint not found for the given project",
			zap.String("project_id", projectID.String()),
			zap.String("sprint_id", sprintID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found",
		}
	}

	return nil
}

func (d *sprintDatabase) DeleteSprint(id uuid.UUID) *response.Error {

	result := d.db.
		Where("id = ?", id).
		Delete(&models.Sprint{})

	if result.Error != nil {
		d.logger.Error("Failed to delete sprint",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Sprint could not be found for deletion",
			zap.String("sprint_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found",
		}
	}

	return nil
}

func (d *sprintDatabase) GetSprintByID(sprintID, projectID uuid.UUID) (*models.Sprint, *response.Error) {

	var sprint models.Sprint

	err := d.db.
		Preload("Project").
		Preload("Project.Organization").
		Preload("Project.Creator").
		Preload("CreatedBy").
		Preload("CreatedBy.Organization").
		Where("id = ? AND project_id = ?", sprintID, projectID).
		First(&sprint).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.logger.Error("Sprint not found",
				zap.String("sprint_id", sprintID.String()))

			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Sprint not found",
			}
		}

		d.logger.Error("Failed to get sprint",
			zap.Error(err))

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return &sprint, nil
}

func (d *sprintDatabase) GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {

	var sprints []models.Sprint
	var totalItems int64

	filter.PaginationQuery.Normalize(10)

	offset := (filter.Page - 1) * filter.PageSize

	query := d.db.Model(&models.Sprint{}).
		Where("project_id = ?", projectID)

	if filter.Search != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(filter.Search)+"%")
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	// Date range filter
	if !filter.StartDate.IsZero() && !filter.EndDate.IsZero() {
		query = query.
			Where("start_date >= ?", filter.StartDate).
			Where("end_date <= ?", filter.EndDate)
	}

	if err := query.Count(&totalItems).Error; err != nil {
		d.logger.Error("Failed to count sprints",
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := query.
		Preload("Project").
		Preload("Project.Organization").
		Preload("Project.Creator").
		Preload("CreatedBy").
		Preload("CreatedBy.Organization").
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&sprints).Error; err != nil {

		d.logger.Error("Failed to fetch sprints",
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return sprints, pagination, nil
}

func (d *sprintDatabase) CreateSprintSnapshot(snapshot models.SprintSnapshot) *response.Error {
	if err := d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sprint_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"total_story_points", "remaining_story_points"}),
	}).Create(&snapshot).Error; err != nil {
		d.logger.Error("Database error occurred while creating/updating sprint snapshot", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save snapshot.",
		}
	}
	return nil
}

func (d *sprintDatabase) GetSprintSnapshots(sprintID uuid.UUID) ([]models.SprintSnapshot, *response.Error) {
	var snapshots []models.SprintSnapshot
	err := d.db.Where("sprint_id = ?", sprintID).Order("date ASC").Find(&snapshots).Error
	if err != nil {
		d.logger.Error("Failed to fetch sprint snapshots", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch sprint snapshots.",
		}
	}
	return snapshots, nil
}

func (d *sprintDatabase) GetTotalStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	var total int64
	err := d.db.Model(&models.Task{}).Where("sprint_id = ? AND deleted_at IS NULL", sprintID).Select("COALESCE(SUM(story_points), 0)").Scan(&total).Error
	if err != nil {
		d.logger.Error("Failed to sum total story points", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to calculate total story points.",
		}
	}
	return int(total), nil
}

func (d *sprintDatabase) GetRemainingStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	var remaining int64
	err := d.db.Model(&models.Task{}).
		Where("sprint_id = ? AND deleted_at IS NULL AND status_id IN (SELECT id FROM custom_statuses WHERE is_final = false AND deleted_at IS NULL)", sprintID).
		Select("COALESCE(SUM(story_points), 0)").
		Scan(&remaining).Error
	if err != nil {
		d.logger.Error("Failed to sum remaining story points", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to calculate remaining story points.",
		}
	}
	return int(remaining), nil
}

func (d *sprintDatabase) GetActiveSprints() ([]models.Sprint, *response.Error) {
	var sprints []models.Sprint
	err := d.db.Where("status = ?", "active").Find(&sprints).Error
	if err != nil {
		d.logger.Error("Failed to fetch active sprints", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch active sprints.",
		}
	}
	return sprints, nil
}

func (d *sprintDatabase) GetActiveSprintsByProjectID(projectID uuid.UUID) ([]models.Sprint, *response.Error) {
	var sprints []models.Sprint
	err := d.db.Where("project_id = ? AND status = ?", projectID, "active").Find(&sprints).Error
	if err != nil {
		d.logger.Error("Failed to fetch active sprints by project ID", zap.String("project_id", projectID.String()), zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch active sprints.",
		}
	}
	return sprints, nil
}

func (d *sprintDatabase) GetCompletedTasksStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	var completed int64
	err := d.db.Model(&models.Task{}).
		Where("sprint_id = ? AND deleted_at IS NULL AND status_id IN (SELECT id FROM custom_statuses WHERE is_final = true AND deleted_at IS NULL)", sprintID).
		Select("COALESCE(SUM(story_points), 0)").
		Scan(&completed).Error
	if err != nil {
		d.logger.Error("Failed to sum completed tasks story points", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to calculate completed story points.",
		}
	}
	return int(completed), nil
}

func (d *sprintDatabase) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error {
	err := d.db.Model(&models.Task{}).
		Where("sprint_id = ? AND deleted_at IS NULL AND status_id IN (SELECT id FROM custom_statuses WHERE is_final = false AND deleted_at IS NULL)", sprintID).
		Update("sprint_id", nil).Error
	if err != nil {
		d.logger.Error("Failed to move incomplete tasks to backlog during sprint completion", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to move incomplete tasks to backlog during sprint completion",
		}
	}
	return nil
}

func (d *sprintDatabase) GetSprintCountByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int, *response.Error) {
	if len(projectIDs) == 0 {
		return make(map[uuid.UUID]int), nil
	}

	type ProjectSprintCount struct {
		ProjectID uuid.UUID `gorm:"column:project_id"`
		Count     int       `gorm:"column:count"`
	}

	var results []ProjectSprintCount
	err := d.db.Model(&models.Sprint{}).
		Select("project_id, count(*) as count").
		Where("project_id IN (?)", projectIDs).
		Group("project_id").
		Scan(&results).Error

	if err != nil {
		d.logger.Error("Database error occurred while fetching sprint counts", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	counts := make(map[uuid.UUID]int)
	for _, r := range results {
		counts[r.ProjectID] = r.Count
	}

	return counts, nil
}

func (d *sprintDatabase) GetSprintsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID][]models.Sprint, *response.Error) {
	if len(projectIDs) == 0 {
		return make(map[uuid.UUID][]models.Sprint), nil
	}

	var sprints []models.Sprint
	err := d.db.Where("project_id IN (?)", projectIDs).
		Order("project_id ASC, created_at DESC").
		Find(&sprints).Error
	if err != nil {
		d.logger.Error("Database error occurred while fetching project sprints", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	result := make(map[uuid.UUID][]models.Sprint, len(projectIDs))
	for _, projectID := range projectIDs {
		result[projectID] = []models.Sprint{}
	}
	for _, sprint := range sprints {
		result[sprint.ProjectID] = append(result[sprint.ProjectID], sprint)
	}

	return result, nil
}

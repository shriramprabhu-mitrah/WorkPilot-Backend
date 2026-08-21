package userstoryrepo

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *userStoryDatabase) CreateUserStory(userStory *models.UserStory) *response.Error {
	if err := d.db.Create(userStory).Error; err != nil {
		d.logger.Error("Failed to create user story", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create user story",
		}
	}
	return nil
}

func (d *userStoryDatabase) GetUserStoryByID(id uuid.UUID, projectID uuid.UUID) (*models.UserStory, *response.Error) {
	var story models.UserStory
	err := d.db.Model(&models.UserStory{}).
		Preload("Sprint").Preload("Assignee").Preload("Reporter").
		Select("user_stories.*, user_story_statuses.name AS status").
		Joins("LEFT JOIN user_story_statuses ON user_story_statuses.id = user_stories.status_id").
		Where("user_stories.id = ? AND user_stories.project_id = ?", id, projectID).
		First(&story).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "User story not found",
			}
		}
		d.logger.Error("Failed to fetch user story", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story",
		}
	}
	return &story, nil
}

func (d *userStoryDatabase) UpdateUserStory(userStoryID uuid.UUID, updates map[string]interface{}) *response.Error {
	if err := d.db.Model(&models.UserStory{}).Where("id = ?", userStoryID).Updates(updates).Error; err != nil {
		d.logger.Error("Failed to update user story", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update user story",
		}
	}
	return nil
}

func (d *userStoryDatabase) DeleteUserStory(id uuid.UUID, projectID uuid.UUID) *response.Error {
	if err := d.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&models.UserStory{}).Error; err != nil {
		d.logger.Error("Failed to delete user story", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete user story",
		}
	}
	return nil
}

func (d *userStoryDatabase) GetUserStories(projectID uuid.UUID, filter dto.UserStoryFilter) ([]models.UserStory, response.Pagination, *response.Error) {
	var stories []models.UserStory
	var totalItems int64

	// Normalize inputs (defaulting to 10 items/page and created_at DESC)
	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	query := d.db.Model(&models.UserStory{}).
		Preload("Sprint").
		Preload("Assignee").
		Preload("Reporter").
		Select("user_stories.*, user_story_statuses.name AS status").
		Joins("LEFT JOIN user_story_statuses ON user_story_statuses.id = user_stories.status_id").
		Where("user_stories.project_id = ?", projectID)

	if filter.Status != "" {
		query = query.Where("user_story_statuses.name = ?", filter.Status)
	}

	if filter.Assignee != "" {
		query = query.Where("assignee_id = ?", filter.Assignee)
	}

	if filter.Reporter != "" {
		query = query.Where("reporter_id = ?", filter.Reporter)
	}

	if filter.Sprint != "" {
		if filter.Sprint == "null" || filter.Sprint == "none" {
			query = query.Where("sprint_id IS NULL")
		} else {
			query = query.Where("sprint_id = ?", filter.Sprint)
		}
	}

	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}

	if filter.SequenceNumber != nil {
		query = query.Where("serial_number = ?", *filter.SequenceNumber)
	}

	if filter.SerialNumber != nil {
		query = query.Where("serial_number = ?", *filter.SerialNumber)
	}

	if filter.IsClosed != nil {
		query = query.Where("is_closed = ?", *filter.IsClosed)
	}

	if filter.Search != "" {
		cleanSearch := strings.TrimPrefix(strings.TrimSpace(filter.Search), "#")
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		cleanTerm := "%" + strings.ToLower(cleanSearch) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR CAST(serial_number AS TEXT) LIKE ?", searchTerm, searchTerm, cleanTerm)
	}

	// 1. Get the total count of filtered items
	if err := query.Count(&totalItems).Error; err != nil {
		d.logger.Error("Failed to count user stories", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user stories",
		}
	}

	// 2. Build the order clause dynamically
	orderClause := "user_stories.created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"title":         "user_stories.title",
			"created_at":    "user_stories.created_at",
			"updated_at":    "user_stories.updated_at",
			"priority":      "user_stories.priority",
			"status":        "user_story_statuses.name",
			"serial_number": "user_stories.serial_number",
		}
		if col, ok := allowed[filter.SortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", col, direction)
		}
	}

	// 3. Retrieve paginated results
	if err := query.
		Order(orderClause).
		Limit(filter.PageSize).
		Offset(offset).
		Find(&stories).Error; err != nil {
		d.logger.Error("Failed to fetch user stories list", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user stories",
		}
	}

	// 4. Calculate total pages
	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return stories, pagination, nil
}

func (d *userStoryDatabase) IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error) {
	var count int64
	err := d.db.Model(&models.Sprint{}).Where("id = ? AND project_id = ?", sprintID, projectID).Count(&count).Error
	if err != nil {
		d.logger.Error("Failed to check if sprint is in project", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to validate sprint",
		}
	}
	return count > 0, nil
}

func (d *userStoryDatabase) GetMaxBacklogOrder(projectID uuid.UUID) (int, *response.Error) {
	var maxOrder int64
	err := d.db.Model(&models.UserStory{}).Unscoped().
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(backlog_order), 0)").
		Scan(&maxOrder).Error
	if err != nil {
		d.logger.Error("Failed to get max backlog order", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to count backlog order",
		}
	}
	return int(maxOrder), nil
}

func (d *userStoryDatabase) ReorderUserStories(projectID uuid.UUID, storyIDs []uuid.UUID) *response.Error {
	err := d.db.Transaction(func(tx *gorm.DB) error {
		for idx, id := range storyIDs {
			// Update the order (1-indexed) for each story belonging to the project
			res := tx.Model(&models.UserStory{}).
				Where("id = ? AND project_id = ?", id, projectID).
				Update("backlog_order", idx+1)
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
	if err != nil {
		d.logger.Error("Failed to reorder user stories", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to persist user stories reordering",
		}
	}
	return nil
}

func (d *userStoryDatabase) GetStoryTaskStats(projectID uuid.UUID) (map[uuid.UUID]models.StoryTaskStats, *response.Error) {
	type QueryResult struct {
		UserStoryID uuid.UUID `gorm:"column:user_story_id"`
		TotalTasks  int64     `gorm:"column:total_tasks"`
		Completed   int64     `gorm:"column:completed_tasks"`
	}
	var results []QueryResult

	err := d.db.Model(&models.Task{}).
		Select("tasks.user_story_id, COUNT(tasks.id) as total_tasks, COUNT(CASE WHEN custom_statuses.is_final = true THEN 1 END) as completed_tasks").
		Joins("JOIN custom_statuses ON custom_statuses.id = tasks.status_id AND custom_statuses.deleted_at IS NULL").
		Where("tasks.project_id = ? AND tasks.user_story_id IS NOT NULL", projectID).
		Group("tasks.user_story_id").
		Scan(&results).Error

	if err != nil {
		d.logger.Error("Failed to query task statistics for user stories", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve user story progress",
		}
	}

	statsMap := make(map[uuid.UUID]models.StoryTaskStats)
	for _, res := range results {
		statsMap[res.UserStoryID] = models.StoryTaskStats{
			UserStoryID: res.UserStoryID,
			TotalTasks:  res.TotalTasks,
			Completed:   res.Completed,
		}
	}
	return statsMap, nil
}

func (d *userStoryDatabase) GetUserStoryAccessContext(id uuid.UUID) (*models.UserStoryAccessContext, *response.Error) {
	var ctx models.UserStoryAccessContext
	err := d.db.Table("user_stories").
		Select("user_stories.id as user_story_id, user_stories.project_id as project_id, projects.organization_id as organization_id, user_stories.title as title").
		Joins("join projects on projects.id = user_stories.project_id").
		Where("user_stories.id = ? AND user_stories.deleted_at IS NULL", id).
		Scan(&ctx).Error
	if err != nil {
		d.logger.Error("Failed to fetch user story access context", zap.Error(err), zap.String("user_story_id", id.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story security context",
		}
	}
	if ctx.UserStoryID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User story not found",
		}
	}
	return &ctx, nil
}

func (d *userStoryDatabase) RecalculateUserStoryIsClosed(userStoryID uuid.UUID) *response.Error {
	if userStoryID == uuid.Nil {
		return nil
	}
	type TaskStats struct {
		Total     int64
		Completed int64
	}
	var stats TaskStats
	err := d.db.
		Model(&models.Task{}).
		Select("COUNT(tasks.id) as total, COUNT(CASE WHEN custom_statuses.is_final = true THEN 1 END) as completed").
		Joins("JOIN custom_statuses ON custom_statuses.id = tasks.status_id AND custom_statuses.deleted_at IS NULL").
		Where("tasks.user_story_id = ?", userStoryID).
		Scan(&stats).Error

	if err != nil {
		d.logger.Error("Failed to count tasks for user story recalculation",
			zap.Error(err),
			zap.String("user_story_id", userStoryID.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to recalculate user story status",
		}
	}

	isClosed := false
	if stats.Total > 0 {
		isClosed = (stats.Completed == stats.Total)
	} else {
		var story models.UserStory
		err := d.db.Table("user_stories").
			Where("id = ? AND deleted_at IS NULL", userStoryID).
			First(&story).Error
		if err == nil {
			var storyStatusIsClosed bool
			err = d.db.Table("user_story_statuses").
				Where("id = ? AND deleted_at IS NULL", story.StatusID).
				Select("is_closed").
				Scan(&storyStatusIsClosed).Error
			if err == nil {
				isClosed = storyStatusIsClosed
			}
		}
	}

	if err := d.db.
		Model(&models.UserStory{}).
		Where("id = ?", userStoryID).
		Update("is_closed", isClosed).Error; err != nil {
		d.logger.Error("Failed to update user story is_closed",
			zap.Error(err),
			zap.String("user_story_id", userStoryID.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update user story closed status",
		}
	}

	return nil
}

func (d *userStoryDatabase) CountStoriesByStatusID(projectID, statusID uuid.UUID) (int64, *response.Error) {
	var count int64
	err := d.db.Model(&models.UserStory{}).
		Where("project_id = ? AND status_id = ?", projectID, statusID).
		Count(&count).Error
	if err != nil {
		d.logger.Error("Failed to count user stories by status ID", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check user stories count by status ID",
		}
	}
	return count, nil
}

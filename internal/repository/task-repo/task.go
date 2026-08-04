package taskrepo

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *taskDatabase) CreateTask(task *models.Task) *response.Error {
	if err := d.db.Create(task).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Task key already exists",
			}
		}
		d.logger.Error("Failed to create task", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create task",
		}
	}
	return nil
}

func (d *taskDatabase) GetTaskByID(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	var task models.Task
	err := d.db.Preload("Sprint").Preload("Assignee").
		Where("id = ? AND project_id = ?", id, projectID).
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Task not found",
			}
		}
		d.logger.Error("Failed to fetch task", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch task",
		}
	}
	return &task, nil
}

func (d *taskDatabase) GetTaskByIDUnscoped(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	var task models.Task
	err := d.db.Unscoped().Preload("Sprint").Preload("Assignee").
		Where("id = ? AND project_id = ?", id, projectID).
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Task not found",
			}
		}
		d.logger.Error("Failed to fetch unscoped task", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch task",
		}
	}
	return &task, nil
}

func (d *taskDatabase) UpdateTask(task *models.Task) *response.Error {
	if err := d.db.Updates(task).Error; err != nil {
		d.logger.Error("Failed to update task", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update task",
		}
	}
	return nil
}

func (d *taskDatabase) DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	if err := d.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&models.Task{}).Error; err != nil {
		d.logger.Error("Failed to delete task", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete task",
		}
	}
	return nil
}

func (d *taskDatabase) RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	err := d.db.Unscoped().Model(&models.Task{}).
		Where("id = ? AND project_id = ?", id, projectID).
		Update("deleted_at", nil).Error
	if err != nil {
		d.logger.Error("Failed to restore task", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to restore task",
		}
	}
	return nil
}

func (d *taskDatabase) GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error) {
	var tasks []models.Task
	var totalItems int64

	// Normalize inputs (defaulting to 10 items/page and created_at DESC)
	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	query := d.db.Model(&models.Task{}).
		Preload("Sprint").
		Preload("Assignee").
		Where("project_id = ?", projectID)

	// ... [apply status, assignee, sprint, type, priority, search, isDeleted filters as usual] ...

	// 1. Get the total count of filtered items
	if err := query.Count(&totalItems).Error; err != nil {
		d.logger.Error("Failed to count tasks", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch tasks",
		}
	}

	// 2. Build the order clause dynamically
	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"title":      "title",
			"created_at": "created_at",
			"updated_at": "updated_at",
			"priority":   "priority",
			"status":     "status",
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
		Find(&tasks).Error; err != nil {
		d.logger.Error("Failed to fetch tasks list", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch tasks",
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

	return tasks, pagination, nil
}

func (d *taskDatabase) GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error) {
	var maxSeq int64
	err := d.db.Model(&models.Task{}).Unscoped().
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(sequence_number), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		d.logger.Error("Failed to fetch next task sequence number", zap.Error(err))
		return 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to generate task key",
		}
	}
	return int(maxSeq) + 1, nil
}

package taskrepo

import (
	"net/http"

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
	if err := d.db.Save(task).Error; err != nil {
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

func (d *taskDatabase) GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, *response.Error) {
	var tasks []models.Task
	query := d.db.Preload("Sprint").Preload("Assignee").Where("project_id = ?", projectID)

	if filter.IsDeleted {
		query = query.Unscoped().Where("deleted_at IS NOT NULL")
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Assignee != "" {
		query = query.Where("assignee_id = ?", filter.Assignee)
	}
	if filter.Sprint != "" {
		query = query.Where("sprint_id = ?", filter.Sprint)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.Search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ? OR key ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := query.Order("created_at DESC").Find(&tasks).Error; err != nil {
		d.logger.Error("Failed to fetch tasks list", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch tasks",
		}
	}
	return tasks, nil
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

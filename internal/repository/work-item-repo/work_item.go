package workitemrepo

import (
	"net/http"

	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *workItemDatabase) GetTaskBySerialNumber(serialNumber int64) (*models.Task, *response.Error) {
	var task models.Task
	err := d.db.Preload("Sprint").Preload("Assignee").Preload("Reporter").Preload("Labels").
		Where("serial_number = ?", serialNumber).
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Task not found",
			}
		}
		d.logger.Error("Failed to fetch task by serial number", zap.Error(err), zap.Int64("serial_number", serialNumber))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch task",
		}
	}
	return &task, nil
}

func (d *workItemDatabase) GetUserStoryBySerialNumber(serialNumber int64) (*models.UserStory, *response.Error) {
	var userStory models.UserStory
	err := d.db.Preload("Sprint").Preload("Assignee").Preload("Reporter").
		Where("serial_number = ?", serialNumber).
		First(&userStory).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "User story not found",
			}
		}
		d.logger.Error("Failed to fetch user story by serial number", zap.Error(err), zap.Int64("serial_number", serialNumber))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story",
		}
	}
	return &userStory, nil
}

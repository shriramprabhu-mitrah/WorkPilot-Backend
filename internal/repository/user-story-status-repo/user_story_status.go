package userstorystatusrepo

import (
	"errors"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *userStoryStatusDatabase) CreateStatus(status *models.UserStoryStatus) *response.Error {
	if err := d.db.Create(status).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Status name already exists in this project",
			}
		}
		d.logger.Error("Failed to create user story status", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create user story status",
		}
	}
	return nil
}

func (d *userStoryStatusDatabase) GetStatusByID(id, projectID uuid.UUID) (*models.UserStoryStatus, *response.Error) {
	var status models.UserStoryStatus
	err := d.db.Where("id = ? AND project_id = ?", id, projectID).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Status not found",
			}
		}
		d.logger.Error("Failed to fetch user story status", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story status",
		}
	}
	return &status, nil
}

func (d *userStoryStatusDatabase) GetStatusByName(projectID uuid.UUID, name string) (*models.UserStoryStatus, *response.Error) {
	var status models.UserStoryStatus
	err := d.db.Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Status not found",
			}
		}
		d.logger.Error("Failed to fetch user story status by name", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story status by name",
		}
	}
	return &status, nil
}

func (d *userStoryStatusDatabase) UpdateStatus(status *models.UserStoryStatus) *response.Error {
	if err := d.db.Save(status).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Status name already exists in this project",
			}
		}
		d.logger.Error("Failed to update user story status", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update user story status",
		}
	}
	return nil
}

func (d *userStoryStatusDatabase) DeleteStatus(id, projectID uuid.UUID) *response.Error {
	result := d.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&models.UserStoryStatus{})
	if result.Error != nil {
		d.logger.Error("Failed to delete user story status", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete user story status",
		}
	}
	if result.RowsAffected == 0 {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Status not found",
		}
	}
	return nil
}

func (d *userStoryStatusDatabase) GetStatusesByProjectID(projectID uuid.UUID) ([]models.UserStoryStatus, *response.Error) {
	var statuses []models.UserStoryStatus
	err := d.db.Where("project_id = ?", projectID).Order("display_order ASC").Find(&statuses).Error
	if err != nil {
		d.logger.Error("Failed to fetch user story statuses", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch user story statuses",
		}
	}
	return statuses, nil
}

func (d *userStoryStatusDatabase) IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	var count int64
	err := d.db.Model(&models.UserStoryStatus{}).Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).Count(&count).Error
	if err != nil {
		d.logger.Error("Failed to check user story status name existence", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check user story status name existence",
		}
	}
	return count > 0, nil
}

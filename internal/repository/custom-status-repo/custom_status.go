package customstatusrepo

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

func (d *customStatusDatabase) CreateStatus(status *models.CustomStatus) *response.Error {
	if err := d.db.Create(status).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Status name already exists in this project",
			}
		}
		d.logger.Error("Failed to create custom status", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create custom status",
		}
	}
	return nil
}

func (d *customStatusDatabase) GetStatusByID(id, projectID uuid.UUID) (*models.CustomStatus, *response.Error) {
	var status models.CustomStatus
	err := d.db.Where("id = ? AND project_id = ?", id, projectID).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Status not found",
			}
		}
		d.logger.Error("Failed to fetch custom status", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch custom status",
		}
	}
	return &status, nil
}

func (d *customStatusDatabase) GetStatusByName(projectID uuid.UUID, name string) (*models.CustomStatus, *response.Error) {
	var status models.CustomStatus
	err := d.db.Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Status not found",
			}
		}
		d.logger.Error("Failed to fetch custom status by name", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch custom status by name",
		}
	}
	return &status, nil
}

func (d *customStatusDatabase) UpdateStatus(status *models.CustomStatus) *response.Error {
	if err := d.db.Save(status).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Status name already exists in this project",
			}
		}
		d.logger.Error("Failed to update custom status", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update custom status",
		}
	}
	return nil
}

func (d *customStatusDatabase) DeleteStatus(id, projectID uuid.UUID) *response.Error {
	result := d.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&models.CustomStatus{})
	if result.Error != nil {
		d.logger.Error("Failed to delete custom status", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete custom status",
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

func (d *customStatusDatabase) GetStatusesByProjectID(projectID uuid.UUID) ([]models.CustomStatus, *response.Error) {
	var statuses []models.CustomStatus
	err := d.db.Where("project_id = ?", projectID).Order("display_order ASC").Find(&statuses).Error
	if err != nil {
		d.logger.Error("Failed to fetch custom statuses", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch custom statuses",
		}
	}
	return statuses, nil
}

func (d *customStatusDatabase) IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	var count int64
	err := d.db.Model(&models.CustomStatus{}).Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).Count(&count).Error
	if err != nil {
		d.logger.Error("Failed to check status name existence", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check status name existence",
		}
	}
	return count > 0, nil
}

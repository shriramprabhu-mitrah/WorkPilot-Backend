package labelrepo

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

func (d *labelDatabase) CreateLabel(label *models.Label) *response.Error {
	if err := d.db.Create(label).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Label name already exists in this project",
			}
		}
		d.logger.Error("Failed to create label", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create label",
		}
	}
	return nil
}

func (d *labelDatabase) GetLabelByID(id, projectID uuid.UUID) (*models.Label, *response.Error) {
	var label models.Label
	err := d.db.Where("id = ? AND project_id = ?", id, projectID).First(&label).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Label not found",
			}
		}
		d.logger.Error("Failed to fetch label", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch label",
		}
	}
	return &label, nil
}

func (d *labelDatabase) UpdateLabel(label *models.Label) *response.Error {
	if err := d.db.Save(label).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Label name already exists in this project",
			}
		}
		d.logger.Error("Failed to update label", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update label",
		}
	}
	return nil
}

func (d *labelDatabase) DeleteLabel(id, projectID uuid.UUID) *response.Error {
	result := d.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&models.Label{})
	if result.Error != nil {
		d.logger.Error("Failed to delete label", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete label",
		}
	}
	if result.RowsAffected == 0 {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Label not found",
		}
	}
	return nil
}

func (d *labelDatabase) GetLabelsByProjectID(projectID uuid.UUID) ([]models.Label, *response.Error) {
	var labels []models.Label
	err := d.db.Where("project_id = ?", projectID).Order("name ASC").Find(&labels).Error
	if err != nil {
		d.logger.Error("Failed to fetch labels", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch labels",
		}
	}
	return labels, nil
}

func (d *labelDatabase) IsLabelNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	var count int64
	err := d.db.Model(&models.Label{}).Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).Count(&count).Error
	if err != nil {
		d.logger.Error("Failed to check label name existence", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check label name existence",
		}
	}
	return count > 0, nil
}

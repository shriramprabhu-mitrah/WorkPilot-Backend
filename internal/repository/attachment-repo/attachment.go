package attachmentrepo

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *attachmentDatabase) CreateAttachment(attachment *models.TaskAttachment) *response.Error {
	if err := d.db.Create(attachment).Error; err != nil {
		d.logger.Error("Failed to create task attachment in database", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save attachment metadata",
		}
	}
	return nil
}

func (d *attachmentDatabase) GetAttachmentByID(id uuid.UUID) (*models.TaskAttachment, *response.Error) {
	var attachment models.TaskAttachment
	if err := d.db.Where("id = ?", id).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Attachment not found",
			}
		}
		d.logger.Error("Failed to query task attachment by ID", zap.Error(err), zap.String("id", id.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve attachment information",
		}
	}
	return &attachment, nil
}

func (d *attachmentDatabase) GetAttachmentsByTaskID(taskID uuid.UUID) ([]models.TaskAttachment, *response.Error) {
	var attachments []models.TaskAttachment
	if err := d.db.Where("task_id = ?", taskID).Order("uploaded_at asc").Find(&attachments).Error; err != nil {
		d.logger.Error("Failed to query task attachments by task ID", zap.Error(err), zap.String("task_id", taskID.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve task attachments",
		}
	}
	return attachments, nil
}

func (d *attachmentDatabase) DeleteAttachment(id uuid.UUID) *response.Error {
	if err := d.db.Where("id = ?", id).Delete(&models.TaskAttachment{}).Error; err != nil {
		d.logger.Error("Failed to delete task attachment from database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

func (d *attachmentDatabase) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	txErr := d.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ?", attachmentID).Delete(&models.TaskAttachment{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		orphan := models.OrphanedFile{
			StoragePath: storagePath,
		}
		return tx.Create(&orphan).Error
	})

	if txErr != nil {
		if txErr == gorm.ErrRecordNotFound {
			return &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Attachment not found",
			}
		}
		d.logger.Error("Failed to transactionally delete attachment and record orphan", zap.Error(txErr))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}




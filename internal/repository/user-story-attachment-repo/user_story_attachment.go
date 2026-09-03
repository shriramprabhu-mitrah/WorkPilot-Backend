package userstoryattachmentrepo

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *userStoryAttachmentDatabase) CreateAttachment(attachment *models.UserStoryAttachment) *response.Error {
	if err := d.db.Create(attachment).Error; err != nil {
		d.logger.Error("Failed to create user story attachment in database", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save attachment metadata",
		}
	}
	return nil
}

func (d *userStoryAttachmentDatabase) GetAttachmentByID(id uuid.UUID) (*models.UserStoryAttachment, *response.Error) {
	var attachment models.UserStoryAttachment
	if err := d.db.Where("id = ?", id).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Attachment not found",
			}
		}
		d.logger.Error("Failed to query user story attachment by ID", zap.Error(err), zap.String("id", id.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve attachment information",
		}
	}
	return &attachment, nil
}

func (d *userStoryAttachmentDatabase) GetAttachmentsByUserStoryID(userStoryID uuid.UUID) ([]models.UserStoryAttachment, *response.Error) {
	var attachments []models.UserStoryAttachment
	if err := d.db.Where("user_story_id = ?", userStoryID).Order("uploaded_at asc").Find(&attachments).Error; err != nil {
		d.logger.Error("Failed to query user story attachments by user story ID", zap.Error(err), zap.String("user_story_id", userStoryID.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve user story attachments",
		}
	}
	return attachments, nil
}

func (d *userStoryAttachmentDatabase) DeleteAttachment(id uuid.UUID) *response.Error {
	if err := d.db.Where("id = ?", id).Delete(&models.UserStoryAttachment{}).Error; err != nil {
		d.logger.Error("Failed to delete user story attachment from database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

func (d *userStoryAttachmentDatabase) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	txErr := d.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ?", attachmentID).Delete(&models.UserStoryAttachment{})
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
		d.logger.Error("Failed to transactionally delete user story attachment and record orphan", zap.Error(txErr))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

func (d *userStoryAttachmentDatabase) UpdateAttachmentsUserStoryID(attachmentIDs []uuid.UUID, userStoryID uuid.UUID) *response.Error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	if err := d.db.Model(&models.UserStoryAttachment{}).Where("id IN ?", attachmentIDs).Update("user_story_id", userStoryID).Error; err != nil {
		d.logger.Error("Failed to update user story ID for attachments", zap.Error(err), zap.Any("attachment_ids", attachmentIDs), zap.String("user_story_id", userStoryID.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to link attachments to user story",
		}
	}
	return nil
}

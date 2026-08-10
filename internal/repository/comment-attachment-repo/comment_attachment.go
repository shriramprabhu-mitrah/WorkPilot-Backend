package commentattachmentrepo

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *commentAttachmentDatabase) CreateAttachment(attachment *models.CommentAttachment) *response.Error {
	if err := d.db.Create(attachment).Error; err != nil {
		d.logger.Error("Failed to create comment attachment in database", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save attachment metadata",
		}
	}
	return nil
}

func (d *commentAttachmentDatabase) GetAttachmentByID(id uuid.UUID) (*models.CommentAttachment, *response.Error) {
	var attachment models.CommentAttachment
	if err := d.db.Where("id = ?", id).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Attachment not found",
			}
		}
		d.logger.Error("Failed to query comment attachment by ID", zap.Error(err), zap.String("id", id.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve attachment information",
		}
	}
	return &attachment, nil
}

func (d *commentAttachmentDatabase) GetAttachmentsByCommentID(commentID uuid.UUID) ([]models.CommentAttachment, *response.Error) {
	var attachments []models.CommentAttachment
	if err := d.db.Where("comment_id = ?", commentID).Order("uploaded_at asc").Find(&attachments).Error; err != nil {
		d.logger.Error("Failed to query comment attachments by comment ID", zap.Error(err), zap.String("comment_id", commentID.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve comment attachments",
		}
	}
	return attachments, nil
}

func (d *commentAttachmentDatabase) DeleteAttachment(id uuid.UUID) *response.Error {
	if err := d.db.Where("id = ?", id).Delete(&models.CommentAttachment{}).Error; err != nil {
		d.logger.Error("Failed to delete comment attachment from database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

func (d *commentAttachmentDatabase) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	txErr := d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", attachmentID).Delete(&models.CommentAttachment{}).Error; err != nil {
			return err
		}

		orphan := models.OrphanedFile{
			StoragePath: storagePath,
		}
		if err := tx.Create(&orphan).Error; err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		d.logger.Error("Failed to transactionally delete comment attachment and record orphan", zap.Error(txErr))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

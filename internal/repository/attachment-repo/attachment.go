package attachmentrepo

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		if err := tx.Where("id = ?", attachmentID).Delete(&models.TaskAttachment{}).Error; err != nil {
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
		d.logger.Error("Failed to transactionally delete attachment and record orphan", zap.Error(txErr))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete attachment metadata",
		}
	}
	return nil
}

func (d *attachmentDatabase) CreateOrphanedFile(file *models.OrphanedFile) *response.Error {
	if err := d.db.Create(file).Error; err != nil {
		d.logger.Error("Failed to record orphaned file in database", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to record cleanup task",
		}
	}
	return nil
}

func (d *attachmentDatabase) GetOrphanedFiles() ([]models.OrphanedFile, *response.Error) {
	var files []models.OrphanedFile
	if err := d.db.Order("created_at asc").Find(&files).Error; err != nil {
		d.logger.Error("Failed to fetch orphaned files from database", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch cleanup tasks",
		}
	}
	return files, nil
}

func (d *attachmentDatabase) DeleteOrphanedFile(id uuid.UUID) *response.Error {
	if err := d.db.Where("id = ?", id).Delete(&models.OrphanedFile{}).Error; err != nil {
		d.logger.Error("Failed to delete orphaned file task from database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to complete cleanup task",
		}
	}
	return nil
}

func (d *attachmentDatabase) ClaimOrphanedFiles(now time.Time, claimedUntil time.Time, limit int) ([]models.OrphanedFile, *response.Error) {
	var claimed []models.OrphanedFile

	txErr := d.db.Transaction(func(tx *gorm.DB) error {
		var temp []models.OrphanedFile
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(claimed_until IS NULL OR claimed_until < ?) AND attempts < ?", now, 5).
			Order("created_at asc").
			Limit(limit).
			Find(&temp).Error
		if err != nil {
			return err
		}

		if len(temp) == 0 {
			return nil
		}

		var ids []uuid.UUID
		for _, f := range temp {
			ids = append(ids, f.ID)
		}

		err = tx.Model(&models.OrphanedFile{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"claimed_until":    claimedUntil,
				"attempts":         gorm.Expr("attempts + 1"),
				"last_attempt_at":  now,
			}).Error
		if err != nil {
			return err
		}

		claimed = temp
		return nil
	})

	if txErr != nil {
		d.logger.Error("Failed to claim orphaned files from database", zap.Error(txErr))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to claim cleanup tasks",
		}
	}

	return claimed, nil
}

func (d *attachmentDatabase) ReleaseOrphanedFile(id uuid.UUID, lastErr string, lastAttempt time.Time) *response.Error {
	err := d.db.Model(&models.OrphanedFile{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"claimed_until":   nil,
			"last_error":      lastErr,
			"last_attempt_at": lastAttempt,
		}).Error

	if err != nil {
		d.logger.Error("Failed to release orphaned file in database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update cleanup task status",
		}
	}
	return nil
}

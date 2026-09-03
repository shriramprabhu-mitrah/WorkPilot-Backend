package attachmentrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AttachmentRepository interface {
	CreateAttachment(attachment *models.TaskAttachment) *response.Error
	GetAttachmentByID(id uuid.UUID) (*models.TaskAttachment, *response.Error)
	GetAttachmentsByTaskID(taskID uuid.UUID) ([]models.TaskAttachment, *response.Error)
	DeleteAttachment(id uuid.UUID) *response.Error

	// Transactional outbox pattern
	DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error
	UpdateAttachmentsTaskID(attachmentIDs []uuid.UUID, taskID uuid.UUID) *response.Error
}

type attachmentDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitAttachmentRepository(deps models.Config) AttachmentRepository {
	return &attachmentDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

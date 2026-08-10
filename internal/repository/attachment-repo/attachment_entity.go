package attachmentrepo

import (
	"time"

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
	CreateOrphanedFile(file *models.OrphanedFile) *response.Error
	GetOrphanedFiles() ([]models.OrphanedFile, *response.Error)
	DeleteOrphanedFile(id uuid.UUID) *response.Error
	ClaimOrphanedFiles(now time.Time, claimedUntil time.Time, limit int) ([]models.OrphanedFile, *response.Error)
	ReleaseOrphanedFile(id uuid.UUID, lastErr string, lastAttempt time.Time) *response.Error
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

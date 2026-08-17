package userstoryattachmentrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserStoryAttachmentRepository interface {
	CreateAttachment(attachment *models.UserStoryAttachment) *response.Error
	GetAttachmentByID(id uuid.UUID) (*models.UserStoryAttachment, *response.Error)
	GetAttachmentsByUserStoryID(userStoryID uuid.UUID) ([]models.UserStoryAttachment, *response.Error)
	DeleteAttachment(id uuid.UUID) *response.Error

	// Transactional outbox pattern
	DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error
}

type userStoryAttachmentDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitUserStoryAttachmentRepository(deps models.Config) UserStoryAttachmentRepository {
	return &userStoryAttachmentDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

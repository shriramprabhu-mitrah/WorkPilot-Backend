package commentattachmentrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CommentAttachmentRepository interface {
	CreateAttachment(attachment *models.CommentAttachment) *response.Error
	GetAttachmentByID(id uuid.UUID) (*models.CommentAttachment, *response.Error)
	GetAttachmentsByCommentID(commentID uuid.UUID) ([]models.CommentAttachment, *response.Error)
	DeleteAttachment(id uuid.UUID) *response.Error
}

type commentAttachmentDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitCommentAttachmentRepository(deps models.Config) CommentAttachmentRepository {
	return &commentAttachmentDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

package commentsrepo

import (
	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CommentsRepository interface {
	CreateComment(comment models.Comments) *response.Error
	GetCommentByID(commentID uuid.UUID) (*models.Comments, *response.Error)
	GetCommentsByTaskID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error)
	UpdateComment(commentID uuid.UUID, req models.Comments) *response.Error
	DeleteComment(commentID uuid.UUID) *response.Error
	GetCommentsByParentID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error)
	HasReplies(commentID uuid.UUID) (bool, *response.Error)
	MarkCommentAsDeleted(commentID uuid.UUID) *response.Error
}

type commentsDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitCommentsRepository(deps models.Config) CommentsRepository {
	return &commentsDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

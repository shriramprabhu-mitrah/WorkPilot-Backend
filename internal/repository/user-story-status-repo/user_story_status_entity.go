package userstorystatusrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserStoryStatusRepository interface {
	CreateStatus(status *models.UserStoryStatus) *response.Error
	GetStatusByID(id, projectID uuid.UUID) (*models.UserStoryStatus, *response.Error)
	GetStatusByName(projectID uuid.UUID, name string) (*models.UserStoryStatus, *response.Error)
	UpdateStatus(status *models.UserStoryStatus) *response.Error
	DeleteStatus(id, projectID uuid.UUID) *response.Error
	GetStatusesByProjectID(projectID uuid.UUID) ([]models.UserStoryStatus, *response.Error)
	IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error)
}

func InitUserStoryStatusRepository(deps models.Config) UserStoryStatusRepository {
	return &userStoryStatusDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type userStoryStatusDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

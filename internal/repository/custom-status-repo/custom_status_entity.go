package customstatusrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CustomStatusRepository interface {
	CreateStatus(status *models.CustomStatus) *response.Error
	GetStatusByID(id, projectID uuid.UUID) (*models.CustomStatus, *response.Error)
	GetStatusByName(projectID uuid.UUID, name string) (*models.CustomStatus, *response.Error)
	UpdateStatus(status *models.CustomStatus) *response.Error
	DeleteStatus(id, projectID uuid.UUID) *response.Error
	GetStatusesByProjectID(projectID uuid.UUID) ([]models.CustomStatus, *response.Error)
	IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error)
}

func InitCustomStatusRepository(deps models.Config) CustomStatusRepository {
	return &customStatusDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type customStatusDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

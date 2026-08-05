package labelrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type LabelRepository interface {
	CreateLabel(label *models.Label) *response.Error
	GetLabelByID(id, projectID uuid.UUID) (*models.Label, *response.Error)
	UpdateLabel(label *models.Label) *response.Error
	DeleteLabel(id, projectID uuid.UUID) *response.Error
	GetLabelsByProjectID(projectID uuid.UUID) ([]models.Label, *response.Error)
	IsLabelNameExists(projectID uuid.UUID, name string) (bool, *response.Error)
}

func InitLabelRepository(deps models.Config) LabelRepository {
	return &labelDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type labelDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

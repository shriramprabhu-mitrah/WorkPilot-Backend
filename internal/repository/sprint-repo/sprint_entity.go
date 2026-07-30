package sprintrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SprintRepository interface {
	CreateSprint(row models.Sprint) *response.Error
	UpdateSprint(projectID, sprintID uuid.UUID, req models.Sprint) *response.Error
	DeleteSprint(id uuid.UUID) *response.Error
	GetSprintByID(sprintID, projectID uuid.UUID) (*models.Sprint, *response.Error)
	GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error)
}

func InitSprintRepository(deps models.Config) SprintRepository {
	return &sprintDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type sprintDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

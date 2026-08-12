package sprintrepo

import (
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SprintRepository interface {
	CreateSprint(row models.Sprint) *response.Error
	UpdateSprint(projectID, sprintID uuid.UUID, updates map[string]interface{}) *response.Error
	DeleteSprint(id uuid.UUID) *response.Error
	GetSprintByID(sprintID, projectID uuid.UUID) (*models.Sprint, *response.Error)
	GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error)
	IsSprintExists(projectID uuid.UUID, name string) (bool, *response.Error)
	IsSprintDateRangeExists(projectID uuid.UUID, startDate, endDate time.Time, excludeSprintID uuid.UUID) (bool, *response.Error)
	CreateSprintSnapshot(snapshot models.SprintSnapshot) *response.Error
	GetSprintSnapshots(sprintID uuid.UUID) ([]models.SprintSnapshot, *response.Error)
	GetTotalStoryPoints(sprintID uuid.UUID) (int, *response.Error)
	GetRemainingStoryPoints(sprintID uuid.UUID) (int, *response.Error)
	GetActiveSprints() ([]models.Sprint, *response.Error)
	GetCompletedTasksStoryPoints(sprintID uuid.UUID) (int, *response.Error)
	MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error
	GetSprintCountByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int, *response.Error)
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

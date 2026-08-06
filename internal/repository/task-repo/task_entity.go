package taskrepo

import (
	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TaskRepository interface {
	CreateTask(task *models.Task) *response.Error
	GetTaskByID(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error)
	GetTaskByIDUnscoped(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error)
	UpdateTask(task *models.Task) *response.Error
	DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error
	RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error
	GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error)
	GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error)
	GetTaskDetailsByID(id uuid.UUID) (*models.Task, *response.Error)
}

func InitTaskRepository(deps models.Config) TaskRepository {
	return &taskDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type taskDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

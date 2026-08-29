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
	GetTaskByKey(key string, projectID uuid.UUID) (*models.Task, *response.Error)
	GetTaskByIDUnscoped(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error)
	UpdateTask(taskID uuid.UUID, updates map[string]interface{}) *response.Error
	DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error
	RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error
	GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error)
	GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error)
	IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error)
	IsUserStoryInProject(userStoryID, projectID uuid.UUID) (bool, *response.Error)
	VerifyLabelIDs(projectID uuid.UUID, labelIDs []uuid.UUID) ([]models.Label, *response.Error)
	UpdateTaskLabels(taskID uuid.UUID, labels []models.Label) *response.Error
	UpdateTaskWithLabels(taskID uuid.UUID, updates map[string]interface{}, labels []models.Label) *response.Error
	AttachLabel(taskID uuid.UUID, label *models.Label) *response.Error
	RemoveLabel(taskID uuid.UUID, label *models.Label) *response.Error
	MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error
	GetSprintStatus(sprintID uuid.UUID) (string, *response.Error)
	GetTaskDetailsByID(id uuid.UUID) (*models.Task, *response.Error)
	GetTaskDetailsByIDOrKey(idOrKey string) (*models.Task, *response.Error)
	GetTaskAccessContext(id uuid.UUID) (*models.TaskAccessContext, *response.Error)
	GetTaskAccessContextByIDOrKey(idOrKey string) (*models.TaskAccessContext, *response.Error)
	GetTasksByUserStoryID(userStoryID uuid.UUID) ([]models.Task, *response.Error)
	CountTasksByStatus(projectID uuid.UUID, status string) (int64, *response.Error)
	UpdateTaskStatusName(projectID uuid.UUID, oldStatus, newStatus string) *response.Error
	GetTaskCountsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error)
	UpdateAttachmentsTaskID(attachmentIDs []uuid.UUID, taskID uuid.UUID) *response.Error
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

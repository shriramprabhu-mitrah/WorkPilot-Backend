package userstoryrepo

import (
	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserStoryRepository interface {
	CreateUserStory(userStory *models.UserStory) *response.Error
	GetUserStoryByID(id uuid.UUID, projectID uuid.UUID) (*models.UserStory, *response.Error)
	UpdateUserStory(userStoryID uuid.UUID, updates map[string]interface{}) *response.Error
	DeleteUserStory(id uuid.UUID, projectID uuid.UUID) *response.Error
	GetUserStories(projectID uuid.UUID, filter dto.UserStoryFilter) ([]models.UserStory, response.Pagination, *response.Error)
	IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error)
	GetMaxBacklogOrder(projectID uuid.UUID) (int, *response.Error)
	ReorderUserStories(projectID uuid.UUID, storyIDs []uuid.UUID) *response.Error
	GetStoryTaskStats(projectID uuid.UUID) (map[uuid.UUID]models.StoryTaskStats, *response.Error)
	GetUserStoryAccessContext(id uuid.UUID) (*models.UserStoryAccessContext, *response.Error)
	RecalculateUserStoryIsClosed(userStoryID uuid.UUID) *response.Error
}

func InitUserStoryRepository(deps models.Config) UserStoryRepository {
	return &userStoryDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type userStoryDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

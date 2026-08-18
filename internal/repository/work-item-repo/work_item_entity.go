package workitemrepo

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WorkItemRepository interface {
	GetTaskBySerialNumber(serialNumber int64) (*models.Task, *response.Error)
	GetUserStoryBySerialNumber(serialNumber int64) (*models.UserStory, *response.Error)
}

func InitWorkItemRepository(deps models.Config) WorkItemRepository {
	return &workItemDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type workItemDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

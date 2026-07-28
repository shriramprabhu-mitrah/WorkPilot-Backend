package projectrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProjectRepository interface {
	CreateProject(row models.Project) *response.Error
	UpdateProject(projectID uuid.UUID, req models.Project) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error)
	GetProjectByID(id uuid.UUID) (models.Project, *response.Error)
}

func InitProjectRepository(deps models.Config) ProjectRepository {
	return &projectDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type projectDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

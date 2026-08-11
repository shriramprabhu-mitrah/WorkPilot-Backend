package projectrepo

import (
	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProjectRepository interface {
	CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error
	UpdateProject(projectID uuid.UUID, updates map[string]interface{}) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error)
	GetProjectByID(id uuid.UUID) (models.Project, *response.Error)
	CreateProjectMember(row models.ProjectMember) *response.Error
	GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error)
	RemoveProjectMember(projectID, userID uuid.UUID) *response.Error
	GetProjectActivity(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error)
	IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error)
	DeleteProject(projectID, organizationID uuid.UUID) *response.Error
	GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error)
	GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error)
	UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error
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

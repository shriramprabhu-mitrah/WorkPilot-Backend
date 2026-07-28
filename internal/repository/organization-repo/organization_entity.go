package organizationrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OrganizationRepository interface {
	CreateOrganization(row models.Organization) *response.Error
	GetByName(name string) (models.Organization, *response.Error)
	GetByID(id uuid.UUID) (models.Organization, *response.Error)
	UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error
	DeleteOrganization(id uuid.UUID) *response.Error
	UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error
	CreateOrganizationInvitation(invitation models.OrganizationInvitation) *response.Error
	GetPendingInvitationByEmail(orgID uuid.UUID, email string) (models.OrganizationInvitation, *response.Error)
	GetInvitationByToken(token string) (models.OrganizationInvitation, *response.Error)
	UpdateInvitation(invitation models.OrganizationInvitation) *response.Error
	CreateAuditLog(log models.AuditLog) *response.Error
	GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error)
	DeleteUser(id uuid.UUID) *response.Error
}

func InitOrganizationRepository(deps models.Config) OrganizationRepository {
	return &organizationDatabase{
		DB:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type organizationDatabase struct {
	DB          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

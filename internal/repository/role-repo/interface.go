package rolerepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type RoleRepository interface {
	CreateRole(role *models.Role, permissions []models.Permission) *response.Error
	GetRolesByOrganizationID(orgID uuid.UUID) ([]models.Role, *response.Error)
	GetRoleByID(roleID uuid.UUID) (*models.Role, *response.Error)
	UpdateRole(role *models.Role, permissions []models.Permission) *response.Error
	DeleteRole(roleID uuid.UUID) *response.Error
	GetPermissionByResourceAction(resource, action string) (*models.Permission, *response.Error)
	IsRoleAssigned(roleID uuid.UUID) (bool, *response.Error)
}

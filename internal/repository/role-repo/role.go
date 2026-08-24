package rolerepo

import (
	"errors"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type roleDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitRoleRepository(deps models.Config) RoleRepository {
	return &roleDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

func (d *roleDatabase) CreateRole(role *models.Role, permissions []models.Permission) *response.Error {
	tx := d.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(role).Error; err != nil {
		tx.Rollback()
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "A role with this name already exists in the organization",
			}
		}
		d.logger.Error("Failed to create role", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create role",
		}
	}

	if len(permissions) > 0 {
		if err := tx.Model(role).Association("Permissions").Replace(permissions); err != nil {
			tx.Rollback()
			d.logger.Error("Failed to associate permissions", zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to associate permissions with role",
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		d.logger.Error("Failed to commit role creation", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to commit database transaction",
		}
	}

	return nil
}

func (d *roleDatabase) GetRolesByOrganizationID(orgID uuid.UUID) ([]models.Role, *response.Error) {
	var roles []models.Role
	err := d.db.Preload("Permissions").
		Where("organization_id = ? OR (organization_id IS NULL AND is_system = true)", orgID).
		Find(&roles).Error

	if err != nil {
		d.logger.Error("Failed to fetch roles", zap.Error(err), zap.String("org_id", orgID.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch roles",
		}
	}

	return roles, nil
}

func (d *roleDatabase) GetRoleByID(roleID uuid.UUID) (*models.Role, *response.Error) {
	var role models.Role
	err := d.db.Preload("Permissions").Where("id = ?", roleID).First(&role).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Role not found",
			}
		}
		d.logger.Error("Failed to fetch role by id", zap.Error(err), zap.String("role_id", roleID.String()))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch role details",
		}
	}

	return &role, nil
}

func (d *roleDatabase) UpdateRole(role *models.Role, permissions []models.Permission) *response.Error {
	tx := d.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(role).Error; err != nil {
		tx.Rollback()
		if utils.IsDuplicateKeyError(err) {
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "A role with this name already exists in the organization",
			}
		}
		d.logger.Error("Failed to update role", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update role",
		}
	}

	if err := tx.Model(role).Association("Permissions").Replace(permissions); err != nil {
		tx.Rollback()
		d.logger.Error("Failed to update permissions associations", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update role permissions",
		}
	}

	if err := tx.Commit().Error; err != nil {
		d.logger.Error("Failed to commit role update", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to commit database transaction",
		}
	}

	return nil
}

func (d *roleDatabase) DeleteRole(roleID uuid.UUID) *response.Error {
	err := d.db.Delete(&models.Role{}, "id = ?", roleID).Error
	if err != nil {
		d.logger.Error("Failed to delete role", zap.Error(err), zap.String("role_id", roleID.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete role",
		}
	}
	return nil
}

func (d *roleDatabase) GetPermissionByResourceAction(resource, action string) (*models.Permission, *response.Error) {
	var perm models.Permission
	err := d.db.Where("resource = ? AND action = ?", resource, action).First(&perm).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Permission not found",
			}
		}
		d.logger.Error("Failed to fetch permission", zap.Error(err), zap.String("resource", resource), zap.String("action", action))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch permission",
		}
	}

	return &perm, nil
}

func (d *roleDatabase) IsRoleAssigned(roleID uuid.UUID) (bool, *response.Error) {
	var userCount int64
	var memberCount int64
	var inviteCount int64

	if err := d.db.Model(&models.User{}).Where("role_id = ?", roleID).Count(&userCount).Error; err != nil {
		d.logger.Error("Failed to count users for role check", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Database error during assignment validation",
		}
	}

	if err := d.db.Model(&models.ProjectMember{}).Where("role_id = ?", roleID).Count(&memberCount).Error; err != nil {
		d.logger.Error("Failed to count project members for role check", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Database error during assignment validation",
		}
	}

	if err := d.db.Model(&models.OrganizationInvitation{}).Where("role_id = ?", roleID).Count(&inviteCount).Error; err != nil {
		d.logger.Error("Failed to count invitations for role check", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Database error during assignment validation",
		}
	}

	return (userCount > 0 || memberCount > 0 || inviteCount > 0), nil
}

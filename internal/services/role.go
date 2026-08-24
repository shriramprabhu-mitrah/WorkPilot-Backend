package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	rolerepo "github.com/ms-kanban-server/internal/repository/role-repo"
	"go.uber.org/zap"
)

type RoleService interface {
	CreateRole(orgID uuid.UUID, req requestdto.CreateRoleRequest) (*responsedto.RoleResponse, *response.Error)
	GetRolesByOrganizationID(orgID uuid.UUID) ([]responsedto.RoleResponse, *response.Error)
	GetRoleByID(orgID, roleID uuid.UUID) (*responsedto.RoleResponse, *response.Error)
	UpdateRole(orgID, roleID uuid.UUID, req requestdto.UpdateRoleRequest) (*responsedto.RoleResponse, *response.Error)
	DeleteRole(orgID, roleID uuid.UUID) *response.Error
}

type roleService struct {
	roleRepo rolerepo.RoleRepository
	logger   *zap.Logger
}

func InitRoleService(roleRepo rolerepo.RoleRepository, logger *zap.Logger) RoleService {
	return &roleService{
		roleRepo: roleRepo,
		logger:   logger,
	}
}

func (s *roleService) CreateRole(orgID uuid.UUID, req requestdto.CreateRoleRequest) (*responsedto.RoleResponse, *response.Error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Role name is required",
		}
	}

	var permissionsToAssociate []models.Permission
	for resource, actionMap := range req.Permissions {
		for action, enabled := range actionMap {
			if !enabled {
				continue
			}
			perm, err := s.roleRepo.GetPermissionByResourceAction(resource, action)
			if err != nil {
				return nil, &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusBadRequest,
					Message:    fmt.Sprintf("Invalid permission: %s for resource %s", action, resource),
				}
			}
			permissionsToAssociate = append(permissionsToAssociate, *perm)
		}
	}

	role := &models.Role{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: &orgID,
		Name:           name,
		Description:    req.Description,
		IsSystem:       false,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.roleRepo.CreateRole(role, permissionsToAssociate); err != nil {
		return nil, err
	}

	// Fetch to load fully with associations
	savedRole, err := s.roleRepo.GetRoleByID(role.ID)
	if err != nil {
		return nil, err
	}

	resp := responsedto.MapToRoleResponse(*savedRole)
	return &resp, nil
}

func (s *roleService) GetRolesByOrganizationID(orgID uuid.UUID) ([]responsedto.RoleResponse, *response.Error) {
	roles, err := s.roleRepo.GetRolesByOrganizationID(orgID)
	if err != nil {
		return nil, err
	}

	var resp []responsedto.RoleResponse
	for _, role := range roles {
		resp = append(resp, responsedto.MapToRoleResponse(role))
	}
	return resp, nil
}

func (s *roleService) GetRoleByID(orgID, roleID uuid.UUID) (*responsedto.RoleResponse, *response.Error) {
	role, err := s.roleRepo.GetRoleByID(roleID)
	if err != nil {
		return nil, err
	}

	// Validate role belongs to the organization (or is a global system role)
	if role.OrganizationID != nil && *role.OrganizationID != orgID {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have access to this role",
		}
	}

	resp := responsedto.MapToRoleResponse(*role)
	return &resp, nil
}

func (s *roleService) UpdateRole(orgID, roleID uuid.UUID, req requestdto.UpdateRoleRequest) (*responsedto.RoleResponse, *response.Error) {
	role, err := s.roleRepo.GetRoleByID(roleID)
	if err != nil {
		return nil, err
	}

	if role.OrganizationID == nil || *role.OrganizationID != orgID {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to modify this role",
		}
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Role name cannot be empty",
			}
		}
		role.Name = name
	}

	if req.Description != nil {
		role.Description = *req.Description
	}

	var permissionsToAssociate []models.Permission
	if req.Permissions != nil {
		for resource, actionMap := range req.Permissions {
			for action, enabled := range actionMap {
				if !enabled {
					continue
				}
				perm, err := s.roleRepo.GetPermissionByResourceAction(resource, action)
				if err != nil {
					return nil, &response.Error{
						Code:       response.ErrValidation,
						StatusCode: http.StatusBadRequest,
						Message:    fmt.Sprintf("Invalid permission: %s for resource %s", action, resource),
					}
				}
				permissionsToAssociate = append(permissionsToAssociate, *perm)
			}
		}
	} else {
		permissionsToAssociate = role.Permissions
	}

	role.UpdatedAt = time.Now()

	if err := s.roleRepo.UpdateRole(role, permissionsToAssociate); err != nil {
		return nil, err
	}

	// Re-fetch to get fresh state with preloaded associations
	updatedRole, err := s.roleRepo.GetRoleByID(role.ID)
	if err != nil {
		return nil, err
	}

	resp := responsedto.MapToRoleResponse(*updatedRole)
	return &resp, nil
}

func (s *roleService) DeleteRole(orgID, roleID uuid.UUID) *response.Error {
	role, err := s.roleRepo.GetRoleByID(roleID)
	if err != nil {
		return err
	}

	if role.IsSystem {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "System roles cannot be deleted",
		}
	}

	if role.OrganizationID == nil || *role.OrganizationID != orgID {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete this role",
		}
	}

	assigned, err := s.roleRepo.IsRoleAssigned(roleID)
	if err != nil {
		return err
	}
	if assigned {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Role cannot be deleted because it is currently assigned to users, project members, or invitations",
		}
	}

	return s.roleRepo.DeleteRole(roleID)
}

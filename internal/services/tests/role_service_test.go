package services_test

import (
	"net/http"
	"testing"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubRoleRepo struct {
	roles             map[uuid.UUID]*models.Role
	permissions       map[string]map[string]*models.Permission
	isAssigned        bool
	createRoleErr     *response.Error
	getRolesErr       *response.Error
	getRoleByIDErr    *response.Error
	updateRoleErr     *response.Error
	deleteRoleErr     *response.Error
	getPermissionErr  *response.Error
	isRoleAssignedErr *response.Error
}

func (s *stubRoleRepo) CreateRole(role *models.Role, permissions []models.Permission) *response.Error {
	if s.createRoleErr != nil {
		return s.createRoleErr
	}
	role.Permissions = permissions
	s.roles[role.ID] = role
	return nil
}

func (s *stubRoleRepo) GetRolesByOrganizationID(orgID uuid.UUID) ([]models.Role, *response.Error) {
	if s.getRolesErr != nil {
		return nil, s.getRolesErr
	}
	var res []models.Role
	for _, r := range s.roles {
		if (r.OrganizationID != nil && *r.OrganizationID == orgID) || (r.OrganizationID == nil && r.IsSystem) {
			res = append(res, *r)
		}
	}
	return res, nil
}

func (s *stubRoleRepo) GetRoleByID(roleID uuid.UUID) (*models.Role, *response.Error) {
	if s.getRoleByIDErr != nil {
		return nil, s.getRoleByIDErr
	}
	r, ok := s.roles[roleID]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Role not found"}
	}
	return r, nil
}

func (s *stubRoleRepo) UpdateRole(role *models.Role, permissions []models.Permission) *response.Error {
	if s.updateRoleErr != nil {
		return s.updateRoleErr
	}
	role.Permissions = permissions
	s.roles[role.ID] = role
	return nil
}

func (s *stubRoleRepo) DeleteRole(roleID uuid.UUID) *response.Error {
	if s.deleteRoleErr != nil {
		return s.deleteRoleErr
	}
	delete(s.roles, roleID)
	return nil
}

func (s *stubRoleRepo) GetPermissionByResourceAction(resource, action string) (*models.Permission, *response.Error) {
	if s.getPermissionErr != nil {
		return nil, s.getPermissionErr
	}
	resMap, ok := s.permissions[resource]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Permission not found"}
	}
	perm, ok := resMap[action]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Permission not found"}
	}
	return perm, nil
}

func (s *stubRoleRepo) IsRoleAssigned(roleID uuid.UUID) (bool, *response.Error) {
	if s.isRoleAssignedErr != nil {
		return false, s.isRoleAssignedErr
	}
	return s.isAssigned, nil
}

func TestRoleService_CreateRole_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	roleRepo := &stubRoleRepo{
		roles: make(map[uuid.UUID]*models.Role),
		permissions: map[string]map[string]*models.Permission{
			"projects": {
				"view": {ID: uuid.Must(uuid.NewV4()), Resource: "projects", Action: "view"},
				"add":  {ID: uuid.Must(uuid.NewV4()), Resource: "projects", Action: "add"},
			},
		},
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	req := requestdto.CreateRoleRequest{
		Name:        "Custom Manager",
		Description: "Manager role with projects access",
		Permissions: map[string]map[string]bool{
			"projects": {
				"view": true,
				"add":  true,
				"edit": false,
			},
		},
	}

	resp, err := service.CreateRole(orgID, req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Name != "Custom Manager" {
		t.Errorf("expected role name 'Custom Manager', got: %s", resp.Name)
	}
	if !resp.Permissions["projects"]["view"] || !resp.Permissions["projects"]["add"] {
		t.Errorf("expected projects view/add permissions to be true")
	}
	if resp.Permissions["projects"]["delete"] {
		t.Errorf("expected projects delete permission to be false")
	}
}

func TestRoleService_CreateRole_ValidationError(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	roleRepo := &stubRoleRepo{
		roles: make(map[uuid.UUID]*models.Role),
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	// Empty Name
	req := requestdto.CreateRoleRequest{
		Name: "   ",
	}
	_, err := service.CreateRole(orgID, req)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty name, got: %v", err)
	}

	// Invalid permission
	req = requestdto.CreateRoleRequest{
		Name: "Valid Name",
		Permissions: map[string]map[string]bool{
			"projects": {
				"non_existent_action": true,
			},
		},
	}
	_, err = service.CreateRole(orgID, req)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid permission, got: %v", err)
	}
}

func TestRoleService_GetRolesByOrganizationID(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	otherOrgID := uuid.Must(uuid.NewV4())
	roleRepo := &stubRoleRepo{
		roles: map[uuid.UUID]*models.Role{
			uuid.Must(uuid.NewV4()): {Name: "System Role", IsSystem: true},
			uuid.Must(uuid.NewV4()): {Name: "Org Role", OrganizationID: &orgID, IsSystem: false},
			uuid.Must(uuid.NewV4()): {Name: "Other Org Role", OrganizationID: &otherOrgID, IsSystem: false},
		},
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	roles, err := service.GetRolesByOrganizationID(orgID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles (system + org role), got %d", len(roles))
	}
}

func TestRoleService_GetRoleByID(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	otherOrgID := uuid.Must(uuid.NewV4())
	roleID := uuid.Must(uuid.NewV4())
	otherRoleID := uuid.Must(uuid.NewV4())

	roleRepo := &stubRoleRepo{
		roles: map[uuid.UUID]*models.Role{
			roleID:      {ID: roleID, Name: "My Role", OrganizationID: &orgID},
			otherRoleID: {ID: otherRoleID, Name: "Other Role", OrganizationID: &otherOrgID},
		},
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	// Success
	resp, err := service.GetRoleByID(orgID, roleID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Name != "My Role" {
		t.Errorf("expected role name 'My Role', got: %s", resp.Name)
	}

	// Org mismatch forbidden
	_, err = service.GetRoleByID(orgID, otherRoleID)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got: %v", err)
	}
}

func TestRoleService_UpdateRole(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	roleID := uuid.Must(uuid.NewV4())
	systemRoleID := uuid.Must(uuid.NewV4())

	roleRepo := &stubRoleRepo{
		roles: map[uuid.UUID]*models.Role{
			roleID:       {ID: roleID, Name: "Org Role", OrganizationID: &orgID, IsSystem: false},
			systemRoleID: {ID: systemRoleID, Name: "System Role", IsSystem: true},
		},
		permissions: map[string]map[string]*models.Permission{
			"projects": {
				"view": {ID: uuid.Must(uuid.NewV4()), Resource: "projects", Action: "view"},
			},
		},
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	// Success Update Name and description
	newName := "Updated Name"
	newDesc := "Updated Desc"
	req := requestdto.UpdateRoleRequest{
		Name:        &newName,
		Description: &newDesc,
	}
	resp, err := service.UpdateRole(orgID, roleID, req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Name != "Updated Name" || resp.Description != "Updated Desc" {
		t.Errorf("update failed to apply values")
	}

	// Validation - Empty Name
	emptyName := "  "
	req = requestdto.UpdateRoleRequest{
		Name: &emptyName,
	}
	_, err = service.UpdateRole(orgID, roleID, req)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty update name, got: %v", err)
	}

	// Forbidden - System Role
	_, err = service.UpdateRole(orgID, systemRoleID, req)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for system role update, got: %v", err)
	}
}

func TestRoleService_DeleteRole(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	roleID := uuid.Must(uuid.NewV4())
	systemRoleID := uuid.Must(uuid.NewV4())

	roleRepo := &stubRoleRepo{
		roles: map[uuid.UUID]*models.Role{
			roleID:       {ID: roleID, Name: "Org Role", OrganizationID: &orgID, IsSystem: false},
			systemRoleID: {ID: systemRoleID, Name: "System Role", IsSystem: true},
		},
	}
	service := services.InitRoleService(roleRepo, zap.NewNop())

	// Forbidden - System Role
	err := service.DeleteRole(orgID, systemRoleID)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for system role deletion, got: %v", err)
	}

	// Conflict - Role Assigned
	roleRepo.isAssigned = true
	err = service.DeleteRole(orgID, roleID)
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict for assigned role, got: %v", err)
	}

	// Success - Unassigned
	roleRepo.isAssigned = false
	err = service.DeleteRole(orgID, roleID)
	if err != nil {
		t.Fatalf("expected delete to succeed, got: %v", err)
	}
}

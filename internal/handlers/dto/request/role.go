package request

import "github.com/gofrs/uuid"

type CreateRoleRequest struct {
	Name        string                    `json:"name" binding:"required,min=2,max=100"`
	Description string                    `json:"description"`
	Permissions map[string]map[string]bool `json:"permissions" binding:"required"`
}

type UpdateRoleRequest struct {
	Name        *string                   `json:"name,omitempty" binding:"omitempty,min=2,max=100"`
	Description *string                   `json:"description,omitempty"`
	Permissions map[string]map[string]bool `json:"permissions,omitempty"`
}

type RoleDetailsRequest struct {
	RoleID         uuid.UUID
	OrganizationID uuid.UUID
}

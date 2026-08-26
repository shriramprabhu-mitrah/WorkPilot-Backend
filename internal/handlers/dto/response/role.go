package response

import (
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

type RoleResponse struct {
	ID             uuid.UUID                  `json:"id"`
	OrganizationID *uuid.UUID                 `json:"organization_id,omitempty"`
	Name           string                     `json:"name"`
	Description    string                     `json:"description,omitempty"`
	IsSystem       bool                       `json:"is_system"`
	Permissions    map[string]map[string]bool `json:"permissions"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

func MapToRoleResponse(role models.Role) RoleResponse {
	permissionsMap := make(map[string]map[string]bool)
	resourceActions := map[string][]string{
		"projects":     {"view", "add", "modify", "delete"},
		"sprints":      {"view", "add", "modify", "delete"},
		"user_stories": {"view", "add", "modify", "delete"},
		"tasks":        {"view", "add", "modify", "delete"},
		"comments":     {"view", "add", "modify", "delete"},
	}

	for res, actions := range resourceActions {
		permissionsMap[res] = make(map[string]bool)
		for _, act := range actions {
			permissionsMap[res][act] = false
		}
	}

	for _, perm := range role.Permissions {
		if _, exists := permissionsMap[perm.Resource]; exists {
			permissionsMap[perm.Resource][perm.Action] = true
		}
	}

	return RoleResponse{
		ID:             role.ID,
		OrganizationID: role.OrganizationID,
		Name:           role.Name,
		Description:    role.Description,
		IsSystem:       role.IsSystem,
		Permissions:    permissionsMap,
		CreatedAt:      role.CreatedAt,
		UpdatedAt:      role.UpdatedAt,
	}
}

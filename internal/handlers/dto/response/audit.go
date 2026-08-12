package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type AuditLogResponse struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	Action         string     `json:"action"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     string     `json:"resource_id,omitempty"`
	Details        string     `json:"details,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	TaskKey        string     `json:"task_key,omitempty"`
	Title          string     `json:"title,omitempty"`
}

type AuditLogResponseWrapper struct {
	User       *UserSummary       `json:"user,omitempty"`
	Activities []AuditLogResponse `json:"activities"`
}

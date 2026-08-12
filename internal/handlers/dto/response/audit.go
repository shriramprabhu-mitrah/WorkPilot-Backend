package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type AuditLogResponse struct {
	ProjectID    *uuid.UUID `json:"project_id,omitempty"`
	Action       string     `json:"action"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id"`
	CreatedAt    time.Time  `json:"created_at"`
}

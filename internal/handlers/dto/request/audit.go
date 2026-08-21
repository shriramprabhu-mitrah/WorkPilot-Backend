package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type GetAudit struct {
	response.PaginationQuery
	UserID         *uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID *uuid.UUID `json:"-" swaggerignore:"true"`
	ResourceType   string     `form:"resource_type"`
	ResourceID     string     `form:"resource_id"`
	TaskID         *uuid.UUID `form:"task_id"`
	UserStoryID    *uuid.UUID `form:"user_story_id"`
	ProjectID      *uuid.UUID `form:"project_id"`
	Type           string     `form:"type"`
}

package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type GetAudit struct {
	response.PaginationQuery
	UserID         *uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID *uuid.UUID `json:"-" swaggerignore:"true"`
	ActivityType   string     `json:"-" swaggerignore:"true"`
}

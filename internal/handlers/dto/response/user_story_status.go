package response

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

type UserStoryStatusResponse struct {
	ID           *uuid.UUID `json:"id,omitempty"`
	ProjectID    uuid.UUID  `json:"project_id"`
	Name         string     `json:"name"`
	Color        string     `json:"color"`
	DisplayOrder int        `json:"display_order"`
	IsDefault    bool       `json:"is_default"`
	IsClosed     bool       `json:"is_closed"`
	IsFinal      bool       `json:"is_final"`
}

func UserStoryStatusFromModel(cs models.UserStoryStatus) UserStoryStatusResponse {
	idVal := cs.ID
	return UserStoryStatusResponse{
		ID:           &idVal,
		ProjectID:    cs.ProjectID,
		Name:         cs.Name,
		Color:        cs.Color,
		DisplayOrder: cs.DisplayOrder,
		IsDefault:    cs.IsDefault,
		IsClosed:     cs.IsClosed,
		IsFinal:      cs.IsFinal,
	}
}

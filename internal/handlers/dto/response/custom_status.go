package response

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

type CustomStatusResponse struct {
	ID           *uuid.UUID `json:"id,omitempty"`
	ProjectID    uuid.UUID  `json:"project_id"`
	Name         string     `json:"name"`
	Color        string     `json:"color"`
	DisplayOrder int        `json:"display_order"`
	IsDefault    bool       `json:"is_default"`
	IsFinal      bool       `json:"is_final"`
}

func CustomStatusFromModel(cs models.CustomStatus) CustomStatusResponse {
	idVal := cs.ID
	return CustomStatusResponse{
		ID:           &idVal,
		ProjectID:    cs.ProjectID,
		Name:         cs.Name,
		Color:        cs.Color,
		DisplayOrder: cs.DisplayOrder,
		IsDefault:    cs.IsDefault,
		IsFinal:      cs.IsFinal,
	}
}

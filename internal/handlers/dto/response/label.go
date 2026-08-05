package response

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

type LabelResponse struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

func LabelFromModel(l models.Label) LabelResponse {
	return LabelResponse{
		ID:    l.ID,
		Name:  l.Name,
		Color: l.Color,
	}
}

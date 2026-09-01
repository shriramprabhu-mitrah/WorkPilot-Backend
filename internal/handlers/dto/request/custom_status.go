package request

import (
	"github.com/gofrs/uuid"
)

type CreateCustomStatusRequest struct {
	Name           string    `json:"name" binding:"required,min=1,max=50"`
	Color          string    `json:"color" binding:"required"`
	DisplayOrder   int       `json:"display_order" binding:"gte=0"`
	IsFinal        *bool     `json:"is_final" binding:"omitempty"`
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type UpdateCustomStatusRequest struct {
	Name           *string   `json:"name" binding:"omitempty,min=1,max=50"`
	Color          *string   `json:"color" binding:"omitempty"`
	DisplayOrder   *int      `json:"display_order" binding:"omitempty,gte=0"`
	IsFinal        *bool     `json:"is_final" binding:"omitempty"`
	StatusID       uuid.UUID `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

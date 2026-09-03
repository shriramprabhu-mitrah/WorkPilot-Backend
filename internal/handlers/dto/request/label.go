package request

import (
	"regexp"

	"github.com/gofrs/uuid"
)

var hexColorRegex = regexp.MustCompile(`^#[[:xdigit:]]{6}$`)

type CreateLabelRequest struct {
	Name           string    `json:"name" binding:"required,min=1,max=30"`
	Color          string    `json:"color" binding:"required"`
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type UpdateLabelRequest struct {
	Name           *string   `json:"name" binding:"omitempty,min=1,max=30"`
	Color          *string   `json:"color" binding:"omitempty"`
	LabelID        uuid.UUID `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

func ValidateColor(color string) bool {
	return hexColorRegex.MatchString(color)
}

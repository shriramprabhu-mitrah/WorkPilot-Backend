package request

import (
	"regexp"

	"github.com/gofrs/uuid"
)

var hexColorRegex = regexp.MustCompile(`^#[[:xdigit:]]{6}$`)

type CreateLabelRequest struct {
	Name           string    `json:"name" binding:"required,min=1,max=30"`
	Color          string    `json:"color" binding:"required"`
	ProjectID      uuid.UUID `json:"-"`
	UserID         uuid.UUID `json:"-"`
	OrganizationID uuid.UUID `json:"-"`
}

type UpdateLabelRequest struct {
	Name           *string   `json:"name" binding:"omitempty,min=1,max=30"`
	Color          *string   `json:"color" binding:"omitempty"`
	LabelID        uuid.UUID `json:"-"`
	ProjectID      uuid.UUID `json:"-"`
	UserID         uuid.UUID `json:"-"`
	OrganizationID uuid.UUID `json:"-"`
}

func ValidateColor(color string) bool {
	return hexColorRegex.MatchString(color)
}

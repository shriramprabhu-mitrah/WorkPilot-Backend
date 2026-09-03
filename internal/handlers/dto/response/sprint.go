package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type SprintResponse struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Goal          string     `json:"goal"`
	Status        string     `json:"status"`
	StartDate     *time.Time `json:"start_date"`
	EndDate       *time.Time `json:"end_date,omitzero"`
	ActualEndDate *time.Time `json:"actual_end_date,omitzero"`
}

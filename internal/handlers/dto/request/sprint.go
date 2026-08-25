package request

import (
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type SprintStatus string

const (
	SprintStatusPlanned   SprintStatus = "planned"
	SprintStatusActive    SprintStatus = "active"
	SprintStatusOnHold    SprintStatus = "on_hold"
	SprintStatusCompleted SprintStatus = "completed"
	SprintStatusCancelled SprintStatus = "cancelled"
	SprintStatusArchived  SprintStatus = "archived"
)

type CreateSprintRequest struct {
	Sprints        []CreateSprint `json:"sprints" binding:"required"`
	ProjectID      uuid.UUID      `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID      `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID      `json:"-" swaggerignore:"true"`
}

type CreateSprint struct {
	Name      string  `json:"name" binding:"required,min=2,max=1000"`
	Goal      string  `json:"goal"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

type UpdateSprintRequest struct {
	Name           *string       `json:"name" binding:"omitempty,min=2,max=100"`
	Goal           *string       `json:"goal" binding:"omitempty,max=500"`
	StartDate      *string       `json:"start_date"`
	EndDate        *string       `json:"end_date"`
	Status         *SprintStatus `json:"status" binding:"omitempty,oneof=planned active on_hold completed cancelled archived"`
	ProjectID      uuid.UUID     `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID     `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID     `json:"-" swaggerignore:"true"`
	SprintID       uuid.UUID     `json:"-" swaggerignore:"true"`
}

type SprintFilter struct {
	response.PaginationQuery
	Status    SprintStatus `form:"status"`
	Search    string       `form:"search"`
	Fields    string       `form:"fields"`
	StartDate time.Time    `form:"start_date" time_format:"2006-01-02"`
	EndDate   time.Time    `form:"end_date" time_format:"2006-01-02"`
}

type DeleteSprint struct {
	ProjectID      uuid.UUID
	SprintID       uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type GetSprint struct {
	SprintID       uuid.UUID
	ProjectID      uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type BurndownDataPoint struct {
	Date            string  `json:"date"`
	RemainingPoints *int    `json:"remaining_points"`
	IdealValue      float64 `json:"ideal_value"`
}

type SprintBurndownResponse struct {
	SprintID         string              `json:"sprint_id"`
	SprintName       string              `json:"sprint_name"`
	TotalStoryPoints int                 `json:"total_story_points"`
	BurndownData     []BurndownDataPoint `json:"burndown_data"`
}

type StartSprintRequest struct {
	ProjectID uuid.UUID `json:"project_id"`
	SprintID  uuid.UUID `json:"-"`
	UserID    uuid.UUID `json:"-"`
	StartDate string    `json:"start_date" form:"start_date" binding:"required"`
	EndDate   string    `json:"end_date" form:"end_date" binding:"required"`
}

type CompleteSprintRequest struct {
	ProjectID uuid.UUID `json:"project_id"`
	SprintID  uuid.UUID `json:"-"`
	UserID    uuid.UUID `json:"-"`
}

package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type SprintStatus string

const (
	SprintStatusPlanning  SprintStatus = "planning"
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
	Name      string `json:"name" binding:"required,min=2,max=100"`
	Goal      string `json:"goal" `
	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
}

type UpdateSprintRequest struct {
	Name           string       `json:"name" binding:"omitempty,min=2,max=100"`
	Goal           string       `json:"goal" binding:"omitempty,max=500"`
	StartDate      string       `json:"start_date"`
	EndDate        string       `json:"end_date"`
	Status         SprintStatus `json:"status" binding:"omitempty,oneof=planning active on_hold completed cancelled archived"`
	ProjectID      uuid.UUID    `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID    `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID    `json:"-" swaggerignore:"true"`
	SprintID       uuid.UUID    `json:"-" swaggerignore:"true"`
}

type SprintFilter struct {
	response.PaginationQuery
	Status    SprintStatus `form:"status"`
	Search    string       `form:"search"`
	FieldName string       `form:"fieldName"`
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

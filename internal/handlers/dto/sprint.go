package dto

import (
	"time"

	"github.com/gofrs/uuid"
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
	Name           string `json:"name" binding:"required,min=2,max=100"`
	Goal           string `json:"goal" `
	StartDate      string `json:"start_date" binding:"required"`
	EndDate        string `json:"end_date" binding:"required"`
	ProjectID      uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type UpdateSprintRequest struct {
	Name           string       `json:"name" binding:"omitempty,min=2,max=100"`
	Goal           string       `json:"goal" binding:"omitempty,max=500"`
	StartDate      *time.Time   `json:"start_date"`
	EndDate        *time.Time   `json:"end_date"`
	Status         SprintStatus `json:"status" binding:"omitempty,oneof=planning active on_hold completed cancelled archived"`
	ProjectID      uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	SprintID       uuid.UUID
}

type SprintFilter struct {
	Page     int          `form:"page"`
	PageSize int          `form:"page_size"`
	Status   SprintStatus `form:"status"`
	Search   string       `form:"search"`
}

type DeleteSprint struct {
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

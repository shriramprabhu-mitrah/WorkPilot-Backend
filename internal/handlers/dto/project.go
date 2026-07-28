package dto

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
)

type ProjectStatus string

const (
	ProjectStatusPlanning  ProjectStatus = "planning"
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusOnHold    ProjectStatus = "on_hold"
	ProjectStatusCompleted ProjectStatus = "completed"
	ProjectStatusCancelled ProjectStatus = "cancelled"
	ProjectStatusArchived  ProjectStatus = "archived"
)

type CreateProjectRequest struct {
	Name           string `json:"name" binding:"required,min=3,max=150"`
	Key            string `json:"key" binding:"required"`
	Description    string `json:"description" `
	StartDate      string `json:"start_date" binding:"required"`
	EndDate        string `json:"end_date" binding:"required"`
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type UpdateProjectRequest struct {
	Name           string        `json:"name" binding:"omitempty,min=3,max=150"`
	Description    string        `json:"description"`
	StartDate      string        `json:"start_date"`
	EndDate        string        `json:"end_date"`
	Status         ProjectStatus `form:"status"`
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
}

type ProjectFilterRequest struct {
	Page      int           `form:"page"`
	PageSize  int           `form:"page_size"`
	Key       string        `form:"project_key"`
	Name      string        `form:"name"`
	Status    ProjectStatus `form:"archive_status"`
	StartDate string        `form:"start_date"`
	EndDate   string        `form:"end_date"`
}

type ProjectFilter struct {
	Page      int        `form:"page"`
	PageSize  int        `form:"page_size"`
	Key       string     `form:"key"`
	Name      string     `form:"name"`
	Status    string     `form:"status"`
	StartDate *time.Time `form:"start_date"`
	EndDate   *time.Time `form:"end_date"`
}

func (r ProjectStatus) Validate() error {
	switch r {
	case ProjectStatusPlanning,
		ProjectStatusActive,
		ProjectStatusOnHold,
		ProjectStatusCompleted,
		ProjectStatusCancelled,
		ProjectStatusArchived:
		return nil
	default:
		return fmt.Errorf("Invalid role: %s", r)
	}
}

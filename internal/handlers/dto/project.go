package dto

import (
	"fmt"

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
	Description    string `json:"description" `
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type UpdateProjectRequest struct {
	Name           string        `json:"name" binding:"omitempty,min=3,max=150"`
	Description    string        `json:"description"`
	Status         ProjectStatus `form:"status"`
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
}

type ProjectFilterRequest struct {
	Page     int           `form:"page"`
	PageSize int           `form:"page_size"`
	Name     string        `form:"name"`
	Status   ProjectStatus `form:"status"`
}

type ProjectFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Name     string `form:"name"`
	Status   string `form:"status"`
}

type CreateProjectMemberRequest struct {
	ProjectID      uuid.UUID   `json:"project_id" binding:"required"`
	UserIDs        []uuid.UUID `json:"user_id" binding:"required"`
	AddedByID      uuid.UUID
	OrganizationID uuid.UUID
}

type ProjectMemberFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Name     string `form:"name"`
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

type ProjectActivityFilterRequest struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Action       string `form:"action"`
	UserID       string `form:"user_id"`
	ResourceType string `form:"resource_type"`
	StartDate    string `form:"start_date"`
	EndDate      string `form:"end_date"`
}

type ProjectActivityFilter struct {
	Page         int
	PageSize     int
	Action       string
	UserID       *uuid.UUID
	ResourceType string
	StartDate    string
	EndDate      string
}

type UserSummaryDTO struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Role      string    `json:"role,omitempty"`
}

type ProjectActivityResponseDTO struct {
	ID             uuid.UUID       `json:"id"`
	ProjectID      *uuid.UUID      `json:"project_id,omitempty"`
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty"`
	User           *UserSummaryDTO `json:"user,omitempty"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     string          `json:"resource_id,omitempty"`
	Details        string          `json:"details,omitempty"`
	CreatedAt      string          `json:"timestamp"`
}


package request

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type ProjectStatus string
type ProjectRole string

const (
	ProjectStatusPlanning  ProjectStatus = "planning"
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusOnHold    ProjectStatus = "on_hold"
	ProjectStatusCompleted ProjectStatus = "completed"
	ProjectStatusCancelled ProjectStatus = "cancelled"
	ProjectStatusArchived  ProjectStatus = "archived"
)

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

const (
	ProjectRoleOrgAdmin       ProjectRole = "org_admin"
	ProjectRoleProjectManager ProjectRole = "project_manager"
	ProjectRoleDeveloper      ProjectRole = "developer"
	ProjectRoleTester         ProjectRole = "tester"
	ProjectRoleViewer         ProjectRole = "viewer"
)

type CreateProjectRequest struct {
	Name           string    `json:"name" binding:"required,min=3,max=150"`
	Description    string    `json:"description" `
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type UpdateProjectRequest struct {
	Name           *string        `json:"name" binding:"omitempty,min=3,max=150"`
	Description    *string        `json:"description"`
	Status         *ProjectStatus `json:"status" form:"status"`
	UserID         uuid.UUID      `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID      `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID      `json:"-" swaggerignore:"true"`
}

type ProjectFilterRequest struct {
	response.PaginationQuery
	response.SortQuery
	Name           string        `form:"name"`
	Status         ProjectStatus `form:"status"`
	FieldName      string        `form:"fieldName"`
	IncludeSprints bool          `form:"include_sprints"`
	UserID         uuid.UUID     `form:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID     `form:"-" swaggerignore:"true"`
}

type ProjectFilter struct {
	response.PaginationQuery
	response.SortQuery
	Name           string `form:"name"`
	Status         string `form:"status"`
	IncludeSprints bool   `form:"include_sprints"`
}

type ProjectMemberRequest struct {
	UserID      uuid.UUID   `json:"user_id" binding:"required"`
	ProjectRole ProjectRole `json:"project_role" binding:"required,oneof=project_manager developer tester viewer"`
}

type CreateProjectMemberRequest struct {
	ProjectID      uuid.UUID              `json:"project_id" binding:"required"`
	Members        []ProjectMemberRequest `json:"members" binding:"required,min=1"`
	AddedByID      uuid.UUID              `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID              `json:"-" swaggerignore:"true"`
}

type ProjectMemberFilter struct {
	response.PaginationQuery
	Name           string    `form:"name"`
	UserID         uuid.UUID `form:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `form:"-" swaggerignore:"true"`
}

type ProjectActivityFilterRequest struct {
	response.PaginationQuery
	Action       string `form:"action"`
	UserID       string `form:"user_id"`
	ResourceType string `form:"resource_type"`
	StartDate    string `form:"start_date"`
	EndDate      string `form:"end_date"`
}

type ProjectActivityFilter struct {
	response.PaginationQuery
	Action       string
	UserID       *uuid.UUID
	ResourceType string
	StartDate    string
	EndDate      string
}

type GetProjectDetails struct {
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type GetProjectByUserID struct {
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type DeleteProject struct {
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
}

type ProjectResponse struct {
	ProjectID      uuid.UUID    `json:"id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	Name           string       `json:"name"`
	Description    string       `json:"description,omitempty"`
	Status         string       `json:"status"`
	CreatedBy      uuid.UUID    `json:"created_by"`
	Creator        string       `json:"creator"`
	CreatedAt      time.Time    `json:"created_at"`
	Members        []Member     `json:"members"`
	Sprints        []SprintItem `json:"sprints"`
}

type Member struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	ProjectRole string    `json:"project_role"`
}

type SprintItem struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal,omitempty"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type RemoveProjectMember struct {
	ProjectID        uuid.UUID
	TargetUserID     uuid.UUID
	PerformingUserID uuid.UUID
	OrganizationID   uuid.UUID
}

type UpdateProjectMemberRequest struct {
	MemberID       uuid.UUID   `json:"-" swaggerignore:"true"`
	ProjectRole    ProjectRole `json:"project_role" binding:"required,oneof=project_manager developer tester viewer"`
	OrganizationID uuid.UUID   `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID   `json:"-" swaggerignore:"true"`
	UpdatedBy      uuid.UUID   `json:"-" swaggerignore:"true"`
}

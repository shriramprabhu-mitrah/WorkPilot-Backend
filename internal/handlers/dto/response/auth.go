package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type OrganizationSummary struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug,omitempty"`
	Domain        string    `json:"domain"`
	Industry      string    `json:"industry,omitempty"`
	TeamSize      string    `json:"team_size,omitempty"`
	Country       string    `json:"country,omitempty"`
	LogoURL       string    `json:"logo_url,omitempty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	TotalProjects int       `json:"total_projects"`
	TotalMembers  int       `json:"total_members"`
}

type UserProfile struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   *uuid.UUID `json:"organization_id,omitempty"`
	OrganizationName string     `json:"organization_name,omitempty"`
	Name             string     `json:"name"`
	Username         string     `json:"username"`
	Email            string     `json:"email"`
	Role             string     `json:"role,omitempty"`
	AvatarURL        *string    `json:"avatar_url"`
	Timezone         string     `json:"timezone,omitempty"`
	IsActive         bool       `json:"is_active"`
	IsVerified       bool       `json:"is_verified"`
	CreatedAt        time.Time  `json:"created_at"`
	JoinedAt         time.Time  `json:"joined_at,omitempty"`
}

type ProjectSummary struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	OrganizationName string    `json:"organization_name,omitempty"`
	Name             string    `json:"name"`
	Key              string    `json:"key,omitempty"`
	ProjectKey       string    `json:"project_key,omitempty"`
	Description      string    `json:"description,omitempty"`
	Status           string    `json:"status"`
	CreatedBy        uuid.UUID `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	SprintCount      int       `json:"sprint_count"`
	TotalTasks       int       `json:"total_tasks"`
	TotalMembers     int       `json:"total_members"`
	Sprints          []Sprint  `json:"sprints,omitempty"`
}

type ProjectMember struct {
	UserID           uuid.UUID `json:"user_id"`
	Username         string    `json:"username"`
	FullName         string    `json:"full_name"`
	Role             string    `json:"role"`
	AvatarURL        *string   `json:"avatar_url"`
	OrganizationName string    `json:"organization_name,omitempty"`
	ProjectKey       string    `json:"project_key,omitempty"`
}

type Sprint struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Goal      string         `json:"goal,omitempty"`
	Status    string         `json:"status"`
	StartDate *time.Time     `json:"start_date"`
	EndDate   *time.Time     `json:"end_date"`
	Tasks     []TaskResponse `json:"tasks,omitempty"`
}

type ProjectActivity struct {
	ID             uuid.UUID    `json:"id"`
	ProjectID      *uuid.UUID   `json:"project_id,omitempty"`
	OrganizationID *uuid.UUID   `json:"organization_id,omitempty"`
	User           *UserSummary `json:"user,omitempty"`
	Action         string       `json:"action"`
	ResourceType   string       `json:"resource_type"`
	ResourceID     string       `json:"resource_id,omitempty"`
	Details        string       `json:"details,omitempty"`
	CreatedAt      string       `json:"timestamp"`
	TaskKey        string       `json:"task_key,omitempty"`
	Title          string       `json:"title,omitempty"`
}

type UserSummary struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL *string   `json:"avatar_url"`
	Role      string    `json:"role,omitempty"`
}

package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type OrganizationSummary struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug,omitempty"`
	Domain    string    `json:"domain"`
	Industry  string    `json:"industry,omitempty"`
	TeamSize  string    `json:"team_size,omitempty"`
	Country   string    `json:"country,omitempty"`
	LogoURL   string    `json:"logo_url,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type UserProfile struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	Name           string     `json:"name"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	Role           string     `json:"role,omitempty"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	Timezone       string     `json:"timezone,omitempty"`
	IsActive       bool       `json:"is_active"`
	IsVerified     bool       `json:"is_verified"`
	CreatedAt      time.Time  `json:"created_at"`
	JoinedAt       time.Time  `json:"joined_at,omitempty"`
}

type ProjectSummary struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	CreatedBy      uuid.UUID `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	SprintCount    int       `json:"sprint_count"`
}

type ProjectMember struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
}

type Sprint struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal,omitempty"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
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
}

type UserSummary struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Role      string    `json:"role,omitempty"`
}

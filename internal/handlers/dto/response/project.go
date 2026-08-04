package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type ProjectDetail struct {
	ProjectID      uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Status         string          `json:"status"`
	CreatedBy      uuid.UUID       `json:"created_by"`
	Creator        string          `json:"creator"`
	CreatedAt      time.Time       `json:"created_at"`
	Members        []ProjectMember `json:"members"`
	Sprints        []Sprint        `json:"sprints"`
}

type ProjectActivityResponse struct {
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

type ProjectMemberResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
}

type SprintResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal,omitempty"`
	Status    string    `json:"status"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type ProjectResponse struct {
	ProjectID *uuid.UUID `json:"project_id"`
	Role      string     `json:"role"`
}

type GetProjectByUserIDResponse struct {
	UserID  uuid.UUID         `json:"user_id"`
	Project []ProjectResponse `json:"project"`
}

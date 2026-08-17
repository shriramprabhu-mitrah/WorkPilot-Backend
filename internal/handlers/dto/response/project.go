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
	Metrics        ProjectMetrics  `json:"metrics"`
}

type ProjectActivityResponse struct {
	ID             uuid.UUID    `json:"id"`
	ProjectID      *uuid.UUID   `json:"project_id,omitempty"`
	ProjectName    string       `json:"project_name,omitempty"`
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

type ProjectMemberResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
}

type ProjectResponse struct {
	ProjectID   uuid.UUID `json:"project_id"`
	Role        string    `json:"role"`
	ProjectName string    `json:"project_name"`
	Status      string    `json:"status"`
}

type GetProjectByUserIDResponse struct {
	UserID    uuid.UUID         `json:"user_id"`
	UserName  string            `json:"user_name"`
	FullName  string            `json:"full_name"`
	Email     string            `json:"email"`
	AvatarURL string            `json:"avatar_url,omitempty"`
	Role      string            `json:"role,omitempty"`
	Project   []ProjectResponse `json:"project"`
}

type ProjectMetrics struct {
	TotalTasks               int `json:"total_tasks"`
	CompletedTasks           int `json:"completed_tasks"`
	PendingTasks             int `json:"pending_tasks"`
	OverdueTasks             int `json:"overdue_tasks"`
	CompletedTasksPercentage int `json:"completed_tasks_percentage"`
	TotalSprints             int `json:"total_sprints"`
	ActiveSprints            int `json:"active_sprints"`
	CompletedSprints         int `json:"completed_sprints"`
	TotalMembers             int `json:"total_members"`
}

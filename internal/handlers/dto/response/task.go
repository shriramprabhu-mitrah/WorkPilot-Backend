package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type TaskResponse struct {
	ID             uuid.UUID       `json:"id"`
	ProjectID      uuid.UUID       `json:"project_id"`
	SprintID       *uuid.UUID      `json:"sprint_id,omitempty"`
	SprintName     string          `json:"sprint_name,omitempty"`
	UserStoryID    *uuid.UUID      `json:"user_story_id,omitempty"`
	Key            string          `json:"key"`
	Title          string          `json:"title"`
	Description    string          `json:"description,omitempty"`
	Type           string          `json:"type"`
	Priority       string          `json:"priority"`
	Status         string          `json:"status"`
	StatusColor    string          `json:"status_color"`
	AssigneeID     *uuid.UUID      `json:"assignee_id,omitempty"`
	ReporterID     *uuid.UUID      `json:"reporter_id,omitempty"`
	AssigneeName   string          `json:"assignee_name,omitempty"`
	StoryPoints    int             `json:"story_points"`
	DueDate        *time.Time      `json:"due_date,omitempty"`
	EstimatedHours *float64        `json:"estimated_hours,omitempty"`
	ActualHours    *float64        `json:"actual_hours,omitempty"`
	BlockedReason  string          `json:"blocked_reason,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Labels         []LabelResponse `json:"labels,omitempty"`
	User           *UserSummary    `json:"reporter,omitempty"`
}

type BulkUpdateTasksResponse struct {
	UpdatedCount   int               `json:"updated_count"`
	FailedTaskIDs  []uuid.UUID       `json:"failed_task_ids"`
	FailureReasons map[string]string `json:"failure_reasons"`
}

type BulkDeleteTasksResponse struct {
	DeletedCount   int               `json:"deleted_count"`
	DeletedTaskIDs []uuid.UUID       `json:"deleted_task_ids"`
	FailedTaskIDs  []uuid.UUID       `json:"failed_task_ids"`
	FailureReasons map[string]string `json:"failure_reasons"`
}

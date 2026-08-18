package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type TaskResponse struct {
	ID                    uuid.UUID       `json:"id"`
	ProjectID             uuid.UUID       `json:"project_id"`
	SprintID              *uuid.UUID      `json:"sprint_id"`
	SprintName            string          `json:"sprint_name"`
	UserStoryID           *uuid.UUID      `json:"user_story_id,omitempty"`
	UserStoryTitle        string          `json:"user_story_title,omitempty"`
	Key                   string          `json:"key"`
	SerialNumber          int64           `json:"serial_number"`
	FormattedSerialNumber string          `json:"formatted_serial_number,omitempty"`
	Title                 string          `json:"title"`
	Description           string          `json:"description,omitempty"`
	Type                  string          `json:"type"`
	Priority              string          `json:"priority"`
	StatusID              uuid.UUID       `json:"status_id"`
	Status                string          `json:"status"`
	StatusColor           string          `json:"status_color"`
	AssigneeID            *uuid.UUID      `json:"assignee_id,omitempty"`
	ReporterID            *uuid.UUID      `json:"reporter_id,omitempty"`
	ReporterName          string          `json:"reporter_name,omitempty"`
	AssigneeName          string          `json:"assignee_name,omitempty"`
	StoryPoints           int             `json:"story_points"`
	DueDate               *time.Time      `json:"due_date,omitempty"`
	EstimatedHours        *float64        `json:"estimated_hours,omitempty"`
	ActualHours           *float64        `json:"actual_hours,omitempty"`
	BlockedReason         string          `json:"blocked_reason,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	Labels                []LabelResponse `json:"labels,omitempty"`
	User                  *UserSummary    `json:"reporter,omitempty"`
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

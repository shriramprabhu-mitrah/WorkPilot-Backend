package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type UserStoryResponse struct {
	ID           uuid.UUID    `json:"id"`
	ProjectID    uuid.UUID    `json:"project_id"`
	SprintID     *uuid.UUID   `json:"sprint_id,omitempty"`
	SprintName            string       `json:"sprint_name,omitempty"`
	SerialNumber          int64        `json:"serial_number"`
	FormattedSerialNumber string       `json:"formatted_serial_number,omitempty"`
	Title                 string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	Priority     string       `json:"priority"`
	StatusID     uuid.UUID    `json:"status_id"`
	Status       string       `json:"status"`
	StatusColor  string       `json:"status_color"`
	StoryPoints  int          `json:"story_points"`
	AssigneeID   *uuid.UUID   `json:"assignee_id,omitempty"`
	AssigneeName string       `json:"assignee_name,omitempty"`
	ReporterID   uuid.UUID    `json:"reporter_id"`
	ReporterName string       `json:"reporter_name"`
	Reporter     *UserSummary `json:"reporter,omitempty"`
	Assignee     *UserSummary `json:"assignee,omitempty"`
	BacklogOrder   int          `json:"backlog_order"`
	TotalTasks     int64        `json:"total_tasks"`
	CompletedTasks int64        `json:"completed_tasks"`
	Progress       float64      `json:"progress"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	Tasks          []TaskResponse `json:"tasks,omitempty"`
}

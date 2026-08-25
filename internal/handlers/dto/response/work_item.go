package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type WorkItemResponse struct {
	WorkItemType          string             `json:"work_item_type"` // "user_story" or "task"
	ID                    uuid.UUID          `json:"id"`
	ProjectID             uuid.UUID          `json:"project_id"`
	SerialNumber          int64              `json:"serial_number"`
	FormattedSerialNumber string             `json:"formatted_serial_number,omitempty"`
	Title                 string             `json:"title"`
	Description           string             `json:"description,omitempty"`
	Priority              string             `json:"priority"`
	StatusID              uuid.UUID          `json:"status_id"`
	Status                string             `json:"status"`
	StatusColor           string             `json:"status_color,omitempty"`
	IsFavourite           bool               `json:"is_favourite"`
	StoryPoints           int                `json:"story_points"`
	SprintID              *uuid.UUID         `json:"sprint_id,omitempty"`
	SprintName            string             `json:"sprint_name,omitempty"`
	AssigneeID            *uuid.UUID         `json:"assignee_id,omitempty"`
	AssigneeName          string             `json:"assignee_name,omitempty"`
	ReporterID            *uuid.UUID         `json:"reporter_id,omitempty"`
	ReporterName          string             `json:"reporter_name,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
	TaskDetails           *TaskResponse      `json:"task_details,omitempty"`
	UserStoryDetails      *UserStoryResponse `json:"user_story_details,omitempty"`
}

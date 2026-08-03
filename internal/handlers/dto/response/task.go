package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type TaskResponse struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	SprintID       *uuid.UUID `json:"sprint_id,omitempty"`
	SprintName     string     `json:"sprint_name,omitempty"`
	Key            string     `json:"key"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	Type           string     `json:"type"`
	Priority       string     `json:"priority"`
	Status         string     `json:"status"`
	AssigneeID     *uuid.UUID `json:"assignee_id,omitempty"`
	AssigneeName   string     `json:"assignee_name,omitempty"`
	StoryPoints    int        `json:"story_points"`
	DueDate        *time.Time `json:"due_date,omitempty"`
	EstimatedHours *float64   `json:"estimated_hours,omitempty"`
	ActualHours    *float64   `json:"actual_hours,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

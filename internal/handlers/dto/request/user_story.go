package request

import (
	"encoding/json"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type CreateUserStoryRequest struct {
	Title          string      `json:"title" binding:"required,min=3,max=255"`
	Description    string      `json:"description"`
	Priority       string      `json:"priority" binding:"required,oneof=low medium high critical"`
	StatusID       *uuid.UUID  `json:"status_id" binding:"omitempty"`
	Status         string      `json:"status" binding:"omitempty"`
	StoryPoints    int         `json:"story_points" binding:"min=0"`
	AssigneeID     *uuid.UUID  `json:"assignee_id"`
	SprintID       *uuid.UUID  `json:"sprint_id"`
	AttachmentIDs  []uuid.UUID `json:"attachment_ids,omitempty"`
	ProjectID      uuid.UUID   `json:"-" swaggerignore:"true"`
	ReporterID     uuid.UUID   `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID   `json:"-" swaggerignore:"true"`
}

type UpdateUserStoryRequest struct {
	Title          *string         `json:"title" binding:"omitempty,min=3,max=255"`
	Description    *string         `json:"description"`
	Priority       *string         `json:"priority" binding:"omitempty,oneof=low medium high critical"`
	StatusID       *uuid.UUID      `json:"status_id" binding:"omitempty"`
	Status         *string         `json:"status" binding:"omitempty"`
	StoryPoints    *int            `json:"story_points" binding:"omitempty,min=0"`
	IsClosed       *bool           `json:"is_closed"`
	AssigneeID     *uuid.UUID      `json:"assignee_id"`
	SprintID       *uuid.UUID      `json:"sprint_id"`
	UserStoryID    uuid.UUID       `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID       `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID       `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID       `json:"-" swaggerignore:"true"`
	IsNullFields   map[string]bool `json:"-" swaggerignore:"true"`
}

type UpdateUserStoryStatusAssignmentRequest struct {
	StatusID       uuid.UUID `json:"status_id" binding:"required"`
	UserStoryID    uuid.UUID `json:"-" swaggerignore:"true"`
	ProjectID      uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

func (r *UpdateUserStoryRequest) UnmarshalJSON(data []byte) error {
	type Alias UpdateUserStoryRequest
	var temp Alias
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*r = UpdateUserStoryRequest(temp)

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	r.IsNullFields = make(map[string]bool)
	if val, exists := rawMap["sprint_id"]; exists && val == nil {
		r.IsNullFields["sprint_id"] = true
	}
	if val, exists := rawMap["assignee_id"]; exists && val == nil {
		r.IsNullFields["assignee_id"] = true
	}

	return nil
}

func (r *UpdateUserStoryRequest) IsSprintIDNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["sprint_id"]
}

func (r *UpdateUserStoryRequest) IsAssigneeIDNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["assignee_id"]
}

type UserStoryFilter struct {
	response.PaginationQuery
	response.SortQuery
	Status            string `form:"status"`
	Assignee          string `form:"assignee_id"`
	Reporter          string `form:"reporter_id"`
	Sprint            string `form:"sprint_id"`
	Search            string `form:"search"`
	Priority          string `form:"priority"`
	Fields            string `form:"fields"`
	SequenceNumber    *int64 `form:"sequence_number"`
	SerialNumber      *int64 `form:"serial_number"`
	IsUnassignedStory bool   `form:"is_unassigned_story"`
	IsClosed          *bool  `form:"is_closed"`
}

type ReorderUserStoriesRequest struct {
	StoryIDs       []uuid.UUID `json:"story_ids" binding:"required,min=1"`
	ProjectID      uuid.UUID   `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID   `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID   `json:"-" swaggerignore:"true"`
}

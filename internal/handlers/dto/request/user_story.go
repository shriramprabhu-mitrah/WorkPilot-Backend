package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type CreateUserStoryRequest struct {
	Title          string     `json:"title" binding:"required,min=3,max=255"`
	Description    string     `json:"description"`
	Priority       string     `json:"priority" binding:"required,oneof=low medium high critical"`
	Status         string     `json:"status" binding:"omitempty,oneof=todo in_progress in_review testing completed blocked"`
	StoryPoints    int        `json:"story_points" binding:"min=0"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	SprintID       *uuid.UUID `json:"sprint_id"`
	ProjectID      uuid.UUID  `json:"-"`
	ReporterID     uuid.UUID  `json:"-"`
	OrganizationID uuid.UUID  `json:"-"`
}

type UpdateUserStoryRequest struct {
	Title          *string    `json:"title" binding:"omitempty,min=3,max=255"`
	Description    *string    `json:"description"`
	Priority       *string    `json:"priority" binding:"omitempty,oneof=low medium high critical"`
	Status         *string    `json:"status" binding:"omitempty,oneof=todo in_progress in_review testing completed blocked"`
	StoryPoints    *int       `json:"story_points" binding:"omitempty,min=0"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	SprintID       *uuid.UUID `json:"sprint_id"`
	UserStoryID    uuid.UUID  `json:"-"`
	ProjectID      uuid.UUID  `json:"-"`
	UserID         uuid.UUID  `json:"-"`
	OrganizationID uuid.UUID  `json:"-"`
}

type UserStoryFilter struct {
	response.PaginationQuery
	response.SortQuery
	Status   string `form:"status"`
	Assignee string `form:"assignee_id"`
	Reporter string `form:"reporter_id"`
	Sprint   string `form:"sprint_id"`
	Search   string `form:"search"`
	Priority string `form:"priority"`
}

type ReorderUserStoriesRequest struct {
	StoryIDs       []uuid.UUID `json:"story_ids" binding:"required,min=1"`
	ProjectID      uuid.UUID   `json:"-"`
	UserID         uuid.UUID   `json:"-"`
	OrganizationID uuid.UUID   `json:"-"`
}

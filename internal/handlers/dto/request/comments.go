package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type CreateCommentsRequest struct {
	TaskID          *uuid.UUID `json:"-" swaggerignore:"true"`
	UserStoryID     *uuid.UUID `json:"-" swaggerignore:"true"`
	UserID          uuid.UUID  `json:"-" swaggerignore:"true"`
	OrganizationID  uuid.UUID  `json:"-" swaggerignore:"true"`
	Content         string     `json:"content" binding:"required,min=1,max=5000"`
	ParentCommentID *uuid.UUID `json:"parent_comment_id,omitempty"`
}

type UpdateCommentsRequest struct {
	CommentID      uuid.UUID  `json:"-" swaggerignore:"true"`
	TaskID         *uuid.UUID `json:"-" swaggerignore:"true"`
	UserStoryID    *uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID  `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID  `json:"-" swaggerignore:"true"`
	Content        string     `json:"content" binding:"required"`
}

type DeleteComments struct {
	CommentID      uuid.UUID  `json:"-" swaggerignore:"true"`
	TaskID         *uuid.UUID `json:"-" swaggerignore:"true"`
	UserStoryID    *uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID  `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID  `json:"-" swaggerignore:"true"`
}

type GetComments struct {
	response.PaginationQuery
	CommentID      uuid.UUID  `json:"-" swaggerignore:"true"`
	TaskID         *uuid.UUID `json:"-" swaggerignore:"true"`
	UserStoryID    *uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID  `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID  `json:"-" swaggerignore:"true"`
}

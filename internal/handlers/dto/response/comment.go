package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type CommentsResponse struct {
	ID              uuid.UUID  `json:"id"`
	TaskID          uuid.UUID  `json:"task_id"`
	UserID          uuid.UUID  `json:"user_id"`
	UserName        string     `json:"user_name"`
	Content         string     `json:"content"`
	ParentCommentID *uuid.UUID `json:"parent_comment_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	IsDeleted       bool       `json:"is_deleted"`
}

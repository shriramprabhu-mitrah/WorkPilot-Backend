package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type CommentsResponse struct {
	ID              uuid.UUID                   `json:"id"`
	TaskID          uuid.UUID                   `json:"task_id"`
	UserID          uuid.UUID                   `json:"user_id"`
	UserName        string                      `json:"user_name"`
	FullName        string                      `json:"full_name"`
	Email           string                      `json:"email"`
	AvatarURL       string                      `json:"avatar_url,omitempty"`
	Content         string                      `json:"content"`
	ParentCommentID *uuid.UUID                  `json:"parent_comment_id,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
	IsDeleted       bool                        `json:"is_deleted"`
	ParentComment   *ParentUserResponse         `json:"parent_comment,omitempty"`
	Attachments     []CommentAttachmentResponse `json:"attachments,omitempty"`
}

type ParentUserResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsDeleted bool      `json:"is_deleted"`
}

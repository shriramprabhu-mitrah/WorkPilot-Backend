package response

import (
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

type AttachmentResponse struct {
	ID               uuid.UUID `json:"id"`
	TaskID           uuid.UUID `json:"task_id"`
	OriginalFilename string    `json:"original_filename"`
	MIMEType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	URL              string    `json:"url,omitempty"`
	UploadedBy       uuid.UUID `json:"uploaded_by"`
	UploadedAt       time.Time `json:"uploaded_at"`
}

func AttachmentFromModel(a models.TaskAttachment) AttachmentResponse {
	return AttachmentResponse{
		ID:               a.ID,
		TaskID:           a.TaskID,
		OriginalFilename: a.OriginalFilename,
		MIMEType:         a.MIMEType,
		FileSize:         a.FileSize,
		URL:              a.URL,
		UploadedBy:       a.UploadedBy,
		UploadedAt:       a.UploadedAt,
	}
}

type CommentAttachmentResponse struct {
	ID               uuid.UUID `json:"id"`
	CommentID        uuid.UUID `json:"comment_id"`
	OriginalFilename string    `json:"original_filename"`
	MIMEType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	URL              string    `json:"url,omitempty"`
	UploadedBy       uuid.UUID `json:"uploaded_by"`
	UploadedAt       time.Time `json:"uploaded_at"`
}

func CommentAttachmentFromModel(a models.CommentAttachment) CommentAttachmentResponse {
	return CommentAttachmentResponse{
		ID:               a.ID,
		CommentID:        a.CommentID,
		OriginalFilename: a.OriginalFilename,
		MIMEType:         a.MIMEType,
		FileSize:         a.FileSize,
		URL:              a.URL,
		UploadedBy:       a.UploadedBy,
		UploadedAt:       a.UploadedAt,
	}
}

type UserStoryAttachmentResponse struct {
	ID               uuid.UUID `json:"id"`
	UserStoryID      uuid.UUID `json:"user_story_id"`
	OriginalFilename string    `json:"original_filename"`
	MIMEType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	URL              string    `json:"url,omitempty"`
	UploadedBy       uuid.UUID `json:"uploaded_by"`
	UploadedAt       time.Time `json:"uploaded_at"`
}

func UserStoryAttachmentFromModel(a models.UserStoryAttachment) UserStoryAttachmentResponse {
	return UserStoryAttachmentResponse{
		ID:               a.ID,
		UserStoryID:      a.UserStoryID,
		OriginalFilename: a.OriginalFilename,
		MIMEType:         a.MIMEType,
		FileSize:         a.FileSize,
		URL:              a.URL,
		UploadedBy:       a.UploadedBy,
		UploadedAt:       a.UploadedAt,
	}
}

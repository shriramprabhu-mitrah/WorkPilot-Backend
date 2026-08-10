package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type CommentAttachment struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	CommentID        uuid.UUID `json:"comment_id" gorm:"type:uuid;not null;index"`
	OriginalFilename string    `json:"original_filename" gorm:"type:varchar(255);not null"`
	StoredFilename   string    `json:"stored_filename" gorm:"type:varchar(255);not null"`
	MIMEType         string    `json:"mime_type" gorm:"type:varchar(100);not null"`
	FileSize         int64     `json:"file_size" gorm:"type:bigint;not null"`
	StoragePath      string    `json:"storage_path" gorm:"type:text;not null"`
	UploadedBy       uuid.UUID `json:"uploaded_by" gorm:"type:uuid;not null;index"`
	UploadedAt       time.Time `json:"uploaded_at" gorm:"type:timestamptz;not null"`
}

func (a *CommentAttachment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	if a.UploadedAt.IsZero() {
		a.UploadedAt = time.Now()
	}
	return
}

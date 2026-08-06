package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Comments struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	TaskID          uuid.UUID      `json:"task_id" gorm:"type:uuid;not null;index"`
	UserID          uuid.UUID      `json:"user_id" gorm:"type:uuid;not null;index"`
	User            User           `json:"user,omitempty" gorm:"foreignKey:UserID"`
	ProjectID       uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index"`
	Project         Project        `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	OrganizationID  uuid.UUID      `json:"organization_id" gorm:"type:uuid;not null;index"`
	Content         string         `json:"content" gorm:"type:text;not null"`
	ParentCommentID *uuid.UUID     `json:"parent_comment_id,omitempty" gorm:"type:uuid;index"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
	Replies         []Comments     `json:"replies,omitempty" gorm:"foreignKey:ParentCommentID"`
}

func (c *Comments) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

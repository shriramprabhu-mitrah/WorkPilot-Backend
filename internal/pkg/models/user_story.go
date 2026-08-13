package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type UserStory struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID   uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index"`
	Project     Project        `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	SprintID    *uuid.UUID     `json:"sprint_id,omitempty" gorm:"type:uuid;index"`
	Sprint      *Sprint        `json:"sprint,omitempty" gorm:"foreignKey:SprintID"`
	Title       string         `json:"title" gorm:"type:varchar(255);not null"`
	Description string         `json:"description,omitempty" gorm:"type:text"`
	Priority    string         `json:"priority" gorm:"type:varchar(50);not null;default:'medium'"`
	Status      string         `json:"status" gorm:"type:varchar(50);not null;default:'todo'"`
	StoryPoints int            `json:"story_points" gorm:"type:integer;not null;default:0"`
	AssigneeID  *uuid.UUID     `json:"assignee_id,omitempty" gorm:"type:uuid;index"`
	ReporterID  uuid.UUID      `json:"reporter_id" gorm:"type:uuid;not null;index"`
	Assignee    *User          `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	Reporter    User           `json:"reporter" gorm:"foreignKey:ReporterID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (u *UserStory) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

const (
	FavoriteItemTypeUserStory = "user_story"
	FavoriteItemTypeTask      = "task"
)

type Favorite struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID      `json:"user_id" gorm:"type:uuid;not null;index"`
	User        *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	ItemType    string         `json:"item_type" gorm:"type:varchar(50);not null"`
	UserStoryID *uuid.UUID     `json:"user_story_id,omitempty" gorm:"type:uuid;index"`
	UserStory   *UserStory     `json:"user_story,omitempty" gorm:"foreignKey:UserStoryID"`
	TaskID      *uuid.UUID     `json:"task_id,omitempty" gorm:"type:uuid;index"`
	Task        *Task          `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (f *Favorite) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == uuid.Nil {
		f.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

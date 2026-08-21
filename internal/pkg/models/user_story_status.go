package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type UserStoryStatus struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index"`
	Project      Project        `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Name         string         `json:"name" gorm:"type:varchar(50);not null"`
	Color        string         `json:"color" gorm:"type:varchar(7);not null"`
	DisplayOrder int            `json:"display_order" gorm:"type:integer;not null"`
	IsDefault    bool           `json:"is_default" gorm:"not null;default:false"`
	IsClosed     bool           `json:"is_closed" gorm:"not null;default:false"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (uss *UserStoryStatus) BeforeCreate(tx *gorm.DB) (err error) {
	if uss.ID == uuid.Nil {
		uss.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

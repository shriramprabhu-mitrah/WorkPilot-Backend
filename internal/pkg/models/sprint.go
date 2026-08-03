package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Sprint struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID   uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;uniqueIndex:idx_project_sprint_name"`
	Project     Project        `json:"project,omitzero" gorm:"foreignKey:ProjectID"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_project_sprint_name"`
	Goal        string         `json:"goal,omitempty" gorm:"type:varchar(500)"`
	Status      string         `json:"status" gorm:"type:varchar(20);default:'planning'"`
	StartDate   time.Time      `json:"start_date" gorm:"type:date;not null"`
	EndDate     time.Time      `json:"end_date" gorm:"type:date;not null"`
	CreatedByID uuid.UUID      `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedBy   User           `json:"created_by,omitzero" gorm:"foreignKey:CreatedByID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (s *Sprint) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

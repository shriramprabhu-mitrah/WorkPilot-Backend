package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Label struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID uuid.UUID `json:"project_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_project_label_name"`
	Project   Project   `json:"project,omitzero" gorm:"foreignKey:ProjectID"`
	Name      string    `json:"name" gorm:"type:varchar(30);not null;uniqueIndex:idx_project_label_name"`
	Color     string    `json:"color" gorm:"type:varchar(7);not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (l *Label) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == uuid.Nil {
		l.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

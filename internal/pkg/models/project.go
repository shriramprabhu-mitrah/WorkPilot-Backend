package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Project struct {
	ID             uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID uuid.UUID      `json:"organization_id" gorm:"not null;index;uniqueIndex:idx_org_project_key"`
	Organization   Organization   `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	Name           string         `json:"name" gorm:"type:varchar(150);not null"`
	Description    string         `json:"description,omitempty" gorm:"type:text"`
	Status         string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedBy      uuid.UUID      `json:"created_by" gorm:"not null;index"`
	Creator        User           `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

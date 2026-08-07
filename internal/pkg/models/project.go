package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Project struct {
	ID             uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID uuid.UUID      `json:"organization_id" gorm:"not null"`
	Organization   Organization   `json:"organization,omitzero" gorm:"foreignKey:OrganizationID"`
	Name           string         `json:"name" gorm:"type:varchar(150);not null"`
	Description    string         `json:"description,omitempty" gorm:"type:text"`
	Status         string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedBy      uuid.UUID      `json:"created_by" gorm:"not null;index"`
	Creator        User           `json:"creator,omitzero" gorm:"foreignKey:CreatedBy"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	SprintCount    int            `json:"sprint_count" gorm:"-"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type ProjectMember struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectRole string    `json:"project_role" gorm:"type:varchar(50)"`
	ProjectID   uuid.UUID `json:"project_id" gorm:"type:uuid;not null"`
	Project     Project   `json:"project,omitzero" gorm:"foreignKey:ProjectID"`
	UserID      uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	User        User      `json:"user,omitzero" gorm:"foreignKey:UserID"`
	JoinedAt    time.Time `json:"joined_at" gorm:"not null"`
	AddedByID   uuid.UUID `json:"added_by_id" gorm:"type:uuid;not null"`
	AddedBy     User      `json:"added_by,omitzero" gorm:"foreignKey:AddedByID"`
	UpdatedAt   time.Time `json:"updated_at"`
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

func (p *ProjectMember) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

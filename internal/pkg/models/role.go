package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Role struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty" gorm:"type:uuid;index"`
	Name           string         `json:"name" gorm:"type:varchar(100);not null"`
	Description    string         `json:"description,omitempty" gorm:"type:text"`
	IsSystem       bool           `json:"is_system" gorm:"not null;default:false"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
	Permissions    []Permission   `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
}

type Permission struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Resource string    `json:"resource" gorm:"type:varchar(50);not null"`
	Action   string    `json:"action" gorm:"type:varchar(50);not null"`
}

type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
	}
	return
}

func (p *Permission) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID, err = uuid.NewV7()
	}
	return
}

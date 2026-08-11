package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID             uuid.UUID  `json:"id" gorm:"primaryKey;type:uuid"`
	UserID         *uuid.UUID `json:"user_id,omitempty" gorm:"type:uuid;index:idx_audit_logs_user_id"`
	User           User       `json:"user,omitzero" gorm:"foreignKey:UserID"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty" gorm:"type:uuid;index:idx_audit_logs_org_id"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty" gorm:"type:uuid;index:idx_audit_logs_project_id"`
	Action         string     `json:"action" gorm:"size:100;not null;index:idx_audit_logs_action"`
	ResourceType   string     `json:"resource_type" gorm:"size:50;not null;index:idx_audit_logs_resource_type"`
	ResourceID     string     `json:"resource_id" gorm:"size:255"`
	Details        string     `json:"details" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at" gorm:"not null;type:timestamptz"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		var err error
		a.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return nil
}

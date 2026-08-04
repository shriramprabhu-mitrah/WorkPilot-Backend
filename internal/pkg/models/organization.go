package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
)

type Organization struct {
	ID        uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	Name      string         `json:"name" gorm:"size:50;not null;unique;index:idx_organization_name"`
	CreatedBy uuid.UUID      `json:"created_by" gorm:"not null"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	Slug      string         `json:"slug" gorm:"size:50;not null;unique;uniqueIndex:idx_organization_slug"`
	Domain    string         `json:"domain" validate:"required" gorm:"size:150;not null"`
	Industry  string         `json:"industry" validate:"required" gorm:"size:150;not null"`
	TeamSize  string         `json:"team_size" validate:"required" gorm:"not null"`
	Country   string         `json:"country" validate:"required" gorm:"not null"`
	LogoURL   string         `json:"logo_url" gorm:"size:500"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index:idx_organization_deleted_at"`
}

type OrganizationInvitation struct {
	ID             uuid.UUID        `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID uuid.UUID        `json:"organization_id" gorm:"type:uuid;index:idx_org_invites_org_id;not null"`
	Organization   Organization     `json:"organization,omitzero" gorm:"foreignKey:OrganizationID"`
	Email          string           `json:"email" gorm:"size:100;not null;index:idx_org_invites_email"`
	Role           string           `json:"role" gorm:"size:30;not null"`
	Token          string           `json:"token" gorm:"size:255;not null;unique;index:idx_org_invites_token"`
	Status         InvitationStatus `json:"status" gorm:"size:20;not null;default:'pending'"`
	ExpiresAt      time.Time        `json:"expires_at" gorm:"not null;type:timestamptz"`
	AcceptedAt     *time.Time       `json:"accepted_at,omitempty" gorm:"type:timestamptz"`
	CreatedBy      uuid.UUID        `json:"created_by" gorm:"not null"`
	CreatedAt      time.Time        `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt      time.Time        `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt      gorm.DeletedAt   `json:"-" gorm:"index:idx_org_invites_deleted_at"`
}

func (r *Organization) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}
func (i *OrganizationInvitation) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		var err error
		i.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return nil
}

package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleSuperAdmin     Role = "super_admin"
	RoleOrgAdmin       Role = "org_admin"
	RoleProjectManager Role = "project_manager"
	RoleDeveloper      Role = "developer"
	RoleViewer         Role = "viewer"
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
	LogoURL   string         `json:"logo_url" validate:"required" gorm:"size:150;not null"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index:idx_organization_deleted_at"`
}

type User struct {
	ID             uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty" gorm:"type:uuid;index:idx_users_organization_id"`
	Organization   Organization   `json:"organization,omitzero"`
	FullName       string         `json:"name" gorm:"size:100;not null"`
	UserName       string         `json:"username" gorm:"column:username;size:30;not null;unique;index:idx_users_username"`
	Email          string         `json:"email" validate:"required,email" gorm:"size:100;not null;unique;index:idx_users_email"`
	PasswordHash   string         `json:"password_hash" validate:"required"`
	Role           string         `json:"role" gorm:"size:30;index:idx_users_role"`
	AvatarURL      string         `json:"avatar_url" gorm:"size:255"`
	Timezone       string         `json:"timezone" gorm:"size:50;default:'UTC'"`
	IsActive       bool           `json:"is_active" gorm:"default:false"`
	IsVerified     bool           `json:"is_verified" gorm:"default:false"`
	CreatedAt      time.Time      `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index:idx_users_deleted_at"`
}

type RefreshToken struct {
	ID        uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	UserID    uuid.UUID      `json:"user_id" gorm:"type:uuid;index:idx_refresh_tokens_user_id;not null;unique"`
	TokenHash string         `json:"token_hash" gorm:"size:255;not null;unique"`
	UserAgent *string        `json:"user_agent,omitempty" gorm:"type:text"`
	IPAddress *string        `json:"ip_address,omitempty" gorm:"size:45"`
	ExpiresAt time.Time      `json:"expires_at" gorm:"not null;type:timestamptz"`
	RevokedAt *time.Time     `json:"revoked_at,omitempty" gorm:"type:timestamptz"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index:idx_refresh_tokens_deleted_at"`
}

type PasswordResetOTP struct {
	ID        uuid.UUID      `json:"id" gorm:"primaryKey"`
	UserID    uuid.UUID      `json:"user_id" gorm:"index:idx_password_reset_otps_user_id;not null"`
	OTPHash   string         `json:"otp_hash" gorm:"column:otp_hash;size:255;not null"`
	ExpiresAt time.Time      `json:"expires_at" gorm:"not null"`
	UsedAt    *time.Time     `json:"used_at,omitempty" gorm:"index"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index:idx_password_reset_otps_deleted_at"`
}

type OrganizationInvitation struct {
	ID             uuid.UUID        `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID uuid.UUID        `json:"organization_id" gorm:"type:uuid;index:idx_org_invites_org_id;not null"`
	Organization   Organization     `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
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

type AuditLog struct {
	ID             uuid.UUID  `json:"id" gorm:"primaryKey;type:uuid"`
	UserID         *uuid.UUID `json:"user_id,omitempty" gorm:"type:uuid;index:idx_audit_logs_user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty" gorm:"type:uuid;index:idx_audit_logs_org_id"`
	Action         string     `json:"action" gorm:"size:100;not null;index:idx_audit_logs_action"`
	ResourceType   string     `json:"resource_type" gorm:"size:50;not null"`
	ResourceID     string     `json:"resource_id" gorm:"size:255"`
	Details        string     `json:"details" gorm:"type:text"`
	CreatedAt      time.Time  `json:"created_at" gorm:"not null;type:timestamptz"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

func (r *RefreshToken) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
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

func (p *PasswordResetOTP) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		var err error
		p.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return nil
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

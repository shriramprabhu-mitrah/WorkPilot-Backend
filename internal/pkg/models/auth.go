package models

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID             uuid.UUID      `json:"id" gorm:"primaryKey;type:uuid"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty" gorm:"type:uuid;index:idx_users_organization_id"`
	Organization   Organization   `json:"organization,omitzero"`
	FullName       string         `json:"name" gorm:"size:100;not null"`
	UserName       string         `json:"username" gorm:"column:username;size:30;not null;unique;index:idx_users_username"`
	Email          string         `json:"email" validate:"required,email" gorm:"size:100;not null;unique;index:idx_users_email"`
	PasswordHash   string         `json:"password_hash" validate:"required"`
	RoleID         uuid.UUID      `json:"role_id" gorm:"type:uuid;index:idx_users_role_id"`
	Role           Role           `json:"role,omitzero" gorm:"foreignKey:RoleID"`
	AvatarURL      string         `json:"avatar_url" gorm:"size:500"`
	Color          string         `json:"color" gorm:"size:7;not null;default:'#3498DB'"`
	Timezone       string         `json:"timezone" gorm:"size:50;default:'UTC'"`
	IsActive       bool           `json:"is_active"`
	IsVerified     bool           `json:"is_verified"`
	Status         string         `json:"status" gorm:"size:50;not null;default:'active'"`
	CreatedAt      time.Time      `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"type:timestamptz"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index:idx_users_deleted_at"`
	JoinedAt       time.Time      `json:"joined_at" gorm:"type:timestamptz"`
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

// GenerateRandomHexColor generates a random valid hex color of format #RRGGBB in uppercase
func GenerateRandomHexColor() string {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		return "#3498DB"
	}
	return "#" + strings.ToUpper(hex.EncodeToString(bytes))
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	if u.Color == "" {
		u.Color = GenerateRandomHexColor()
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

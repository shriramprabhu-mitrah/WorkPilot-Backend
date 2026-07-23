package dto

import (
	"fmt"

	"github.com/gofrs/uuid"
)

type Role string

const (
	RoleSuperAdmin     Role = "super_admin"
	RoleOrgAdmin       Role = "org_admin"
	RoleProjectManager Role = "project_manager"
	RoleDeveloper      Role = "developer"
	RoleViewer         Role = "viewer"
)

type AuthTokensResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

type SignInRequest struct {
	Email          string `json:"email" validate:"required,email"`
	Password       string `json:"password" validate:"required"`
	OrganizationID string `json:"organization_id,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	UserID       string `json:"user_id"`
}

type SignUpRequest struct {
	Email          string `json:"email" validate:"required,email"`
	Password       string `json:"password" validate:"required"`
	FullName       string `json:"full_name" validate:"required"`
	UserName       string `json:"username" validate:"required"`
	OrganizationID string `json:"organization_id,omitempty"`
	AvatarURL      string `json:"avatar_url"`
	Timezone       string `json:"timezone"`
}

type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email"`
	OTP         string `json:"otp" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

func (r Role) Validate() error {
	switch r {
	case RoleSuperAdmin,
		RoleOrgAdmin,
		RoleProjectManager,
		RoleDeveloper,
		RoleViewer:
		return nil
	default:
		return fmt.Errorf("invalid role: %s", r)
	}
}

type ChangePasswordRequest struct {
	UserID      uuid.UUID `json:"user_id"`
	OldPassword string    `json:"old_password"`
	NewPassword string    `json:"new_password"`
}

type UpdateUserRequest struct {
	FullName  string `json:"full_name"`
	UserName  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Timezone  string `json:"timezone"`
}

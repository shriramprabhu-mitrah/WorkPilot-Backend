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
	AccessToken        string `json:"access_token"`
	RefreshToken       string `json:"refresh_token"`
	TokenType          string `json:"token_type"`
	ExpiresIn          int    `json:"expires_in"`
	RefreshExpiresIn   int    `json:"refresh_expires_in"`
	MustChangePassword bool   `json:"must_change_password,omitempty"`
}

type SignInRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	OrganizationID string `json:"organization_id,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	UserID       string `json:"user_id"`
}

type SignUpRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required"`
	FullName  string `json:"full_name" binding:"required"`
	UserName  string `json:"username" binding:"required"`
	AvatarURL string `json:"avatar_url"`
	Timezone  string `json:"timezone"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

type ResendVerificationOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	OTP         string `json:"otp" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
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
		return fmt.Errorf("Invalid role: %s", r)
	}
}

type ChangePasswordRequest struct {
	UserID      uuid.UUID `json:"user_id"`
	OldPassword string    `json:"old_password" binding:"required"`
	NewPassword string    `json:"new_password" binding:"required"`
}

type UpdateUserRequest struct {
	FullName  string `json:"full_name"`
	UserName  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Timezone  string `json:"timezone"`
}

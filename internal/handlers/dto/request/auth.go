package request

import (
	"fmt"

	"github.com/gofrs/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleOrgAdmin   Role = "org_admin"
	RoleMember     Role = "member"
)

func (r Role) Validate() error {
	switch r {
	case RoleSuperAdmin,
		RoleOrgAdmin,
		RoleMember:
		return nil
	default:
		return fmt.Errorf("Invalid role: %s", r)
	}
}

type AuthTokensResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

type SignInRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	OrganizationID string `json:"-" swaggerignore:"true"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	UserID       string `json:"-" swaggerignore:"true"`
}

type SignUpRequest struct {
	Email     string `form:"email" binding:"required,email"`
	Password  string `form:"password" binding:"required"`
	FullName  string `form:"full_name" binding:"required"`
	UserName  string `form:"username" binding:"required"`
	AvatarURL string `form:"-"`
	Timezone  string `form:"timezone"`
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

type ChangePasswordRequest struct {
	UserID      uuid.UUID `json:"-" swaggerignore:"true"`
	OldPassword string    `json:"old_password" binding:"required"`
	NewPassword string    `json:"new_password" binding:"required"`
}

type UpdateUserRequest struct {
	FullName  string `form:"full_name"`
	UserName  string `form:"username"`
	AvatarURL string `form:"-"`
	Timezone  string `form:"timezone"`
}

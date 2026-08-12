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

type Platform string

const (
	PlatformWeb    Platform = "web"
	PlatformMobile Platform = "mobile"
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

// ValidatePlatform validates that the platform is one of the supported values.
// Returns an error if the platform is unsupported or empty.
func (p Platform) Validate() error {
	switch p {
	case PlatformWeb, PlatformMobile:
		return nil
	case "":
		return fmt.Errorf("Platform is required")
	default:
		return fmt.Errorf("Unsupported platform: %s", p)
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
	Email          string   `json:"email" binding:"required,email"`
	Password       string   `json:"password" binding:"required"`
	Platform       Platform `json:"-" swaggerignore:"true"` // Populated from X-Client-Platform header
	OrganizationID string   `json:"-" swaggerignore:"true"`
}

type RefreshTokenRequest struct {
	RefreshToken string   `json:"refresh_token" binding:"required"`
	Platform     Platform `json:"-" swaggerignore:"true"` // Populated from X-Client-Platform header
	UserID       string   `json:"-" swaggerignore:"true"`
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
	Email    string   `json:"email" binding:"required,email"`
	OTP      string   `json:"otp" binding:"required"`
	Platform Platform `json:"-" swaggerignore:"true"` // Populated from X-Client-Platform header
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
	FullName  *string `form:"full_name" json:"full_name"`
	UserName  *string `form:"username" json:"username"`
	AvatarURL *string `form:"avatar_url" json:"avatar_url"`
	Timezone  *string `form:"timezone" json:"timezone"`
}

package dto

import "github.com/gofrs/uuid"

type UpdateOrganizationRequest struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	LogoURL string `json:"logo_url"`
}

type CreateOrganizationRequest struct {
	Name    string `json:"name" validate:"required"`
	Domain  string `json:"domain" validate:"required"`
	LogoURL string `json:"logo_url" validate:"required"`
}

type UserStatusRequest struct {
	IsActive bool   `json:"is_active"`
	UserID   string `json:"user_id"`
}

type UpdateUserStatus struct {
	IsActive       bool      `json:"is_active"`
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id,omitempty"`
}

type InviteOrganizationMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token" validate:"required"`
}

package dto

import "github.com/gofrs/uuid"

type UpdateOrganizationRequest struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	LogoURL string `json:"logo_url"`
}

type CreateOrganizationRequest struct {
	Name     string `json:"name" binding:"required"`
	Slug     string `json:"slug" binding:"required"`
	Domain   string `json:"domain" binding:"required"`
	LogoURL  string `json:"logo_url" binding:"required"`
	Industry string `json:"industry" binding:"required"`
	TeamSize string `json:"team_size" binding:"required"`
	Country  string `json:"country" binding:"required"`
}

type UserStatusRequest struct {
	IsActive bool   `json:"is_active"`
	UserID   string `json:"user_id"`
}

type UpdateUserStatus struct {
	IsActive       bool       `json:"is_active"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

type UserRoleRequest struct {
	Role   Role   `json:"role"`
	UserID string `json:"user_id"`
}

type UpdateUserRole struct {
	Role           string     `json:"role"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

type RemoveUser struct {
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

type RemoveUserRequest struct {
	UserID string `json:"user_id"`
}

type InviteOrganizationMemberItem struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=org_admin project_manager developer viewer guest"`
}

type InviteOrganizationMemberRequest struct {
	Members []InviteOrganizationMemberItem `json:"members,omitempty" binding:"required,dive"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

package dto

import "github.com/gofrs/uuid"

type Industry string

const (
	IndustryInformationTechnology Industry = "Information_Technology"
	IndustryFinance               Industry = "Finance"
	IndustryHealthcare            Industry = "Healthcare"
	IndustryEducation             Industry = "Education"
	IndustryManufacturing         Industry = "Manufacturing"
	IndustryRetail                Industry = "Retail"
	IndustryRealEstate            Industry = "Real Estate"
	IndustryLogistics             Industry = "Logistics"
	IndustryHospitality           Industry = "Hospitality"
	IndustryOther                 Industry = "Other"
)

// TeamSize enum
type TeamSize string

const (
	TeamSize1To10     TeamSize = "1-10"
	TeamSize11To50    TeamSize = "11-50"
	TeamSize51To200   TeamSize = "51-200"
	TeamSize201To500  TeamSize = "201-500"
	TeamSize501To1000 TeamSize = "501-1000"
	TeamSize1000Plus  TeamSize = "1000+"
)

type UpdateOrganizationRequest struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	LogoURL string `json:"logo_url"`
}

type CreateOrganizationRequest struct {
	Name     string   `json:"name" binding:"required"`
	Slug     string   `json:"slug"`
	Domain   string   `json:"domain" binding:"required"`
	LogoURL  string   `json:"logo_url"`
	Industry Industry `json:"industry" binding:"required,oneof=Information_Technology Finance Healthcare Education Manufacturing Retail 'Real Estate' Logistics Hospitality Other"`
	TeamSize TeamSize `json:"team_size" binding:"required,oneof=1-10 11-50 51-200 201-500 501-1000 1000+"`
	Country  string   `json:"country" binding:"required"`
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

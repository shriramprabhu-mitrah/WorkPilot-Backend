package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

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
	Name      string   `form:"name"`
	Domain    string   `form:"domain"`
	TeamSize  TeamSize `form:"team_size" binding:"omitempty,oneof=1-10 11-50 51-200 201-500 501-1000 1000+"`
	CountryID string   `form:"country_id,omitempty" binding:"omitempty,uuid"`
}

type CreateOrganizationRequest struct {
	Name      string   `form:"name"      binding:"required"`
	Domain    string   `form:"domain"    binding:"required"`
	Industry  Industry `form:"industry"  binding:"required,oneof=Information_Technology Finance Healthcare Education Manufacturing Retail 'Real Estate' Logistics Hospitality Other"`
	TeamSize  TeamSize `form:"team_size" binding:"required,oneof=1-10 11-50 51-200 201-500 501-1000 1000+"`
	CountryID string   `form:"country_id" binding:"required,uuid"`
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
	RoleID string `json:"role_id"`
	Role   Role   `json:"role"`
	UserID string `json:"user_id"`
}

type UpdateUserRole struct {
	RoleID         uuid.UUID  `json:"role_id"`
	Role           string     `json:"role"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"-" swaggerignore:"true"`
}

type RemoveUser struct {
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

type RemoveUserRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type InviteOrganizationMemberItem struct {
	Email string `json:"email" binding:"required,email"`
}

type InviteOrganizationMemberRequest struct {
	Members []InviteOrganizationMemberItem `json:"members,omitempty" binding:"required,dive"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token" form:"token" binding:"required"`
}

type OrganizationMemberListFilter struct {
	response.PaginationQuery
	FullName         string `json:"full_name,omitempty"`
	Email            string `json:"email,omitempty"`
	Username         string `json:"username,omitempty"`
	Role             string `json:"role,omitempty"`
	IsActive         *bool  `json:"is_active,omitempty"`
	IsVerified       *bool  `json:"is_verified,omitempty"`
	Timezone         string `json:"timezone,omitempty"`
	IncludeOrgAdmins bool   `json:"include_org_admins,omitempty"`
}

type OrganizationFilterRequest struct {
	response.PaginationQuery
	response.SortQuery
	Name     string `form:"name"`
	Domain   string `form:"domain"`
	Industry string `form:"industry"`
	TeamSize string `form:"team_size"`
	Country  string `form:"country"`
	IsActive *bool  `form:"is_active"`
	Search   string `form:"search"`
}

type GlobalMemberListFilter struct {
	response.PaginationQuery
	response.SortQuery
	Search         string     `form:"search"`
	FullName       string     `form:"full_name"`
	Email          string     `form:"email"`
	Username       string     `form:"username"`
	Role           string     `form:"role"`
	OrganizationID *uuid.UUID `form:"organization_id"`
	IsActive       *bool      `form:"is_active"`
	IsVerified     *bool      `form:"is_verified"`
	Timezone       string     `form:"timezone"`
}

type UpdateOrganizationStatusRequest struct {
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	IsActive       *bool     `json:"is_active" binding:"required"`
}

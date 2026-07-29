package utils

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ms-kanban-server/internal/pkg/response"
	"gorm.io/gorm"
)

// IsDuplicateKeyError checks whether a database error is caused by a duplicate key / unique constraint violation.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "duplicate key") ||
		strings.Contains(errMsg, "unique constraint") ||
		strings.Contains(errMsg, "23505")
}

// ParseUserDuplicateError parses a duplicate key error for User operations and returns a specific response.Error.
func ParseUserDuplicateError(err error) *response.Error {
	if err == nil {
		return nil
	}
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "username") || strings.Contains(errMsg, "idx_users_username") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Username is already taken",
		}
	}

	if strings.Contains(errMsg, "email") || strings.Contains(errMsg, "idx_users_email") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "User with this email already exists",
		}
	}

	// Default conflict fallback if exact index could not be determined
	return &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "User with this email or username already exists",
	}
}

// ParseOrgDuplicateError parses a duplicate key error for Organization operations and returns a specific response.Error.
func ParseOrgDuplicateError(err error) *response.Error {
	if err == nil {
		return nil
	}
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "slug") || strings.Contains(errMsg, "domain") || strings.Contains(errMsg, "idx_organization_slug") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "An organization with this domain or slug already exists",
		}
	}

	if strings.Contains(errMsg, "name") || strings.Contains(errMsg, "idx_organization_name") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "An organization with this name already exists",
		}
	}

	return &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "Organization already exists",
	}
}

// ParseProjectDuplicateError parses a duplicate key error for Project operations
// and returns a specific response.Error.
func ParseProjectDuplicateError(err error) *response.Error {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())

	// Project key already exists within the organization
	if strings.Contains(errMsg, "idx_org_project_key") ||
		strings.Contains(errMsg, "project_key") ||
		strings.Contains(errMsg, "key") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Project key already exists in this organization",
		}
	}

	// Fallback
	return &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "Project already exists",
	}
}

// ParseProjectMemberDuplicateError parses a duplicate key error for ProjectMember operations
// and returns a specific response.Error.
func ParseProjectMemberDuplicateError(err error) *response.Error {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())

	// User is already a member of the project
	if strings.Contains(errMsg, "idx_project_member") ||
		strings.Contains(errMsg, "project_id") && strings.Contains(errMsg, "user_id") {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "User is already a member of this project",
		}
	}

	// Fallback
	return &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "Project member already exists",
	}
}

// ParseSprintDuplicateError parses a duplicate key error for Sprint operations
// and returns a specific response.Error.
func ParseSprintDuplicateError(err error) *response.Error {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())

	// Sprint name already exists within the project
	if strings.Contains(errMsg, "idx_project_sprint_name") ||
		(strings.Contains(errMsg, "project_id") && strings.Contains(errMsg, "name")) {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Sprint name already exists in this project",
		}
	}

	// Fallback
	return &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "Sprint already exists",
	}
}

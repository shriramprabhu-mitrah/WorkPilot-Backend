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

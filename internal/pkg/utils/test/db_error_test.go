package utils_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"gorm.io/gorm"
)

func TestIsDuplicateKeyError(t *testing.T) {
	if !utils.IsDuplicateKeyError(gorm.ErrDuplicatedKey) {
		t.Error("expected gorm.ErrDuplicatedKey to be detected as duplicate key error")
	}

	pgErr := errors.New("ERROR: duplicate key value violates unique constraint \"idx_users_email\" (SQLSTATE 23505)")
	if !utils.IsDuplicateKeyError(pgErr) {
		t.Error("expected postgres duplicate key error string to be detected")
	}

	otherErr := errors.New("connection reset by peer")
	if utils.IsDuplicateKeyError(otherErr) {
		t.Error("expected arbitrary error to not be detected as duplicate key error")
	}

	if utils.IsDuplicateKeyError(nil) {
		t.Error("expected nil error to return false")
	}
}

func TestParseUserDuplicateError(t *testing.T) {
	usernameErr := errors.New("duplicate key value violates unique constraint \"idx_users_username\"")
	parsedUsername := utils.ParseUserDuplicateError(usernameErr)
	if parsedUsername.Code != response.ErrConflict || parsedUsername.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict error, got %#v", parsedUsername)
	}
	if parsedUsername.Message != "Username is already taken" {
		t.Errorf("expected 'Username is already taken', got %q", parsedUsername.Message)
	}

	emailErr := errors.New("duplicate key value violates unique constraint \"idx_users_email\"")
	parsedEmail := utils.ParseUserDuplicateError(emailErr)
	if parsedEmail.Code != response.ErrConflict || parsedEmail.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict error, got %#v", parsedEmail)
	}
	if parsedEmail.Message != "User with this email already exists" {
		t.Errorf("expected 'User with this email already exists', got %q", parsedEmail.Message)
	}
}

func TestParseOrgDuplicateError(t *testing.T) {
	nameErr := errors.New("duplicate key value violates unique constraint \"idx_organization_name\"")
	parsedName := utils.ParseOrgDuplicateError(nameErr)
	if parsedName.Message != "An organization with this name already exists" {
		t.Errorf("expected 'An organization with this name already exists', got %q", parsedName.Message)
	}

	slugErr := errors.New("duplicate key value violates unique constraint \"idx_organization_slug\"")
	parsedSlug := utils.ParseOrgDuplicateError(slugErr)
	if parsedSlug.Message != "An organization with this domain or slug already exists" {
		t.Errorf("expected 'An organization with this domain or slug already exists', got %q", parsedSlug.Message)
	}
}

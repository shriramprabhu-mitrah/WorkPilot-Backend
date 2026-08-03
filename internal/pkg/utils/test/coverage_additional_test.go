package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
)

func TestUtilityHelpersAndParsersAdditionalCoverage(t *testing.T) {
	t.Run("password helpers", func(t *testing.T) {
		hash, err := utils.HashPassword("super-secret")
		if err != nil {
			t.Fatalf("expected hash to be generated, got %v", err)
		}
		if !utils.IsValidPassword(hash, "super-secret") {
			t.Fatal("expected password validation to succeed for the generated hash")
		}
		if utils.IsValidPassword(hash, "wrong-password") {
			t.Fatal("expected password validation to fail for wrong password")
		}
		if utils.ValidatePassword("short") {
			t.Fatal("expected short password to be rejected")
		}
		if !utils.ValidatePassword("longer-password") {
			t.Fatal("expected valid password length to pass")
		}
	})

	t.Run("validate key", func(t *testing.T) {
		if !utils.ValidateKey("ABC123") {
			t.Fatal("expected valid project key to pass")
		}
		if utils.ValidateKey("ab") {
			t.Fatal("expected invalid project key to be rejected")
		}
	})

	t.Run("string converters", func(t *testing.T) {
		id, err := utils.StringToUUID("")
		if err != nil {
			t.Fatalf("expected empty UUID string to return nil error, got %v", err)
		}
		if id != uuid.Nil {
			t.Fatalf("expected nil UUID for empty input, got %s", id)
		}

		validID := uuid.Must(uuid.NewV4())
		parsedID, err := utils.StringToUUID(validID.String())
		if err != nil {
			t.Fatalf("expected valid UUID to parse, got %v", err)
		}
		if parsedID != validID {
			t.Fatalf("expected parsed UUID %s, got %s", validID, parsedID)
		}

		_, badUUIDErr := utils.StringToUUID("not-a-uuid")
		if badUUIDErr == nil {
			t.Fatal("expected invalid UUID format to return an error")
		}
		if badUUIDErr.Code != response.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %s", badUUIDErr.Code)
		}

		intVal, intErr := utils.StringToInt("42")
		if intErr != nil {
			t.Fatalf("expected valid int string to parse, got %v", intErr)
		}
		if intVal != 42 {
			t.Fatalf("expected int 42, got %d", intVal)
		}

		_, badIntErr := utils.StringToInt("abc")
		if badIntErr == nil {
			t.Fatal("expected invalid int string to return an error")
		}
		if badIntErr.Code != response.ErrInternalServerError {
			t.Fatalf("expected ErrInternalServerError, got %s", badIntErr.Code)
		}

		boolVal, boolErr := utils.StringToBool("true")
		if boolErr != nil {
			t.Fatalf("expected valid bool string to parse, got %v", boolErr)
		}
		if !boolVal {
			t.Fatal("expected true string to parse to true")
		}

		_, badBoolErr := utils.StringToBool("not-bool")
		if badBoolErr == nil {
			t.Fatal("expected invalid bool string to return an error")
		}
		if badBoolErr.Code != response.ErrInternalServerError {
			t.Fatalf("expected ErrInternalServerError, got %s", badBoolErr.Code)
		}
	})

	t.Run("slug and time conversion", func(t *testing.T) {
		if got := utils.ExtractSlug("https://example.com/org/"); got != "org" {
			t.Fatalf("expected slug org, got %s", got)
		}

		tm, err := utils.StringToTime("2026-07-31")
		if err != nil {
			t.Fatalf("expected valid date to parse, got %v", err)
		}
		if tm == nil || tm.Format("2006-01-02") != "2026-07-31" {
			t.Fatalf("expected parsed time to match 2026-07-31, got %+v", tm)
		}

		_, invalidDateErr := utils.StringToTime("31-07-2026")
		if invalidDateErr == nil {
			t.Fatal("expected invalid date format to return an error")
		}
	})
}

func TestRenderEmbeddedTemplateErrorBranchesAdditionalCoverage(t *testing.T) {
	t.Run("rejects empty template name", func(t *testing.T) {
		_, err := utils.RenderEmbeddedTemplate("", map[string]string{"OTP": "123456"})
		if err == nil {
			t.Fatal("expected empty template name to return an error")
		}
	})

	t.Run("renders known template", func(t *testing.T) {
		rendered, err := utils.RenderEmbeddedTemplate("password_reset.html", map[string]any{"OTP": "123456", "ExpiryMinutes": 15})
		if err != nil {
			t.Fatalf("expected embedded template to render, got %v", err)
		}
		if !strings.Contains(rendered, "123456") {
			t.Fatalf("expected rendered template to include OTP, got %s", rendered)
		}
	})

	t.Run("returns error for missing template", func(t *testing.T) {
		_, err := utils.RenderEmbeddedTemplate("does_not_exist.html", nil)
		if err == nil {
			t.Fatal("expected missing template to return an error")
		}
	})
}

func TestParseDuplicateErrorHelpers(t *testing.T) {
	projectErr := errors.New("duplicate key value violates unique constraint \"idx_org_project_key\"")
	projectParsed := utils.ParseProjectDuplicateError(projectErr)
	if projectParsed == nil || projectParsed.Code != response.ErrConflict {
		t.Fatalf("expected conflict error, got %#v", projectParsed)
	}
	if projectParsed.Message != "Project key already exists in this organization" {
		t.Fatalf("expected project key conflict message, got %q", projectParsed.Message)
	}

	memberErr := errors.New("duplicate key value violates unique constraint \"idx_project_member\"")
	memberParsed := utils.ParseProjectMemberDuplicateError(memberErr)
	if memberParsed == nil || memberParsed.Code != response.ErrConflict {
		t.Fatalf("expected conflict error, got %#v", memberParsed)
	}
	if memberParsed.Message != "User is already a member of this project" {
		t.Fatalf("expected project member conflict message, got %q", memberParsed.Message)
	}

	sprintErr := errors.New("duplicate key value violates unique constraint \"idx_project_sprint_name\"")
	sprintParsed := utils.ParseSprintDuplicateError(sprintErr)
	if sprintParsed == nil || sprintParsed.Code != response.ErrConflict {
		t.Fatalf("expected conflict error, got %#v", sprintParsed)
	}
	if sprintParsed.Message != "Sprint name already exists in this project" {
		t.Fatalf("expected sprint conflict message, got %q", sprintParsed.Message)
	}
}

func TestValidationErrorMessageHandlesMoreBranchesAdditionalCoverage(t *testing.T) {
	validate := validator.New()

	t.Run("max branch", func(t *testing.T) {
		type payload struct {
			Name string `json:"full_name" validate:"max=5"`
		}
		err := validate.Struct(payload{Name: "abcdef"})
		msg := utils.ValidationErrorMessage(err, payload{})
		if msg != "Full Name must not exceed 5 characters." {
			t.Fatalf("expected max validation message, got %q", msg)
		}
	})

	t.Run("oneof branch", func(t *testing.T) {
		type payload struct {
			Status string `json:"status" validate:"oneof=active archived"`
		}
		err := validate.Struct(payload{Status: "pending"}) 
		msg := utils.ValidationErrorMessage(err, payload{})
		if msg != "Status must be one of active, archived." {
			t.Fatalf("expected oneof validation message, got %q", msg)
		}
	})

	t.Run("min branch", func(t *testing.T) {
		type payload struct {
			Name string `json:"full_name" validate:"min=3"`
		}
		err := validate.Struct(payload{Name: "ab"})
		msg := utils.ValidationErrorMessage(err, payload{})
		if msg != "Full Name must be at least 3 characters." {
			t.Fatalf("expected min validation message, got %q", msg)
		}
	})
}

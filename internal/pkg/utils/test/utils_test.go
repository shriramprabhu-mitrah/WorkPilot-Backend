package utils_test

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
)

type samplePayload struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"full_name" validate:"required"`
	Age   int    `json:"age"`
}

func TestValidationErrorMessage(t *testing.T) {
	validate := validator.New()

	t.Run("returns required message for missing email", func(t *testing.T) {
		payload := samplePayload{Name: "John"}
		err := validate.Struct(payload)
		msg := utils.ValidationErrorMessage(err, payload)
		if msg != "Email is required." {
			t.Fatalf("expected 'Email is required.', got %q", msg)
		}
	})

	t.Run("returns email message for invalid email", func(t *testing.T) {
		payload := samplePayload{Email: "invalid-email", Name: "John"}
		err := validate.Struct(payload)
		msg := utils.ValidationErrorMessage(err, payload)
		if msg != "Email must be a valid email address." {
			t.Fatalf("expected 'Email must be a valid email address.', got %q", msg)
		}
	})

	t.Run("returns json type error message", func(t *testing.T) {
		typeErr := &json.UnmarshalTypeError{
			Field: "age",
			Type:  reflect.TypeOf(0),
		}
		payload := samplePayload{}
		msg := utils.ValidationErrorMessage(typeErr, payload)
		if msg != "Invalid data type for Age. Expected int." {
			t.Fatalf("expected 'Invalid data type for Age. Expected int.', got %q", msg)
		}
	})

	t.Run("returns json format error message for EOF", func(t *testing.T) {
		payload := samplePayload{}
		msg := utils.ValidationErrorMessage(io.EOF, payload)
		if msg != "Invalid JSON request body format." {
			t.Fatalf("expected 'Invalid JSON request body format.', got %q", msg)
		}
	})
}

func TestUtilityHelpersAndParsers(t *testing.T) {
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
		if !utils.ValidatePassword("Longer-password1!") {
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

func TestValidationErrorMessageHandlesMoreBranches(t *testing.T) {
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

package utils_test

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
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

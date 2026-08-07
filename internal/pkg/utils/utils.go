package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
)

var projectKeyRegex = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)

const dateLayout = "2006-01-02"

func IsValidPassword(storedHash, enteredPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(enteredPassword)) == nil
}

func ValidatePassword(password string) bool {
	// Minimum length of 8
	if len(password) < 8 {
		return false
	}

	// No spaces allowed
	if regexp.MustCompile(`\s`).MatchString(password) {
		return false
	}

	// At least one uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)

	// At least one lowercase letter
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)

	// At least one digit
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)

	// At least one special character
	hasSpecial := regexp.MustCompile("[!@#$%^&*()_\\-+=\\[\\]{}|\\\\:;\"'<>,.?/~`]").MatchString(password)

	return hasUpper && hasLower && hasDigit && hasSpecial
}

func ValidateKey(projectKey string) bool {
	return projectKeyRegex.MatchString(projectKey)
}

func HashPassword(password string) (string, *response.Error) {

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			Message:    "Something went wrong. Please try again later.",
			StatusCode: http.StatusInternalServerError,
		}
		return "", &errorResponse
	}
	return string(bytes), nil
}

func StringToUUID(idStr string) (uuid.UUID, *response.Error) {

	if idStr == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.FromString(idStr)
	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid ID format",
		}
		return uuid.Nil, &errorResponse
	}
	return id, nil
}

func StringToInt(str string) (int, *response.Error) {

	num, err := strconv.Atoi(str)
	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
		return 0, &errorResponse
	}
	return num, nil
}

func StringToBool(str string) (bool, *response.Error) {

	b, err := strconv.ParseBool(str)
	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
		return false, &errorResponse
	}
	return b, nil
}

func ValidationErrorMessage(err error, payload any) string {
	if err == nil {
		return "Invalid request payload."
	}

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		fieldErr := verrs[0]
		label := validationFieldLabel(payload, fieldErr.StructField())

		switch fieldErr.Tag() {
		case "required":
			return fmt.Sprintf("%s is required.", label)
		case "email":
			return fmt.Sprintf("%s must be a valid email address.", label)
		case "max":
			return fmt.Sprintf("%s must not exceed %s characters.", label, fieldErr.Param())
		case "min":
			return fmt.Sprintf("%s must be at least %s characters.", label, fieldErr.Param())
		case "oneof":
			return fmt.Sprintf("%s must be one of %s.", label, strings.ReplaceAll(fieldErr.Param(), " ", ", "))
		default:
			return fmt.Sprintf("%s is invalid.", label)
		}
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		label := toTitleCase(typeErr.Field)
		if label == "" || label == "Field" {
			return fmt.Sprintf("Invalid data type for field '%s'. Expected %s.", typeErr.Field, typeErr.Type.String())
		}
		return fmt.Sprintf("Invalid data type for %s. Expected %s.", label, typeErr.Type.String())
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.EOF) {
		return "Invalid JSON request body format."
	}

	return "Invalid request payload."
}

func validationFieldLabel(payload any, structFieldName string) string {
	t := reflect.TypeOf(payload)
	if t == nil {
		return "Field"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "Field"
	}

	f, ok := t.FieldByName(structFieldName)
	if !ok {
		return toTitleCase(structFieldName)
	}

	tag := f.Tag.Get("json")
	if tag == "" {
		return toTitleCase(structFieldName)
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return toTitleCase(structFieldName)
	}

	parts := strings.Split(name, "_")
	for i, p := range parts {
		switch strings.ToLower(p) {
		case "id":
			parts[i] = "ID"
		case "otp":
			parts[i] = "OTP"
		case "url":
			parts[i] = "URL"
		default:
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func toTitleCase(s string) string {
	if s == "" {
		return "Field"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func ExtractSlug(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")

	parts := strings.Split(input, "/")
	return parts[len(parts)-1]
}

func StringToTime(str string) (*time.Time, error) {
	t, err := time.Parse(dateLayout, str)
	if err != nil {
		return nil, fmt.Errorf("Invalid time,Error %e. Expected format: YYYY-MM-DD ", err)
	}
	return &t, nil
}

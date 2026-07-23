package utils

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
)

func IsValidPassword(storedHash, enteredPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(enteredPassword)) == nil
}

func ValidatePassword(password string) bool {

	emailRegex := regexp.MustCompile(`^[a-z0-9A-Z._%+\-]{8,}$`)
	return emailRegex.MatchString(password)
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
	verrs, ok := err.(validator.ValidationErrors)
	if !ok || len(verrs) == 0 {
		return "Invalid request."
	}

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

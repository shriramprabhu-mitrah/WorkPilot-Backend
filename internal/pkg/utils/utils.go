package utils

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

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
			Message:    "Something went wrong",
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
			Message:    "Something went wrong",
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
			Message:    "Something went wrong",
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
			Message:    "Something went wrong",
		}
		return false, &errorResponse
	}
	return b, nil
}

func ExtractSlug(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimSuffix(input, "/")

	parts := strings.Split(input, "/")
	return parts[len(parts)-1]
}

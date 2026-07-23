package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
)

func GenerateJWT(tokencredentials dto.JWtcredentials, logger *zap.Logger) (string, *response.Error) {

	expiresIn, err := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if err != nil {
		logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", err)))
		return "", err
	}

	return generateJWT(tokencredentials, time.Duration(expiresIn)*time.Second, logger)
}

func generateJWT(tokencredentials dto.JWtcredentials, ttl time.Duration, logger *zap.Logger) (string, *response.Error) {

	var organizationID uuid.UUID
	var jwtKey = config.GetEnv("JWT_SECRET_KEY", "")

	expirationTime := time.Now().Add(ttl)

	if tokencredentials.OrganizationID == nil || *tokencredentials.OrganizationID == uuid.Nil {
		organizationID = uuid.Nil
	} else {
		organizationID = *tokencredentials.OrganizationID
	}

	claims := &dto.ClaimsJWT{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Role:           tokencredentials.Role,
		UserId:         tokencredentials.UserId,
		OrganizationID: organizationID,
	}

	//Generate the jwt token using the HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(jwtKey))
	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed generating the token",
		}
		logger.Error("Failed generating the token",
			zap.String("ERROR : ", fmt.Sprintf("%v", errorResponse)),
			zap.Error(err))
		return "", &errorResponse
	}

	return tokenString, nil
}

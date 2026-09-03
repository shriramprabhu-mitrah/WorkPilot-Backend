package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ms-kanban-server/config"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
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

// GenerateJWTWithPlatform generates a JWT token based on the client platform.
// For mobile clients, generates a token without an expiration claim.
// For web clients, generates a token with the configured expiration.
func GenerateJWTWithPlatform(tokencredentials dto.JWtcredentials, logger *zap.Logger) (string, *response.Error) {
	var ttl time.Duration

	// Mobile clients get a token without expiration
	if tokencredentials.Platform == string(dto.PlatformMobile) {
		ttl = 0 // 0 means no expiration
	} else {
		// Web clients use the configured expiration
		expiresIn, err := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
		if err != nil {
			logger.Error("Failed to set the expire time",
				zap.Error(fmt.Errorf("%v", err)))
			return "", err
		}
		ttl = time.Duration(expiresIn) * time.Second
	}

	return generateJWT(tokencredentials, ttl, logger)
}

func generateJWT(tokencredentials dto.JWtcredentials, ttl time.Duration, logger *zap.Logger) (string, *response.Error) {

	var organizationID uuid.UUID
	var jwtKey = config.GetEnv("JWT_SECRET_KEY", "")

	if tokencredentials.OrganizationID == nil || *tokencredentials.OrganizationID == uuid.Nil {
		organizationID = uuid.Nil
	} else {
		organizationID = *tokencredentials.OrganizationID
	}

	role := tokencredentials.Role
	if role == "" {
		logger.Warn("Token credentials role is empty during JWT generation, defaulting to developer",
			zap.String("userID", tokencredentials.UserID.String()))
		role = "developer"
	}

	claims := &dto.ClaimsJWT{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		Role:           role,
		UserID:         tokencredentials.UserID,
		OrganizationID: organizationID,
	}

	// Only set expiration if ttl is greater than 0
	if ttl > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(ttl))
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

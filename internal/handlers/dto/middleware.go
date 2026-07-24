package dto

import (
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
)

type ClaimsJWT struct {
	Role               string    `json:"role"`
	UserId             uuid.UUID `json:"user_id"`
	OrganizationID     uuid.UUID `json:"organization_id"`
	MustChangePassword bool      `json:"must_change_password"`
	jwt.RegisteredClaims
}

type JWtcredentials struct {
	Role               string     `json:"role"`
	UserId             uuid.UUID  `json:"user_id"`
	OrganizationID     *uuid.UUID `json:"organization_id"`
	MustChangePassword bool       `json:"must_change_password"`
}

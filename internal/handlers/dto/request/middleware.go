package request

import (
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
)

type ClaimsJWT struct {
	Role           string    `json:"role"`
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	jwt.RegisteredClaims
}

type JWtcredentials struct {
	Role           string     `json:"role"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Platform       string     `json:"platform"` // "web" or "mobile"
}

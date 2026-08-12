package services_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v5"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
)

// TestSignInWithWebPlatformGeneratesTokenWithExpiration verifies that web clients
// receive access tokens with an "exp" claim.
func TestSignInWithWebPlatformGeneratesTokenWithExpiration(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{
		user: models.User{
			ID:           uuid.Must(uuid.NewV7()),
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
			IsVerified:   true,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{
		Email:    "user@example.com",
		Password: "correct-password",
		Platform: dto.PlatformWeb,
	})

	if authErr != nil {
		t.Fatalf("expected successful login, got error: %v", authErr)
	}
	if result == nil {
		t.Fatalf("expected auth tokens, got nil")
	}

	// Parse the access token and verify it has an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedWebToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedWebToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedWebToken.Claims)
	}

	// Verify exp claim exists for web tokens
	if _, hasExp := (*claims)["exp"]; !hasExp {
		t.Fatal("expected exp claim in web platform token, but it's missing")
	}
}

// TestSignInWithMobilePlatformGeneratesTokenWithoutExpiration verifies that mobile
// clients receive access tokens without an "exp" claim.
func TestSignInWithMobilePlatformGeneratesTokenWithoutExpiration(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{
		user: models.User{
			ID:           uuid.Must(uuid.NewV7()),
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
			IsVerified:   true,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{
		Email:    "user@example.com",
		Password: "correct-password",
		Platform: dto.PlatformMobile,
	})

	if authErr != nil {
		t.Fatalf("expected successful login, got error: %v", authErr)
	}
	if result == nil {
		t.Fatalf("expected auth tokens, got nil")
	}

	// Parse the access token and verify it does NOT have an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedMobileToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedMobileToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedMobileToken.Claims)
	}

	// Verify exp claim does NOT exist for mobile tokens
	if _, hasExp := (*claims)["exp"]; hasExp {
		t.Fatal("expected no exp claim in mobile platform token, but it was found")
	}
}

// TestSignInWithMobilePlatformIncludesRequiredClaims verifies that mobile tokens
// still have the required claims (user_id, role, organization_id).
func TestSignInWithMobilePlatformIncludesRequiredClaims(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userID := uuid.Must(uuid.NewV7())
	repo := &stubAuthRepository{
		user: models.User{
			ID:           userID,
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
			IsVerified:   true,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{
		Email:    "user@example.com",
		Password: "correct-password",
		Platform: dto.PlatformMobile,
	})

	if authErr != nil {
		t.Fatalf("expected successful login, got error: %v", authErr)
	}

	// Parse the access token and verify required claims are present
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedClaimsToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedClaimsToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedClaimsToken.Claims)
	}

	// Verify required claims exist
	if _, hasUserID := (*claims)["user_id"]; !hasUserID {
		t.Fatal("expected user_id claim in token")
	}
	if _, hasRole := (*claims)["role"]; !hasRole {
		t.Fatal("expected role claim in token")
	}
	if _, hasOrgID := (*claims)["organization_id"]; !hasOrgID {
		t.Fatal("expected organization_id claim in token")
	}

	// Verify iat (issued at) is present
	if _, hasIat := (*claims)["iat"]; !hasIat {
		t.Fatal("expected iat claim in token")
	}
}

// TestRefreshTokenWithWebPlatformGeneratesTokenWithExpiration verifies that
// refreshing with web platform generates a token with expiration.
func TestRefreshTokenWithWebPlatformGeneratesTokenWithExpiration(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userID := uuid.Must(uuid.NewV7())
	tokenID := uuid.Must(uuid.NewV7())
	hashedRefreshToken, _ := utils.HashPassword("valid-refresh")
	futureTime := time.Now().Add(24 * time.Hour)

	repo := &stubAuthRepository{
		user: models.User{
			ID:           userID,
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
			IsVerified:   true,
		},
		refreshToken: models.RefreshToken{
			ID:        tokenID,
			UserID:    userID,
			TokenHash: hashedRefreshToken,
			ExpiresAt: futureTime,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.RefreshToken(dto.RefreshTokenRequest{
		RefreshToken: tokenID.String() + ".valid-refresh",
		Platform:     dto.PlatformWeb,
	})

	if authErr != nil {
		t.Fatalf("expected refresh to succeed, got error: %v", authErr)
	}

	// Parse the access token and verify it has an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedWebRefreshToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedWebRefreshToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedWebRefreshToken.Claims)
	}

	// Verify exp claim exists for web tokens
	if _, hasExp := (*claims)["exp"]; !hasExp {
		t.Fatal("expected exp claim in refreshed web platform token")
	}
}

// TestRefreshTokenWithMobilePlatformGeneratesTokenWithoutExpiration verifies that
// refreshing with mobile platform generates a token without expiration.
func TestRefreshTokenWithMobilePlatformGeneratesTokenWithoutExpiration(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userID := uuid.Must(uuid.NewV7())
	tokenID := uuid.Must(uuid.NewV7())
	hashedRefreshToken, _ := utils.HashPassword("valid-refresh")
	futureTime := time.Now().Add(24 * time.Hour)

	repo := &stubAuthRepository{
		user: models.User{
			ID:           userID,
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
			IsVerified:   true,
		},
		refreshToken: models.RefreshToken{
			ID:        tokenID,
			UserID:    userID,
			TokenHash: hashedRefreshToken,
			ExpiresAt: futureTime,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.RefreshToken(dto.RefreshTokenRequest{
		RefreshToken: tokenID.String() + ".valid-refresh",
		Platform:     dto.PlatformMobile,
	})

	if authErr != nil {
		t.Fatalf("expected refresh to succeed, got error: %v", authErr)
	}

	// Parse the access token and verify it does NOT have an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedMobileRefreshToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedMobileRefreshToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedMobileRefreshToken.Claims)
	}

	// Verify exp claim does NOT exist for mobile tokens
	if _, hasExp := (*claims)["exp"]; hasExp {
		t.Fatal("expected no exp claim in refreshed mobile platform token")
	}
}

// TestVerifyEmailWithWebPlatformGeneratesTokenWithExpiration verifies that
// email verification with web platform generates a token with expiration.
func TestVerifyEmailWithWebPlatformGeneratesTokenWithExpiration(t *testing.T) {
	hashedOTP, _ := utils.HashPassword("123456")
	userID := uuid.Must(uuid.NewV7())
	futureTime := time.Now().Add(15 * time.Minute)

	repo := &stubAuthRepository{
		user: models.User{
			ID:         userID,
			Email:      "user@example.com",
			Role:       "developer",
			IsActive:   true,
			IsVerified: false,
		},
		storedOTP: models.PasswordResetOTP{
			UserID:    userID,
			OTPHash:   hashedOTP,
			ExpiresAt: futureTime,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.VerifyEmail(dto.VerifyEmailRequest{
		Email:    "user@example.com",
		OTP:      "123456",
		Platform: dto.PlatformWeb,
	})

	if authErr != nil {
		t.Fatalf("expected email verification to succeed, got error: %v", authErr)
	}

	// Parse the access token and verify it has an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedWebVerifyToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedWebVerifyToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedWebVerifyToken.Claims)
	}

	// Verify exp claim exists for web tokens
	if _, hasExp := (*claims)["exp"]; !hasExp {
		t.Fatal("expected exp claim in web platform token from email verification")
	}
}

// TestVerifyEmailWithMobilePlatformGeneratesTokenWithoutExpiration verifies that
// email verification with mobile platform generates a token without expiration.
func TestVerifyEmailWithMobilePlatformGeneratesTokenWithoutExpiration(t *testing.T) {
	hashedOTP, _ := utils.HashPassword("123456")
	userID := uuid.Must(uuid.NewV7())
	futureTime := time.Now().Add(15 * time.Minute)

	repo := &stubAuthRepository{
		user: models.User{
			ID:         userID,
			Email:      "user@example.com",
			Role:       "developer",
			IsActive:   true,
			IsVerified: false,
		},
		storedOTP: models.PasswordResetOTP{
			UserID:    userID,
			OTPHash:   hashedOTP,
			ExpiresAt: futureTime,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.VerifyEmail(dto.VerifyEmailRequest{
		Email:    "user@example.com",
		OTP:      "123456",
		Platform: dto.PlatformMobile,
	})

	if authErr != nil {
		t.Fatalf("expected email verification to succeed, got error: %v", authErr)
	}

	// Parse the access token and verify it does NOT have an exp claim
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedMobileVerifyToken, parseErr := parser.ParseWithClaims(result.AccessToken, &jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse access token: %v", parseErr)
	}

	claims, ok := parsedMobileVerifyToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedMobileVerifyToken.Claims)
	}

	// Verify exp claim does NOT exist for mobile tokens
	if _, hasExp := (*claims)["exp"]; hasExp {
		t.Fatal("expected no exp claim in mobile platform token from email verification")
	}
}

// TestPlatformValidation tests the Platform.Validate() method
func TestPlatformValidation(t *testing.T) {
	tests := []struct {
		name        string
		platform    dto.Platform
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid web platform",
			platform:    dto.PlatformWeb,
			expectError: false,
		},
		{
			name:        "valid mobile platform",
			platform:    dto.PlatformMobile,
			expectError: false,
		},
		{
			name:        "empty platform",
			platform:    "",
			expectError: true,
			errorMsg:    "Platform is required",
		},
		{
			name:        "unsupported platform",
			platform:    "desktop",
			expectError: true,
			errorMsg:    "Unsupported platform: desktop",
		},
		{
			name:        "invalid platform case sensitive",
			platform:    "Web", // Should be lowercase
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.platform.Validate()
			if tt.expectError && err == nil {
				t.Errorf("expected error for platform %q, got nil", tt.platform)
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error for platform %q, got %v", tt.platform, err)
			}
			if tt.expectError && tt.errorMsg != "" && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

// TestGenerateJWTWithPlatformWeb tests JWT generation for web platform
func TestGenerateJWTWithPlatformWeb(t *testing.T) {
	credentials := dto.JWtcredentials{
		Role:     "developer",
		UserID:   uuid.Must(uuid.NewV7()),
		Platform: string(dto.PlatformWeb),
	}

	accessToken, err := middleware.GenerateJWTWithPlatform(credentials, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedWebPlatformToken, parseErr := parser.ParseWithClaims(accessToken, &jwt.MapClaims{}, func(jwtToken *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse generated JWT: %v", parseErr)
	}

	claims, ok := parsedWebPlatformToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedWebPlatformToken.Claims)
	}

	// Verify exp claim exists for web tokens
	if _, hasExp := (*claims)["exp"]; !hasExp {
		t.Fatal("expected exp claim in web platform JWT")
	}

	// Verify required claims
	if _, hasUserID := (*claims)["user_id"]; !hasUserID {
		t.Fatal("expected user_id claim")
	}
	if _, hasRole := (*claims)["role"]; !hasRole {
		t.Fatal("expected role claim")
	}
}

// TestGenerateJWTWithPlatformMobile tests JWT generation for mobile platform
func TestGenerateJWTWithPlatformMobile(t *testing.T) {
	credentials := dto.JWtcredentials{
		Role:     "developer",
		UserID:   uuid.Must(uuid.NewV7()),
		Platform: string(dto.PlatformMobile),
	}

	accessToken, err := middleware.GenerateJWTWithPlatform(credentials, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsedMobilePlatformToken, parseErr := parser.ParseWithClaims(accessToken, &jwt.MapClaims{}, func(jwtToken *jwt.Token) (any, error) {
		return []byte(""), nil
	})
	if parseErr != nil {
		t.Fatalf("failed to parse generated JWT: %v", parseErr)
	}

	claims, ok := parsedMobilePlatformToken.Claims.(*jwt.MapClaims)
	if !ok {
		t.Fatalf("expected *MapClaims, got %T", parsedMobilePlatformToken.Claims)
	}

	// Verify exp claim does NOT exist for mobile tokens
	if _, hasExp := (*claims)["exp"]; hasExp {
		t.Fatal("expected no exp claim in mobile platform JWT")
	}

	// Verify required claims still exist
	if _, hasUserID := (*claims)["user_id"]; !hasUserID {
		t.Fatal("expected user_id claim")
	}
	if _, hasRole := (*claims)["role"]; !hasRole {
		t.Fatal("expected role claim")
	}

	// Verify iat (issued at) is present
	if _, hasIat := (*claims)["iat"]; !hasIat {
		t.Fatal("expected iat claim in mobile token")
	}
}

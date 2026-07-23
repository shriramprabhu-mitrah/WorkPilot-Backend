package services

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
)

type stubAuthRepository struct {
	user                 models.User
	userByEmail          map[string]models.User
	refreshToken         models.RefreshToken
	otp                  models.PasswordResetOTP
	err                  *response.Error
	storedOTP            models.PasswordResetOTP
	updatedPasswordHash  string
	revokedRefreshTokens bool
	storedOTPs           []models.PasswordResetOTP
	emailExists          bool
	usernameExists       bool
}

func (s *stubAuthRepository) GetByEmail(email string) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	if s.userByEmail != nil {
		if user, ok := s.userByEmail[email]; ok {
			return user, nil
		}
		return models.User{}, &response.Error{Code: response.ErrUnauthorized, StatusCode: http.StatusUnauthorized, Message: "User not found"}
	}
	if email == s.user.Email {
		return s.user, nil
	}
	return models.User{}, &response.Error{Code: response.ErrUnauthorized, StatusCode: http.StatusUnauthorized, Message: "User not found"}
}

func (s *stubAuthRepository) GetByID(id uuid.UUID) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	return s.user, nil
}

func (s *stubAuthRepository) CreateUser(row models.User) *response.Error {
	return nil
}

func (s *stubAuthRepository) StoreRefreshToken(token models.RefreshToken) *response.Error {
	return nil
}

func (s *stubAuthRepository) GetRefreshToken(userID string) (models.RefreshToken, *response.Error) {
	if s.err != nil {
		return models.RefreshToken{}, s.err
	}
	return s.refreshToken, nil
}

func (s *stubAuthRepository) ExistsByEmail(email string) (bool, *response.Error) {
	if s.err != nil {
		return false, s.err
	}
	return s.emailExists, nil
}

func (s *stubAuthRepository) ExistsByUsername(username string) (bool, *response.Error) {
	if s.err != nil {
		return false, s.err
	}
	return s.usernameExists, nil
}

func (s *stubAuthRepository) ChangePassword(tokenHash string, userID uuid.UUID) *response.Error {
	if s.err != nil {
		return s.err
	}
	return nil
}
func (s *stubAuthRepository) UpdateUser(userID uuid.UUID, req models.User) *response.Error {
	if s.err != nil {
		return s.err
	}
	s.user = req
	return nil
}

func (s *stubAuthRepository) RequestPasswordReset(email string) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	return s.user, nil
}

func (s *stubAuthRepository) SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error {
	s.storedOTP = otp
	s.storedOTPs = append(s.storedOTPs, otp)
	return nil
}

func (s *stubAuthRepository) InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error {
	return nil
}

func (s *stubAuthRepository) GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	if s.err != nil {
		return models.PasswordResetOTP{}, s.err
	}
	if !utils.IsValidPassword(s.otp.OTPHash, otp) {
		return models.PasswordResetOTP{}, &response.Error{Code: response.ErrUnauthorized, StatusCode: http.StatusUnauthorized, Message: "Invalid OTP", Details: []response.Details{{Field: "otp", Message: "The provided OTP is invalid or expired"}}}
	}
	return s.otp, nil
}

func (s *stubAuthRepository) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {
	s.updatedPasswordHash = passwordHash
	return nil
}

func (s *stubAuthRepository) RevokeRefreshTokens(userID uuid.UUID) *response.Error {
	s.revokedRefreshTokens = true
	return nil
}

func TestSignInReturnsUnauthorizedForInvalidPassword(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{
		user: models.User{ID: uuid.Must(uuid.NewV4()),
			Email:        "user@example.com",
			PasswordHash: hash,
			Role:         "developer",
			IsActive:     true,
		},
	}
	service := InitAuthService(repo, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{Email: "user@example.com", Password: "wrong-password"})
	if authErr == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if authErr.StatusCode != 401 {
		t.Fatalf("expected 401 status, got %d", authErr.StatusCode)
	}
	if result != nil {
		t.Fatalf("expected no auth tokens, got %#v", result)
	}
}

func TestSignInRejectsInactiveUser(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "user@example.com", PasswordHash: hash, Role: "developer", IsActive: false}}
	service := InitAuthService(repo, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{Email: "user@example.com", Password: "correct-password"})
	if authErr == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if authErr.StatusCode != 403 {
		t.Fatalf("expected 403 status, got %d", authErr.StatusCode)
	}
	if result != nil {
		t.Fatalf("expected no auth tokens, got %#v", result)
	}
}

func TestSignInReturnsAuthTokensForValidCredentials(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV7()), Email: "user@example.com", PasswordHash: hash, Role: "developer", IsActive: true}}
	service := InitAuthService(repo, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{Email: "user@example.com", Password: "correct-password"})
	if authErr != nil {
		t.Fatalf("expected successful login, got error: %v", authErr)
	}
	if result == nil {
		t.Fatalf("expected auth tokens, got nil")
	}
	if result.AccessToken == "" {
		t.Fatal("expected access token to be populated")
	}
	if result.RefreshToken == "" {
		t.Fatal("expected refresh token to be populated")
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", result.TokenType)
	}
}

func TestRefreshTokenReturnsNewAccessTokenForValidRefreshToken(t *testing.T) {
	refreshHash, err := utils.HashPassword("valid-refresh")
	if err != nil {
		t.Fatalf("failed to hash refresh token: %v", err)
	}

	repo := &stubAuthRepository{
		user:         models.User{ID: uuid.Must(uuid.NewV7()), Email: "user@example.com", Role: "developer", IsActive: true},
		refreshToken: models.RefreshToken{UserID: uuid.Must(uuid.NewV7()), TokenHash: refreshHash, ExpiresAt: time.Now().Add(7 * 24 * time.Hour)},
	}
	service := InitAuthService(repo, zap.NewNop())

	result, authErr := service.RefreshToken(dto.RefreshTokenRequest{RefreshToken: "valid-refresh"})
	if authErr != nil {
		t.Fatalf("expected refresh to succeed, got error: %v", authErr)
	}
	if result == nil {
		t.Fatal("expected access token payload, got nil")
	}
	if result.AccessToken == "" {
		t.Fatal("expected access token to be populated")
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", result.TokenType)
	}
}

func TestResetPasswordUpdatesHashAndRevokesTokens(t *testing.T) {
	hashedOTP, hashErr := utils.HashPassword("123456")
	if hashErr != nil {
		t.Fatalf("failed to hash otp: %v", hashErr)
	}

	repo := &stubAuthRepository{
		user: models.User{ID: uuid.Must(uuid.NewV7()), Email: "user@example.com", IsActive: true},
		otp:  models.PasswordResetOTP{UserID: uuid.Must(uuid.NewV7()), OTPHash: hashedOTP, ExpiresAt: time.Now().Add(15 * time.Minute)},
	}
	service := InitAuthService(repo, zap.NewNop()).(*authservice)

	resetErr := service.ResetPassword(dto.ResetPasswordRequest{Email: "user@example.com", OTP: "123456", NewPassword: "NewPassword123!"})
	if resetErr != nil {
		t.Fatalf("expected password reset to succeed, got error: %v", resetErr)
	}
	if repo.updatedPasswordHash == "" {
		t.Fatal("expected password hash to be updated")
	}
	if !repo.revokedRefreshTokens {
		t.Fatal("expected refresh tokens to be revoked")
	}
	if repo.storedOTP.UsedAt == nil {
		t.Fatal("expected otp to be marked as used")
	}
}

func TestIsEmailAvailableReturnsTrueWhenEmailDoesNotExist(t *testing.T) {
	repo := &stubAuthRepository{emailExists: false}
	service := InitAuthService(repo, zap.NewNop())

	available, err := service.IsEmailAvailable("new@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !available {
		t.Fatal("expected email to be available")
	}
}

func TestIsUsernameAvailableReturnsFalseWhenUsernameAlreadyExists(t *testing.T) {
	repo := &stubAuthRepository{usernameExists: true}
	service := InitAuthService(repo, zap.NewNop())

	available, err := service.IsUsernameAvailable("existinguser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if available {
		t.Fatal("expected username to be unavailable")
	}
}

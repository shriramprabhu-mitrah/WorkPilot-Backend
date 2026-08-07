package services_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var InitAuthService = services.InitAuthService

type stubAuthRepository struct {
	user                    models.User
	userByEmail             map[string]models.User
	refreshToken            models.RefreshToken
	otp                     models.PasswordResetOTP
	err                     *response.Error
	storedOTP               models.PasswordResetOTP
	updatedPasswordHash     string
	revokedRefreshTokens    bool
	storedOTPs              []models.PasswordResetOTP
	emailExists             bool
	usernameExists          bool
	createdUser             models.User
	savedVerificationOTP    models.PasswordResetOTP
	verifiedUserID          uuid.UUID
	createOrganizationCalls int
	updateUserCalls         int
	createdOrganization     models.Organization
	invitation              models.OrganizationInvitation
	invitationErr           *response.Error
}

func (s *stubAuthRepository) GetByEmail(email string) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	if s.userByEmail != nil {
		if user, ok := s.userByEmail[strings.ToLower(email)]; ok {
			return user, nil
		}
		return models.User{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User not found"}
	}
	if strings.EqualFold(email, s.user.Email) {
		return s.user, nil
	}
	return models.User{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User not found"}
}

func (s *stubAuthRepository) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	if s.user.ID == id {
		return s.user, nil
	}
	return models.User{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User not found"}
}

func (s *stubAuthRepository) CreateUser(row models.User) *response.Error {
	if row.ID == uuid.Nil {
		row.ID = uuid.Must(uuid.NewV4())
	}
	s.createdUser = row
	s.user = row
	return nil
}

func (s *stubAuthRepository) CreateOrganization(row models.Organization) *response.Error {
	s.createdOrganization = row
	s.createOrganizationCalls++
	return nil
}

func (s *stubAuthRepository) GetOrganizationByName(name string) (models.Organization, *response.Error) {
	if s.err != nil {
		return models.Organization{}, s.err
	}
	return models.Organization{Name: name}, nil
}

func (s *stubAuthRepository) StoreRefreshToken(token models.RefreshToken) (models.RefreshToken, *response.Error) {
	s.refreshToken = token
	return token, nil
}

func (s *stubAuthRepository) GetRefreshToken(userID string) (models.RefreshToken, *response.Error) {
	if s.err != nil {
		return models.RefreshToken{}, s.err
	}
	return s.refreshToken, nil
}

func (s *stubAuthRepository) GetRefreshTokenByID(id uuid.UUID) (models.RefreshToken, *response.Error) {
	if s.err != nil {
		return models.RefreshToken{}, s.err
	}
	if s.refreshToken.ID == id {
		return s.refreshToken, nil
	}
	return models.RefreshToken{}, &response.Error{Code: response.ErrUnauthorized, StatusCode: http.StatusUnauthorized, Message: "Authentication required"}
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
	s.updateUserCalls++
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
		return models.PasswordResetOTP{}, &response.Error{Code: response.ErrBadRequest, StatusCode: http.StatusBadRequest, Message: "Invalid OTP"}
	}
	return s.otp, nil
}

func (s *stubAuthRepository) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {
	s.updatedPasswordHash = passwordHash
	return nil
}

func (s *stubAuthRepository) SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error {
	s.savedVerificationOTP = otp
	return nil
}

func (s *stubAuthRepository) InvalidateEmailVerificationOTPs(userID uuid.UUID) *response.Error {
	return nil
}

func (s *stubAuthRepository) GetEmailVerificationOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	if s.err != nil {
		return models.PasswordResetOTP{}, s.err
	}
	if !utils.IsValidPassword(s.otp.OTPHash, otp) {
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "The provided OTP is invalid or expired",
		}
	}
	return s.otp, nil
}

func (s *stubAuthRepository) MarkUserEmailVerified(userID uuid.UUID) *response.Error {
	s.verifiedUserID = userID
	return nil
}

func (s *stubAuthRepository) StoreUserTemp(row models.User) *response.Error {
	s.createdUser = row
	s.user = row
	return nil
}

func (s *stubAuthRepository) IsEmailVerificationResendAllowed(email string, interval time.Duration) (bool, *response.Error) {
	if s.err != nil {
		return false, s.err
	}
	return true, nil
}

func (s *stubAuthRepository) RecordEmailVerificationResend(email string, sentAt time.Time) *response.Error {
	return nil
}

func (s *stubAuthRepository) RevokeRefreshTokens(userID uuid.UUID) *response.Error {
	s.revokedRefreshTokens = true
	return nil
}

func (s *stubAuthRepository) GetUserFromRedis(email string) (*models.User, *response.Error) {

	return nil, nil
}

func (s *stubAuthRepository) GetPendingInvitationByEmail(email string) (models.OrganizationInvitation, *response.Error) {
	if s.invitationErr != nil {
		return models.OrganizationInvitation{}, s.invitationErr
	}
	if strings.EqualFold(s.invitation.Email, email) {
		return s.invitation, nil
	}
	return models.OrganizationInvitation{}, nil
}

func (s *stubAuthRepository) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
	if s.err != nil {
		return s.err
	}
	s.invitation = invitation
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
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.SignIn(dto.SignInRequest{Email: "user@example.com", Password: "wrong-password"})
	if authErr == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if authErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", authErr.StatusCode)
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

	repo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "user@example.com", PasswordHash: hash, Role: "developer", IsActive: false, IsVerified: true}}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

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

func TestSignInRejectsUnverifiedUser(t *testing.T) {
	hash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "user@example.com", PasswordHash: hash, Role: "developer", IsActive: true, IsVerified: false}}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

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

	repo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV7()), Email: "user@example.com", PasswordHash: hash, Role: "developer", IsActive: true, IsVerified: true}}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

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

func TestSignUpCreatesUnverifiedAccountAndSendsVerificationOTP(t *testing.T) {
	repo := &stubAuthRepository{}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	err := service.SignUp(dto.SignUpRequest{Email: "new@example.com", Password: "StrongPass123!", FullName: "Jane Doe", UserName: "janedoe"})
	if err != nil {
		t.Fatalf("expected signup to succeed, got error: %v", err)
	}
	if repo.createdUser.IsVerified {
		t.Fatal("expected newly created user to be unverified")
	}
	if repo.createdUser.IsActive {
		t.Fatal("expected newly created user account to be inactive until verified")
	}
	if repo.savedVerificationOTP.UserID != repo.createdUser.ID {
		t.Fatalf("expected verification OTP for user %s, got %s", repo.createdUser.ID, repo.savedVerificationOTP.UserID)
	}
}

func TestSignUpDoesNotCreateOrganizationDuringInitialRegistration(t *testing.T) {
	repo := &stubAuthRepository{}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	err := service.SignUp(dto.SignUpRequest{Email: "new@example.com", Password: "StrongPass123!", FullName: "Jane Doe", UserName: "janedoe"})
	if err != nil {
		t.Fatalf("expected signup to succeed, got error: %v", err)
	}
	if repo.createOrganizationCalls != 0 {
		t.Fatalf("expected no organization creation during signup, got %d calls", repo.createOrganizationCalls)
	}
	if repo.createdUser.OrganizationID != nil {
		t.Fatalf("expected user organization to remain unset during signup, got %v", repo.createdUser.OrganizationID)
	}
}

func TestRefreshTokenReturnsNewAccessTokenForValidRefreshToken(t *testing.T) {
	refreshHash, err := utils.HashPassword("valid-refresh")
	if err != nil {
		t.Fatalf("failed to hash refresh token: %v", err)
	}

	userID := uuid.Must(uuid.NewV7())
	tokenID := uuid.Must(uuid.NewV7())
	repo := &stubAuthRepository{
		user: models.User{ID: userID, Email: "user@example.com", Role: "developer", IsActive: true, IsVerified: true},
		refreshToken: models.RefreshToken{
			ID:        tokenID,
			UserID:    userID,
			TokenHash: refreshHash,
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	result, authErr := service.RefreshToken(dto.RefreshTokenRequest{RefreshToken: tokenID.String() + ".valid-refresh"})
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
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

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
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

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
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	available, err := service.IsUsernameAvailable("existinguser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if available {
		t.Fatal("expected username to be unavailable")
	}
}

func TestSignUpReturnsConflictForDuplicateEmail(t *testing.T) {
	repo := &stubAuthRepository{emailExists: true}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	err := service.SignUp(dto.SignUpRequest{Email: "existing@example.com", Password: "StrongPassword123!", FullName: "John", UserName: "johnny"})
	if err == nil {
		t.Fatal("expected error for duplicate email signup, got nil")
	}
	if err.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict status, got %d", err.StatusCode)
	}
	if err.Message != "User with this email already exists" {
		t.Fatalf("expected 'User with this email already exists', got %q", err.Message)
	}
}

func TestSignUpReturnsConflictForDuplicateUsername(t *testing.T) {
	repo := &stubAuthRepository{emailExists: false, usernameExists: true}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	err := service.SignUp(dto.SignUpRequest{Email: "new@example.com", Password: "StrongPassword123!", FullName: "John", UserName: "existinguser"})
	if err == nil {
		t.Fatal("expected error for duplicate username signup, got nil")
	}
	if err.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict status, got %d", err.StatusCode)
	}
	if err.Message != "Username is already taken" {
		t.Fatalf("expected 'Username is already taken', got %q", err.Message)
	}
}

func TestSignUpRejectsShortPassword(t *testing.T) {
	repo := &stubAuthRepository{}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	err := service.SignUp(dto.SignUpRequest{Email: "new@example.com", Password: "short", FullName: "John", UserName: "johnny"})
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request status, got %d", err.StatusCode)
	}
	if err.Message != "Password must be at least 8 characters long and include uppercase, lowercase, number, and special character with no spaces." {
		t.Fatalf("expected 'Password must be at least 8 characters long and include uppercase, lowercase, number, and special character with no spaces.', got %q", err.Message)
	}
}

func TestChangePasswordVerifiesOldPassword(t *testing.T) {
	correctHash, _ := utils.HashPassword("OldPassword123")
	userUUID := uuid.Must(uuid.NewV7())
	repo := &stubAuthRepository{
		user: models.User{ID: userUUID, PasswordHash: correctHash},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	// Test incorrect old password
	wrongErr := service.ChangePassword(dto.ChangePasswordRequest{UserID: userUUID, OldPassword: "WrongPassword123", NewPassword: "NewPassword123!"})
	if wrongErr == nil {
		t.Fatal("expected error when old password is incorrect, got nil")
	}
	if wrongErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized status, got %d", wrongErr.StatusCode)
	}
	if wrongErr.Message != "Current password is incorrect" {
		t.Fatalf("expected 'Current password is incorrect', got %q", wrongErr.Message)
	}

	// Test correct old password
	validErr := service.ChangePassword(dto.ChangePasswordRequest{UserID: userUUID, OldPassword: "OldPassword123", NewPassword: "NewPassword123!"})
	if validErr != nil {
		t.Fatalf("expected password change to succeed, got error: %v", validErr)
	}
}

func TestSignIn_AutoActivatesUserWithPendingInvitation(t *testing.T) {
	correctHash, _ := utils.HashPassword("password123")
	userUUID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	repo := &stubAuthRepository{
		user: models.User{
			ID:           userUUID,
			Email:        "inactive-invited@example.com",
			PasswordHash: correctHash,
			Role:         "member",
			IsActive:     false,
			IsVerified:   true,
		},
		invitation: models.OrganizationInvitation{
			ID:             uuid.Must(uuid.NewV4()),
			OrganizationID: orgID,
			Email:          "inactive-invited@example.com",
			Role:           "member",
			Status:         models.InvitationStatusPending,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			Token:          "invite-token-abc",
		},
	}
	auditRepo := &stubAuditLogRepo{}
	service := InitAuthService(repo, auditRepo, zap.NewNop())

	resp, err := service.SignIn(dto.SignInRequest{
		Email:    "inactive-invited@example.com",
		Password: "password123",
	})

	if err != nil {
		t.Fatalf("expected successful login and auto-activation, got error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected auth tokens response, got nil")
	}

	if !repo.user.IsActive {
		t.Fatal("expected user to be activated (IsActive=true)")
	}
	if repo.invitation.Status != models.InvitationStatusAccepted {
		t.Fatalf("expected invitation status to be accepted, got %s", repo.invitation.Status)
	}
	if len(auditRepo.createdLogs) != 1 {
		t.Fatalf("expected 1 audit log to be created, got %d", len(auditRepo.createdLogs))
	}
}

func TestSignIn_RejectsInactiveUserWithoutPendingInvitation(t *testing.T) {
	correctHash, _ := utils.HashPassword("password123")
	userUUID := uuid.Must(uuid.NewV4())

	repo := &stubAuthRepository{
		user: models.User{
			ID:           userUUID,
			Email:        "inactive-blocked@example.com",
			PasswordHash: correctHash,
			Role:         "member",
			IsActive:     false,
		},
	}
	service := InitAuthService(repo, &stubAuditLogRepo{}, zap.NewNop())

	_, err := service.SignIn(dto.SignInRequest{
		Email:    "inactive-blocked@example.com",
		Password: "password123",
	})

	if err == nil {
		t.Fatal("expected sign-in to be rejected for inactive user without invitation, got nil")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", err.StatusCode)
	}
	if err.Message != "Your account has been deactivated. Please contact support." {
		t.Fatalf("unexpected error message: %s", err.Message)
	}
}


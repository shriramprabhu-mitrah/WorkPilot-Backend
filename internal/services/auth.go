package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/middleware"
	mail "github.com/ms-kanban-server/internal/pkg/email"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"go.uber.org/zap"
)

type AuthService interface {
	SignIn(credentials dto.SignInRequest) (*dto.AuthTokensResponse, *response.Error)
	RefreshToken(credentials dto.RefreshTokenRequest) (*dto.AuthTokensResponse, *response.Error)
	SignUp(credentials dto.SignUpRequest) *response.Error
	VerifyEmail(credentials dto.VerifyEmailRequest) (*dto.AuthTokensResponse, *response.Error)
	ResendVerificationOTP(email string) *response.Error
	Logout(UserID string) *response.Error
	ChangePassword(payload dto.ChangePasswordRequest) *response.Error
	RequestPasswordReset(email string) *response.Error
	ResetPassword(credentials dto.ResetPasswordRequest) *response.Error
	UpdateUser(payload dto.UpdateUserRequest, userID uuid.UUID) *response.Error
	GetUser(userID uuid.UUID) (models.User, *response.Error)
	IsEmailAvailable(email string) (bool, *response.Error)
	IsUsernameAvailable(username string) (bool, *response.Error)
}

func InitAuthService(repo authrepo.AuthRepository, logger *zap.Logger) AuthService {
	return &authService{
		Repo:   repo,
		logger: logger,
	}
}

type authService struct {
	Repo   authrepo.AuthRepository
	logger *zap.Logger
}

func (s *authService) SignIn(credentials dto.SignInRequest) (*dto.AuthTokensResponse, *response.Error) {

	result, err := s.Repo.GetByEmail(credentials.Email)
	if err != nil {
		return nil, err
	}

	var organizationID uuid.UUID

	if result.OrganizationID == nil || *result.OrganizationID == uuid.Nil {
		organizationID = uuid.Nil
	} else {
		organizationID = *result.OrganizationID
	}

	if !utils.IsValidPassword(result.PasswordHash, credentials.Password) {
		s.logger.Error("Login failed due to incorrect password",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid email or password",
		}
	}

	if !result.IsActive {
		s.logger.Error("The account is deactivated or locked",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Your account has been deactivated. Please contact support.",
		}
	}

	if !result.IsVerified {
		s.logger.Error("Login rejected for unverified email",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Email address must be verified before login",
		}
	}

	tokencredentials := dto.JWtcredentials{
		Role:           result.Role,
		UserId:         result.ID,
		OrganizationID: &organizationID,
	}

	//generating the JWT token
	accessToken, tokenErr := middleware.GenerateJWT(tokencredentials, s.logger)
	if tokenErr != nil {
		return nil, tokenErr
	}

	//generating the Refresh Token Value
	refreshTokenValue, refreshTokenErr := generateRefreshTokenValue()
	if refreshTokenErr != nil {
		s.logger.Error("Failed to create refresh token",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	hashedRefreshToken, hashErr := utils.HashPassword(refreshTokenValue)
	if hashErr != nil {
		s.logger.Error("Failed hashing the password",
			zap.String("email", credentials.Email),
			zap.Error(fmt.Errorf("%v", hashErr)))
		return nil, hashErr
	}

	expiresIn, err := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if err != nil {
		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", err)))
		return nil, err
	}

	refreshExpiresIn, err := utils.StringToInt(config.GetEnv("REFRESH_TOKEN_EXPIRY", "604800"))
	if err != nil {
		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", err)))
		return nil, err
	}

	expiresAt := time.Now().Add(time.Duration(refreshExpiresIn) * time.Second)

	storeErr := s.Repo.StoreRefreshToken(models.RefreshToken{
		UserID:    result.ID,
		TokenHash: hashedRefreshToken,
		ExpiresAt: expiresAt,
	})
	if storeErr != nil {
		return nil, storeErr
	}

	return &dto.AuthTokensResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshTokenValue,
		TokenType:        "Bearer",
		ExpiresIn:        expiresIn,
		RefreshExpiresIn: refreshExpiresIn,
	}, nil
}

func generateRefreshTokenValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *authService) RefreshToken(credentials dto.RefreshTokenRequest) (*dto.AuthTokensResponse, *response.Error) {

	oldToken, err := s.Repo.GetRefreshToken(credentials.UserID)
	if err != nil {
		return nil, err
	}

	if !utils.IsValidPassword(oldToken.TokenHash, credentials.RefreshToken) {
		s.logger.Error("The given refresh token is wrong",
			zap.String("UserID", credentials.UserID))
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Authentication required",
		}
	}

	if time.Now().After(oldToken.ExpiresAt) {
		s.logger.Error("Refresh token expired",
			zap.String("UserID", credentials.UserID))
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Session has expired. Please sign in again.",
		}
	}

	user, userErr := s.Repo.GetByID(oldToken.UserID)
	if userErr != nil {
		return nil, userErr
	}
	var organizationID uuid.UUID

	if user.OrganizationID == nil || *user.OrganizationID == uuid.Nil {
		organizationID = uuid.Nil
	} else {
		organizationID = *user.OrganizationID
	}

	if !user.IsActive {
		s.logger.Error("The account is deactivated or locked",
			zap.String("UserID", user.ID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Please verify your email address before signing in",
		}
	}
	tokencredentials := dto.JWtcredentials{
		Role:           user.Role,
		UserId:         user.ID,
		OrganizationID: &organizationID,
	}

	//Generate Jwt token
	accessToken, tokenErr := middleware.GenerateJWT(tokencredentials, s.logger)
	if tokenErr != nil {
		return nil, tokenErr
	}

	expiresIn, err := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if err != nil {

		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", err)))
		return nil, err
	}

	return &dto.AuthTokensResponse{
		AccessToken:      accessToken,
		RefreshToken:     credentials.RefreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        expiresIn,
		RefreshExpiresIn: int(time.Until(oldToken.ExpiresAt).Seconds()),
	}, nil
}

func (s *authService) RequestPasswordReset(email string) *response.Error {

	user, err := s.Repo.RequestPasswordReset(email)
	if err != nil {
		return err
	}

	otpValue := generateOTP(6)
	otpExpiryMinutes, parseErr := strconv.Atoi(config.GetEnv("OTP_EXPIRY_MINUTES", "15"))
	if parseErr != nil || otpExpiryMinutes <= 0 {
		otpExpiryMinutes = 15
	}

	expiresAt := time.Now().Add(time.Duration(otpExpiryMinutes) * time.Minute)
	hashedOTP, hashErr := utils.HashPassword(otpValue)
	if hashErr != nil {
		s.logger.Error("Failed hashing the OTP",
			zap.Error(fmt.Errorf("%v", hashErr)))
		return hashErr
	}

	otpRecord := models.PasswordResetOTP{
		UserID:    user.ID,
		OTPHash:   hashedOTP,
		ExpiresAt: expiresAt,
	}

	if invalidateErr := s.Repo.InvalidatePasswordResetOTPs(user.ID); invalidateErr != nil {
		return invalidateErr
	}

	if saveErr := s.Repo.SavePasswordResetOTP(otpRecord); saveErr != nil {
		return saveErr
	}

	if err := mail.SendPasswordResetOTP(user.Email, otpValue); err != nil {
		s.logger.Error("Failed to send password reset OTP",
			zap.String("email", user.Email),
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	s.logger.Info("Password reset OTP generated",
		zap.String("email", user.Email))
	return nil
}

func generateOTP(length int) string {
	chars := "0123456789"
	result := make([]byte, length)
	for i := range length {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

func (s *authService) ResetPassword(credentials dto.ResetPasswordRequest) *response.Error {

	if !utils.ValidatePassword(credentials.NewPassword) {
		s.logger.Error("Validation failure in Password")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Password must be at least 8 characters long",
		}
	}

	user, err := s.Repo.RequestPasswordReset(credentials.Email)
	if err != nil {
		return err
	}

	otpRecord, otpErr := s.Repo.GetPasswordResetOTP(user.ID, credentials.OTP)
	if otpErr != nil {
		return otpErr
	}
	if otpRecord.ExpiresAt.Before(time.Now()) || otpRecord.UsedAt != nil || !utils.IsValidPassword(otpRecord.OTPHash, credentials.OTP) {
		s.logger.Error("")
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Invalid or expired OTP",
		}
	}

	passwordHash, hashErr := utils.HashPassword(credentials.NewPassword)
	if hashErr != nil {
		s.logger.Error("Failed hashing the password",
			zap.Error(fmt.Errorf("%v", hashErr)))
		return hashErr
	}

	if updateErr := s.Repo.UpdateUserPassword(user.ID, passwordHash); updateErr != nil {
		return updateErr
	}

	if revokeErr := s.Repo.RevokeRefreshTokens(user.ID); revokeErr != nil {
		return revokeErr
	}

	usedAt := time.Now()
	otpRecord.UsedAt = &usedAt
	if saveErr := s.Repo.SavePasswordResetOTP(otpRecord); saveErr != nil {
		return saveErr
	}

	s.logger.Info("Password reset completed",
		zap.String("email", credentials.Email))
	return nil
}

func (s *authService) SignUp(credentials dto.SignUpRequest) *response.Error {

	cleanEmail := strings.ToLower(strings.TrimSpace(credentials.Email))
	cleanUsername := strings.TrimSpace(credentials.UserName)

	if !utils.ValidatePassword(credentials.Password) {
		s.logger.Error("Validation failure in Password before signup")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Password must be at least 8 characters long",
		}
	}

	emailExists, emailErr := s.Repo.ExistsByEmail(cleanEmail)
	if emailErr != nil {
		return emailErr
	}
	if emailExists {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "User with this email already exists",
		}
	}

	usernameExists, usernameErr := s.Repo.ExistsByUsername(cleanUsername)
	if usernameErr != nil {
		return usernameErr
	}
	if usernameExists {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Username is already taken",
		}
	}

	passwordhash, errorResponse := utils.HashPassword(credentials.Password)
	if errorResponse != nil {
		s.logger.Error("Failed Hashing Password", zap.Error(fmt.Errorf("%v", errorResponse)))
		return errorResponse
	}

	result := models.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        cleanEmail,
		PasswordHash: passwordhash,
		FullName:     strings.TrimSpace(credentials.FullName),
		UserName:     cleanUsername,
		AvatarURL:    credentials.AvatarURL,
		Timezone:     credentials.Timezone,
		IsActive:     false,
		IsVerified:   false,
	}

	if createErr := s.Repo.StoreUserTemp(result); createErr != nil {
		return createErr
	}

	if otpErr := s.sendEmailVerificationOTP(result.ID, result.Email); otpErr != nil {
		return otpErr
	}

	s.logger.Info("User registered successfully", zap.String("email", result.Email))
	return nil
}

func (s *authService) VerifyEmail(credentials dto.VerifyEmailRequest) (*dto.AuthTokensResponse, *response.Error) {
	user, err := s.Repo.GetUserFromRedis(credentials.Email)
	if err != nil {
		return nil, err
	}

	otpRecord, otpErr := s.Repo.GetEmailVerificationOTP(user.ID, credentials.OTP)
	if otpErr != nil {
		return nil, otpErr
	}

	if otpRecord.ExpiresAt.Before(time.Now()) || otpRecord.UsedAt != nil || !utils.IsValidPassword(otpRecord.OTPHash, credentials.OTP) {
		s.logger.Error("Verification OTP rejected", zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "The provided OTP is invalid or expired",
		}
	}

	var organizationID uuid.UUID
	if user.OrganizationID == nil || *user.OrganizationID == uuid.Nil {
		organizationID = uuid.Nil
	} else {
		organizationID = *user.OrganizationID
	}

	user.IsVerified = true

	err = s.Repo.CreateUser(*user)
	if err != nil {
		return nil, err
	}

	tokencredentials := dto.JWtcredentials{
		Role:           user.Role,
		UserId:         user.ID,
		OrganizationID: &organizationID,
	}

	accessToken, tokenErr := middleware.GenerateJWT(tokencredentials, s.logger)
	if tokenErr != nil {
		return nil, tokenErr
	}

	refreshTokenValue, refreshTokenErr := generateRefreshTokenValue()
	if refreshTokenErr != nil {
		s.logger.Error("Failed to create refresh token after email verification", zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	hashedRefreshToken, hashErr := utils.HashPassword(refreshTokenValue)
	if hashErr != nil {
		s.logger.Error("Failed hashing the refresh token after email verification", zap.String("email", credentials.Email), zap.Error(fmt.Errorf("%v", hashErr)))
		return nil, hashErr
	}

	expiresIn, parseErr := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if parseErr != nil {
		s.logger.Error("Failed to set the expire time", zap.Error(fmt.Errorf("%v", parseErr)))
		return nil, parseErr
	}

	refreshExpiresIn, refreshParseErr := utils.StringToInt(config.GetEnv("REFRESH_TOKEN_EXPIRY", "604800"))
	if refreshParseErr != nil {
		s.logger.Error("Failed to set the expire time", zap.Error(fmt.Errorf("%v", refreshParseErr)))
		return nil, refreshParseErr
	}

	expiresAt := time.Now().Add(time.Duration(refreshExpiresIn) * time.Second)
	if storeErr := s.Repo.StoreRefreshToken(models.RefreshToken{UserID: user.ID, TokenHash: hashedRefreshToken, ExpiresAt: expiresAt}); storeErr != nil {
		return nil, storeErr
	}

	s.logger.Info("Email verification completed", zap.String("email", credentials.Email))
	return &dto.AuthTokensResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshTokenValue,
		TokenType:        "Bearer",
		ExpiresIn:        expiresIn,
		RefreshExpiresIn: refreshExpiresIn,
	}, nil
}

func (s *authService) ResendVerificationOTP(email string) *response.Error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.Repo.GetByEmail(cleanEmail)
	if err != nil {
		return err
	}

	if user.IsVerified {
		s.logger.Warn("Resend verification OTP requested for verified account", zap.String("email", cleanEmail))
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Email address is already verified",
		}
	}

	allowed, rateLimitErr := s.Repo.IsEmailVerificationResendAllowed(cleanEmail, time.Minute)
	if rateLimitErr != nil {
		return rateLimitErr
	}
	if !allowed {
		s.logger.Warn("Verification OTP resend rate limit exceeded", zap.String("email", cleanEmail))
		return &response.Error{
			Code:       response.ErrRateLimitExceeded,
			StatusCode: http.StatusTooManyRequests,
			Message:    "Please wait before requesting another verification code",
		}
	}

	if recordErr := s.Repo.RecordEmailVerificationResend(cleanEmail, time.Now()); recordErr != nil {
		return recordErr
	}

	if otpErr := s.sendEmailVerificationOTP(user.ID, cleanEmail); otpErr != nil {
		return otpErr
	}

	s.logger.Info("Verification OTP resent", zap.String("email", cleanEmail))
	return nil
}

func (s *authService) sendEmailVerificationOTP(userID uuid.UUID, email string) *response.Error {
	otpValue := generateOTP(6)
	otpExpiryMinutes, parseErr := strconv.Atoi(config.GetEnv("OTP_EXPIRY_MINUTES", "15"))
	if parseErr != nil || otpExpiryMinutes <= 0 {
		otpExpiryMinutes = 15
	}

	expiresAt := time.Now().Add(time.Duration(otpExpiryMinutes) * time.Minute)
	hashedOTP, hashErr := utils.HashPassword(otpValue)
	if hashErr != nil {
		s.logger.Error("Failed hashing the OTP", zap.Error(fmt.Errorf("%v", hashErr)))
		return hashErr
	}

	otpRecord := models.PasswordResetOTP{UserID: userID, OTPHash: hashedOTP, ExpiresAt: expiresAt}

	if invalidateErr := s.Repo.InvalidateEmailVerificationOTPs(userID); invalidateErr != nil {
		return invalidateErr
	}

	if saveErr := s.Repo.SaveEmailVerificationOTP(otpRecord); saveErr != nil {
		return saveErr
	}

	if err := mail.SendEmailVerificationOTP(email, otpValue, otpExpiryMinutes); err != nil {
		s.logger.Warn("Failed to send email verification OTP; continuing with registration", zap.String("email", email), zap.Error(err))
		return nil
	}

	s.logger.Info("Email verification OTP generated", zap.String("email", email))
	return nil
}

func (s *authService) Logout(UserID string) *response.Error {

	oldToken, err := s.Repo.GetRefreshToken(UserID)
	if err != nil {
		return err
	}

	expiresAt := time.Now()

	s.Repo.StoreRefreshToken(models.RefreshToken{
		UserID:    oldToken.ID,
		ExpiresAt: expiresAt,
	})

	return nil
}

func (s *authService) ChangePassword(payload dto.ChangePasswordRequest) *response.Error {

	result, err := s.Repo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if !utils.IsValidPassword(result.PasswordHash, payload.OldPassword) {
		s.logger.Error("Password change failed due to incorrect old password")
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Current password is incorrect",
		}
	}

	if !utils.ValidatePassword(payload.NewPassword) {
		s.logger.Error("Validation failure in Password")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Password must be at least 8 characters long",
		}

	}

	passwordhash, errorResponse := utils.HashPassword(payload.NewPassword)
	if errorResponse != nil {
		s.logger.Error("Failed Hashing Password")
		return errorResponse
	}

	return s.Repo.ChangePassword(passwordhash, payload.UserID)

}

func (s *authService) UpdateUser(payload dto.UpdateUserRequest, userID uuid.UUID) *response.Error {

	if len(payload.FullName) > 30 {
		s.logger.Error("Validation failure in Full Name")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Full name must not exceed 30 characters",
		}
	}

	if len(payload.UserName) > 30 {
		s.logger.Error("Validation failure in Username")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Username must not exceed 30 characters",
		}
	}

	req := models.User{
		FullName:  payload.FullName,
		UserName:  payload.UserName,
		AvatarURL: payload.AvatarURL,
		Timezone:  payload.Timezone,
	}

	return s.Repo.UpdateUser(userID, req)

}

func (s *authService) GetUser(userID uuid.UUID) (models.User, *response.Error) {

	return s.Repo.GetByID(userID)
}

func (s *authService) IsEmailAvailable(email string) (bool, *response.Error) {
	exists, err := s.Repo.ExistsByEmail(email)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (s *authService) IsUsernameAvailable(username string) (bool, *response.Error) {
	exists, err := s.Repo.ExistsByUsername(username)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

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
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
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
	GetUserByID(userID, organizationID uuid.UUID) (*models.User, *response.Error)
}

func InitAuthService(authRepo authrepo.AuthRepository, logger *zap.Logger) AuthService {
	return &authService{
		authRepo: authRepo,
		logger:   logger,
	}
}

type authService struct {
	authRepo authrepo.AuthRepository
	logger   *zap.Logger
}

func (s *authService) SignIn(credentials dto.SignInRequest) (*dto.AuthTokensResponse, *response.Error) {

	result, err := s.authRepo.GetByEmail(credentials.Email)
	if err != nil {
		if err.StatusCode == http.StatusNotFound {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid email or password",
			}
		}
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
		UserID:         result.ID,
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

	storedToken, storeErr := s.authRepo.StoreRefreshToken(models.RefreshToken{
		UserID:    result.ID,
		TokenHash: hashedRefreshToken,
		ExpiresAt: expiresAt,
	})
	if storeErr != nil {
		return nil, storeErr
	}

	// Build token value as <token_id>.<secret>
	refreshTokenValue = fmt.Sprintf("%s.%s", storedToken.ID.String(), refreshTokenValue)

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
	// Expect token format: <id>.<secret>
	parts := strings.Split(credentials.RefreshToken, ".")
	if len(parts) != 2 {
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Authentication required",
		}
	}

	tokenIDStr := parts[0]
	secret := parts[1]

	tokenID, parseErr := uuid.FromString(tokenIDStr)
	if parseErr != nil {
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Authentication required",
		}
	}

	oldToken, err := s.authRepo.GetRefreshTokenByID(tokenID)
	if err != nil {
		return nil, err
	}

	if !utils.IsValidPassword(oldToken.TokenHash, secret) {
		s.logger.Error("The given refresh token is wrong",
			zap.String("token_id", tokenIDStr),
			zap.String("token_hash_prefix", func() string {
				if len(oldToken.TokenHash) > 20 {
					return oldToken.TokenHash[:20]
				}
				return oldToken.TokenHash
			}()),
			zap.Int("secret_len", len(secret)))
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

	user, userErr := s.authRepo.GetUserByID(oldToken.UserID)
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
		UserID:         user.ID,
		OrganizationID: &organizationID,
	}

	//Generate Jwt token
	accessToken, tokenErr := middleware.GenerateJWT(tokencredentials, s.logger)
	if tokenErr != nil {
		return nil, tokenErr
	}

	// Generate new Refresh Token
	newRefreshTokenValue, refreshTokenErr := generateRefreshTokenValue()
	if refreshTokenErr != nil {
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	hashedRefreshToken, hashErr := utils.HashPassword(newRefreshTokenValue)
	if hashErr != nil {
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	refreshExpiresIn, _ := utils.StringToInt(config.GetEnv("REFRESH_TOKEN_EXPIRY", "604800"))
	expiresAt := time.Now().Add(time.Duration(refreshExpiresIn) * time.Second)

	storedToken, err := s.authRepo.StoreRefreshToken(models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashedRefreshToken,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}

	// Build token value as <token_id>.<secret>
	newRefreshTokenValue = fmt.Sprintf("%s.%s", storedToken.ID.String(), newRefreshTokenValue)

	expiresIn, err := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if err != nil {

		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", err)))
		return nil, err
	}

	return &dto.AuthTokensResponse{
		AccessToken:      accessToken,
		RefreshToken:     newRefreshTokenValue,
		TokenType:        "Bearer",
		ExpiresIn:        expiresIn,
		RefreshExpiresIn: refreshExpiresIn,
	}, nil
}

func (s *authService) RequestPasswordReset(email string) *response.Error {

	user, err := s.authRepo.RequestPasswordReset(email)
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

	if invalidateErr := s.authRepo.InvalidatePasswordResetOTPs(user.ID); invalidateErr != nil {
		return invalidateErr
	}

	if saveErr := s.authRepo.SavePasswordResetOTP(otpRecord); saveErr != nil {
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
			Message:    "Password must be at least 8 characters long and include uppercase, lowercase, number, and special character with no spaces.",
		}
	}

	user, err := s.authRepo.RequestPasswordReset(credentials.Email)
	if err != nil {
		return err
	}

	otpRecord, otpErr := s.authRepo.GetPasswordResetOTP(user.ID, credentials.OTP)
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

	if updateErr := s.authRepo.UpdateUserPassword(user.ID, passwordHash); updateErr != nil {
		return updateErr
	}

	if revokeErr := s.authRepo.RevokeRefreshTokens(user.ID); revokeErr != nil {
		return revokeErr
	}

	usedAt := time.Now()
	otpRecord.UsedAt = &usedAt
	if saveErr := s.authRepo.SavePasswordResetOTP(otpRecord); saveErr != nil {
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
			Message:    "Password must be at least 8 characters long and include uppercase, lowercase, number, and special character with no spaces.",
		}
	}

	emailExists, emailErr := s.authRepo.ExistsByEmail(cleanEmail)
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

	usernameExists, usernameErr := s.authRepo.ExistsByUsername(cleanUsername)
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

	if createErr := s.authRepo.StoreUserTemp(result); createErr != nil {
		return createErr
	}

	if otpErr := s.sendEmailVerificationOTP(result.ID, result.Email); otpErr != nil {
		return otpErr
	}

	s.logger.Info("User registered successfully", zap.String("email", result.Email))
	return nil
}

func (s *authService) VerifyEmail(credentials dto.VerifyEmailRequest) (*dto.AuthTokensResponse, *response.Error) {
	user, err := s.authRepo.GetUserFromRedis(credentials.Email)
	if err != nil {
		return nil, err
	}

	otpRecord, otpErr := s.authRepo.GetEmailVerificationOTP(user.ID, credentials.OTP)
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

	err = s.authRepo.CreateUser(*user)
	if err != nil {
		return nil, err
	}

	tokencredentials := dto.JWtcredentials{
		Role:           user.Role,
		UserID:         user.ID,
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
	storedToken, storeErr := s.authRepo.StoreRefreshToken(models.RefreshToken{UserID: user.ID, TokenHash: hashedRefreshToken, ExpiresAt: expiresAt})
	if storeErr != nil {
		return nil, storeErr
	}

	// Prefix the returned token value with the stored token ID
	refreshTokenValue = fmt.Sprintf("%s.%s", storedToken.ID.String(), refreshTokenValue)

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
	user, err := s.authRepo.GetByEmail(cleanEmail)
	if err != nil {
		if err.StatusCode == http.StatusNotFound {
			redisUser, redisErr := s.authRepo.GetUserFromRedis(cleanEmail)
			if redisErr != nil {
				if redisErr.StatusCode == http.StatusNotFound {
					return err
				}
				return redisErr
			}
			user = *redisUser
		} else {
			return err
		}
	}

	if user.IsVerified {
		s.logger.Warn("Resend verification OTP requested for verified account", zap.String("email", cleanEmail))
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Email address is already verified",
		}
	}

	allowed, rateLimitErr := s.authRepo.IsEmailVerificationResendAllowed(cleanEmail, time.Minute)
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

	if recordErr := s.authRepo.RecordEmailVerificationResend(cleanEmail, time.Now()); recordErr != nil {
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

	if invalidateErr := s.authRepo.InvalidateEmailVerificationOTPs(userID); invalidateErr != nil {
		return invalidateErr
	}

	if saveErr := s.authRepo.SaveEmailVerificationOTP(otpRecord); saveErr != nil {
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
	userID, parseErr := uuid.FromString(UserID)
	if parseErr != nil {
		s.logger.Error("Invalid user ID for logout", zap.String("UserID", UserID), zap.Error(parseErr))
		return &response.Error{Code: response.ErrBadRequest, StatusCode: http.StatusBadRequest, Message: "Invalid user ID"}
	}
	if err := s.authRepo.RevokeRefreshTokens(userID); err != nil {
		return err
	}
	return nil
}

func (s *authService) ChangePassword(payload dto.ChangePasswordRequest) *response.Error {

	result, err := s.authRepo.GetUserByID(payload.UserID)
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
			Message:    "Password must be at least 8 characters long and include uppercase, lowercase, number, and special character with no spaces.",
		}
	}

	passwordhash, errorResponse := utils.HashPassword(payload.NewPassword)
	if errorResponse != nil {
		s.logger.Error("Failed Hashing Password")
		return errorResponse
	}

	return s.authRepo.ChangePassword(passwordhash, payload.UserID)

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

	return s.authRepo.UpdateUser(userID, req)

}

func (s *authService) GetUser(userID uuid.UUID) (models.User, *response.Error) {

	return s.authRepo.GetUserByID(userID)
}

func (s *authService) IsEmailAvailable(email string) (bool, *response.Error) {
	exists, err := s.authRepo.ExistsByEmail(email)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (s *authService) IsUsernameAvailable(username string) (bool, *response.Error) {
	exists, err := s.authRepo.ExistsByUsername(username)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (s *authService) GetUserByID(userID, organizationID uuid.UUID) (*models.User, *response.Error) {

	if userID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid user ID",
		}
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if user.OrganizationID == nil || organizationID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *user.OrganizationID != organizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", organizationID.String()),
			zap.String("User Organization Id", user.OrganizationID.String()))

		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	return &user, nil
}

package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/middleware"
	mail "github.com/ms-kanban-server/internal/pkg/email"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/repository"
	"go.uber.org/zap"
)

type AuthService interface {
	SignIn(credentials dto.SignInRequest) (*dto.AuthTokensResponse, *response.Error)
	RefreshToken(credentials dto.RefreshTokenRequest) (*dto.AuthTokensResponse, *response.Error)
	SignUp(credentials dto.SignUpRequest) *response.Error
	Logout(UserID string) *response.Error
	ChangePassword(payload dto.ChangePasswordRequest) *response.Error
	RequestPasswordReset(email string) *response.Error
	ResetPassword(credentials dto.ResetPasswordRequest) *response.Error
	UpdateUser(payload dto.UpdateUserRequest, userID uuid.UUID) *response.Error
	GetUser(userID uuid.UUID) (models.User, *response.Error)
}

func InitAuthService(repo repository.AuthRepository, logger *zap.Logger) AuthService {
	return &authservice{
		Repo:   repo,
		logger: logger,
	}
}

type authservice struct {
	Repo   repository.AuthRepository
	logger *zap.Logger
}

func (s *authservice) SignIn(credentials dto.SignInRequest) (*dto.AuthTokensResponse, *response.Error) {

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

	if !result.IsActive {
		s.logger.Error("The account is deactivated or locked",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Forbidden",
			Details: []response.Details{{
				Field:   "IsActive",
				Message: "Account is Inactive",
			}},
		}
	}

	if !utils.IsValidPassword(result.PasswordHash, credentials.Password) {
		s.logger.Error("Login failed due to incorrect password",
			zap.String("email", credentials.Email))
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
			Details: []response.Details{
				{
					Field:   "Email/Password",
					Message: "Email/Password is incorrect",
				},
			},
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
			Message:    "Internal Server Error",
			Details: []response.Details{
				{
					Field:   "Refresh token",
					Message: "Failed to create refresh token",
				},
			},
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

func (s *authservice) RefreshToken(credentials dto.RefreshTokenRequest) (*dto.AuthTokensResponse, *response.Error) {

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
			Message:    "Unauthorized",
			Details: []response.Details{
				{
					Field:   "Refresh token",
					Message: "Invalid Refresh token",
				},
			},
		}
	}

	if time.Now().After(oldToken.ExpiresAt) {
		s.logger.Error("Refresh token expired",
			zap.String("UserID", credentials.UserID))
		return nil, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
			Details: []response.Details{
				{
					Field:   "Refresh token",
					Message: "Refresh token expired",
				},
			},
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
			zap.String("UserID", fmt.Sprint(user.ID)))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Forbidden",
			Details: []response.Details{
				{
					Field:   "account",
					Message: "Account is inactive",
				},
			},
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

func (s *authservice) RequestPasswordReset(email string) *response.Error {

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
			Message:    "Internal Server Error",
			Details: []response.Details{
				{
					Field:   "email",
					Message: "Failed to send password reset OTP",
				},
			},
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

func (s *authservice) ResetPassword(credentials dto.ResetPasswordRequest) *response.Error {

	if utils.ValidatePassword(credentials.NewPassword) {
		s.logger.Error("Validation failure in Password")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "new_password",
					Message: "Password must meet the minimum complexity requirements",
				},
			},
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
			Message:    "Invalid OTP",
			Details: []response.Details{
				{
					Field:   "Unauthorized",
					Message: "The provided OTP is invalid or expired",
				},
			},
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

func (s *authservice) SignUp(credentials dto.SignUpRequest) *response.Error {

	validate := validator.New()
	err := validate.Struct(credentials)
	if err != nil {
		s.logger.Error(" Validation failure in Email/Password",
			zap.String("Email", credentials.Email),
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "Email/Password",
					Message: "Invalid Email/Password",
				},
			},
		}
	}

	if utils.ValidatePassword(credentials.Password) {
		s.logger.Error("Validated failure in Password before")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "Password",
					Message: "Password must meet the minimum complexity requirements",
				},
			},
		}

	}

	passwordhash, errorResponse := utils.HashPassword(credentials.Password)
	if errorResponse != nil {
		s.logger.Error("Failed Hashing Password",
			zap.Error(fmt.Errorf("%v", errorResponse)))
		return errorResponse
	}

	
	result := models.User{
		Email:        credentials.Email,
		PasswordHash: passwordhash,
		FullName:     credentials.FullName,
		UserName:     credentials.UserName,
		AvatarURL:    credentials.AvatarURL,
		Timezone:     credentials.Timezone,
	}

	organizationID, errorResponse := utils.StringToUUID(credentials.OrganizationID)
	if errorResponse != nil {
		s.logger.Error("Failed to convert the string into UUID",
			zap.Error(err))
		return errorResponse
	}

	if organizationID != uuid.Nil {
		result.OrganizationID = &organizationID
	}

	return s.Repo.CreateUser(result)

}

func (s *authservice) Logout(UserID string) *response.Error {

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

func (s *authservice) ChangePassword(payload dto.ChangePasswordRequest) *response.Error {

	result, err := s.Repo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if utils.IsValidPassword(result.PasswordHash, payload.OldPassword) {
		s.logger.Error("Login failed due to invalid credentials")
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Invaild Old Password",
			Details: []response.Details{{
				Field:   "Password",
				Message: "Unauthorized",
			}},
		}
	}

	if utils.ValidatePassword(payload.NewPassword) {
		s.logger.Error("Validation failure in Password")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "Password",
					Message: "Password must meet the minimum complexity requirements",
				},
			},
		}

	}

	passwordhash, errorResponse := utils.HashPassword(payload.NewPassword)
	if errorResponse != nil {
		s.logger.Error("Failed Hashing Password")
		return errorResponse
	}

	return s.Repo.ChangePassword(passwordhash, payload.UserID)

}

func (s *authservice) UpdateUser(payload dto.UpdateUserRequest, userID uuid.UUID) *response.Error {

	if len(payload.FullName) > 30 {
		s.logger.Error("Validation failure in Full Name")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "Full Name",
					Message: "Full Name length exceeds the limit, Must be smaller than 30",
				},
			},
		}
	}

	if len(payload.UserName) > 30 {
		s.logger.Error("Validation failure in Username")
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
			Details: []response.Details{
				{
					Field:   "UserName",
					Message: "Username length exceeds the limit, Must be smaller than 30",
				},
			},
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

func (s *authservice) GetUser(userID uuid.UUID) (models.User, *response.Error) {

	return s.Repo.GetByID(userID)
}

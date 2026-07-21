package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthRepository interface {
	GetByEmail(email string) (models.User, *response.Error)
	GetByID(id uuid.UUID) (models.User, *response.Error)
	CreateUser(row models.User) *response.Error
	StoreRefreshToken(token models.RefreshToken) *response.Error
	GetRefreshToken(userID string) (models.RefreshToken, *response.Error)
	ChangePassword(password string, userID uuid.UUID) *response.Error
	RequestPasswordReset(email string) (models.User, *response.Error)
	SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error
	InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error
	GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error)
	UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error
	RevokeRefreshTokens(userID uuid.UUID) *response.Error
	UpdateUser(userID uuid.UUID, req models.User) *response.Error
}

func InitAuthRepository(deps models.Config) AuthRepository {
	return &authdatabase{
		DB:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type authdatabase struct {
	DB          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

func (d *authdatabase) GetByEmail(email string) (models.User, *response.Error) {

	var row models.User

	err := d.DB.Where("email = ?", email).Preload("Organization").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Enter valid Email/Password",
				Details: []response.Details{
					{
						Field:   "Email/Password",
						Message: "Unauthorized",
					},
				},
			}
			d.logger.Error("User not found in database",
				zap.String("Email", email), zap.Error(err))
			return models.User{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "InternalServerError",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}

		d.logger.Error("Database error occurred",
			zap.String("Email", email),
			zap.Error(err))
		return models.User{}, &errorResponse
	}

	return row, nil
}

func (d *authdatabase) GetByID(id uuid.UUID) (models.User, *response.Error) {

	var row models.User

	if err := d.DB.Where("id = ?", id).Preload("Organization").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {

			d.logger.Error("The user associated with the refresh token could not be found",
				zap.Error(err))
			return models.User{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "User not found",
				Details: []response.Details{{
					Field:   "User",
					Message: "InternalServerError",
				}},
			}
		}

		d.logger.Error("Failed to retrieve user",
			zap.Error(err))
		return models.User{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve user",
			Details: []response.Details{{
				Message: "InternalServerError",
			}},
		}
	}
	return row, nil
}

func (d *authdatabase) CreateUser(row models.User) *response.Error {

	if err := d.DB.Create(&row).Error; err != nil {

		switch {
		case errors.Is(err, gorm.ErrDuplicatedKey):
			d.logger.Error("Duplicated Key conflict",
				zap.Error(err))
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "User already exists",
			}

		case errors.Is(err, gorm.ErrForeignKeyViolated):
			d.logger.Error("Foreign Key Violated",
				zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid reference",
			}

		default:
			d.logger.Error("Database error occurred",
				zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to register",
			}
		}
	}

	return nil
}

func (d *authdatabase) StoreRefreshToken(token models.RefreshToken) *response.Error {

	err := d.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, // Conflict target
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"token_hash",
			"user_agent",
			"ip_address",
			"expires_at",
			"revoked_at",
			"updated_at", // if your model has UpdatedAt
		}),
	}).Create(&token).Error

	if err != nil {
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to store refresh token",
			Details: []response.Details{{
				Message: "InternalServerError",
			}},
		}

		d.logger.Error("Database error occurred while storing refresh token",
			zap.Error(err))

		return &errorResponse
	}

	return nil
}

func (d *authdatabase) GetRefreshToken(userID string) (models.RefreshToken, *response.Error) {

	var token models.RefreshToken

	if err := d.DB.Where("user_id = ?", userID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {

			d.logger.Error("Database error occurred while storing refresh token",
				zap.Error(err))
			return models.RefreshToken{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Invalid refresh token",
				Details: []response.Details{{
					Field:   "refresh_token",
					Message: "InternalServerError",
				}},
			}
		}

		d.logger.Error("Database error occurred while storing refresh token",
			zap.Error(err))
		return models.RefreshToken{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to read refresh token",
			Details: []response.Details{{
				Message: "InternalServerError",
			}},
		}
	}

	return token, nil
}

func (d *authdatabase) ChangePassword(password string, userID uuid.UUID) *response.Error {

	result := d.DB.
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("password_hash", password)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating password",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update paswword",
			Details: []response.Details{{
				Message: "InternalServerError",
			}},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("User not found while updating Password",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "User not found",
			Details: []response.Details{{
				Field:   "user_id",
				Message: "Unauthorized",
			}},
		}
	}

	return nil
}

func (d *authdatabase) RequestPasswordReset(email string) (models.User, *response.Error) {

	var row models.User

	if err := d.DB.Where("email = ?", email).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "The provided email does not match a known account",
				Details: []response.Details{
					{
						Field:   "Email",
						Message: "Unauthorized",
					},
				},
			}
			d.logger.Error("User not found in database",
				zap.String("Email", email), zap.Error(err))
			return models.User{}, &errorResponse
		}

		d.logger.Error("Database error occurred",
			zap.String("Email", email),
			zap.Error(err))
		return models.User{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to lookup user",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}
	return row, nil
}

func (d *authdatabase) SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error {

	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	payload, err := json.Marshal(otp)
	if err != nil {
		d.logger.Error("Database error occurred in redis",
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	key := otpRedisKey(otp.UserID)
	ttl := time.Until(otp.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}

	if err := d.redisClient.Set(context.Background(), key, payload, ttl).Err(); err != nil {
		d.logger.Error("Database error occurred in redis",
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to save OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}
	return nil
}

func (d *authdatabase) InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error {

	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to invalidate OTPs",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	if err := d.redisClient.Del(context.Background(), otpRedisKey(userID)).Err(); err != nil {
		d.logger.Error("Database error occurred in redis",
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to invalidate OTPs",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}
	return nil
}

func (d *authdatabase) GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {

	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to read OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	var row models.PasswordResetOTP
	value, err := d.redisClient.Get(context.Background(), otpRedisKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redisclient.Nil) {
			d.logger.Error("The provided OTP is invalid or expired",
				zap.Error(err))
			return models.PasswordResetOTP{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Invalid OTP",
				Details: []response.Details{
					{
						Field:   "otp",
						Message: "Unauthorized",
					},
				},
			}
		}

		d.logger.Error("Database error occurred in redis",
			zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to read OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	if err := json.Unmarshal([]byte(value), &row); err != nil {
		d.logger.Error("Database error occurred in redis",
			zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to read OTP",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	if row.ExpiresAt.Before(time.Now()) || row.UsedAt != nil {
		if err := d.redisClient.Del(context.Background(), otpRedisKey(userID)).Err(); err != nil {
			d.logger.Error("Database error occurred in redis",
				zap.Error(err))
			return models.PasswordResetOTP{}, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to read OTP",
				Details: []response.Details{
					{
						Message: "InternalServerError",
					},
				},
			}
		}
		d.logger.Error("The provided OTP is invalid or expired",
			zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Invalid OTP",
			Details: []response.Details{
				{
					Field:   "otp",
					Message: "Unauthorized"}}}
	}

	return row, nil
}

func otpRedisKey(userID uuid.UUID) string {
	return fmt.Sprintf("password-reset-otp:%s", userID.String())
}

func (d *authdatabase) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {

	result := d.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", passwordHash)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating Password",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update user",
			Details: []response.Details{{
				Message: "InternalServerError",
			}},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified user does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "User not found",
			Details: []response.Details{
				{
					Field:   "user_id",
					Message: "Unauthorized",
				},
			},
		}
	}

	return nil
}

func (d *authdatabase) RevokeRefreshTokens(userID uuid.UUID) *response.Error {

	result := d.DB.Model(&models.RefreshToken{}).Where("user_id = ?", userID).Update("revoked_at", time.Now())

	if result.Error != nil {

		d.logger.Error("Failed to revoke refresh tokens",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to revoke refresh tokens",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified Token does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Token not found",
			Details: []response.Details{
				{
					Field:   "Token",
					Message: "Unauthorized",
				},
			},
		}

	}
	return nil
}

func (d *authdatabase) UpdateUser(userID uuid.UUID, req models.User) *response.Error {

	result := d.DB.
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(req)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating user",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update user",
			Details: []response.Details{
				{
					Message: "InternalServerError",
				},
			},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified user does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "User not found",
			Details: []response.Details{{
				Field:   "user_id",
				Message: "Unauthorized",
			}},
		}
	}

	return nil
}

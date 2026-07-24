package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthRepository interface {
	GetByEmail(email string) (models.User, *response.Error)
	GetByID(id uuid.UUID) (models.User, *response.Error)
	ExistsByEmail(email string) (bool, *response.Error)
	ExistsByUsername(username string) (bool, *response.Error)
	CreateUser(row models.User) *response.Error
	StoreRefreshToken(token models.RefreshToken) *response.Error
	GetRefreshToken(userID string) (models.RefreshToken, *response.Error)
	ChangePassword(password string, userID uuid.UUID) *response.Error
	RequestPasswordReset(email string) (models.User, *response.Error)
	SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error
	InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error
	GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error)
	SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error
	InvalidateEmailVerificationOTPs(userID uuid.UUID) *response.Error
	GetEmailVerificationOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error)
	IsEmailVerificationResendAllowed(email string, interval time.Duration) (bool, *response.Error)
	RecordEmailVerificationResend(email string, sentAt time.Time) *response.Error
	CreateOrganization(row models.Organization) *response.Error
	GetOrganizationByName(name string) (models.Organization, *response.Error)
	UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error
	RevokeRefreshTokens(userID uuid.UUID) *response.Error
	UpdateUser(userID uuid.UUID, req models.User) *response.Error
	StoreUserTemp(row models.User) *response.Error
	GetUserFromRedis(email string) (*models.User, *response.Error)
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

func (d *authdatabase) GetUserFromRedis(email string) (*models.User, *response.Error) {
	ctx := context.Background()

	key := "user:email:" + strings.ToLower(email)

	val, err := d.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	var user models.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	return &user, nil
}

func (d *authdatabase) StoreUserTemp(row models.User) *response.Error {
	ctx := context.Background()

	key := "user:email:" + strings.ToLower(row.Email)

	data, err := json.Marshal(row)
	if err != nil {
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	if err := d.redisClient.Set(ctx, key, data, 3*time.Minute).Err(); err != nil {
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	return nil
}

func (d *authdatabase) GetByEmail(email string) (models.User, *response.Error) {

	var row models.User

	err := d.DB.Where("email = ?", email).Preload("Organization").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid email or password",
			}
			d.logger.Error("User not found in database",
				zap.String("Email", email), zap.Error(err))
			return models.User{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
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
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "User not found",
			}
		}

		d.logger.Error("Failed to retrieve user",
			zap.Error(err))
		return models.User{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return row, nil
}

func (d *authdatabase) ExistsByEmail(email string) (bool, *response.Error) {
	var count int64
	if err := d.DB.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		d.logger.Error("Database error checking email existence",
			zap.String("Email", email), zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return count > 0, nil
}

func (d *authdatabase) ExistsByUsername(username string) (bool, *response.Error) {
	var count int64
	if err := d.DB.Model(&models.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		d.logger.Error("Database error checking username existence",
			zap.String("Username", username), zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return count > 0, nil
}

func (d *authdatabase) CreateUser(row models.User) *response.Error {

	if err := d.DB.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict", zap.Error(err))
			return utils.ParseUserDuplicateError(err)
		}

		switch {
		case errors.Is(err, gorm.ErrForeignKeyViolated):
			d.logger.Error("Foreign Key Violated",
				zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}

		default:
			d.logger.Error("Database error occurred",
				zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
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
			Message:    "Something went wrong. Please try again later.",
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
				Message:    "Authentication required",
			}
		}

		d.logger.Error("Database error occurred while storing refresh token",
			zap.Error(err))
		return models.RefreshToken{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
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
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("User not found while updating Password",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
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
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return row, nil
}

func (d *authdatabase) SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error {
	return d.saveOTP(otp, otpRedisKey(otp.UserID))
}

func (d *authdatabase) SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error {
	return d.saveOTP(otp, emailVerificationOTPRedisKey(otp.UserID))
}

func (d *authdatabase) saveOTP(otp models.PasswordResetOTP, key string) *response.Error {
	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	payload, err := json.Marshal(otp)
	if err != nil {
		d.logger.Error("Database error occurred in redis", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	ttl := time.Until(otp.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}

	if err := d.redisClient.Set(context.Background(), key, payload, ttl).Err(); err != nil {
		d.logger.Error("Database error occurred in redis", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *authdatabase) InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error {
	return d.invalidateOTP(otpRedisKey(userID))
}

func (d *authdatabase) InvalidateEmailVerificationOTPs(userID uuid.UUID) *response.Error {
	return d.invalidateOTP(emailVerificationOTPRedisKey(userID))
}

func (d *authdatabase) invalidateOTP(key string) *response.Error {
	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := d.redisClient.Del(context.Background(), key).Err(); err != nil {
		d.logger.Error("Database error occurred in redis", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *authdatabase) GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	return d.getOTP(otpRedisKey(userID))
}

func (d *authdatabase) GetEmailVerificationOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	return d.getOTP(emailVerificationOTPRedisKey(userID))
}

func (d *authdatabase) getOTP(key string) (models.PasswordResetOTP, *response.Error) {
	if d.redisClient == nil {
		d.logger.Error("Database error occurred in redis")
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	var row models.PasswordResetOTP
	value, err := d.redisClient.Get(context.Background(), key).Result()
	if err != nil {
		if errors.Is(err, redisclient.Nil) {
			d.logger.Error("The provided OTP is invalid or expired", zap.Error(err))
			return models.PasswordResetOTP{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Invalid or expired OTP",
			}
		}

		d.logger.Error("Database error occurred in redis", zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := json.Unmarshal([]byte(value), &row); err != nil {
		d.logger.Error("Database error occurred in redis", zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if row.ExpiresAt.Before(time.Now()) || row.UsedAt != nil {
		if err := d.redisClient.Del(context.Background(), key).Err(); err != nil {
			d.logger.Error("Database error occurred in redis", zap.Error(err))
			return models.PasswordResetOTP{}, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}
		}
		d.logger.Error("The provided OTP is invalid or expired", zap.Error(err))
		return models.PasswordResetOTP{}, &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Invalid or expired OTP",
		}
	}

	return row, nil
}

func otpRedisKey(userID uuid.UUID) string {
	return fmt.Sprintf("password-reset-otp:%s", userID.String())
}

func emailVerificationOTPRedisKey(userID uuid.UUID) string {
	return fmt.Sprintf("email-verification-otp:%s", userID.String())
}

func emailVerificationResendRedisKey(email string) string {
	return fmt.Sprintf("email-verification-resend:%s", strings.ToLower(strings.TrimSpace(email)))
}

func (d *authdatabase) IsEmailVerificationResendAllowed(email string, interval time.Duration) (bool, *response.Error) {
	if d.redisClient == nil {
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	key := emailVerificationResendRedisKey(email)
	value, err := d.redisClient.Get(context.Background(), key).Result()
	if errors.Is(err, redisclient.Nil) {
		return true, nil
	}
	if err != nil {
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	lastSentAt, parseErr := time.Parse(time.RFC3339Nano, value)
	if parseErr != nil {
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	return time.Since(lastSentAt) >= interval, nil
}

func (d *authdatabase) RecordEmailVerificationResend(email string, sentAt time.Time) *response.Error {
	if d.redisClient == nil {
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}

	key := emailVerificationResendRedisKey(email)
	if err := d.redisClient.Set(context.Background(), key, sentAt.Format(time.RFC3339Nano), time.Hour).Err(); err != nil {
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}
	}
	return nil
}

func (d *authdatabase) CreateOrganization(row models.Organization) *response.Error {
	if err := d.DB.Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return &response.Error{Code: response.ErrConflict, StatusCode: http.StatusConflict, Message: "Organization already exists"}
		}
		return &response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Failed to create organization"}
	}
	return nil
}

func (d *authdatabase) GetOrganizationByName(name string) (models.Organization, *response.Error) {
	var row models.Organization
	if err := d.DB.Where("name = ?", name).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Organization{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Organization not found"}
		}
		return models.Organization{}, &response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Failed to retrieve organization"}
	}
	return row, nil
}

func (d *authdatabase) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {

	result := d.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", passwordHash)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating Password",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified user does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
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
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified Token does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Token not found",
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
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating user", zap.Error(result.Error))
			return utils.ParseUserDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating user",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("The specified user does not exist",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

package authrepo

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
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

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
func (d *authdatabase) SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error {
	return d.saveOTP(otp, otpRedisKey(otp.UserID))
}

func (d *authdatabase) SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error {
	return d.saveOTP(otp, emailVerificationOTPRedisKey(otp.UserID))
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

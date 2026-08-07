package authrepo

import (
	"context"
	"encoding/json"
	"errors"
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

func (d *authDatabase) GetUserFromRedis(email string) (*models.User, *response.Error) {
	ctx := context.Background()

	key := "user:email:" + strings.ToLower(email)

	val, err := d.redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redisclient.Nil) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "User not found",
			}
		}

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

func (d *authDatabase) StoreUserTemp(row models.User) *response.Error {
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

func (d *authDatabase) GetByEmail(email string) (models.User, *response.Error) {

	var row models.User

	err := d.db.Where("email = ?", email).Preload("Organization").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "User not found",
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

func (d *authDatabase) GetUserByID(id uuid.UUID) (models.User, *response.Error) {

	var row models.User

	if err := d.db.Where("id = ?", id).Preload("Organization").First(&row).Error; err != nil {
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

func (d *authDatabase) CreateUser(row models.User) *response.Error {
	if row.Role == "" {
		row.Role = "developer"
	}
	if row.Timezone == "" {
		row.Timezone = "UTC"
	}

	if err := d.db.Select("*").Create(&row).Error; err != nil {
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

func (d *authDatabase) StoreRefreshToken(token models.RefreshToken) (models.RefreshToken, *response.Error) {

	if token.TokenHash == "" {
		return models.RefreshToken{}, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Refresh token hash cannot be empty",
		}
	}

	err := d.db.Clauses(clause.OnConflict{
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

		return models.RefreshToken{}, &errorResponse
	}

	// Reload the stored token row to ensure we return the DB row (important when ON CONFLICT does an update)
	var stored models.RefreshToken
	if err := d.db.Where("user_id = ?", token.UserID).First(&stored).Error; err != nil {
		d.logger.Error("Failed to reload stored refresh token",
			zap.Error(err))
		return models.RefreshToken{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return stored, nil
}

func (d *authDatabase) GetRefreshToken(userID string) (models.RefreshToken, *response.Error) {

	var token models.RefreshToken

	if err := d.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
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

func (d *authDatabase) GetRefreshTokenByID(id uuid.UUID) (models.RefreshToken, *response.Error) {
	var token models.RefreshToken

	if err := d.db.Where("id = ?", id).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.logger.Error("Refresh token not found",
				zap.Error(err))
			return models.RefreshToken{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			}
		}

		d.logger.Error("Database error occurred while retrieving refresh token",
			zap.Error(err))
		return models.RefreshToken{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return token, nil
}

func (d *authDatabase) ChangePassword(password string, userID uuid.UUID) *response.Error {

	result := d.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"password_hash": password,
		})

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
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *authDatabase) RequestPasswordReset(email string) (models.User, *response.Error) {

	var row models.User

	if err := d.db.Where("email = ?", email).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
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

func (d *authDatabase) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {

	result := d.db.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", passwordHash)

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
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *authDatabase) RevokeRefreshTokens(userID uuid.UUID) *response.Error {

	result := d.db.Model(&models.RefreshToken{}).Where("user_id = ?", userID).Update("revoked_at", time.Now())

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
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Token not found",
		}

	}
	return nil
}

func (d *authDatabase) UpdateUser(userID uuid.UUID, req models.User) *response.Error {
	var result *gorm.DB
	if req.ID != uuid.Nil {
		result = d.db.
			Model(&models.User{}).
			Where("id = ?", userID).
			Select("*").
			Updates(req)
	} else {
		result = d.db.
			Model(&models.User{}).
			Where("id = ?", userID).
			Updates(req)
	}

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
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *authDatabase) GetPendingInvitationByEmail(email string) (models.OrganizationInvitation, *response.Error) {
	var invitation models.OrganizationInvitation
	result := d.db.Where("email = ? AND status = ?", strings.ToLower(strings.TrimSpace(email)), models.InvitationStatusPending).First(&invitation)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return models.OrganizationInvitation{}, nil
		}
		d.logger.Error("Database error occurred while fetching invitation", zap.Error(result.Error))
		return models.OrganizationInvitation{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return invitation, nil
}

func (d *authDatabase) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
	result := d.db.Save(&invitation)
	if result.Error != nil {
		d.logger.Error("Database error occurred while updating invitation", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}


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
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
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

	err := d.db.Where("email = ?", email).
		Preload("Organization").
		Preload("Role").
		Preload("Role.Permissions").
		First(&row).Error
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

	if row.RoleID == uuid.Nil || row.Role.Name == "" {
		var role models.Role
		if err := d.db.Where("name = ? AND organization_id IS NULL", "developer").First(&role).Error; err == nil {
			row.RoleID = role.ID
			row.Role = role
			_ = d.db.Model(&models.User{}).Where("id = ?", row.ID).Update("role_id", role.ID).Error
		}
	}

	return row, nil
}

func (d *authDatabase) GetUserByID(id uuid.UUID) (models.User, *response.Error) {

	var row models.User

	if err := d.db.Where("id = ?", id).
		Preload("Organization").
		Preload("Role").
		Preload("Role.Permissions").
		First(&row).Error; err != nil {
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

	if row.RoleID == uuid.Nil || row.Role.Name == "" {
		var role models.Role
		if err := d.db.Where("name = ? AND organization_id IS NULL", "developer").First(&role).Error; err == nil {
			row.RoleID = role.ID
			row.Role = role
			_ = d.db.Model(&models.User{}).Where("id = ?", row.ID).Update("role_id", role.ID).Error
		}
	}

	return row, nil
}

func (d *authDatabase) CreateUser(row models.User) *response.Error {
	if row.RoleID == uuid.Nil || row.Role.Name == "" {
		var role models.Role
		if err := d.db.Where("name = ? AND organization_id IS NULL", "developer").First(&role).Error; err == nil {
			row.RoleID = role.ID
			row.Role = role
		}
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
			"password_hash":           password,
			"require_password_change": false,
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

	result := d.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash":           passwordHash,
		"require_password_change": false,
	})

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

func (d *authDatabase) UpdateUserFields(userID uuid.UUID, updates map[string]interface{}) *response.Error {
	result := d.db.
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(updates)

	if result.Error != nil {
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating user fields", zap.Error(result.Error))
			return utils.ParseUserDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating user fields", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("User not found while updating fields", zap.String("user_id", userID.String()))
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *authDatabase) GetRoleByName(name string) (*models.Role, *response.Error) {
	var row models.Role
	err := d.db.Where("name = ? AND organization_id IS NULL", name).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Role not found",
			}
		}
		d.logger.Error("Failed to get role by name", zap.String("name", name), zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve role",
		}
	}
	return &row, nil
}

func (d *authDatabase) GetRoleByNameAndOrg(name string, orgID uuid.UUID) (*models.Role, *response.Error) {
	var row models.Role
	err := d.db.Where("name = ? AND organization_id = ?", name, orgID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Role not found",
			}
		}
		d.logger.Error("Failed to get role by name and organization", zap.String("name", name), zap.String("org_id", orgID.String()), zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve role",
		}
	}
	return &row, nil
}

func (d *authDatabase) GetRoleByID(roleID uuid.UUID) (*models.Role, *response.Error) {
	var row models.Role
	err := d.db.Where("id = ? AND deleted_at IS NULL", roleID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Role not found",
			}
		}
		d.logger.Error("Failed to get role by ID", zap.String("role_id", roleID.String()), zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve role",
		}
	}
	return &row, nil
}

func (d *authDatabase) GetUserInsights(userID, organizationID uuid.UUID) (total int64, inProgress int64, completed int64, err *response.Error) {
	type rawInsights struct {
		TotalAssigned int64
		InProgress    int64
		Completed     int64
	}

	var status rawInsights

	dbErr := d.db.Table("tasks").
		Select(`
			COUNT(tasks.id) AS total_assigned,
			COUNT(CASE WHEN custom_statuses.is_final != true THEN 1 END) AS in_progress,
			COUNT(CASE WHEN custom_statuses.is_final = true THEN 1 END) AS completed
		`).
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN custom_statuses ON custom_statuses.id = tasks.status_id").
		Where("projects.organization_id = ? AND tasks.assignee_id = ?", organizationID, userID).
		Where("tasks.deleted_at IS NULL").
		Where("projects.deleted_at IS NULL").
		Where("custom_statuses.deleted_at IS NULL").
		Scan(&status).Error

	if dbErr != nil {
		d.logger.Error("Failed to fetch user task insights", zap.Error(dbErr), zap.String("user_id", userID.String()))
		return 0, 0, 0, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch task insights",
		}
	}

	return status.TotalAssigned, status.InProgress, status.Completed, nil
}

func (d *authDatabase) GetUsersInsights(userIDs []uuid.UUID, organizationID uuid.UUID) (map[uuid.UUID]responsedto.UserTaskStats, *response.Error) {
	type rawInsights struct {
		UserID        uuid.UUID `gorm:"column:user_id"`
		TotalAssigned int64     `gorm:"column:total_assigned"`
		InProgress    int64     `gorm:"column:in_progress"`
		Completed     int64     `gorm:"column:completed"`
	}

	var results []rawInsights

	dbErr := d.db.Table("tasks").
		Select(`
			tasks.assignee_id AS user_id,
			COUNT(tasks.id) AS total_assigned,
			COUNT(CASE WHEN custom_statuses.is_final != true THEN 1 END) AS in_progress,
			COUNT(CASE WHEN custom_statuses.is_final = true THEN 1 END) AS completed
		`).
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN custom_statuses ON custom_statuses.id = tasks.status_id").
		Where("projects.organization_id = ? AND tasks.assignee_id IN ?", organizationID, userIDs).
		Where("tasks.deleted_at IS NULL").
		Where("projects.deleted_at IS NULL").
		Where("custom_statuses.deleted_at IS NULL").
		Group("tasks.assignee_id").
		Scan(&results).Error

	if dbErr != nil {
		d.logger.Error("Failed to fetch batch user task insights", zap.Error(dbErr))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch task insights",
		}
	}

	statsMap := make(map[uuid.UUID]responsedto.UserTaskStats)
	for _, res := range results {
		statsMap[res.UserID] = responsedto.UserTaskStats{
			TotalTasks: res.TotalAssigned,
			InProgress: res.InProgress,
			Completed:  res.Completed,
		}
	}

	return statsMap, nil
}

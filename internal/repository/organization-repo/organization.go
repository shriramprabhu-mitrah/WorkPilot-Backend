package organizationrepo

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *organizationDatabase) CreateOrganization(row models.Organization) *response.Error {

	if err := d.DB.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict", zap.Error(err))
			return utils.ParseOrgDuplicateError(err)
		}

		d.logger.Error("Database error occurred", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return nil
}

func (d *organizationDatabase) GetByName(name string) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Where("name = ?", name).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Organization not found",
			}
			d.logger.Error("Organization not found in database",
				zap.String("Name", name),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Name", name),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}

	return row, nil
}

func (d *organizationDatabase) GetByID(id uuid.UUID) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Organization not found",
			}
			d.logger.Error("Organization not found in database",
				zap.String("Id", id.String()),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", id.String()),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}
	return row, nil
}

func (d *organizationDatabase) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

	result := d.DB.
		Model(&models.Organization{}).
		Where("id = ?", OrganizationID).
		Updates(req)

	if result.Error != nil {
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating Organization", zap.Error(result.Error))
			return utils.ParseOrgDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating Organization",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("Organization not found while updating Organization",
			zap.String("Organization_id", OrganizationID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *organizationDatabase) DeleteOrganization(id uuid.UUID) *response.Error {

	result := d.DB.Where("id = ?", id).Delete(&models.Organization{})

	if result.Error != nil {
		d.logger.Error("Failed to delete Organization",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("The Organization could not be found for deletion",
			zap.String("organization_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *organizationDatabase) DeleteUser(id uuid.UUID) *response.Error {

	result := d.DB.Where("id = ?", id).Delete(&models.User{})

	if result.Error != nil {
		d.logger.Error("Failed to delete User",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("The User could not be found for deletion",
			zap.String("user_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *organizationDatabase) UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error {

	result := d.DB.
		Model(&models.User{}).
		Where("id = ?", userID).
		Save(req)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating user",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("User not found while updating user",
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *organizationDatabase) GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	var users []models.User
	var totalItems int64

	filter.PaginationQuery.Normalize(10)

	offset := (filter.Page - 1) * filter.PageSize
	baseQuery := d.DB.Model(&models.User{}).Where("organization_id = ?", organizationID)

	if filter.FullName != "" {
		baseQuery = baseQuery.Where("full_name ILIKE ?", "%"+strings.TrimSpace(filter.FullName)+"%")
	}
	if filter.Email != "" {
		baseQuery = baseQuery.Where("email ILIKE ?", "%"+strings.TrimSpace(filter.Email)+"%")
	}
	if filter.Username != "" {
		baseQuery = baseQuery.Where("username ILIKE ?", "%"+strings.TrimSpace(filter.Username)+"%")
	}
	if filter.Role != "" {
		baseQuery = baseQuery.Where("LOWER(role) = ?", strings.ToLower(strings.TrimSpace(filter.Role)))
	}
	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.IsVerified != nil {
		baseQuery = baseQuery.Where("is_verified = ?", *filter.IsVerified)
	}
	if filter.Timezone != "" {
		baseQuery = baseQuery.Where("timezone ILIKE ?", "%"+strings.TrimSpace(filter.Timezone)+"%")
	}
	if !filter.IncludeOrgAdmins {
		baseQuery = baseQuery.Where("LOWER(role) != ? OR role IS NULL", "org_admin")
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return users, pagination, nil
}

package repository

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OrganizationRepository interface {
	CreateOrganization(row models.Organization) *response.Error
	GetByName(name string) (models.Organization, *response.Error)
	GetByID(id uuid.UUID) (models.Organization, *response.Error)
	UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error
	DeleteOrganization(id uuid.UUID) *response.Error
	UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error
	CreateOrganizationInvitation(invitation models.OrganizationInvitation) *response.Error
	GetPendingInvitationByEmail(orgID uuid.UUID, email string) (models.OrganizationInvitation, *response.Error)
	GetInvitationByToken(token string) (models.OrganizationInvitation, *response.Error)
	UpdateInvitation(invitation models.OrganizationInvitation) *response.Error
	CreateAuditLog(log models.AuditLog) *response.Error
	GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error)
}

func InitOrganizationRepository(deps models.Config) OrganizationRepository {
	return &Organizationdatabase{
		DB:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type Organizationdatabase struct {
	DB          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}

func (d *Organizationdatabase) CreateOrganization(row models.Organization) *response.Error {

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

func (d *Organizationdatabase) GetByName(name string) (models.Organization, *response.Error) {

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

func (d *Organizationdatabase) GetByID(id uuid.UUID) (models.Organization, *response.Error) {

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
				zap.String("Id", fmt.Sprint(id)),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", fmt.Sprint(id)),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}
	return row, nil
}

func (d *Organizationdatabase) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

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
			zap.String("Organization_id", fmt.Sprint(OrganizationID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *Organizationdatabase) DeleteOrganization(id uuid.UUID) *response.Error {

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

func (d *Organizationdatabase) DeleteUser(id uuid.UUID) *response.Error {

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

func (d *Organizationdatabase) UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error {

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
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *Organizationdatabase) CreateOrganizationInvitation(invitation models.OrganizationInvitation) *response.Error {
	if err := d.DB.Create(&invitation).Error; err != nil {
		d.logger.Error("Database error occurred while creating organization invitation", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *Organizationdatabase) GetPendingInvitationByEmail(orgID uuid.UUID, email string) (models.OrganizationInvitation, *response.Error) {
	var row models.OrganizationInvitation
	err := d.DB.Where("organization_id = ? AND email = ? AND status = ?", orgID, email, models.InvitationStatusPending).Order("created_at desc").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.OrganizationInvitation{}, nil
		}
		return models.OrganizationInvitation{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return row, nil
}

func (d *Organizationdatabase) GetInvitationByToken(token string) (models.OrganizationInvitation, *response.Error) {
	var row models.OrganizationInvitation
	err := d.DB.Where("token = ?", token).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.OrganizationInvitation{}, nil
		}
		return models.OrganizationInvitation{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return row, nil
}

func (d *Organizationdatabase) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
	if err := d.DB.Save(&invitation).Error; err != nil {
		d.logger.Error("Database error occurred while updating invitation", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *Organizationdatabase) CreateAuditLog(log models.AuditLog) *response.Error {
	if err := d.DB.Create(&log).Error; err != nil {
		d.logger.Error("Database error occurred while creating audit log", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *Organizationdatabase) GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	var users []models.User
	var totalItems int64

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	offset := (filter.Page - 1) * filter.PageSize
	baseQuery := d.DB.Model(&models.User{}).Where("organization_id = ?", organizationID)

	if filter.FullName != "" {
		fullNameTerm := "%" + strings.ToLower(strings.TrimSpace(filter.FullName)) + "%"
		baseQuery = baseQuery.Where("LOWER(full_name) LIKE ?", fullNameTerm)
	}
	if filter.Email != "" {
		emailTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Email)) + "%"
		baseQuery = baseQuery.Where("LOWER(email) LIKE ?", emailTerm)
	}
	if filter.Username != "" {
		usernameTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Username)) + "%"
		baseQuery = baseQuery.Where("LOWER(username) LIKE ?", usernameTerm)
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
		timezoneTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Timezone)) + "%"
		baseQuery = baseQuery.Where("LOWER(timezone) LIKE ?", timezoneTerm)
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

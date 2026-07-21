package repository

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
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
		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to Register",
			Details: []response.Details{
				{
					Message: "Failed inserting the row",
				},
			},
		}

		d.logger.Error("Database error occurred",
			zap.Error(err))
		return &errorResponse
	}

	return nil
}

func (d *Organizationdatabase) GetByName(name string) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Where("name = ?", name).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Organization Invalid/Missing",
				Details: []response.Details{
					{
						Field:   "Organization",
						Message: "Organization not found :" + name,
					},
				},
			}
			d.logger.Error("Organization not found in database",
				zap.String("Name", name), zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "InternalServerError",
		}

		d.logger.Error("Database error occurred",
			zap.String("Name", name), zap.Error(err))
		return models.Organization{}, &errorResponse
	}

	return row, nil
}

func (d *Organizationdatabase) GetByID(id uuid.UUID) (models.Organization, *response.Error) {

	var row models.Organization

	if err := d.DB.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {

			d.logger.Error("The Organization associated with the refresh token could not be found",
				zap.Error(err))
			return models.Organization{}, &response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Organization not found",
				Details: []response.Details{{
					Field:   "Organization",
					Message: "The Organization associated with the refresh token could not be found",
				}},
			}
		}

		d.logger.Error("Failed to retrieve Organization",
			zap.Error(err))
		return models.Organization{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve Organization",
			Details: []response.Details{{
				Message: "Failed querying Organization",
			}},
		}
	}
	return row, nil
}

func (d *Organizationdatabase) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

	result := d.DB.
		Model(&models.Organization{}).
		Where("id = ?", OrganizationID).
		Updates(req)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating Organization",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update Organization",
			Details: []response.Details{{
				Message: "Failed updating Organization: " + result.Error.Error(),
			}},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("Organization not found while updating Organization",
			zap.String("Organization_id", fmt.Sprint(OrganizationID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Organization not found",
			Details: []response.Details{{
				Field:   "Organization_id",
				Message: "The specified Organization does not exist",
			}},
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
			Message:    "Failed to delete Organization",
			Details: []response.Details{{
				Message: "Failed deleting Organization",
			}},
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("The Organization could not be found for deletion",
			zap.String("organization_id", id.String()))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Organization not found",
			Details: []response.Details{{
				Field:   "Organization",
				Message: "The Organization could not be found for deletion",
			}},
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
			Message:    "Failed to update user",
			Details: []response.Details{{
				Message: "Failed updating user",
			}},
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("User not found while updating user",
			zap.String("user_id", fmt.Sprint(userID)))

		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "User not found",
			Details: []response.Details{{
				Field:   "user_id",
				Message: "The specified user does not exist",
			}},
		}
	}

	return nil
}

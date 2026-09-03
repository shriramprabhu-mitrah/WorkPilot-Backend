package organizationrepo

import (
	"errors"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *organizationDatabase) CreateOrganizationInvitation(invitation models.OrganizationInvitation) *response.Error {
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

func (d *organizationDatabase) GetPendingInvitationByEmail(orgID uuid.UUID, email string) (models.OrganizationInvitation, *response.Error) {
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

func (d *organizationDatabase) GetInvitationByToken(token string) (models.OrganizationInvitation, *response.Error) {
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

func (d *organizationDatabase) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
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


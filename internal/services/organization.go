package services

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/repository"
	"go.uber.org/zap"
)

type OrganizationService interface {
	GetOrganizationByID(id uuid.UUID) (models.Organization, *response.Error)
	CreateOrganization(row models.Organization) *response.Error
	UpdateOrganization(id uuid.UUID, req models.Organization) *response.Error
	DeleteOrganization(id uuid.UUID) *response.Error
	UpdateUserStatus(payload dto.UpdateUserStatus) *response.Error
	UpdateUserRole(payload dto.UpdateUserRole) *response.Error
	GetUserInOrganization(id uuid.UUID, page int, pageSize int) ([]models.User, response.Pagination, *response.Error)
}

func InitOrganizationService(repo repository.OrganizationRepository, AuthRepo repository.AuthRepository, logger *zap.Logger) OrganizationService {
	return &Organizationservice{
		OrganizationRepo: repo,
		logger:           logger,
		AuthRepo:         AuthRepo,
	}
}

type Organizationservice struct {
	AuthRepo         repository.AuthRepository
	OrganizationRepo repository.OrganizationRepository
	logger           *zap.Logger
}

func (s *Organizationservice) GetOrganizationByID(id uuid.UUID) (models.Organization, *response.Error) {

	return s.OrganizationRepo.GetByID(id)
}

func (s *Organizationservice) CreateOrganization(row models.Organization) *response.Error {

	err := s.OrganizationRepo.CreateOrganization(row)
	if err != nil {
		return err
	}

	organization, err := s.OrganizationRepo.GetByName(row.Name)
	if err != nil {
		return err
	}

	user := models.User{
		OrganizationID: &organization.ID,
		Role:           string(dto.RoleOrgAdmin),
	}

	err = s.AuthRepo.UpdateUser(row.CreatedBy, user)
	if err != nil {
		s.OrganizationRepo.DeleteOrganization(organization.ID)
		return err
	}

	return nil
}

func (s *Organizationservice) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

	return s.OrganizationRepo.UpdateOrganization(OrganizationID, req)
}

func (s *Organizationservice) DeleteOrganization(id uuid.UUID) *response.Error {

	return s.OrganizationRepo.DeleteOrganization(id)
}

func (s *Organizationservice) UpdateUserStatus(payload dto.UpdateUserStatus) *response.Error {

	result, err := s.AuthRepo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if *result.OrganizationID != payload.OrganizationID {
		s.logger.Error("Unauthorized",
			zap.String("Organization Id", payload.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
			Details: []response.Details{
				{
					Field:   "Organization Id",
					Message: "Invalid Organization Id",
				},
			},
		}
	}
	req := models.User{
		ID:             result.ID,
		OrganizationID: result.OrganizationID,
		UserName:       result.UserName,
		Email:          result.Email,
		PasswordHash:   result.PasswordHash,
		Role:           result.Role,
		FullName:       result.FullName,
		IsActive:       payload.IsActive,
		AvatarURL:      result.AvatarURL,
		Timezone:       result.Timezone,
	}

	return s.OrganizationRepo.UpdateStatusAndRole(payload.UserID, req)

}

func (s *Organizationservice) UpdateUserRole(payload dto.UpdateUserRole) *response.Error {

	result, err := s.AuthRepo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if *result.OrganizationID != payload.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", payload.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized Access",
			Details: []response.Details{{
				Field:   "Organization Id",
				Message: "Unauthorized Access",
			}},
		}
	}
	req := models.User{
		ID:             result.ID,
		OrganizationID: result.OrganizationID,
		UserName:       result.UserName,
		Email:          result.Email,
		PasswordHash:   result.PasswordHash,
		Role:           payload.Role,
		FullName:       result.FullName,
		IsActive:       result.IsActive,
		AvatarURL:      result.AvatarURL,
		Timezone:       result.Timezone,
	}

	return s.OrganizationRepo.UpdateStatusAndRole(payload.UserID, req)

}

func (s *Organizationservice) GetUserInOrganization(id uuid.UUID, page int, pageSize int) ([]models.User, response.Pagination, *response.Error) {

	return s.OrganizationRepo.GetUsersByOrganizationID(id, page, pageSize)
}

package services

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/email"
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
	InviteOrganizationMember(inviterID uuid.UUID, organizationID uuid.UUID, payload dto.InviteOrganizationMemberRequest) *response.Error
	AcceptInvitation(userID uuid.UUID, token string) *response.Error
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

	return s.OrganizationRepo.UpdateUserStatus(payload.UserID, req)

}

func (s *Organizationservice) InviteOrganizationMember(inviterID uuid.UUID, organizationID uuid.UUID, payload dto.InviteOrganizationMemberRequest) *response.Error {
	inviter, invErr := s.AuthRepo.GetByID(inviterID)
	if invErr != nil {
		return invErr
	}
	if inviter.Role != string(dto.RoleOrgAdmin) || inviter.OrganizationID == nil || *inviter.OrganizationID != organizationID {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only organization administrators can invite members",
			Details:    []response.Details{{Field: "role", Message: "Only organization administrators can invite members"}}}
	}

	existingPending, pendingErr := s.OrganizationRepo.GetPendingInvitationByEmail(organizationID, payload.Email)
	if pendingErr != nil {
		return pendingErr
	}

	org, orgErr := s.OrganizationRepo.GetByID(organizationID)
	if orgErr != nil {
		return orgErr
	}

	expiresAt := time.Now().Add(1 * 24 * time.Hour)
	invitation := models.OrganizationInvitation{
		OrganizationID: organizationID,
		Email:          payload.Email,
		Role:           payload.Role,
		Status:         models.InvitationStatusPending,
		ExpiresAt:      expiresAt,
		CreatedBy:      inviterID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	invitationToken, err := s.generateInvitationToken()
	if err != nil {
		return err
	}
	invitation.Token = invitationToken
	invitation.Status = models.InvitationStatusPending
	invitation.ExpiresAt = expiresAt
	invitation.AcceptedAt = nil
	invitation.UpdatedAt = time.Now()
	if existingPending.ID != uuid.Nil {
		invitation.ID = existingPending.ID
		invitation.CreatedAt = existingPending.CreatedAt
		if err := s.OrganizationRepo.UpdateInvitation(invitation); err != nil {
			return err
		}
	} else {
		if err := s.OrganizationRepo.CreateOrganizationInvitation(invitation); err != nil {
			return err
		}
	}

	inviteLink := fmt.Sprintf("%s/invitations/accept?token=%s", config.GetEnv("FRONTEND_URL", "http://localhost:3000"), invitation.Token)
	if err := email.SendOrganizationInvitation(payload.Email, org.Name, invitation.Role, inviteLink); err != nil {
		s.logger.Warn("Failed to send organization invitation email", zap.Error(err))
	}

	auditErr := s.OrganizationRepo.CreateAuditLog(models.AuditLog{
		UserID:         &inviterID,
		OrganizationID: &organizationID,
		Action:         "organization_invitation_created",
		ResourceType:   "organization_invitation",
		ResourceID:     invitation.Token,
		Details:        fmt.Sprintf("Invited %s to %s as %s", payload.Email, org.Name, payload.Role),
		CreatedAt:      time.Now(),
	})
	if auditErr != nil {
		return auditErr
	}
	return nil
}

func (s *Organizationservice) generateInvitationToken() (string, *response.Error) {
	newToken, err := uuid.NewV7()
	if err != nil {
		return "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to generate invitation token",
			Details:    []response.Details{{Message: err.Error()}},
		}
	}
	return newToken.String(), nil
}

func (s *Organizationservice) AcceptInvitation(userID uuid.UUID, token string) *response.Error {
	if token == "" {
		return &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invitation token is required",
			Details: []response.Details{{
				Field:   "token",
				Message: "Invitation token is required"}},
		}
	}

	invitation, invErr := s.OrganizationRepo.GetInvitationByToken(token)
	if invErr != nil {
		return invErr
	}
	if invitation.ID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Invitation not found",
			Details: []response.Details{{
				Field:   "token",
				Message: "Invitation not found"}},
		}
	}
	if invitation.Status == models.InvitationStatusAccepted {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Invitation has already been accepted",
			Details: []response.Details{{
				Field:   "token",
				Message: "Invitation has already been accepted"}},
		}
	}
	if invitation.Status == models.InvitationStatusExpired || invitation.ExpiresAt.Before(time.Now()) {
		invitation.Status = models.InvitationStatusExpired
		invitation.UpdatedAt = time.Now()
		if err := s.OrganizationRepo.UpdateInvitation(invitation); err != nil {
			return err
		}
		return &response.Error{
			Code:       response.ErrGone,
			StatusCode: http.StatusGone,
			Message:    "Invitation has expired",
			Details: []response.Details{{
				Field:   "token",
				Message: "Invitation has expired"}},
		}
	}

	user, userErr := s.AuthRepo.GetByID(userID)
	if userErr != nil {
		return userErr
	}

	user.OrganizationID = &invitation.OrganizationID
	user.Role = invitation.Role
	user.IsActive = true
	if err := s.AuthRepo.UpdateUser(userID, user); err != nil {
		return err
	}

	acceptedAt := time.Now()
	invitation.Status = models.InvitationStatusAccepted
	invitation.AcceptedAt = &acceptedAt
	invitation.UpdatedAt = acceptedAt
	if err := s.OrganizationRepo.UpdateInvitation(invitation); err != nil {
		return err
	}

	auditErr := s.OrganizationRepo.CreateAuditLog(models.AuditLog{
		UserID:         &userID,
		OrganizationID: &invitation.OrganizationID,
		Action:         "organization_invitation_accepted",
		ResourceType:   "organization_invitation",
		ResourceID:     invitation.Token,
		Details:        fmt.Sprintf("Accepted invitation for %s", invitation.Email),
		CreatedAt:      acceptedAt,
	})
	if auditErr != nil {
		return auditErr
	}
	return nil
}

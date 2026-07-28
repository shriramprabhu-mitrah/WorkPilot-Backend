package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/email"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	organizationrepo "github.com/ms-kanban-server/internal/repository/organization-repo"
	"go.uber.org/zap"
)

type OrganizationService interface {
	GetOrganizationByID(id uuid.UUID) (models.Organization, *response.Error)
	CreateOrganization(row models.Organization) (*dto.AuthTokensResponse, *response.Error)
	UpdateOrganization(id uuid.UUID, req models.Organization) *response.Error
	DeleteOrganization(id uuid.UUID) *response.Error
	UpdateUserStatus(payload dto.UpdateUserStatus) *response.Error
	UpdateUserRole(payload dto.UpdateUserRole) *response.Error
	InviteOrganizationMember(inviterID uuid.UUID, organizationID uuid.UUID, payload dto.InviteOrganizationMemberRequest) *response.Error
	AcceptInvitation(userID uuid.UUID, token string) *response.Error
	GetUserInOrganization(id uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error)
	RemoveUser(payload dto.RemoveUser) *response.Error
}

func InitOrganizationService(repo organizationrepo.OrganizationRepository, AuthRepo authrepo.AuthRepository, logger *zap.Logger) OrganizationService {
	return &organizationService{
		OrganizationRepo: repo,
		logger:           logger,
		AuthRepo:         AuthRepo,
	}
}

type organizationService struct {
	AuthRepo         authrepo.AuthRepository
	OrganizationRepo organizationrepo.OrganizationRepository
	logger           *zap.Logger
}

func (s *organizationService) GetOrganizationByID(id uuid.UUID) (models.Organization, *response.Error) {

	return s.OrganizationRepo.GetByID(id)
}

func (s *organizationService) CreateOrganization(row models.Organization) (*dto.AuthTokensResponse, *response.Error) {

	slug := utils.ExtractSlug(row.Domain)
	row.Slug = slug

	err := s.OrganizationRepo.CreateOrganization(row)
	if err != nil {
		return nil, err
	}

	organization, err := s.OrganizationRepo.GetByName(row.Name)
	if err != nil {
		return nil, err
	}

	user := models.User{
		OrganizationID: &organization.ID,
		Role:           string(dto.RoleOrgAdmin),
		IsActive:       true,
		JoinedAt:       time.Now(),
	}

	err = s.AuthRepo.UpdateUser(row.CreatedBy, user)
	if err != nil {
		s.OrganizationRepo.DeleteOrganization(organization.ID)
		return nil, err
	}

	tokencredentials := dto.JWtcredentials{
		Role:           user.Role,
		UserID:         row.CreatedBy,
		OrganizationID: &organization.ID,
	}

	accessToken, tokenErr := middleware.GenerateJWT(tokencredentials, s.logger)
	if tokenErr != nil {
		return nil, tokenErr
	}

	refreshTokenValue, refreshTokenErr := generateRefreshTokenValue()
	if refreshTokenErr != nil {
		s.logger.Error("Failed to create refresh token after email verification",
			zap.String("email", user.Email))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	hashedRefreshToken, hashErr := utils.HashPassword(refreshTokenValue)
	if hashErr != nil {
		s.logger.Error("Failed hashing the refresh token after email verification",
			zap.String("email", user.Email), zap.Error(fmt.Errorf("%v", hashErr)))
		return nil, hashErr
	}

	expiresIn, parseErr := utils.StringToInt(config.GetEnv("JWT_EXPIRY", "900"))
	if parseErr != nil {
		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", parseErr)))
		return nil, parseErr
	}

	refreshExpiresIn, refreshParseErr := utils.StringToInt(config.GetEnv("REFRESH_TOKEN_EXPIRY", "604800"))
	if refreshParseErr != nil {
		s.logger.Error("Failed to set the expire time",
			zap.Error(fmt.Errorf("%v", refreshParseErr)))
		return nil, refreshParseErr
	}

	expiresAt := time.Now().Add(time.Duration(refreshExpiresIn) * time.Second)
	if storeErr := s.AuthRepo.StoreRefreshToken(models.RefreshToken{UserID: user.ID, TokenHash: hashedRefreshToken, ExpiresAt: expiresAt}); storeErr != nil {
		return nil, storeErr
	}

	s.logger.Info("Email verification completed", zap.String("email", user.Email))
	return &dto.AuthTokensResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshTokenValue,
		TokenType:        "Bearer",
		ExpiresIn:        expiresIn,
		RefreshExpiresIn: refreshExpiresIn,
	}, nil
}

func (s *organizationService) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

	return s.OrganizationRepo.UpdateOrganization(OrganizationID, req)
}

func (s *organizationService) DeleteOrganization(id uuid.UUID) *response.Error {

	return s.OrganizationRepo.DeleteOrganization(id)
}

func (s *organizationService) UpdateUserStatus(payload dto.UpdateUserStatus) *response.Error {

	result, err := s.AuthRepo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || payload.OrganizationID == nil {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != *payload.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", payload.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	request := result
	request.IsActive = payload.IsActive

	return s.OrganizationRepo.UpdateStatusAndRole(payload.UserID, request)

}

func (s *organizationService) UpdateUserRole(payload dto.UpdateUserRole) *response.Error {

	result, err := s.AuthRepo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || payload.OrganizationID == nil {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != *payload.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Payload Organization Id", payload.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	request := result
	request.Role = payload.Role

	return s.OrganizationRepo.UpdateStatusAndRole(payload.UserID, request)

}

func (s *organizationService) InviteOrganizationMember(inviterID uuid.UUID, organizationID uuid.UUID, payload dto.InviteOrganizationMemberRequest) *response.Error {
	inviter, invErr := s.AuthRepo.GetByID(inviterID)
	if invErr != nil {
		return invErr
	}
	if inviter.Role != string(dto.RoleOrgAdmin) || inviter.OrganizationID == nil || *inviter.OrganizationID != organizationID {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	inviteItems := payload.Members
	if len(inviteItems) == 0 {
		return &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "At least one member invitation is required",
		}
	}

	org, orgErr := s.OrganizationRepo.GetByID(organizationID)
	if orgErr != nil {
		return orgErr
	}

	for _, inviteItem := range inviteItems {
		existingUser, userErr := s.AuthRepo.GetByEmail(inviteItem.Email)
		if userErr == nil {
			if existingUser.OrganizationID != nil && *existingUser.OrganizationID == organizationID && existingUser.IsActive {
				return &response.Error{
					Code:       response.ErrConflict,
					StatusCode: http.StatusConflict,
					Message:    "User is already an active member of the organization",
				}
			}
		} else if userErr.StatusCode != http.StatusNotFound && userErr.StatusCode != http.StatusUnauthorized {
			return userErr
		}

		existingPending, pendingErr := s.OrganizationRepo.GetPendingInvitationByEmail(organizationID, inviteItem.Email)
		if pendingErr != nil {
			return pendingErr
		}

		expiresAt := time.Now().Add(1 * 24 * time.Hour)
		invitation := models.OrganizationInvitation{
			OrganizationID: organizationID,
			Email:          inviteItem.Email,
			Role:           inviteItem.Role,
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

		inviteLink := fmt.Sprintf("%s/invitations/accept?token=%s", config.GetEnv("FRONTEND_DASHBOARD_URL", "http://localhost:3000"), invitation.Token)
		tempPassword := ""
		if userErr != nil {
			invitationTempPassword, err := s.inviteUserWithTemporaryCredentials(inviteItem.Email, inviteItem.Role)
			if err != nil {
				return err
			}
			tempPassword = invitationTempPassword
		}

		if err := email.SendOrganizationInvitation(inviteItem.Email, org.Name, invitation.Role, inviteLink, tempPassword); err != nil {
			s.logger.Warn("Failed to send organization invitation email", zap.Error(err))
		}

		auditErr := s.OrganizationRepo.CreateAuditLog(models.AuditLog{
			UserID:         &inviterID,
			OrganizationID: &organizationID,
			Action:         "organization_invitation_created",
			ResourceType:   "organization_invitation",
			ResourceID:     invitation.Token,
			Details:        fmt.Sprintf("Invited %s to %s as %s", inviteItem.Email, org.Name, inviteItem.Role),
			CreatedAt:      time.Now(),
		})
		if auditErr != nil {
			return auditErr
		}
	}
	return nil
}

func (s *organizationService) generateInvitationToken() (string, *response.Error) {
	newToken, err := uuid.NewV7()
	if err != nil {
		return "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return newToken.String(), nil
}

func (s *organizationService) generateTemporaryPassword(length int) (string, *response.Error) {
	if length < 8 {
		length = 8
	}
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var password strings.Builder
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			s.logger.Error("Failed to generate temporary password", zap.Error(err))
			return "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}
		}
		password.WriteByte(chars[idx.Int64()])
	}
	return password.String(), nil
}

func (s *organizationService) generateUsernameFromEmail(email string) string {
	local := strings.TrimSpace(strings.Split(strings.ToLower(email), "@")[0])
	if local == "" {
		id, err := uuid.NewV7()
		if err == nil {
			return fmt.Sprintf("user_%s", strings.ReplaceAll(id.String(), "-", "")[:8])
		}
		return fmt.Sprintf("user_%d", time.Now().UnixNano()%1000000)
	}

	parts := strings.FieldsFunc(local, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	if len(parts) == 0 {
		return local
	}

	username := strings.Join(parts, "_")
	if username == "" {
		return "user"
	}
	if len(username) > 30 {
		username = username[:30]
	}
	return username
}

func (s *organizationService) generateFullNameFromEmail(email string) string {
	local := strings.TrimSpace(strings.Split(strings.ToLower(email), "@")[0])
	if local == "" {
		return "User"
	}

	parts := strings.FieldsFunc(local, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	if len(parts) == 0 {
		return "User"
	}

	words := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		words = append(words, string(runes))
	}
	if len(words) == 0 {
		return "User"
	}
	return strings.Join(words, " ")
}

func (s *organizationService) generateUniqueUsername(email string) (string, *response.Error) {
	base := s.generateUsernameFromEmail(email)
	for attempt := 0; attempt < 5; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s%d", base, attempt)
		}
		exists, err := s.AuthRepo.ExistsByUsername(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", &response.Error{
		Code:       response.ErrConflict,
		StatusCode: http.StatusConflict,
		Message:    "Unable to generate unique username for invited user",
	}
}

func (s *organizationService) inviteUserWithTemporaryCredentials(email, role string) (string, *response.Error) {
	tempPassword, err := s.generateTemporaryPassword(12)
	if err != nil {
		return "", err
	}
	passwordHash, hashErr := utils.HashPassword(tempPassword)
	if hashErr != nil {
		return "", hashErr
	}
	username, usernameErr := s.generateUniqueUsername(email)
	if usernameErr != nil {
		return "", usernameErr
	}

	user := models.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        strings.ToLower(strings.TrimSpace(email)),
		FullName:     s.generateFullNameFromEmail(email),
		UserName:     username,
		PasswordHash: passwordHash,
		Role:         role,
		Timezone:     "UTC",
		IsActive:     true,
		IsVerified:   true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.AuthRepo.CreateUser(user); err != nil {
		return "", err
	}
	return tempPassword, nil
}

func (s *organizationService) AcceptInvitation(userID uuid.UUID, token string) *response.Error {
	if token == "" {
		return &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invitation token is required",
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
		}
	}
	if invitation.Status == models.InvitationStatusAccepted {
		return &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Invitation has already been accepted",
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
		}
	}

	user, userErr := s.AuthRepo.GetByID(userID)
	if userErr != nil {
		return userErr
	}

	if !strings.EqualFold(user.Email, invitation.Email) {
		s.logger.Error("Invitation email does not match authenticated user",
			zap.String("user_email", user.Email),
			zap.String("invitation_email", invitation.Email))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You are not authorized to accept this invitation",
		}
	}

	user.OrganizationID = &invitation.OrganizationID
	user.Role = invitation.Role
	user.IsActive = true
	user.JoinedAt = time.Now()
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

func (s *organizationService) GetUserInOrganization(id uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	if filter.Role != "" {
		filter.Role = strings.ToLower(strings.TrimSpace(filter.Role))
	}
	if filter.FullName != "" {
		filter.FullName = strings.TrimSpace(filter.FullName)
	}
	if filter.Email != "" {
		filter.Email = strings.TrimSpace(filter.Email)
	}
	if filter.Username != "" {
		filter.Username = strings.TrimSpace(filter.Username)
	}
	if filter.Timezone != "" {
		filter.Timezone = strings.TrimSpace(filter.Timezone)
	}

	return s.OrganizationRepo.GetUsersByOrganizationID(id, filter)
}

func (s *organizationService) RemoveUser(payload dto.RemoveUser) *response.Error {

	result, err := s.AuthRepo.GetByID(payload.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || payload.OrganizationID == nil {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != *payload.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", payload.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	return s.OrganizationRepo.DeleteUser(payload.UserID)

}

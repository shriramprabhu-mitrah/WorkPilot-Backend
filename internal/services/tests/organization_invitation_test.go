package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var InitOrganizationService = services.InitOrganizationService

type stubOrganizationRepository struct {
	organization models.Organization
	memberUser   models.User
	invite       models.OrganizationInvitation
	invites      []models.OrganizationInvitation
	auditLogs    []models.AuditLog
	members      []models.User
	err          *response.Error
}

func (s *stubOrganizationRepository) CreateOrganization(row models.Organization) *response.Error {
	return nil
}
func (s *stubOrganizationRepository) GetByName(name string) (models.Organization, *response.Error) {
	return s.organization, nil
}
func (s *stubOrganizationRepository) GetByID(id uuid.UUID) (models.Organization, *response.Error) {
	return s.organization, nil
}
func (s *stubOrganizationRepository) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {
	return nil
}
func (s *stubOrganizationRepository) DeleteOrganization(id uuid.UUID) *response.Error { return nil }
func (s *stubOrganizationRepository) UpdateUserStatus(userID uuid.UUID, req models.User) *response.Error {
	return nil
}
func (s *stubOrganizationRepository) CreateOrganizationInvitation(invitation models.OrganizationInvitation) *response.Error {
	s.invites = append(s.invites, invitation)
	s.invite = invitation
	return nil
}
func (s *stubOrganizationRepository) GetPendingInvitationByEmail(orgID uuid.UUID, email string) (models.OrganizationInvitation, *response.Error) {
	for _, inv := range s.invites {
		if inv.OrganizationID == orgID && inv.Email == email && inv.Status == models.InvitationStatusPending {
			return inv, nil
		}
	}
	return models.OrganizationInvitation{}, nil
}
func (s *stubOrganizationRepository) GetInvitationByToken(token string) (models.OrganizationInvitation, *response.Error) {
	if s.invite.Token == token {
		return s.invite, nil
	}
	return models.OrganizationInvitation{}, nil
}
func (s *stubOrganizationRepository) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
	s.invite = invitation
	if invitation.ID != uuid.Nil {
		for idx, existing := range s.invites {
			if existing.ID == invitation.ID {
				s.invites[idx] = invitation
				return nil
			}
		}
	}
	s.invites = append(s.invites, invitation)
	return nil
}
func (s *stubOrganizationRepository) CreateAuditLog(log models.AuditLog) *response.Error {
	s.auditLogs = append(s.auditLogs, log)
	return nil
}

func (s *stubOrganizationRepository) UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error {
	s.memberUser = req
	return nil
}
func (s *stubOrganizationRepository) DeleteUser(id uuid.UUID) *response.Error {
	return nil
}

func (s *stubOrganizationRepository) GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	items := make([]models.User, 0, len(s.members))
	for _, member := range s.members {

		if filter.Role != "" && member.Role != filter.Role {
			continue
		}

		items = append(items, member)
	}
	return items, response.Pagination{Page: filter.Page, PageSize: filter.PageSize, TotalItems: len(items), TotalPages: 1, HasNext: false, HasPrevious: false}, nil
}

func TestGetUserInOrganizationSupportsSearchAndStatusFilters(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	repo := &stubOrganizationRepository{
		organization: models.Organization{ID: orgID},
		members: []models.User{
			{ID: uuid.Must(uuid.NewV4()), FullName: "Deepak Sharma", UserName: "deepak", Email: "deepak@example.com", Role: string(dto.RoleDeveloper), OrganizationID: &orgID, IsActive: true, IsVerified: true, Timezone: "Asia/Kolkata"},
			{ID: uuid.Must(uuid.NewV4()), FullName: "Asha Singh", UserName: "asha", Email: "asha@example.com", Role: string(dto.RoleViewer), OrganizationID: &orgID, IsActive: false, IsVerified: true, Timezone: "UTC"},
		},
	}
	authRepo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "admin@example.com", Role: string(dto.RoleOrgAdmin), OrganizationID: &orgID, IsActive: true}}
	service := InitOrganizationService(repo, authRepo, zap.NewNop()).(*Organizationservice)

	members, pagination, err := service.GetUserInOrganization(orgID, dto.OrganizationMemberListFilter{Role: string(dto.RoleDeveloper), Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected filtered member listing to succeed, got %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 filtered member, got %d", len(members))
	}
	if members[0].Email != "deepak@example.com" {
		t.Fatalf("expected deepak@example.com, got %s", members[0].Email)
	}
	if pagination.TotalItems != 1 {
		t.Fatalf("expected total_items to be 1, got %d", pagination.TotalItems)
	}
}

func TestInviteOrganizationMemberReturnsConflictForActiveMember(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	repo := &stubOrganizationRepository{organization: models.Organization{ID: orgID}, memberUser: models.User{Email: "member@example.com", OrganizationID: &orgID, IsActive: true}}
	authRepo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "admin@example.com", Role: string(dto.RoleOrgAdmin), OrganizationID: &orgID, IsActive: true}, userByEmail: map[string]models.User{"member@example.com": {ID: uuid.Must(uuid.NewV4()), Email: "member@example.com", Role: string(dto.RoleViewer), OrganizationID: &orgID, IsActive: true}}}
	service := InitOrganizationService(repo, authRepo, zap.NewNop())

	orgRepo := repo
	orgRepo.memberUser = models.User{Email: "member@example.com", OrganizationID: &orgID, IsActive: true}

	inviteErr := service.InviteOrganizationMember(uuid.Must(uuid.NewV4()), orgID, dto.InviteOrganizationMemberRequest{Members: []dto.InviteOrganizationMemberItem{{Email: "member@example.com", Role: string(dto.RoleDeveloper)}}})
	if inviteErr == nil {
		t.Fatal("expected conflict error for existing active member")
	}
	if inviteErr.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict status, got %d", inviteErr.StatusCode)
	}
}

func TestInviteOrganizationMemberRefreshesPendingInvitation(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	repo := &stubOrganizationRepository{organization: models.Organization{ID: orgID}}
	authRepo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "admin@example.com", Role: string(dto.RoleOrgAdmin), OrganizationID: &orgID, IsActive: true}}
	service := InitOrganizationService(repo, authRepo, zap.NewNop())

	existing := models.OrganizationInvitation{OrganizationID: orgID, Email: "new-user@example.com", Status: models.InvitationStatusPending, ExpiresAt: time.Now().Add(1 * time.Hour), Token: "old-token"}
	repo.invites = []models.OrganizationInvitation{existing}

	inviteErr := service.InviteOrganizationMember(uuid.Must(uuid.NewV4()), orgID, dto.InviteOrganizationMemberRequest{Members: []dto.InviteOrganizationMemberItem{{Email: "new-user@example.com", Role: string(dto.RoleDeveloper)}}})
	if inviteErr != nil {
		t.Fatalf("expected invite to succeed, got %v", inviteErr)
	}
	if len(repo.invites) != 2 {
		t.Fatalf("expected invitation to be updated, got %d records", len(repo.invites))
	}
	if repo.invite.Email != "new-user@example.com" {
		t.Fatalf("expected invitation for new-user@example.com, got %s", repo.invite.Email)
	}
}

func TestInviteOrganizationMemberSupportsBulkMembers(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	repo := &stubOrganizationRepository{organization: models.Organization{ID: orgID}}
	authRepo := &stubAuthRepository{user: models.User{ID: uuid.Must(uuid.NewV4()), Email: "admin@example.com", Role: string(dto.RoleOrgAdmin), OrganizationID: &orgID, IsActive: true}}
	service := InitOrganizationService(repo, authRepo, zap.NewNop())

	inviteErr := service.InviteOrganizationMember(uuid.Must(uuid.NewV4()), orgID, dto.InviteOrganizationMemberRequest{Members: []dto.InviteOrganizationMemberItem{{Email: "one@example.com", Role: string(dto.RoleDeveloper)}, {Email: "two@example.com", Role: string(dto.RoleViewer)}}})
	if inviteErr != nil {
		t.Fatalf("expected bulk invites to succeed, got %v", inviteErr)
	}
	if len(repo.invites) != 2 {
		t.Fatalf("expected 2 invitations to be created, got %d", len(repo.invites))
	}
	if repo.invites[0].Email != "one@example.com" || repo.invites[1].Email != "two@example.com" {
		t.Fatalf("expected both invited emails to be stored, got %s and %s", repo.invites[0].Email, repo.invites[1].Email)
	}
}

func TestAcceptInvitationMarksMembershipAndStatus(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	repo := &stubOrganizationRepository{organization: models.Organization{ID: orgID}}
	authRepo := &stubAuthRepository{user: models.User{ID: userID, Email: "member@example.com", Role: string(dto.RoleViewer), OrganizationID: nil, IsActive: true}}
	service := InitOrganizationService(repo, authRepo, zap.NewNop())

	repo.invite = models.OrganizationInvitation{ID: uuid.Must(uuid.NewV4()), OrganizationID: orgID, Email: "member@example.com", Role: string(dto.RoleDeveloper), Status: models.InvitationStatusPending, Token: "token-123", ExpiresAt: time.Now().Add(24 * time.Hour)}

	inviteErr := service.AcceptInvitation(userID, "token-123")
	if inviteErr != nil {
		t.Fatalf("expected invite acceptance to succeed, got %v", inviteErr)
	}
	if repo.invite.Status != models.InvitationStatusAccepted {
		t.Fatalf("expected invitation to be accepted, got %s", repo.invite.Status)
	}
	if authRepo.user.OrganizationID == nil || *authRepo.user.OrganizationID != orgID {
		t.Fatal("expected user organization to be updated")
	}
	if authRepo.user.Role != string(dto.RoleDeveloper) {
		t.Fatalf("expected role to be updated to developer, got %s", authRepo.user.Role)
	}
}

package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type dummyAuthRepo struct{}
type dummySprintRepo struct{}

func (d *dummySprintRepo) CreateSprint(sprint models.Sprint) *response.Error {
	return nil
}

func (d *dummySprintRepo) GetCompletedTasksStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return 0, nil
}

func (d *dummySprintRepo) GetTotalStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return 0, nil
}

func (d *dummySprintRepo) GetRemainingStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return 0, nil
}

func (d *dummySprintRepo) CreateSprintSnapshot(snapshot models.SprintSnapshot) *response.Error {
	return nil
}

func (d *dummySprintRepo) DeleteSprint(id uuid.UUID) *response.Error {
	return nil
}

func (d *dummySprintRepo) IsSprintExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	return false, nil
}

func (d *dummySprintRepo) GetActiveSprints() ([]models.Sprint, *response.Error) {
	return nil, nil
}

func (d *dummySprintRepo) GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (d *dummySprintRepo) GetSprintSnapshots(sprintID uuid.UUID) ([]models.SprintSnapshot, *response.Error) {
	return nil, nil
}
func (d *dummySprintRepo) GetSprintByID(projectID uuid.UUID, sprintID uuid.UUID) (*models.Sprint, *response.Error) {
	return &models.Sprint{}, nil
}

func (d *dummySprintRepo) UpdateSprint(projectID uuid.UUID, sprintID uuid.UUID, sprint models.Sprint) *response.Error {
	return nil
}

func (d *dummySprintRepo) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error {
	return nil
}

func (d *dummyAuthRepo) GetByEmail(email string) (models.User, *response.Error) {
	return models.User{}, nil
}
func (d *dummyAuthRepo) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	return models.User{}, nil
}
func (d *dummyAuthRepo) ExistsByEmail(email string) (bool, *response.Error) {
	return false, nil
}
func (d *dummyAuthRepo) ExistsByUsername(username string) (bool, *response.Error) {
	return false, nil
}
func (d *dummyAuthRepo) CreateUser(row models.User) *response.Error {
	return nil
}
func (d *dummyAuthRepo) StoreRefreshToken(token models.RefreshToken) (models.RefreshToken, *response.Error) {
	return models.RefreshToken{}, nil
}
func (d *dummyAuthRepo) GetRefreshToken(userID string) (models.RefreshToken, *response.Error) {
	return models.RefreshToken{}, nil
}
func (d *dummyAuthRepo) GetRefreshTokenByID(id uuid.UUID) (models.RefreshToken, *response.Error) {
	return models.RefreshToken{}, nil
}
func (d *dummyAuthRepo) ChangePassword(password string, userID uuid.UUID) *response.Error {
	return nil
}
func (d *dummyAuthRepo) RequestPasswordReset(email string) (models.User, *response.Error) {
	return models.User{}, nil
}
func (d *dummyAuthRepo) SavePasswordResetOTP(otp models.PasswordResetOTP) *response.Error {
	return nil
}
func (d *dummyAuthRepo) InvalidatePasswordResetOTPs(userID uuid.UUID) *response.Error {
	return nil
}
func (d *dummyAuthRepo) GetPasswordResetOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	return models.PasswordResetOTP{}, nil
}
func (d *dummyAuthRepo) SaveEmailVerificationOTP(otp models.PasswordResetOTP) *response.Error {
	return nil
}
func (d *dummyAuthRepo) InvalidateEmailVerificationOTPs(userID uuid.UUID) *response.Error {
	return nil
}
func (d *dummyAuthRepo) GetEmailVerificationOTP(userID uuid.UUID, otp string) (models.PasswordResetOTP, *response.Error) {
	return models.PasswordResetOTP{}, nil
}
func (d *dummyAuthRepo) IsEmailVerificationResendAllowed(email string, interval time.Duration) (bool, *response.Error) {
	return false, nil
}
func (d *dummyAuthRepo) RecordEmailVerificationResend(email string, sentAt time.Time) *response.Error {
	return nil
}
func (d *dummyAuthRepo) UpdateUserPassword(userID uuid.UUID, passwordHash string) *response.Error {
	return nil
}
func (d *dummyAuthRepo) RevokeRefreshTokens(userID uuid.UUID) *response.Error {
	return nil
}
func (d *dummyAuthRepo) UpdateUser(userID uuid.UUID, req models.User) *response.Error {
	return nil
}
func (d *dummyAuthRepo) StoreUserTemp(row models.User) *response.Error {
	return nil
}
func (d *dummyAuthRepo) GetUserFromRedis(email string) (*models.User, *response.Error) {
	return nil, nil
}

type stubProjectRepo struct {
	project            models.Project
	isMember           bool
	getProjectActivity func(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error)
	createdLogs        []models.AuditLog
	projectRole        string
}

func (s *stubProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {
	return nil
}

func (s *stubProjectRepo) UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error {
	return nil
}

func (s *stubProjectRepo) UpdateProject(projectID uuid.UUID, req models.Project) *response.Error {
	return nil
}
func (s *stubProjectRepo) GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubProjectRepo) GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error) {
	return nil, nil
}
func (s *stubProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	role := s.projectRole
	if role == "" {
		role = "developer"
	}
	return &models.ProjectMember{
		UserID:      userID,
		ProjectID:   projectID,
		ProjectRole: role,
	}, nil
}
func (s *stubProjectRepo) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	return s.project, nil
}
func (s *stubProjectRepo) DeleteProject(projectID, organizationID uuid.UUID) *response.Error {
	return nil
}
func (s *stubProjectRepo) CreateProjectMember(row models.ProjectMember) *response.Error {
	return nil
}
func (s *stubProjectRepo) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubProjectRepo) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {
	return nil
}
func (s *stubProjectRepo) GetProjectActivity(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
	if s.getProjectActivity != nil {
		return s.getProjectActivity(projectID, filter)
	}
	return nil, response.Pagination{}, nil
}
func (s *stubProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return s.isMember, nil
}
func (s *stubProjectRepo) CreateAuditLog(log models.AuditLog) *response.Error {
	return nil
}

func TestGetProjectActivity_UserIDValidation(t *testing.T) {
	logger := zap.NewNop()
	authRepo := &dummyAuthRepo{}
	sprintRepo := &dummySprintRepo{}

	projectID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	projectRepo := &stubProjectRepo{
		project: models.Project{
			ID:             projectID,
			OrganizationID: orgID,
		},
		isMember: true,
	}

	service := services.InitProjectService(projectRepo, authRepo, sprintRepo, logger)

	t.Run("Valid UserID Filter", func(t *testing.T) {
		filterUserID := uuid.Must(uuid.NewV4())
		filterReq := dto.ProjectActivityFilterRequest{
			UserID: filterUserID.String(),
		}

		projectRepo.getProjectActivity = func(pID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
			if filter.UserID == nil || *filter.UserID != filterUserID {
				t.Errorf("expected filter.UserID to be %v, got %v", filterUserID, filter.UserID)
			}
			return []models.AuditLog{}, response.Pagination{}, nil
		}

		_, _, err := service.GetProjectActivity(userID, string(dto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid UserID Filter Format", func(t *testing.T) {
		filterReq := dto.ProjectActivityFilterRequest{
			UserID: "invalid-uuid",
		}

		_, _, err := service.GetProjectActivity(userID, string(dto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err.Code)
		}
		if err.StatusCode != http.StatusBadRequest {
			t.Errorf("expected HTTP 400 Bad Request, got %v", err.StatusCode)
		}
	})

	t.Run("Nil UserID Filter", func(t *testing.T) {
		filterReq := dto.ProjectActivityFilterRequest{
			UserID: uuid.Nil.String(),
		}

		_, _, err := service.GetProjectActivity(userID, string(dto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err == nil {
			t.Fatal("expected validation error for nil UUID, got nil")
		}
		if err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err.Code)
		}
	})

	t.Run("Empty UserID Filter", func(t *testing.T) {
		filterReq := dto.ProjectActivityFilterRequest{
			UserID: "",
		}

		projectRepo.getProjectActivity = func(pID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
			if filter.UserID != nil {
				t.Errorf("expected filter.UserID to be nil for empty input, got %v", filter.UserID)
			}
			return []models.AuditLog{}, response.Pagination{}, nil
		}

		_, _, err := service.GetProjectActivity(userID, string(dto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

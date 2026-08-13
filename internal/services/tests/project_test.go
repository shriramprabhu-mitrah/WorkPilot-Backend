package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type dummyAuthRepo struct{}
type dummySprintRepo struct {
	sprintCountsByProject map[uuid.UUID]int
	sprintsByProjectID    map[uuid.UUID][]models.Sprint
}

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

func (d *dummySprintRepo) IsSprintDateRangeExists(projectID uuid.UUID, startDate, endDate time.Time, excludeSprintID uuid.UUID) (bool, *response.Error) {
	return false, nil
}

func (d *dummySprintRepo) GetActiveSprints() ([]models.Sprint, *response.Error) {
	return nil, nil
}

func (d *dummySprintRepo) GetSprints(projectID uuid.UUID, filter requestdto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func (d *dummySprintRepo) GetSprintsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID][]models.Sprint, *response.Error) {
	if d.sprintsByProjectID != nil {
		return d.sprintsByProjectID, nil
	}
	return make(map[uuid.UUID][]models.Sprint), nil
}

func (d *dummySprintRepo) GetSprintSnapshots(sprintID uuid.UUID) ([]models.SprintSnapshot, *response.Error) {
	return nil, nil
}
func (d *dummySprintRepo) GetSprintByID(projectID uuid.UUID, sprintID uuid.UUID) (*models.Sprint, *response.Error) {
	return &models.Sprint{}, nil
}

func (d *dummySprintRepo) UpdateSprint(projectID uuid.UUID, sprintID uuid.UUID, updates map[string]interface{}) *response.Error {
	return nil
}

func (d *dummySprintRepo) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error {
	return nil
}

func (d *dummySprintRepo) GetSprintCountByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int, *response.Error) {
	return make(map[uuid.UUID]int), nil
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
func (d *dummyAuthRepo) UpdateUserFields(userID uuid.UUID, updates map[string]interface{}) *response.Error {
	return nil
}
func (d *dummyAuthRepo) StoreUserTemp(row models.User) *response.Error {
	return nil
}
func (d *dummyAuthRepo) GetUserFromRedis(email string) (*models.User, *response.Error) {
	return nil, nil
}
func (d *dummyAuthRepo) GetPendingInvitationByEmail(email string) (models.OrganizationInvitation, *response.Error) {
	return models.OrganizationInvitation{}, nil
}
func (d *dummyAuthRepo) UpdateInvitation(invitation models.OrganizationInvitation) *response.Error {
	return nil
}

type stubProjectRepo struct {
	project            models.Project
	isMember           bool
	getProjectActivity func(projectID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error)
	updateProjectFunc  func(projectID uuid.UUID, updates map[string]interface{}) *response.Error
	createdLogs        []models.AuditLog
	projectRole        string
}

func (s *stubProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {
	return nil
}

func (s *stubProjectRepo) UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error {
	return nil
}

func (s *stubProjectRepo) UpdateProject(projectID uuid.UUID, req map[string]interface{}) *response.Error {
	if s.updateProjectFunc != nil {
		return s.updateProjectFunc(projectID, req)
	}
	return nil
}
func (s *stubProjectRepo) GetProjectsByOrganizationID(organizationID uuid.UUID, filter requestdto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {
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
func (s *stubProjectRepo) GetProjectsMembersByProjectID(projectID uuid.UUID, filter requestdto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubProjectRepo) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {
	return nil
}
func (s *stubProjectRepo) GetProjectActivity(projectID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
	if s.getProjectActivity != nil {
		return s.getProjectActivity(projectID, filter)
	}
	return nil, response.Pagination{}, nil
}
func (s *stubProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return s.isMember, nil
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

	auditRepo := &stubAuditLogRepo{}
	taskRepo := &stubTaskRepo{}
	service := services.InitProjectService(projectRepo, authRepo, sprintRepo, taskRepo, auditRepo, logger)

	t.Run("Valid UserID Filter", func(t *testing.T) {
		filterUserID := uuid.Must(uuid.NewV4())
		filterReq := requestdto.ProjectActivityFilterRequest{
			UserID: filterUserID.String(),
		}

		projectRepo.getProjectActivity = func(pID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
			if filter.UserID == nil || *filter.UserID != filterUserID {
				t.Errorf("expected filter.UserID to be %v, got %v", filterUserID, filter.UserID)
			}
			return []models.AuditLog{}, response.Pagination{}, nil
		}

		_, _, err := service.GetProjectActivity(userID, string(requestdto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid UserID Filter Format", func(t *testing.T) {
		filterReq := requestdto.ProjectActivityFilterRequest{
			UserID: "invalid-uuid",
		}

		_, _, err := service.GetProjectActivity(userID, string(requestdto.RoleOrgAdmin), orgID, projectID, filterReq)
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
		filterReq := requestdto.ProjectActivityFilterRequest{
			UserID: uuid.Nil.String(),
		}

		_, _, err := service.GetProjectActivity(userID, string(requestdto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err == nil {
			t.Fatal("expected validation error for nil UUID, got nil")
		}
		if err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err.Code)
		}
	})

	t.Run("Empty UserID Filter", func(t *testing.T) {
		filterReq := requestdto.ProjectActivityFilterRequest{
			UserID: "",
		}

		projectRepo.getProjectActivity = func(pID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
			if filter.UserID != nil {
				t.Errorf("expected filter.UserID to be nil for empty input, got %v", filter.UserID)
			}
			return []models.AuditLog{}, response.Pagination{}, nil
		}

		_, _, err := service.GetProjectActivity(userID, string(requestdto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGetProjectActivity_TaskAndUserMapping(t *testing.T) {
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

	auditRepo := &stubAuditLogRepo{}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
	service := services.InitProjectService(projectRepo, authRepo, sprintRepo, taskRepo, auditRepo, logger)

	t.Run("Maps User Profile and Task Details correctly", func(t *testing.T) {
		taskID := uuid.Must(uuid.NewV4())
		filterReq := requestdto.ProjectActivityFilterRequest{}

		projectRepo.getProjectActivity = func(pID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
			logs := []models.AuditLog{
				{
					ID:             uuid.Must(uuid.NewV4()),
					ProjectID:      &projectID,
					OrganizationID: &orgID,
					UserID:         &userID,
					Action:         "task_created",
					ResourceType:   "task",
					ResourceID:     taskID.String(),
					Details:        "Task created",
					CreatedAt:      time.Now(),
					User: models.User{
						ID:        userID,
						FullName:  "John Doe",
						Email:     "john@example.com",
						AvatarURL: "http://example.com/avatar.png",
						Role:      "member",
					},
					Title:   "My Awesome Task",
					TaskKey: "PROJ-123",
				},
			}
			return logs, response.Pagination{}, nil
		}

		res, _, err := service.GetProjectActivity(userID, string(requestdto.RoleOrgAdmin), orgID, projectID, filterReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(res) != 1 {
			t.Fatalf("expected 1 log, got %d", len(res))
		}

		log := res[0]
		if log.User == nil || log.User.ID != userID || log.User.FullName != "John Doe" || log.User.Email != "john@example.com" {
			t.Errorf("incorrect user details mapping: %+v", log.User)
		}

		if log.TaskKey != "PROJ-123" {
			t.Errorf("expected TaskKey to be 'PROJ-123', got '%s'", log.TaskKey)
		}

		if log.Title != "My Awesome Task" {
			t.Errorf("expected Title to be 'My Awesome Task', got '%s'", log.Title)
		}
	})
}

type stubAuditLogRepo struct {
	createdLogs []models.AuditLog
	err         *response.Error
}

func (s *stubAuditLogRepo) CreateAuditLog(log models.AuditLog) *response.Error {
	s.createdLogs = append(s.createdLogs, log)
	return s.err
}

func (s *stubAuditLogRepo) GetAuditLogs(req requestdto.GetAudit) ([]models.AuditLog, response.Pagination, *response.Error) {

	return nil, response.Pagination{}, nil
}

func TestProjectService_UpdateProject_PatchSemantics(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	t.Run("Omitted fields resulting in empty updates map", func(t *testing.T) {
		projectRepo := &stubProjectRepo{
			projectRole: string(requestdto.ProjectRoleProjectManager),
			isMember:    true,
			project: models.Project{
				ID:             projectID,
				OrganizationID: orgID,
				Name:           "Original Name",
				Description:    "Original Description",
			},
			updateProjectFunc: func(pID uuid.UUID, updates map[string]interface{}) *response.Error {
				t.Error("expected update to not be called since there are no changes")
				return nil
			},
		}

		dummyAuth := &dummyAuthRepo{} // embeds and stubs auth interface methods returning default values
		taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
		service := services.InitProjectService(projectRepo, dummyAuth, &dummySprintRepo{}, taskRepo, &stubAuditLogRepo{}, logger)

		req := requestdto.UpdateProjectRequest{
			ProjectID:      projectID,
			UserID:         userID,
			OrganizationID: orgID,
		}

		err := service.UpdateProject(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Updates name and sets description to empty string", func(t *testing.T) {
		var capturedUpdates map[string]interface{}
		projectRepo := &stubProjectRepo{
			projectRole: string(requestdto.ProjectRoleProjectManager),
			isMember:    true,
			project: models.Project{
				ID:             projectID,
				OrganizationID: orgID,
				Name:           "Original Name",
				Description:    "Original Description",
			},
			updateProjectFunc: func(pID uuid.UUID, updates map[string]interface{}) *response.Error {
				capturedUpdates = updates
				return nil
			},
		}

		dummyAuth := &dummyAuthRepo{}
		taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
		service := services.InitProjectService(projectRepo, dummyAuth, &dummySprintRepo{}, taskRepo, &stubAuditLogRepo{}, logger)

		// Title is pointer to "New Name"
		newName := "New Name"
		// Description is pointer to "" (explicitly clearing it)
		newDesc := ""

		req := requestdto.UpdateProjectRequest{
			ProjectID:      projectID,
			UserID:         userID,
			OrganizationID: orgID,
			Name:           &newName,
			Description:    &newDesc,
		}

		err := service.UpdateProject(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedUpdates == nil {
			t.Fatal("expected repository UpdateProject to be called")
		}

		if name, ok := capturedUpdates["name"].(string); !ok || name != "New Name" {
			t.Errorf("expected updates['name'] to be 'New Name', got %v", capturedUpdates["name"])
		}

		if desc, ok := capturedUpdates["description"].(string); !ok || desc != "" {
			t.Errorf("expected updates['description'] to be empty string, got %v", capturedUpdates["description"])
		}

		// Status should not be present in updates map since it was omitted
		if _, ok := capturedUpdates["status"]; ok {
			t.Errorf("expected updates['status'] to be omitted, but was present")
		}
	})
}

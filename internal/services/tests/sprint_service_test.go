package services_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var testProjRepo = &stubProjectRepo{
	projectRole: string(dto.ProjectRoleProjectManager),
}

func strPtr(s string) *string {
	return &s
}

func statusPtr(s dto.SprintStatus) *dto.SprintStatus {
	return &s
}

type sprintAuthRepoStub struct {
	dummyAuthRepo
	user models.User
	err  *response.Error
}

func (s *sprintAuthRepoStub) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	return s.user, s.err
}

type sprintRepoStub struct {
	dummySprintRepo
	createErr                          *response.Error
	deleteErr                          *response.Error
	getSprintsRes                      []models.Sprint
	getSprintsPage                     response.Pagination
	getSprintsErr                      *response.Error
	getSprintByIDRes                   *models.Sprint
	getSprintByIDErr                   *response.Error
	updateErr                          *response.Error
	lastCreateSprint                   models.Sprint
	lastDeleteSprint                   uuid.UUID
	lastUpdateProject                  uuid.UUID
	lastUpdateSprint                   uuid.UUID
	lastUpdatePayload                  map[string]interface{}
	totalStoryPoints                   int
	totalStoryPointsErr                *response.Error
	remainingStoryPoints               int
	remainingStoryPointsErr            *response.Error
	completedStoryPoints               int
	completedStoryPointsErr            *response.Error
	activeSprints                      []models.Sprint
	activeSprintsErr                   *response.Error
	snapshots                          []models.SprintSnapshot
	snapshotsErr                       *response.Error
	isSprintDateRangeExistsRes         bool
	isSprintDateRangeExistsErr         *response.Error
	lastIsSprintDateRangeExistsProject uuid.UUID
	lastIsSprintDateRangeExistsStart   time.Time
	lastIsSprintDateRangeExistsEnd     time.Time
	lastIsSprintDateRangeExistsExclude uuid.UUID
	createSnapshotErr                  *response.Error
	lastSnapshotCreated                models.SprintSnapshot
}

func (s *sprintRepoStub) CreateSprint(row *models.Sprint) *response.Error {
	s.lastCreateSprint = *row
	return s.createErr
}

func (s *sprintRepoStub) IsSprintDateRangeExists(projectID uuid.UUID, startDate, endDate time.Time, excludeSprintID uuid.UUID) (bool, *response.Error) {
	s.lastIsSprintDateRangeExistsProject = projectID
	s.lastIsSprintDateRangeExistsStart = startDate
	s.lastIsSprintDateRangeExistsEnd = endDate
	s.lastIsSprintDateRangeExistsExclude = excludeSprintID
	return s.isSprintDateRangeExistsRes, s.isSprintDateRangeExistsErr
}

func (s *sprintRepoStub) DeleteSprint(id uuid.UUID) *response.Error {
	s.lastDeleteSprint = id
	return s.deleteErr
}

func (s *sprintRepoStub) GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {
	return s.getSprintsRes, s.getSprintsPage, s.getSprintsErr
}

func (s *sprintRepoStub) GetSprintsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID][]models.Sprint, *response.Error) {
	result := make(map[uuid.UUID][]models.Sprint)
	for _, projectID := range projectIDs {
		result[projectID] = []models.Sprint{}
	}
	return result, nil
}

func (s *sprintRepoStub) GetSprintByID(projectID uuid.UUID, sprintID uuid.UUID) (*models.Sprint, *response.Error) {
	return s.getSprintByIDRes, s.getSprintByIDErr
}

func (s *sprintRepoStub) UpdateSprint(projectID uuid.UUID, sprintID uuid.UUID, updates map[string]interface{}) *response.Error {
	s.lastUpdateProject = projectID
	s.lastUpdateSprint = sprintID
	s.lastUpdatePayload = updates
	return s.updateErr
}

func (s *sprintRepoStub) CreateSprintSnapshot(snapshot models.SprintSnapshot) *response.Error {
	s.lastSnapshotCreated = snapshot
	return s.createSnapshotErr
}

func (s *sprintRepoStub) GetSprintSnapshots(sprintID uuid.UUID) ([]models.SprintSnapshot, *response.Error) {
	return s.snapshots, s.snapshotsErr
}

func (s *sprintRepoStub) GetTotalStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return s.totalStoryPoints, s.totalStoryPointsErr
}

func (s *sprintRepoStub) GetRemainingStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return s.remainingStoryPoints, s.remainingStoryPointsErr
}

func (s *sprintRepoStub) GetActiveSprints() ([]models.Sprint, *response.Error) {
	return s.activeSprints, s.activeSprintsErr
}

func (s *sprintRepoStub) GetCompletedTasksStoryPoints(sprintID uuid.UUID) (int, *response.Error) {
	return s.completedStoryPoints, s.completedStoryPointsErr
}

func TestSprintService_CreateSprint_RejectsUnauthorizedUserOrganization(t *testing.T) {
	logger := zap.NewNop()
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: nil}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	_, err := service.CreateSprint(dto.CreateSprintRequest{
		ProjectID:      uuid.Must(uuid.NewV4()),
		UserID:         uuid.Must(uuid.NewV4()),
		OrganizationID: uuid.Must(uuid.NewV4()),
	})
	if err == nil {
		t.Fatal("expected forbidden error, got nil")
	}
	if err.Code != response.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %s", err.Code)
	}
}

func TestSprintService_CreateSprint_ReturnsBadRequestForInvalidDateRange(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	_, err := service.CreateSprint(dto.CreateSprintRequest{
		ProjectID:      uuid.Must(uuid.NewV4()),
		UserID:         userID,
		OrganizationID: orgID,
		Sprints: []dto.CreateSprint{{
			Name:      "Sprint 1",
			StartDate: "2026-07-12",
			EndDate:   "2026-07-10",
		}},
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Code != response.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %s", err.Code)
	}
	if err.Message != "end_date cannot be before start_date" {
		t.Fatalf("unexpected message: %s", err.Message)
	}
}

func TestSprintService_CreateSprint_SuccessfullyPersistsSprints(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	createReq := dto.CreateSprintRequest{
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		Sprints: []dto.CreateSprint{{
			Name:      "Sprint 1",
			Goal:      "Ship MVP",
			StartDate: "2026-07-12",
			EndDate:   "2026-07-18",
		}},
	}

	_, err := service.CreateSprint(createReq)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sprintRepo.lastCreateSprint.Name != "Sprint 1" {
		t.Fatalf("expected created sprint name Sprint 1, got %s", sprintRepo.lastCreateSprint.Name)
	}
	if sprintRepo.lastCreateSprint.ProjectID != projectID {
		t.Fatalf("expected project id %s, got %s", projectID, sprintRepo.lastCreateSprint.ProjectID)
	}
	if sprintRepo.lastCreateSprint.CreatedByID != userID {
		t.Fatalf("expected created by user id %s, got %s", userID, sprintRepo.lastCreateSprint.CreatedByID)
	}
}

func TestSprintService_DeleteSprint_RejectsInvalidSprintID(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.DeleteSprint(dto.DeleteSprint{
		UserID:         userID,
		OrganizationID: orgID,
		SprintID:       uuid.Nil,
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Code != response.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %s", err.Code)
	}
}

func TestSprintService_DeleteSprint_DelegatesToRepository(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.DeleteSprint(dto.DeleteSprint{UserID: userID, OrganizationID: orgID, SprintID: sprintID})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sprintRepo.lastDeleteSprint != sprintID {
		t.Fatalf("expected delete call for sprint %s, got %s", sprintID, sprintRepo.lastDeleteSprint)
	}
}

func TestSprintService_UpdateSprint_RejectsInvalidDateFormat(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{getSprintByIDRes: &models.Sprint{StartDate: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)}}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.UpdateSprint(dto.UpdateSprintRequest{
		UserID:         userID,
		OrganizationID: orgID,
		ProjectID:      projectID,
		SprintID:       sprintID,
		StartDate:      strPtr("not-a-date"),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Code != response.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %s", err.Code)
	}
}

func TestSprintService_UpdateSprint_DelegatesToRepository(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{getSprintByIDRes: &models.Sprint{StartDate: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)}}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.UpdateSprint(dto.UpdateSprintRequest{
		Name:           strPtr("Updated sprint"),
		Goal:           strPtr("Refined goal"),
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		SprintID:       sprintID,
		Status:         statusPtr(dto.SprintStatusActive),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sprintRepo.lastUpdateProject != projectID {
		t.Fatalf("expected project id %s, got %s", projectID, sprintRepo.lastUpdateProject)
	}
	if sprintRepo.lastUpdateSprint != sprintID {
		t.Fatalf("expected sprint id %s, got %s", sprintID, sprintRepo.lastUpdateSprint)
	}
}

func TestSprintService_GetSprints_RejectsOrganizationMismatch(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	_, _, err := service.GetSprints(dto.GetSprint{UserID: userID, OrganizationID: uuid.Must(uuid.NewV4()), ProjectID: uuid.Must(uuid.NewV4())}, dto.SprintFilter{})
	if err == nil {
		t.Fatal("expected forbidden error, got nil")
	}
	if err.Code != response.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %s", err.Code)
	}
}

func TestSprintService_GetSprints_DelegatesToRepository(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{getSprintsRes: []models.Sprint{{Name: "Sprint A"}}, getSprintsPage: response.Pagination{Page: 1, PageSize: 10, TotalItems: 1, TotalPages: 1}}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	sprints, pagination, err := service.GetSprints(dto.GetSprint{UserID: userID, OrganizationID: orgID, ProjectID: projectID}, dto.SprintFilter{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sprints) != 1 {
		t.Fatalf("expected 1 sprint, got %d", len(sprints))
	}
	if pagination.TotalItems != 1 {
		t.Fatalf("expected total items 1, got %d", pagination.TotalItems)
	}
}

func TestSprintService_GetSprintByID_RejectsInvalidProjectID(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	_, err := service.GetSprintByID(dto.GetSprint{UserID: userID, OrganizationID: orgID, ProjectID: uuid.Nil, SprintID: uuid.Must(uuid.NewV4())})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Code != response.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %s", err.Code)
	}
}

func TestSprintService_GetSprintByID_DelegatesToRepository(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	want := &models.Sprint{Name: "Sprint B"}
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{getSprintByIDRes: want}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	got, err := service.GetSprintByID(dto.GetSprint{UserID: userID, OrganizationID: orgID, ProjectID: projectID, SprintID: sprintID})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got == nil || got.Name != "Sprint B" {
		t.Fatalf("expected sprint name Sprint B, got %+v", got)
	}
}

func TestSprintService_UpdateSprint_CalculatesVelocityUponCompletion(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	existingSprint := &models.Sprint{
		ID:        sprintID,
		ProjectID: projectID,
		Name:      "Sprint 1",
		Goal:      "Test Goal",
		Status:    "active",
		StartDate: time.Now(),
		EndDate:   time.Now().Add(7 * 24 * time.Hour),
	}

	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{
		getSprintByIDRes:     existingSprint,
		completedStoryPoints: 15,
	}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.UpdateSprint(dto.UpdateSprintRequest{
		SprintID:       sprintID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		Status:         statusPtr(dto.SprintStatusCompleted),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	statusVal, ok := sprintRepo.lastUpdatePayload["status"].(string)
	if !ok || statusVal != "completed" {
		t.Fatalf("expected status completed, got %v", sprintRepo.lastUpdatePayload["status"])
	}

	velocityVal, ok := sprintRepo.lastUpdatePayload["velocity"].(int)
	if !ok || velocityVal != 15 {
		t.Fatalf("expected velocity 15, got %v", sprintRepo.lastUpdatePayload["velocity"])
	}
}

func TestSprintService_GetSprintBurndown_GeneratesCorrectMetrics(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	sprintStart := time.Now().Add(-2 * 24 * time.Hour)
	sprintEnd := time.Now().Add(2 * 24 * time.Hour)

	existingSprint := &models.Sprint{
		ID:        sprintID,
		ProjectID: projectID,
		Name:      "Sprint 1",
		StartDate: sprintStart,
		EndDate:   sprintEnd,
		Status:    "active",
	}

	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{
		getSprintByIDRes:     existingSprint,
		totalStoryPoints:     20,
		remainingStoryPoints: 12,
		snapshots: []models.SprintSnapshot{
			{SprintID: sprintID, Date: sprintStart, TotalStoryPoints: 20, RemainingStoryPoints: 20},
			{SprintID: sprintID, Date: sprintStart.Add(24 * time.Hour), TotalStoryPoints: 20, RemainingStoryPoints: 16},
		},
	}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	resp, err := service.GetSprintBurndown(sprintID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.TotalStoryPoints != 20 {
		t.Fatalf("expected total points 20, got %d", resp.TotalStoryPoints)
	}

	if len(resp.BurndownData) < 4 {
		t.Fatalf("expected at least 4 data points, got %d", len(resp.BurndownData))
	}

	if resp.BurndownData[0].RemainingPoints == nil || *resp.BurndownData[0].RemainingPoints != 20 {
		t.Fatalf("expected day 0 remaining points 20, got %v", resp.BurndownData[0].RemainingPoints)
	}

	if resp.BurndownData[1].RemainingPoints == nil || *resp.BurndownData[1].RemainingPoints != 16 {
		t.Fatalf("expected day 1 remaining points 16, got %v", resp.BurndownData[1].RemainingPoints)
	}

	if resp.BurndownData[2].RemainingPoints == nil || *resp.BurndownData[2].RemainingPoints != 12 {
		t.Fatalf("expected day 2 remaining points 12, got %v", resp.BurndownData[2].RemainingPoints)
	}

	if resp.BurndownData[3].RemainingPoints != nil {
		t.Fatalf("expected day 3 remaining points nil, got %v", resp.BurndownData[3].RemainingPoints)
	}
}

func TestSprintService_TriggerDailySnapshots_SavesActiveSprintSnapshots(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}

	sprintID := uuid.Must(uuid.NewV4())
	activeSprints := []models.Sprint{
		{ID: sprintID, Status: "active"},
	}

	sprintRepo := &sprintRepoStub{
		activeSprints:        activeSprints,
		totalStoryPoints:     30,
		remainingStoryPoints: 18,
	}

	projRepo := &stubProjectRepo{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     models.Project{OrganizationID: orgID},
	}

	service := services.InitSprintService(sprintRepo, projRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.TriggerDailySnapshots(uuid.Must(uuid.NewV4()), uuid.Must(uuid.NewV4()), orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sprintRepo.lastSnapshotCreated.SprintID != sprintID {
		t.Fatalf("expected snapshot sprint ID %s, got %s", sprintID, sprintRepo.lastSnapshotCreated.SprintID)
	}

	if sprintRepo.lastSnapshotCreated.TotalStoryPoints != 30 {
		t.Fatalf("expected total story points 30, got %d", sprintRepo.lastSnapshotCreated.TotalStoryPoints)
	}

	if sprintRepo.lastSnapshotCreated.RemainingStoryPoints != 18 {
		t.Fatalf("expected remaining story points 18, got %d", sprintRepo.lastSnapshotCreated.RemainingStoryPoints)
	}
}

func TestSprintService_CreateSprint_AllowsDuplicateDateRangeAndName(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	_, err := service.CreateSprint(dto.CreateSprintRequest{
		ProjectID:      uuid.Must(uuid.NewV4()),
		UserID:         userID,
		OrganizationID: orgID,
		Sprints: []dto.CreateSprint{
			{Name: "Sprint 1", StartDate: "2026-07-12", EndDate: "2026-07-18"},
			{Name: "Sprint 1", StartDate: "2026-07-12", EndDate: "2026-07-18"},
		},
	})
	if err != nil {
		t.Fatalf("expected duplicate date range and name to be allowed, got error %v", err)
	}
}

func TestSprintService_UpdateSprint_AllowsDuplicateDateRangeAndName(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	authRepo := &sprintAuthRepoStub{user: models.User{OrganizationID: &orgID}}
	sprintRepo := &sprintRepoStub{
		getSprintByIDRes: &models.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Sprint 1",
			StartDate: time.Now(),
			EndDate:   time.Now().Add(7 * 24 * time.Hour),
		},
	}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, &stubAuditLogRepo{}, logger)

	err := service.UpdateSprint(dto.UpdateSprintRequest{
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		SprintID:       sprintID,
		StartDate:      strPtr("2026-08-01"),
		EndDate:        strPtr("2026-08-08"),
		Name:           strPtr("Sprint 1"),
	})
	if err != nil {
		t.Fatalf("expected update to allow duplicate date range/name, got error %v", err)
	}
}

func TestSprintService_UpdateSprint_AuditLogDetails(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{
		user: models.User{
			ID:             userID,
			OrganizationID: &orgID,
			UserName:       "sprint_master",
		},
	}
	sprintRepo := &sprintRepoStub{
		getSprintByIDRes: &models.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Old Sprint",
			Goal:      "Old Goal",
			Status:    "planning",
			StartDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		},
	}
	auditRepo := &stubAuditLogRepo{}
	service := services.InitSprintService(sprintRepo, testProjRepo, authRepo, auditRepo, logger)

	newName := "New Sprint"
	newGoal := "New Goal"
	newStatus := dto.SprintStatusActive
	newStartDate := "2026-08-02"
	newEndDate := "2026-08-12"

	err := service.UpdateSprint(dto.UpdateSprintRequest{
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		SprintID:       sprintID,
		Name:           &newName,
		Goal:           &newGoal,
		Status:         &newStatus,
		StartDate:      &newStartDate,
		EndDate:        &newEndDate,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(auditRepo.createdLogs) != 1 {
		t.Fatalf("expected 1 audit log created, got %d", len(auditRepo.createdLogs))
	}

	log := auditRepo.createdLogs[0]
	expectedDetail := "Sprint updated by sprint_master: start date changed from '2026-08-01' to '2026-08-02', end date changed from '2026-08-10' to '2026-08-12', name changed from 'Old Sprint' to 'New Sprint', goal changed from 'Old Goal' to 'New Goal', status changed from 'planning' to 'active'"
	if log.Details != expectedDetail {
		t.Errorf("expected audit log detail:\n%s\ngot:\n%s", expectedDetail, log.Details)
	}
}

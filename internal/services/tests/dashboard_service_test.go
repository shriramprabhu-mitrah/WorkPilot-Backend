package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type dummyDashboardRepo struct {
	burndown map[uuid.UUID][]responsedto.SprintBurndown
	err      *response.Error
}

func (d *dummyDashboardRepo) GetOverview(projectID uuid.UUID, sprintID uuid.UUID) (responsedto.DashboardOverview, *response.Error) {
	return responsedto.DashboardOverview{}, nil
}

func (d *dummyDashboardRepo) GetTaskStatus(projectID uuid.UUID, sprintID uuid.UUID) (map[string]any, *response.Error) {
	return nil, nil
}

func (d *dummyDashboardRepo) GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.SprintBurndown, float64, float64, *response.Error) {
	if d.err != nil {
		return nil, 0, 0, d.err
	}
	return d.burndown[sprintID], 120.0, 80.0, nil
}

func (d *dummyDashboardRepo) GetWeeklyProgress(projectID uuid.UUID, startDate, endDate time.Time) ([]responsedto.WeeklyProgress, *response.Error) {
	return nil, nil
}

func (d *dummyDashboardRepo) GetTeamWorkload(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error) {
	return nil, nil
}

type sprintRepoStubForDashboard struct {
	dummySprintRepo
	activeSprints []models.Sprint
	sprintByID    map[uuid.UUID]*models.Sprint
	err           *response.Error
}

func (s *sprintRepoStubForDashboard) GetSprintByID(sprintID, projectID uuid.UUID) (*models.Sprint, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	sprint, ok := s.sprintByID[sprintID]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Sprint not found"}
	}
	return sprint, nil
}

func (s *sprintRepoStubForDashboard) GetActiveSprintsByProjectID(projectID uuid.UUID) ([]models.Sprint, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.activeSprints, nil
}

func TestDashboardService_GetSprintBurndown(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userUUID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	project := models.Project{
		ID:             projectID,
		OrganizationID: orgID,
	}

	authRepo := &stubAuthRepository{
		user: models.User{
			ID:             userUUID,
			OrganizationID: &orgID,
			IsActive:       true,
		},
	}

	projectRepo := &stubProjectRepo{
		project:  project,
		isMember: true,
	}

	sprint1ID := uuid.Must(uuid.NewV4())
	sprint2ID := uuid.Must(uuid.NewV4())

	now := time.Now()
	past := now.AddDate(0, 0, -10)
	future := now.AddDate(0, 0, 10)

	sprintsMap := map[uuid.UUID]*models.Sprint{
		sprint1ID: {
			ID:        sprint1ID,
			Name:      "Sprint 1",
			ProjectID: projectID,
			Status:    "active",
			StartDate: &past,
			EndDate:   &future,
		},
		sprint2ID: {
			ID:        sprint2ID,
			Name:      "Sprint 2",
			ProjectID: projectID,
			Status:    "active",
			StartDate: &past,
			EndDate:   &future,
		},
	}

	sprintRepo := &sprintRepoStubForDashboard{
		sprintByID: sprintsMap,
		activeSprints: []models.Sprint{
			*sprintsMap[sprint1ID],
			*sprintsMap[sprint2ID],
		},
	}

	burndownMap := map[uuid.UUID][]responsedto.SprintBurndown{
		sprint1ID: {
			{Day: 1, Date: "2026-08-01", IdealHours: 10, ActualHours: 10},
			{Day: 2, Date: "2026-08-02", IdealHours: 5, ActualHours: 4},
		},
		sprint2ID: {
			{Day: 1, Date: "2026-08-10", IdealHours: 20, ActualHours: 20},
		},
	}

	dashboardRepo := &dummyDashboardRepo{
		burndown: burndownMap,
	}

	service := services.InitDashboardService(dashboardRepo, projectRepo, authRepo, sprintRepo, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

	t.Run("successfully retrieves burndown for a specific sprint", func(t *testing.T) {
		res, err := service.GetSprintBurndown(projectID, sprint1ID, userUUID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sprintData, ok := res.SprintBurndown.(responsedto.SprintBurndownData)
		if !ok {
			t.Fatalf("expected SprintBurndown to be SprintBurndownData, got %T", res.SprintBurndown)
		}
		if sprintData.SprintID != sprint1ID {
			t.Errorf("expected sprint ID %s, got %s", sprint1ID, sprintData.SprintID)
		}
		if len(sprintData.Data) != 2 {
			t.Errorf("expected 2 burndown points, got %d", len(sprintData.Data))
		}
	})

	t.Run("successfully retrieves burndown for all active sprints when sprint_id is omitted", func(t *testing.T) {
		res, err := service.GetSprintBurndown(projectID, uuid.Nil, userUUID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sprintList, ok := res.SprintBurndown.([]responsedto.SprintBurndownData)
		if !ok {
			t.Fatalf("expected SprintBurndown to be []SprintBurndownData, got %T", res.SprintBurndown)
		}
		if len(sprintList) != 2 {
			t.Fatalf("expected 2 sprints in response, got %d", len(sprintList))
		}
		if sprintList[0].SprintID != sprint1ID || sprintList[1].SprintID != sprint2ID {
			t.Errorf("unexpected sprints in response")
		}
	})

	t.Run("skips active sprints with missing dates, invalid date ranges, or database errors when sprint_id is omitted", func(t *testing.T) {
		invalidSprintID1 := uuid.Must(uuid.NewV4())
		invalidSprintID2 := uuid.Must(uuid.NewV4())
		errorSprintID := uuid.Must(uuid.NewV4())

		now := time.Now()
		past := now.AddDate(0, 0, -10)
		future := now.AddDate(0, 0, 10)

		sprintsWithInvalid := []models.Sprint{
			*sprintsMap[sprint1ID],
			{
				ID:        invalidSprintID1, // missing dates
				Name:      "Invalid Sprint 1",
				ProjectID: projectID,
				Status:    "active",
			},
			{
				ID:        invalidSprintID2,
				Name:      "Invalid Sprint 2",
				ProjectID: projectID,
				Status:    "active",
				StartDate: &future, // end before start
				EndDate:   &past,
			},
			{
				ID:        errorSprintID,
				Name:      "Error Sprint",
				ProjectID: projectID,
				Status:    "active",
				StartDate: &past,
				EndDate:   &future,
			},
		}

		dashboardRepoWithError := &dummyDashboardRepo{
			burndown: burndownMap,
			err:      &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: "DB error"},
		}

		sprintRepoWithInvalid := &sprintRepoStubForDashboard{
			sprintByID:    sprintsMap,
			activeSprints: sprintsWithInvalid,
		}

		testService := services.InitDashboardService(dashboardRepoWithError, projectRepo, authRepo, sprintRepoWithInvalid, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

		res, err := testService.GetSprintBurndown(projectID, uuid.Nil, userUUID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sprintList, ok := res.SprintBurndown.([]responsedto.SprintBurndownData)
		if ok && len(sprintList) != 0 {
			t.Errorf("expected 0 sprints to be returned, got %d", len(sprintList))
		}
	})

	t.Run("returns unauthorized error if user is not a project member", func(t *testing.T) {
		unauthorizedProjectRepo := &stubProjectRepo{
			project:  project,
			isMember: false,
		}
		unauthService := services.InitDashboardService(dashboardRepo, unauthorizedProjectRepo, authRepo, sprintRepo, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

		_, err := unauthService.GetSprintBurndown(projectID, sprint1ID, userUUID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 status code, got %d", err.StatusCode)
		}
	})
}

func TestDashboardService_GetDashboardData(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userUUID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	project := models.Project{
		ID:             projectID,
		OrganizationID: orgID,
	}

	authRepo := &stubAuthRepository{
		user: models.User{
			ID:             userUUID,
			OrganizationID: &orgID,
			IsActive:       true,
		},
	}

	projectRepo := &stubProjectRepo{
		project:  project,
		isMember: true,
	}

	sprintID := uuid.Must(uuid.NewV4())
	now := time.Now()
	past := now.AddDate(0, 0, -10)
	future := now.AddDate(0, 0, 10)

	sprint := models.Sprint{
		ID:        sprintID,
		Name:      "Active Sprint 1",
		ProjectID: projectID,
		Status:    "active",
		StartDate: &past,
		EndDate:   &future,
	}

	sprintRepo := &sprintRepoStubForDashboard{
		sprintByID: map[uuid.UUID]*models.Sprint{
			sprintID: &sprint,
		},
		activeSprints: []models.Sprint{sprint},
	}

	dashboardRepo := &dummyDashboardRepo{
		burndown: map[uuid.UUID][]responsedto.SprintBurndown{
			sprintID: {
				{Day: 1, Date: "2026-08-01", IdealHours: 10, ActualHours: 10},
			},
		},
	}

	service := services.InitDashboardService(dashboardRepo, projectRepo, authRepo, sprintRepo, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

	t.Run("successfully retrieves dashboard data (finds active sprint)", func(t *testing.T) {
		res, err := service.GetDashboardData(projectID, userUUID, uuid.Nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sprintList, ok := res.SprintBurndown.([]responsedto.SprintBurndownData)
		if !ok {
			t.Fatalf("expected SprintBurndown to be []SprintBurndownData, got %T", res.SprintBurndown)
		}
		if len(sprintList) != 1 {
			t.Errorf("expected 1 sprint burndown, got %d", len(sprintList))
		}
	})

	t.Run("successfully retrieves dashboard data when no active sprints exist", func(t *testing.T) {
		emptySprintRepo := &sprintRepoStubForDashboard{
			activeSprints: []models.Sprint{},
		}
		emptyService := services.InitDashboardService(dashboardRepo, projectRepo, authRepo, emptySprintRepo, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

		res, err := emptyService.GetDashboardData(projectID, userUUID, uuid.Nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sprintList, ok := res.SprintBurndown.([]responsedto.SprintBurndownData)
		if ok && len(sprintList) != 0 {
			t.Errorf("expected 0 burndown data points, got %d", len(sprintList))
		}
	})
}

func TestDashboardService_GetOverview(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userUUID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	project := models.Project{
		ID:             projectID,
		OrganizationID: orgID,
	}

	authRepo := &stubAuthRepository{
		user: models.User{
			ID:             userUUID,
			OrganizationID: &orgID,
			IsActive:       true,
		},
	}

	projectRepo := &stubProjectRepo{
		project:  project,
		isMember: true,
	}

	dashboardRepo := &dummyDashboardRepo{}

	service := services.InitDashboardService(dashboardRepo, projectRepo, authRepo, &sprintRepoStubForDashboard{}, &stubTaskRepo{}, &stubAuditLogRepo{}, zap.NewNop())

	t.Run("successfully retrieves overview without sprint_id", func(t *testing.T) {
		_, err := service.GetOverview(projectID, userUUID, uuid.Nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("successfully retrieves overview with sprint_id", func(t *testing.T) {
		_, err := service.GetOverview(projectID, userUUID, sprintID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

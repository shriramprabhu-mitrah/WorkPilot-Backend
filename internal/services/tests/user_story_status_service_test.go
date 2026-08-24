package services_test

import (
	"net/http"
	"testing"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubUserStoryStatusRepo struct {
	statuses            map[uuid.UUID]map[string]*models.UserStoryStatus
	DisableAutoDefaults bool
}

func (s *stubUserStoryStatusRepo) ensureDefaultStatuses(projectID uuid.UUID) {
	if s.statuses == nil {
		s.statuses = make(map[uuid.UUID]map[string]*models.UserStoryStatus)
	}
	if s.statuses[projectID] == nil {
		s.statuses[projectID] = make(map[string]*models.UserStoryStatus)
	}
	if s.DisableAutoDefaults {
		return
	}
	defaults := []struct {
		name  string
		color string
		order int
	}{
		{"Todo", "#808080", 0},
		{"In Progress", "#1E90FF", 1},
		{"In Review", "#FF8C00", 2},
		{"Testing", "#8A2BE2", 3},
		{"Completed", "#228B22", 4},
		{"Blocked", "#DC143C", 5},
	}
	for _, d := range defaults {
		norm := models.NormalizeTaskStatus(d.name)
		if _, exists := s.statuses[projectID][norm]; !exists {
			id := uuid.Must(uuid.NewV4())
			s.statuses[projectID][norm] = &models.UserStoryStatus{
				ID:           id,
				ProjectID:    projectID,
				Name:         d.name,
				Color:        d.color,
				DisplayOrder: d.order,
				IsDefault:    true,
				IsClosed:     d.name == "Completed",
			}
		}
	}
}

func (s *stubUserStoryStatusRepo) CreateStatus(status *models.UserStoryStatus) *response.Error {
	s.ensureDefaultStatuses(status.ProjectID)
	s.statuses[status.ProjectID][models.NormalizeTaskStatus(status.Name)] = status
	return nil
}

func (s *stubUserStoryStatusRepo) GetStatusByID(id, projectID uuid.UUID) (*models.UserStoryStatus, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		for _, st := range projStatuses {
			if st.ID == id {
				return st, nil
			}
		}
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Status not found"}
}

func (s *stubUserStoryStatusRepo) GetStatusByName(projectID uuid.UUID, name string) (*models.UserStoryStatus, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		if st, ok := projStatuses[models.NormalizeTaskStatus(name)]; ok {
			return st, nil
		}
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Status not found"}
}

func (s *stubUserStoryStatusRepo) UpdateStatus(status *models.UserStoryStatus) *response.Error {
	s.ensureDefaultStatuses(status.ProjectID)
	s.statuses[status.ProjectID][models.NormalizeTaskStatus(status.Name)] = status
	return nil
}

func (s *stubUserStoryStatusRepo) DeleteStatus(id, projectID uuid.UUID) *response.Error {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		for norm, st := range projStatuses {
			if st.ID == id {
				delete(projStatuses, norm)
				return nil
			}
		}
	}
	return &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Status not found"}
}

func (s *stubUserStoryStatusRepo) GetStatusesByProjectID(projectID uuid.UUID) ([]models.UserStoryStatus, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	var list []models.UserStoryStatus
	if projStatuses, ok := s.statuses[projectID]; ok {
		for _, st := range projStatuses {
			list = append(list, *st)
		}
	}
	return list, nil
}

func (s *stubUserStoryStatusRepo) IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		if _, ok := projStatuses[models.NormalizeTaskStatus(name)]; ok {
			return true, nil
		}
	}
	return false, nil
}

type stubUserStoryRepoForStatus struct {
	userstoryrepo.UserStoryRepository
	storyCount int64
}

func (m *stubUserStoryRepoForStatus) CountStoriesByStatusID(projectID, statusID uuid.UUID) (int64, *response.Error) {
	return m.storyCount, nil
}

func TestUserStoryStatusService_CreateStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, Role: "member", IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubUserStoryStatusRepo{}
	storyRepo := &stubUserStoryRepoForStatus{}

	service := services.InitUserStoryStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, storyRepo, zap.NewNop())

	// Test 1: Successful custom status creation
	req := dto.CreateUserStoryStatusRequest{
		Name:         "Ready for Review",
		Color:        "#FFA500",
		DisplayOrder: 2,
		ProjectID:    projectID,
		UserID:       userID,
	}
	res, err := service.CreateStatus(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Name != "Ready for Review" || res.Color != "#FFA500" || res.DisplayOrder != 2 {
		t.Errorf("Unexpected response content: %+v", res)
	}
	if res.IsClosed {
		t.Errorf("Expected is_closed to default to false, got true")
	}

	// Test 1b: Successful custom status creation with IsClosed = true
	isClosedTrue := true
	req1b := req
	req1b.Name = "Done Custom"
	req1b.IsClosed = &isClosedTrue
	res1b, err := service.CreateStatus(req1b)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !res1b.IsClosed {
		t.Errorf("Expected is_closed to be true, got false")
	}

	// Test 2: Invalid color (422)
	req2 := req
	req2.Color = "FFA500" // missing '#'
	_, err = service.CreateStatus(req2)
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for invalid color, got %v", err)
	}

	// Test 3: Name too long (422)
	req3 := req
	req3.Name = "this_is_a_very_long_status_name_that_exceeds_fifty_characters_limit"
	_, err = service.CreateStatus(req3)
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for too long status name, got %v", err)
	}

	// Test 4: Creating a status with default status name (fails as duplicate 409)
	req4 := req
	req4.Name = "todo"
	_, err = service.CreateStatus(req4)
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("Expected 409 for duplicate default status name, got %v", err)
	}

	// Test 5: Duplicate custom status name (409)
	req5 := req
	req5.Name = "ready for review" // case-insensitive check
	_, err = service.CreateStatus(req5)
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("Expected 409 for duplicate custom status name, got %v", err)
	}

	// Test 6: Invalid display order (422)
	req6 := req
	req6.Name = "another_one"
	req6.DisplayOrder = -1
	_, err = service.CreateStatus(req6)
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for negative display order, got %v", err)
	}

	// Test 7: Unauthorized project user (Developer role cannot manage statuses -> 403)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	_, err = service.CreateStatus(req)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for unauthorized project role, got %v", err)
	}
}

func TestUserStoryStatusService_UpdateStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV7())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, Role: "member", IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubUserStoryStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.UserStoryStatus{
			projectID: {
				"ready_for_review": {
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
				},
			},
		},
	}
	storyRepo := &stubUserStoryRepoForStatus{}

	service := services.InitUserStoryStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, storyRepo, zap.NewNop())

	// Test 1: Successful update (name, color, display_order)
	newName := "Awaiting QA"
	newColor := "#00FFFF"
	newOrder := 4
	isClosed := true
	req := dto.UpdateUserStoryStatusRequest{
		Name:         &newName,
		Color:        &newColor,
		DisplayOrder: &newOrder,
		IsClosed:     &isClosed,
		StatusID:     statusID,
		ProjectID:    projectID,
		UserID:       userID,
	}

	res, err := service.UpdateStatus(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Name != newName || res.Color != newColor || res.DisplayOrder != newOrder || !res.IsClosed {
		t.Errorf("Unexpected updated status details: %+v", res)
	}

	// Test 2: Unauthorized user (403)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	_, err = service.UpdateStatus(req)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for unauthorized status manager update, got %v", err)
	}
	projectRepo.projectRole = string(dto.ProjectRoleProjectManager)

	// Test 3: Invalid color (422)
	badColor := "123456"
	req3 := req
	req3.Color = &badColor
	_, err = service.UpdateStatus(req3)
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for invalid color update, got %v", err)
	}
}

func TestUserStoryStatusService_DeleteStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV7())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, Role: "member", IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubUserStoryStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.UserStoryStatus{
			projectID: {
				"ready_for_review": {
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
					IsDefault:    false,
				},
			},
		},
	}
	storyRepo := &stubUserStoryRepoForStatus{}

	service := services.InitUserStoryStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, storyRepo, zap.NewNop())

	// Test 1: Fail deleting when assigned to active stories (Bad Request)
	statusRepo.ensureDefaultStatuses(projectID) // Seed defaults so that "only status" check doesn't trigger first.
	storyRepo.storyCount = 1
	err := service.DeleteStatus(statusID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected bad request when status is assigned to stories, got %v", err)
	}
	storyRepo.storyCount = 0

	// Test 2: Fail deleting when it's the only custom status in the project
	statusRepo.DisableAutoDefaults = true
	// We clear all default statuses so ready_for_review is the only status
	statusRepo.statuses[projectID] = map[string]*models.UserStoryStatus{
		"ready_for_review": {
			ID:           statusID,
			ProjectID:    projectID,
			Name:         "Ready for Review",
			Color:        "#FFA500",
			DisplayOrder: 2,
			IsDefault:    false,
		},
	}
	err = service.DeleteStatus(statusID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected bad request when deleting project's only status, got %v", err)
	}

	// Test 2b: Successful deletion when multiple statuses exist
	statusRepo.DisableAutoDefaults = false
	statusRepo.ensureDefaultStatuses(projectID)
	// Make sure our custom status is also there
	statusRepo.statuses[projectID]["ready_for_review"] = &models.UserStoryStatus{
		ID:           statusID,
		ProjectID:    projectID,
		Name:         "Ready for Review",
		Color:        "#FFA500",
		DisplayOrder: 2,
		IsDefault:    false,
	}
	err = service.DeleteStatus(statusID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("Expected successful deletion, got %v", err)
	}

	// Test 3: Fail deleting default status
	// Get ID of the default 'Todo' status
	statuses, _ := statusRepo.GetStatusesByProjectID(projectID)
	var defaultStatusID uuid.UUID
	for _, st := range statuses {
		if st.IsDefault {
			defaultStatusID = st.ID
			break
		}
	}
	err = service.DeleteStatus(defaultStatusID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected bad request when deleting default status, got %v", err)
	}

	// Test 4: Unauthorized user delete (403)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	err = service.DeleteStatus(statusID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for unauthorized delete status, got %v", err)
	}
}

func TestUserStoryStatusService_GetStatuses(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, Role: "member", IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleDeveloper),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubUserStoryStatusRepo{}
	storyRepo := &stubUserStoryRepoForStatus{}

	service := services.InitUserStoryStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, storyRepo, zap.NewNop())

	// Developer is authorized to view statuses
	res, err := service.GetStatuses(projectID, userID, orgID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 6 default statuses total
	if len(res) != 6 {
		t.Errorf("Expected 6 statuses total, got %d", len(res))
	}

	// Verify sorting order
	for i := 0; i < len(res)-1; i++ {
		if res[i].DisplayOrder > res[i+1].DisplayOrder {
			t.Errorf("Expected statuses to be sorted by DisplayOrder")
		}
	}
}

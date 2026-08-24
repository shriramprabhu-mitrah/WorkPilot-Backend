package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type mockProjectRepoForStatus struct {
	projectrepo.ProjectRepository
	projectRole string
	project     *models.Project
	member      *models.ProjectMember
	err         *response.Error
}

func (m *mockProjectRepoForStatus) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	if m.err != nil {
		return models.Project{}, m.err
	}
	if m.project != nil {
		return *m.project, nil
	}
	return models.Project{ID: id, OrganizationID: uuid.Must(uuid.NewV4())}, nil
}
func (m *mockProjectRepoForStatus) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.member != nil {
		return m.member, nil
	}
	roleID := uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004")
	if m.projectRole == "project_manager" {
		roleID = uuid.FromStringOrNil("00000000-0000-0000-0000-000000000003")
	} else if m.projectRole == "org_admin" {
		roleID = uuid.FromStringOrNil("00000000-0000-0000-0000-000000000002")
	}
	return &models.ProjectMember{
		UserID:    userID,
		ProjectID: projectID,
		RoleID:    roleID,
		Role:      models.Role{Name: m.projectRole},
	}, nil
}
func (m *mockProjectRepoForStatus) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return m.projectRole != "", nil
}

type mockAuthRepoForStatus struct {
	authrepo.AuthRepository
	user *models.User
	err  *response.Error
}

func (m *mockAuthRepoForStatus) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	if m.err != nil {
		return models.User{}, m.err
	}
	if m.user != nil {
		return *m.user, nil
	}
	return models.User{ID: id, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true}, nil
}

func (m *mockAuthRepoForStatus) GetRoleByName(name string) (*models.Role, *response.Error) {
	return &models.Role{ID: uuid.Must(uuid.NewV4()), Name: name}, nil
}

func TestCustomStatusService_CreateCustomStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{}
	taskRepo := &stubTaskRepo{}

	service := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, taskRepo, zap.NewNop())

	// Test 1: Successful custom status creation
	req := dto.CreateCustomStatusRequest{
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
	if res.IsFinal {
		t.Errorf("Expected is_final to default to false, got true")
	}

	// Test 1b: Successful custom status creation with IsFinal = true
	isFinalTrue := true
	req1b := req
	req1b.Name = "Done Custom"
	req1b.IsFinal = &isFinalTrue
	res1b, err := service.CreateStatus(req1b)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !res1b.IsFinal {
		t.Errorf("Expected is_final to be true, got false")
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

func TestCustomStatusService_UpdateCustomStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV7())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"ready_for_review": &models.CustomStatus{
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
				},
			},
		},
	}
	taskRepo := &stubTaskRepo{}

	service := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, taskRepo, zap.NewNop())

	// Test 1: Successful update (name, color, display_order)
	newName := "Awaiting QA"
	newColor := "#00FFFF"
	newOrder := 4
	req := dto.UpdateCustomStatusRequest{
		Name:         &newName,
		Color:        &newColor,
		DisplayOrder: &newOrder,
		StatusID:     statusID,
		ProjectID:    projectID,
		UserID:       userID,
	}

	res, err := service.UpdateStatus(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Name != newName || res.Color != newColor || res.DisplayOrder != newOrder {
		t.Errorf("Expected fields to update, got %+v", res)
	}

	// Test 1b: Successful custom status update (update IsFinal to true)
	isFinalTrue := true
	req1b := dto.UpdateCustomStatusRequest{
		IsFinal:   &isFinalTrue,
		StatusID:  statusID,
		ProjectID: projectID,
		UserID:    userID,
	}
	res1b, err := service.UpdateStatus(req1b)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !res1b.IsFinal {
		t.Errorf("Expected is_final to be updated to true, got false")
	}

	// Test 2: Update with invalid color (422)
	invalidColor := "#GG0000"
	req2 := req
	req2.Color = &invalidColor
	_, err = service.UpdateStatus(req2)
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for invalid color patch, got %v", err)
	}

	// Test 3: Unauthorized user (403)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	_, err = service.UpdateStatus(req)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for unauthorized patch, got %v", err)
	}
}

func TestCustomStatusService_DeleteCustomStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV7())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"ready_for_review": &models.CustomStatus{
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
				},
			},
		},
	}
	taskRepo := &stubTaskRepo{
		tasks: make(map[uuid.UUID]*models.Task),
	}

	service := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, taskRepo, zap.NewNop())

	// Test 1: Cannot delete custom status assigned to active tasks (400)
	taskID := uuid.Must(uuid.NewV4())
	taskRepo.tasks[taskID] = &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Status:    "Ready for Review", // matches custom status
	}

	err := service.DeleteStatus(statusID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for deleting status in-use, got %v", err)
	}

	// Test 2: Deleting is allowed when task is soft-deleted
	taskRepo.tasks[taskID].DeletedAt.Time = time.Now()
	taskRepo.tasks[taskID].DeletedAt.Valid = true

	err = service.DeleteStatus(statusID, projectID, userID, orgID)
	if err != nil {
		t.Errorf("Expected successful delete when task is soft-deleted, got %v", err)
	}

	// Verify status is gone
	_, err = statusRepo.GetStatusByID(statusID, projectID)
	if err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status to be deleted from repo, got %v", err)
	}
}

func TestTaskService_StatusTransitionsAndColors(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleDeveloper), // Developer role
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"ready_for_review": &models.CustomStatus{
					ID:           uuid.Must(uuid.NewV4()),
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
				},
			},
		},
	}
	taskRepo := &stubTaskRepo{
		tasks: make(map[uuid.UUID]*models.Task),
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, statusRepo, zap.NewNop())

	// Test 1: Developer valid transition (default -> default sequential check)
	taskID := uuid.Must(uuid.NewV4())
	taskRepo.tasks[taskID] = &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Status:    "todo",
	}

	statusInProg := "in_progress"
	_, err := service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &statusInProg,
	})
	if err != nil {
		t.Errorf("Expected developer to transition todo -> in_progress, got %v", err)
	}

	// Test 2: Developer invalid sequential transition (in_progress -> completed directly) -> 400
	statusCompleted := "completed"
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &statusCompleted,
	})
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected transition check failure (400), got %v", err)
	}

	// Test 3: Transition involving at least one custom status is allowed for developers (in_progress -> ready_for_review)
	statusCustom := "ready_for_review"
	res, err := service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &statusCustom,
	})
	if err != nil {
		t.Fatalf("Expected transition in_progress -> ready_for_review to succeed, got %v", err)
	}

	// Verify status_color in task details response matches custom status color code
	if res.StatusColor != "#FFA500" {
		t.Errorf("Expected status color to be custom #FFA500, got %s", res.StatusColor)
	}

	// Test 4: Assigning completely invalid/unknown status (422)
	statusInvalid := "not_a_valid_status_value"
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &statusInvalid,
	})
	if err == nil || err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("Expected 422 for invalid task status, got %v", err)
	}
}

func TestCustomStatusService_GetStatuses(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleDeveloper),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"ready_for_review": &models.CustomStatus{
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Ready for Review",
					Color:        "#FFA500",
					DisplayOrder: 2,
				},
			},
		},
	}
	taskRepo := &stubTaskRepo{}

	service := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, taskRepo, zap.NewNop())

	res, err := service.GetStatuses(projectID, userID, orgID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 6 default statuses + 1 custom status = 7 statuses total
	if len(res) != 7 {
		t.Errorf("Expected 7 statuses total, got %d", len(res))
	}

	// Verify default statuses are returned
	foundTodo := false
	foundCustom := false
	for _, s := range res {
		if s.Name == "Todo" {
			foundTodo = true
		}
		if s.Name == "Ready for Review" {
			foundCustom = true
			if s.Color != "#FFA500" {
				t.Errorf("Expected custom status color #FFA500, got %s", s.Color)
			}
		}
	}
	if !foundTodo {
		t.Errorf("Expected system status 'Todo' to be present in result")
	}
	if !foundCustom {
		t.Errorf("Expected custom status 'Ready for Review' to be present in result")
	}

	// Verify sorting order
	for i := 0; i < len(res)-1; i++ {
		if res[i].DisplayOrder > res[i+1].DisplayOrder {
			t.Errorf("Expected statuses to be sorted by DisplayOrder, but at index %d display order is %d and at index %d it is %d",
				i, res[i].DisplayOrder, i+1, res[i+1].DisplayOrder)
		}
	}
}

func TestCustomStatusService_OverrideDefaultStatus(t *testing.T) {
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())
	statusID := uuid.Must(uuid.NewV4())

	authRepo := &mockAuthRepoForStatus{
		user: &models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, IsActive: true, OrganizationID: &orgID},
	}
	projectRepo := &mockProjectRepoForStatus{
		projectRole: string(dto.ProjectRoleProjectManager),
		project:     &models.Project{ID: projectID, OrganizationID: orgID},
	}
	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"todo": &models.CustomStatus{
					ID:           statusID,
					ProjectID:    projectID,
					Name:         "Todo",
					Color:        "#808080",
					DisplayOrder: 0,
					IsDefault:    true,
				},
			},
		},
	}
	taskRepo := &stubTaskRepo{}

	service := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, &stubAuditLogRepo{}, taskRepo, zap.NewNop())

	// Test 1: Can rename default status
	newName := "Todo Revised"
	res1, err := service.UpdateStatus(dto.UpdateCustomStatusRequest{
		Name:      &newName,
		StatusID:  statusID,
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		t.Errorf("Expected renaming default status to succeed, got %v", err)
	}
	if res1.Name != newName {
		t.Errorf("Expected name to be updated to %s, got %s", newName, res1.Name)
	}

	// Test 2: Can update color and order
	newColor := "#112233"
	newOrder := 7
	res2, err := service.UpdateStatus(dto.UpdateCustomStatusRequest{
		Color:        &newColor,
		DisplayOrder: &newOrder,
		StatusID:     statusID,
		ProjectID:    projectID,
		UserID:       userID,
	})
	if err != nil {
		t.Fatalf("Expected update to succeed, got %v", err)
	}
	if res2.Color != newColor || res2.DisplayOrder != newOrder {
		t.Errorf("Expected color and order to be updated, got %+v", res2)
	}

	// Test 3: Can delete default status
	err = service.DeleteStatus(statusID, projectID, userID, orgID)
	if err != nil {
		t.Errorf("Expected deleting default status to succeed, got %v", err)
	}
}

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
	"gorm.io/gorm"
)

type stubTaskRepo struct {
	tasks           map[uuid.UUID]*models.Task
	seqNumber       int
	createErr       *response.Error
	getErr          *response.Error
	updateErr       *response.Error
	deleteErr       *response.Error
	restoreErr      *response.Error
	listErr         *response.Error
	lastCreatedTask *models.Task
}

func (s *stubTaskRepo) CreateTask(task *models.Task) *response.Error {
	s.lastCreatedTask = task
	if s.createErr != nil {
		return s.createErr
	}
	if s.tasks == nil {
		s.tasks = make(map[uuid.UUID]*models.Task)
	}
	s.tasks[task.ID] = task
	return nil
}

func (s *stubTaskRepo) GetTaskByID(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	task, ok := s.tasks[id]
	if !ok || task.DeletedAt.Valid {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
	}
	return task, nil
}

func (s *stubTaskRepo) GetTaskByIDUnscoped(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	task, ok := s.tasks[id]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
	}
	return task, nil
}

func (s *stubTaskRepo) UpdateTask(task *models.Task) *response.Error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.tasks[task.ID] = task
	return nil
}

func (s *stubTaskRepo) DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	task, ok := s.tasks[id]
	if ok {
		task.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	}
	return nil
}

func (s *stubTaskRepo) RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	if s.restoreErr != nil {
		return s.restoreErr
	}
	task, ok := s.tasks[id]
	if ok {
		task.DeletedAt = gorm.DeletedAt{Valid: false}
	}
	return nil
}

func (s *stubTaskRepo) GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error) {
	if s.listErr != nil {
		return nil, response.Pagination{}, s.listErr
	}
	var res []models.Task
	for _, t := range s.tasks {
		if filter.IsDeleted && !t.DeletedAt.Valid {
			continue
		}
		if !filter.IsDeleted && t.DeletedAt.Valid {
			continue
		}
		res = append(res, *t)
	}
	return res, response.Pagination{}, nil
}

func (s *stubTaskRepo) GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error) {
	s.seqNumber++
	return s.seqNumber, nil
}

func TestTaskService_CreateTask_IncrementsKeysAndSetsKeyPrefix(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "WorkPilot Backend"},
		isMember: true,
	}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, zap.NewNop())

	req := dto.CreateTaskRequest{
		Title:       "Setup database connections",
		Type:        string(dto.TaskTypeTask),
		Priority:    string(dto.TaskPriorityHigh),
		StoryPoints: 5,
		ProjectID:   projectID,
		UserID:      userID,
	}

	task1, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task1.Key != "WB-1" {
		t.Fatalf("expected key WB-1, got %s", task1.Key)
	}

	task2, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task2.Key != "WB-2" {
		t.Fatalf("expected key WB-2, got %s", task2.Key)
	}
}

func TestTaskService_UpdateTask_UpdatesFieldsSuccessfully(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Key:         "WP-1",
		Title:       "Old Title",
		Description: "Old Desc",
		Type:        string(dto.TaskTypeBug),
		Priority:    string(dto.TaskPriorityLow),
		Status:      string(dto.TaskStatusTodo),
		StoryPoints: 1,
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, zap.NewNop())

	newTitle := "New Title"
	newPriority := string(dto.TaskPriorityCritical)
	newStatus := string(dto.TaskStatusInProgress)
	newPoints := 8

	req := dto.UpdateTaskRequest{
		TaskID:      taskID,
		ProjectID:   projectID,
		UserID:      userID,
		Title:       &newTitle,
		Priority:    &newPriority,
		Status:      &newStatus,
		StoryPoints: &newPoints,
	}

	updated, err := service.UpdateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Title != "New Title" {
		t.Fatalf("expected title New Title, got %s", updated.Title)
	}
	if updated.Priority != string(dto.TaskPriorityCritical) {
		t.Fatalf("expected priority critical, got %s", updated.Priority)
	}
	if updated.Status != string(dto.TaskStatusInProgress) {
		t.Fatalf("expected status in_progress, got %s", updated.Status)
	}
	if updated.StoryPoints != 8 {
		t.Fatalf("expected story points 8, got %d", updated.StoryPoints)
	}
}

func TestTaskService_DeleteAndRestore_RetentionChecks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Key:       "WP-1",
		Title:     "Deploy backend",
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, zap.NewNop())

	// Delete task
	err := service.DeleteTask(taskID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error during delete, got %v", err)
	}
	if !existingTask.DeletedAt.Valid {
		t.Fatal("expected task to be marked soft deleted")
	}

	// Case 1: Restore within retention (succeeds)
	err = service.RestoreTask(taskID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected restore to succeed within retention, got %v", err)
	}
	if existingTask.DeletedAt.Valid {
		t.Fatal("expected task DeletedAt to be cleared/nil")
	}

	// Case 2: Restore after retention expired (fails)
	existingTask.DeletedAt = gorm.DeletedAt{Time: time.Now().Add(-40 * 24 * time.Hour), Valid: true}
	err = service.RestoreTask(taskID, projectID, userID, orgID)
	if err == nil {
		t.Fatal("expected restore to fail after retention expired, got nil")
	}
	if err.Code != response.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %s", err.Code)
	}
}

func TestTaskService_CloneTask_ResetsStatusAndKey(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	assigneeID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Key:         "WP-1",
		Title:       "Write API Documentation",
		Description: "Explain endpoints in details",
		Status:      string(dto.TaskStatusCompleted),
		AssigneeID:  &assigneeID,
		StoryPoints: 3,
	}
	taskRepo := &stubTaskRepo{
		tasks:     map[uuid.UUID]*models.Task{taskID: existingTask},
		seqNumber: 1,
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, zap.NewNop())

	// Clone task with keep_assignee = false
	cloned, err := service.CloneTask(dto.CloneTaskRequest{
		KeepAssignee:   false,
		TaskID:         taskID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("expected clone to succeed, got %v", err)
	}

	if cloned.Key != "WP-2" {
		t.Fatalf("expected new key WP-2, got %s", cloned.Key)
	}
	if cloned.Status != string(dto.TaskStatusTodo) {
		t.Fatalf("expected status reset to todo, got %s", cloned.Status)
	}
	if cloned.AssigneeID != nil {
		t.Fatalf("expected assignee to be cleared, got %s", cloned.AssigneeID)
	}
	if cloned.Title != "Write API Documentation (Cloned)" {
		t.Fatalf("expected cloned title modifier, got %s", cloned.Title)
	}
}

func TestTaskService_UpdateTask_WorkflowAndPermissions(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	// Define users with global roles
	globalMemberUser := models.User{
		ID:             userID,
		OrganizationID: &orgID,
		Role:           string(dto.RoleMember),
	}

	nonAssigneeMember := models.User{
		ID:             uuid.Must(uuid.NewV4()),
		OrganizationID: &orgID,
		Role:           string(dto.RoleMember),
	}

	authRepo := &sprintAuthRepoStub{user: globalMemberUser}
	projectRepo := &stubProjectRepo{
		project:     models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember:    true,
		projectRole: string(dto.ProjectRoleDeveloper), // Project-level Developer
	}

	existingTask := &models.Task{
		ID:         taskID,
		ProjectID:  projectID,
		Key:        "WP-1",
		Title:      "Test Workflow",
		Type:       string(dto.TaskTypeTask),
		Priority:   string(dto.TaskPriorityMedium),
		Status:     string(dto.TaskStatusTodo),
		AssigneeID: &userID,
	}

	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, zap.NewNop())

	// Test 1: Developer valid sequential transition (todo -> in_progress)
	inProgressStatus := string(dto.TaskStatusInProgress)
	_, err := service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &inProgressStatus,
	})
	if err != nil {
		t.Fatalf("expected developer to transition to in_progress successfully, got %v", err)
	}
	if existingTask.Status != string(dto.TaskStatusInProgress) {
		t.Fatalf("expected task status to be in_progress, got %s", existingTask.Status)
	}

	// Test 2: Developer invalid transition (in_progress -> completed directly)
	completedStatus := string(dto.TaskStatusCompleted)
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &completedStatus,
	})
	if err == nil {
		t.Fatal("expected developer transition from in_progress to completed directly to fail")
	}
	if err.Code != response.ErrInvalidStatusTransition {
		t.Fatalf("expected ErrInvalidStatusTransition, got %v", err.Code)
	}

	// Test 3: Project Manager (ProjectRole) can override transition rules (in_progress -> completed directly)
	pmUser := models.User{
		ID:             uuid.Must(uuid.NewV4()),
		OrganizationID: &orgID,
		Role:           string(dto.RoleMember), // Global RoleMember, but PM in project
	}
	authRepo.user = pmUser
	projectRepo.projectRole = string(dto.ProjectRoleProjectManager)
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    pmUser.ID,
		Status:    &completedStatus,
	})
	if err != nil {
		t.Fatalf("expected project manager to successfully transition from in_progress to completed, got %v", err)
	}
	if existingTask.Status != string(dto.TaskStatusCompleted) {
		t.Fatalf("expected task status to be completed, got %s", existingTask.Status)
	}

	// Reset task status to todo and set role back to developer for next tests
	existingTask.Status = string(dto.TaskStatusTodo)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)

	// Test 4: Transition to blocked requires a reason
	blockedStatus := string(dto.TaskStatusBlocked)
	authRepo.user = globalMemberUser
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &blockedStatus,
	})
	if err == nil {
		t.Fatal("expected transition to blocked without reason to fail")
	}

	blockedReason := "API dependency not ready"
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:        taskID,
		ProjectID:     projectID,
		UserID:        userID,
		Status:        &blockedStatus,
		BlockedReason: &blockedReason,
	})
	if err != nil {
		t.Fatalf("expected transition to blocked with reason to succeed, got %v", err)
	}
	if existingTask.Status != string(dto.TaskStatusBlocked) {
		t.Fatalf("expected status to be blocked, got %s", existingTask.Status)
	}
	if existingTask.BlockedReason != blockedReason {
		t.Fatalf("expected blocked reason to be set, got %s", existingTask.BlockedReason)
	}

	// Test 5: Transition out of blocked clears blocked reason
	todoStatus := string(dto.TaskStatusTodo)
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		Status:    &todoStatus,
	})
	if err != nil {
		t.Fatalf("expected transition out of blocked to succeed, got %v", err)
	}
	if existingTask.BlockedReason != "" {
		t.Fatalf("expected blocked reason to be cleared, got %s", existingTask.BlockedReason)
	}

	// Test 6: Non-assignee Developer cannot increment actual hours
	authRepo.user = nonAssigneeMember
	actualHrs := 5.0
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:      taskID,
		ProjectID:   projectID,
		UserID:      nonAssigneeMember.ID,
		ActualHours: &actualHrs,
	})
	if err == nil {
		t.Fatal("expected non-assignee actual hours update to fail")
	}

	// Test 7: Assignee Developer can increment actual hours
	authRepo.user = globalMemberUser
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:      taskID,
		ProjectID:   projectID,
		UserID:      userID,
		ActualHours: &actualHrs,
	})
	if err != nil {
		t.Fatalf("expected assignee actual hours increment to succeed, got %v", err)
	}
	if existingTask.ActualHours == nil || *existingTask.ActualHours != 5.0 {
		t.Fatalf("expected actual hours to be updated to 5.0")
	}

	// Test 8: Assignee Developer cannot decrement actual hours
	lowerHrs := 4.0
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:      taskID,
		ProjectID:   projectID,
		UserID:      userID,
		ActualHours: &lowerHrs,
	})
	if err == nil {
		t.Fatal("expected assignee actual hours decrement to fail")
	}

	// Test 9: PM can update/decrement actual hours
	authRepo.user = pmUser
	projectRepo.projectRole = string(dto.ProjectRoleProjectManager)
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:      taskID,
		ProjectID:   projectID,
		UserID:      pmUser.ID,
		ActualHours: &lowerHrs,
	})
	if err != nil {
		t.Fatalf("expected Project Manager actual hours decrement to succeed, got %v", err)
	}
	if existingTask.ActualHours == nil || *existingTask.ActualHours != 4.0 {
		t.Fatalf("expected actual hours to be decremented to 4.0")
	}

	// Test 10: Assignee must be a member of the project
	projectRepo.isMember = false // mock assignee is not project member
	nonMemberUUID := uuid.Must(uuid.NewV4())
	authRepo.user = pmUser // update by PM
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:     taskID,
		ProjectID:  projectID,
		UserID:     pmUser.ID,
		AssigneeID: &nonMemberUUID,
	})
	if err == nil {
		t.Fatal("expected assignment to non-member to fail")
	}

	// Test 11: Viewers cannot update tasks
	viewerUser := models.User{
		ID:             uuid.Must(uuid.NewV4()),
		OrganizationID: &orgID,
		Role:           string(dto.RoleMember),
	}
	authRepo.user = viewerUser
	projectRepo.projectRole = string(dto.ProjectRoleViewer)
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    viewerUser.ID,
		Title:     &blockedReason,
	})
	if err == nil {
		t.Fatal("expected viewer task update to be rejected")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", err.StatusCode)
	}

	// Test 12: Super Admins are not allowed to update tasks
	superAdminUser := models.User{
		ID:             uuid.Must(uuid.NewV4()),
		OrganizationID: &orgID,
		Role:           string(dto.RoleSuperAdmin),
	}
	authRepo.user = superAdminUser
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    superAdminUser.ID,
		Title:     &blockedReason,
	})
	if err == nil {
		t.Fatal("expected super_admin task update to be rejected")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for super_admin, got %d", err.StatusCode)
	}
}

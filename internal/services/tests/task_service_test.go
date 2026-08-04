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

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(models.RoleDeveloper)}}
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

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(models.RoleDeveloper)}}
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

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(models.RoleDeveloper)}}
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

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(models.RoleDeveloper)}}
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

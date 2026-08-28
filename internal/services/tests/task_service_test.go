package services_test

import (
	"encoding/json"
	"net/http"
	"strings"
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
	tasks            map[uuid.UUID]*models.Task
	seqNumber        int
	serialNumber     int
	createErr        *response.Error
	getErr           *response.Error
	updateErr        *response.Error
	deleteErr        *response.Error
	restoreErr       *response.Error
	listErr          *response.Error
	lastCreatedTask  *models.Task
	validSprints     map[uuid.UUID]bool
	sprintStatuses   map[uuid.UUID]string
	validUserStories map[uuid.UUID]bool
}

func (s *stubTaskRepo) CreateTask(task *models.Task) *response.Error {
	if task.ID == uuid.Nil {
		task.ID = uuid.Must(uuid.NewV4())
	}
	if task.SerialNumber == 0 {
		s.serialNumber++
		task.SerialNumber = int64(s.serialNumber)
	}
	taskCopy := *task
	s.lastCreatedTask = &taskCopy
	if s.createErr != nil {
		return s.createErr
	}
	if s.tasks == nil {
		s.tasks = make(map[uuid.UUID]*models.Task)
	}
	s.tasks[task.ID] = &taskCopy
	return nil
}

func (s *stubTaskRepo) GetTasksByUserStoryID(userStoryID uuid.UUID) ([]models.Task, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	var tasks []models.Task
	for _, t := range s.tasks {
		if t.UserStoryID != nil && *t.UserStoryID == userStoryID {
			tasks = append(tasks, *t)
		}
	}
	if tasks == nil {
		return []models.Task{}, nil
	}
	return tasks, nil
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
func (s *stubTaskRepo) GetTaskByKey(key string, projectID uuid.UUID) (*models.Task, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, task := range s.tasks {
		if task != nil && strings.EqualFold(task.Key, key) && task.ProjectID == projectID && !task.DeletedAt.Valid {
			return task, nil
		}
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
}
func (s *stubTaskRepo) GetTaskDetailsByID(id uuid.UUID) (*models.Task, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	task, ok := s.tasks[id]
	if !ok || task.DeletedAt.Valid {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
	}
	return task, nil
}
func (s *stubTaskRepo) GetTaskAccessContext(id uuid.UUID) (*models.TaskAccessContext, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	task, ok := s.tasks[id]
	if !ok || task.DeletedAt.Valid {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
	}
	return &models.TaskAccessContext{
		TaskID:         task.ID,
		ProjectID:      task.ProjectID,
		OrganizationID: task.Project.OrganizationID,
		TaskKey:        task.Key,
	}, nil
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

func (s *stubTaskRepo) CountTasksByStatus(projectID uuid.UUID, status string) (int64, *response.Error) {
	var count int64
	for _, t := range s.tasks {
		if t.ProjectID == projectID && models.NormalizeTaskStatus(t.Status) == models.NormalizeTaskStatus(status) && !t.DeletedAt.Valid {
			count++
		}
	}
	return count, nil
}

func (s *stubTaskRepo) UpdateTaskStatusName(projectID uuid.UUID, oldStatus, newStatus string) *response.Error {
	for _, t := range s.tasks {
		if t.ProjectID == projectID && models.NormalizeTaskStatus(t.Status) == models.NormalizeTaskStatus(oldStatus) {
			t.Status = newStatus
		}
	}
	return nil
}

func (s *stubTaskRepo) GetTaskCountsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error) {
	counts := make(map[uuid.UUID]int64)
	for _, id := range projectIDs {
		var count int64
		for _, t := range s.tasks {
			if t.ProjectID == id && !t.DeletedAt.Valid {
				count++
			}
		}
		counts[id] = count
	}
	return counts, nil
}

func (s *stubTaskRepo) UpdateAttachmentsTaskID(attachmentIDs []uuid.UUID, taskID uuid.UUID) *response.Error {
	return nil
}

type stubCustomStatusRepo struct {
	statuses map[uuid.UUID]map[string]*models.CustomStatus
}

func (s *stubCustomStatusRepo) ensureDefaultStatuses(projectID uuid.UUID) {
	if s.statuses == nil {
		s.statuses = make(map[uuid.UUID]map[string]*models.CustomStatus)
	}
	if s.statuses[projectID] == nil {
		s.statuses[projectID] = make(map[string]*models.CustomStatus)
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
			s.statuses[projectID][norm] = &models.CustomStatus{
				ID:           id,
				ProjectID:    projectID,
				Name:         d.name,
				Color:        d.color,
				DisplayOrder: d.order,
				IsDefault:    true,
				IsFinal:      d.name == "Completed",
			}
		}
	}
}

func (s *stubCustomStatusRepo) CreateStatus(status *models.CustomStatus) *response.Error {
	s.ensureDefaultStatuses(status.ProjectID)
	s.statuses[status.ProjectID][models.NormalizeTaskStatus(status.Name)] = status
	return nil
}

func (s *stubCustomStatusRepo) GetStatusByID(id, projectID uuid.UUID) (*models.CustomStatus, *response.Error) {
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

func (s *stubCustomStatusRepo) GetStatusByName(projectID uuid.UUID, name string) (*models.CustomStatus, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		if st, ok := projStatuses[models.NormalizeTaskStatus(name)]; ok {
			return st, nil
		}
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Status not found"}
}

func (s *stubCustomStatusRepo) UpdateStatus(status *models.CustomStatus) *response.Error {
	s.ensureDefaultStatuses(status.ProjectID)
	if projStatuses, ok := s.statuses[status.ProjectID]; ok {
		for key, st := range projStatuses {
			if st.ID == status.ID {
				delete(projStatuses, key)
			}
		}
	}
	s.statuses[status.ProjectID][models.NormalizeTaskStatus(status.Name)] = status
	return nil
}

func (s *stubCustomStatusRepo) DeleteStatus(id, projectID uuid.UUID) *response.Error {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		for name, st := range projStatuses {
			if st.ID == id {
				delete(projStatuses, name)
				return nil
			}
		}
	}
	return &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Status not found"}
}

func (s *stubCustomStatusRepo) GetStatusesByProjectID(projectID uuid.UUID) ([]models.CustomStatus, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	var result []models.CustomStatus
	if projStatuses, ok := s.statuses[projectID]; ok {
		for _, st := range projStatuses {
			result = append(result, *st)
		}
	}
	return result, nil
}

func (s *stubCustomStatusRepo) IsStatusNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	s.ensureDefaultStatuses(projectID)
	if projStatuses, ok := s.statuses[projectID]; ok {
		_, exists := projStatuses[models.NormalizeTaskStatus(name)]
		return exists, nil
	}
	return false, nil
}

func applyUpdatesToTask(task *models.Task, updates map[string]interface{}) {
	if val, ok := updates["title"].(string); ok {
		task.Title = val
	}
	if val, ok := updates["description"].(string); ok {
		task.Description = val
	}
	if val, ok := updates["type"].(string); ok {
		task.Type = val
	}
	if val, ok := updates["priority"].(string); ok {
		task.Priority = val
	}
	if val, ok := updates["status"].(string); ok {
		task.Status = val
	}
	if val, ok := updates["status_id"].(uuid.UUID); ok {
		task.StatusID = val
	}
	if val, ok := updates["blocked_reason"].(string); ok {
		task.BlockedReason = val
	}
	if val, ok := updates["assignee_id"]; ok {
		if val == nil {
			task.AssigneeID = nil
		} else if uid, ok := val.(uuid.UUID); ok {
			task.AssigneeID = &uid
		}
	}
	if val, ok := updates["sprint_id"]; ok {
		if val == nil {
			task.SprintID = nil
		} else if uid, ok := val.(uuid.UUID); ok {
			task.SprintID = &uid
		}
	}
	if val, ok := updates["user_story_id"]; ok {
		if val == nil {
			task.UserStoryID = nil
		} else if uid, ok := val.(uuid.UUID); ok {
			task.UserStoryID = &uid
		}
	}
	if val, ok := updates["story_points"].(int); ok {
		task.StoryPoints = val
	}
	if val, ok := updates["due_date"]; ok {
		if val == nil {
			task.DueDate = nil
		} else if t, ok := val.(time.Time); ok {
			task.DueDate = &t
		}
	}
	if val, ok := updates["estimated_hours"]; ok {
		if val == nil {
			task.EstimatedHours = nil
		} else if v, ok := val.(float64); ok {
			task.EstimatedHours = &v
		}
	}
	if val, ok := updates["actual_hours"]; ok {
		if val == nil {
			task.ActualHours = nil
		} else if v, ok := val.(float64); ok {
			task.ActualHours = &v
		}
	}
}

func (s *stubTaskRepo) UpdateTask(taskID uuid.UUID, updates map[string]interface{}) *response.Error {
	if s.updateErr != nil {
		return s.updateErr
	}
	task, ok := s.tasks[taskID]
	if ok {
		applyUpdatesToTask(task, updates)
	}
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
		if t.ProjectID != projectID {
			continue
		}
		if filter.IsDeleted && !t.DeletedAt.Valid {
			continue
		}
		if !filter.IsDeleted && t.DeletedAt.Valid {
			continue
		}

		// Status filter (uses resolved StatusIDs)
		if len(filter.StatusIDs) > 0 {
			matched := false
			for _, id := range filter.StatusIDs {
				if t.StatusID == id {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Assignee filter
		if len(filter.Assignee) > 0 {
			matched := false
			for _, a := range filter.Assignee {
				if a == "none" || a == "null" {
					if t.AssigneeID == nil || *t.AssigneeID == uuid.Nil {
						matched = true
						break
					}
				} else if uid, err := uuid.FromString(a); err == nil {
					if t.AssigneeID != nil && *t.AssigneeID == uid {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Reporter filter
		if len(filter.Reporter) > 0 {
			matched := false
			for _, r := range filter.Reporter {
				if r == "none" || r == "null" {
					if t.ReporterID == nil || *t.ReporterID == uuid.Nil {
						matched = true
						break
					}
				} else if uid, err := uuid.FromString(r); err == nil {
					if t.ReporterID != nil && *t.ReporterID == uid {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Sprint filter
		if len(filter.Sprint) > 0 {
			matched := false
			for _, sp := range filter.Sprint {
				if sp == "none" || sp == "null" {
					if t.SprintID == nil || *t.SprintID == uuid.Nil {
						matched = true
						break
					}
				} else if uid, err := uuid.FromString(sp); err == nil {
					if t.SprintID != nil && *t.SprintID == uid {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// UserStory filter
		if len(filter.UserStory) > 0 {
			matched := false
			for _, us := range filter.UserStory {
				if us == "none" || us == "null" {
					if t.UserStoryID == nil || *t.UserStoryID == uuid.Nil {
						matched = true
						break
					}
				} else if uid, err := uuid.FromString(us); err == nil {
					if t.UserStoryID != nil && *t.UserStoryID == uid {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Type filter
		if len(filter.Type) > 0 {
			matched := false
			for _, ty := range filter.Type {
				if strings.EqualFold(t.Type, ty) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Priority filter
		if len(filter.Priority) > 0 {
			matched := false
			for _, p := range filter.Priority {
				if strings.EqualFold(t.Priority, p) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		if len(filter.Labels) > 0 {
			isMatchAll := (strings.ToLower(filter.Match) == "all")
			if isMatchAll {
				matchedCount := 0
				for _, fl := range filter.Labels {
					found := false
					for _, tl := range t.Labels {
						if tl.ID.String() == fl || strings.EqualFold(tl.Name, fl) {
							found = true
							break
						}
					}
					if found {
						matchedCount++
					}
				}
				if matchedCount < len(filter.Labels) {
					continue
				}
			} else {
				matched := false
				for _, fl := range filter.Labels {
					for _, tl := range t.Labels {
						if tl.ID.String() == fl || strings.EqualFold(tl.Name, fl) {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		res = append(res, *t)
	}
	return res, response.Pagination{}, nil
}

func (s *stubTaskRepo) GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error) {
	s.seqNumber++
	return s.seqNumber, nil
}

func (s *stubTaskRepo) IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error) {
	if s.validSprints == nil {
		return true, nil
	}
	return s.validSprints[sprintID], nil
}

func (s *stubTaskRepo) IsUserStoryInProject(userStoryID, projectID uuid.UUID) (bool, *response.Error) {
	if s.validUserStories == nil {
		return true, nil
	}
	return s.validUserStories[userStoryID], nil
}

func (s *stubTaskRepo) VerifyLabelIDs(projectID uuid.UUID, labelIDs []uuid.UUID) ([]models.Label, *response.Error) {
	uniqueIDsMap := make(map[uuid.UUID]bool)
	var deduplicatedIDs []uuid.UUID
	for _, id := range labelIDs {
		if !uniqueIDsMap[id] {
			uniqueIDsMap[id] = true
			deduplicatedIDs = append(deduplicatedIDs, id)
		}
	}

	var labels []models.Label
	for _, id := range deduplicatedIDs {
		labels = append(labels, models.Label{
			ID:        id,
			ProjectID: projectID,
			Name:      "Mock Label",
			Color:     "#FF0000",
		})
	}
	return labels, nil
}

func (s *stubTaskRepo) UpdateTaskLabels(taskID uuid.UUID, labels []models.Label) *response.Error {
	if task, ok := s.tasks[taskID]; ok {
		task.Labels = labels
	}
	return nil
}

func (s *stubTaskRepo) UpdateTaskWithLabels(taskID uuid.UUID, updates map[string]interface{}, labels []models.Label) *response.Error {
	task, ok := s.tasks[taskID]
	if ok {
		applyUpdatesToTask(task, updates)
		task.Labels = labels
	}
	return nil
}

func (s *stubTaskRepo) AttachLabel(taskID uuid.UUID, label *models.Label) *response.Error {
	if task, ok := s.tasks[taskID]; ok {
		for _, l := range task.Labels {
			if l.ID == label.ID {
				return nil
			}
		}
		task.Labels = append(task.Labels, *label)
	}
	return nil
}

func (s *stubTaskRepo) RemoveLabel(taskID uuid.UUID, label *models.Label) *response.Error {
	if task, ok := s.tasks[taskID]; ok {
		var newLabels []models.Label
		for _, l := range task.Labels {
			if l.ID != label.ID {
				newLabels = append(newLabels, l)
			}
		}
		task.Labels = newLabels
	}
	return nil
}

func (s *stubTaskRepo) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error {
	for _, t := range s.tasks {
		if t.SprintID != nil && *t.SprintID == sprintID && t.Status != "completed" {
			t.SprintID = nil
		}
	}
	return nil
}

func (s *stubTaskRepo) GetSprintStatus(sprintID uuid.UUID) (string, *response.Error) {
	if s.sprintStatuses == nil {
		return "planning", nil
	}
	status, ok := s.sprintStatuses[sprintID]
	if !ok {
		return "planning", nil
	}
	return status, nil
}

func TestTaskService_CreateTask_IncrementsKeysAndSetsKeyPrefix(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "WorkPilot Backend"},
		isMember: true,
	}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	req := dto.CreateTaskRequest{
		Title:       "Setup database connections",
		Type:        string(dto.TaskTypeTask),
		Priority:    string(dto.TaskPriorityHigh),
		StoryPoints: 5,
		ProjectID:   projectID,
		UserID:      userID,
	}

	_, task1, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task1.Key != "WB-1" {
		t.Fatalf("expected key WB-1, got %s", task1.Key)
	}

	_, task2, err := service.CreateTask(req)
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

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
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

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

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
	if updated.Status != "In Progress" {
		t.Fatalf("expected status in_progress, got %s", updated.Status)
	}
	if updated.StoryPoints != 8 {
		t.Fatalf("expected story points 8, got %d", updated.StoryPoints)
	}
}

func TestTaskService_UpdateTask_NullFields(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	assigneeID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	dueTime := time.Now().Add(24 * time.Hour)
	estHours := 8.5
	actHours := 4.0

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Key:            "WP-1",
		Title:          "Title",
		AssigneeID:     &assigneeID,
		SprintID:       &sprintID,
		UserStoryID:    &storyID,
		DueDate:        &dueTime,
		EstimatedHours: &estHours,
		ActualHours:    &actHours,
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Explicitly nullify all fields using IsNullFields map
	req := dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    userID,
		IsNullFields: map[string]bool{
			"assignee_id":     true,
			"sprint_id":       true,
			"user_story_id":   true,
			"due_date":        true,
			"estimated_hours": true,
			"actual_hours":    true,
		},
	}

	updated, err := service.UpdateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if updated.AssigneeID != nil {
		t.Errorf("expected AssigneeID to be nil")
	}
	if updated.SprintID != nil {
		t.Errorf("expected SprintID to be nil")
	}
	if updated.UserStoryID != nil {
		t.Errorf("expected UserStoryID to be nil")
	}
	if updated.DueDate != nil {
		t.Errorf("expected DueDate to be nil")
	}
	if updated.EstimatedHours != nil {
		t.Errorf("expected EstimatedHours to be nil")
	}
	if updated.ActualHours != nil {
		t.Errorf("expected ActualHours to be nil")
	}
}

func TestUpdateTaskRequest_UnmarshalJSON(t *testing.T) {
	// Case 1: Fields omitted
	jsonDataOmitted := []byte(`{"title": "New Title"}`)
	var reqOmitted dto.UpdateTaskRequest
	if err := json.Unmarshal(jsonDataOmitted, &reqOmitted); err != nil {
		t.Fatalf("unmarshal omitted failed: %v", err)
	}
	if reqOmitted.SprintID != nil {
		t.Errorf("expected SprintID to be nil when omitted")
	}
	if reqOmitted.IsSprintIDNull() {
		t.Errorf("expected IsSprintIDNull to be false when omitted")
	}

	// Case 2: Fields explicitly null
	jsonDataNull := []byte(`{
		"title": "New Title",
		"sprint_id": null,
		"assignee_id": null,
		"user_story_id": null,
		"due_date": null,
		"estimated_hours": null,
		"actual_hours": null
	}`)
	var reqNull dto.UpdateTaskRequest
	if err := json.Unmarshal(jsonDataNull, &reqNull); err != nil {
		t.Fatalf("unmarshal null failed: %v", err)
	}
	if !reqNull.IsSprintIDNull() || !reqNull.IsAssigneeIDNull() || !reqNull.IsUserStoryIDNull() || !reqNull.IsDueDateNull() || !reqNull.IsEstimatedHoursNull() || !reqNull.IsActualHoursNull() {
		t.Errorf("expected all IsNull checks to return true")
	}

	// Case 3: Valid values provided
	sprintUUID := uuid.Must(uuid.NewV4())
	jsonDataValid := []byte(`{"title": "New Title", "sprint_id": "` + sprintUUID.String() + `"}`)
	var reqValid dto.UpdateTaskRequest
	if err := json.Unmarshal(jsonDataValid, &reqValid); err != nil {
		t.Fatalf("unmarshal valid failed: %v", err)
	}
	if reqValid.SprintID == nil || *reqValid.SprintID != sprintUUID {
		t.Errorf("expected SprintID to match %v, got %v", sprintUUID, reqValid.SprintID)
	}
	if reqValid.IsSprintIDNull() {
		t.Errorf("expected IsSprintIDNull to be false when valid")
	}
}

func TestTaskService_DeleteAndRestore_RetentionChecks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
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

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Delete task
	_, err := service.BulkDeleteTasks(dto.BulkDeleteTasksRequest{
		TaskIDs:        []uuid.UUID{taskID},
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
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
	if err.Code != response.ErrTaskPermanentlyDeleted {
		t.Fatalf("expected TASK_PERMANENTLY_DELETED, got %s", err.Code)
	}
	if err.StatusCode != 410 {
		t.Fatalf("expected status code 410, got %d", err.StatusCode)
	}
}

func TestTaskService_CloneTask_ResetsStatusAndKey(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	assigneeID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
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

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

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
	if models.NormalizeTaskStatus(cloned.Status) != string(dto.TaskStatusTodo) {
		t.Fatalf("expected status reset to todo, got %s", cloned.Status)
	}
	if cloned.StatusID == uuid.Nil {
		t.Fatalf("expected status ID to be resolved, got Nil")
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
		RoleID:         uuid.Must(uuid.NewV7()),
		Role:           models.Role{Name: string(dto.RoleMember)},
	}

	nonAssigneeMember := models.User{
		ID:             uuid.Must(uuid.NewV4()),
		OrganizationID: &orgID,
		RoleID:         uuid.Must(uuid.NewV7()),
		Role:           models.Role{Name: string(dto.RoleMember)},
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

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

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
	if existingTask.Status != "In Progress" {
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
		RoleID:         uuid.Must(uuid.NewV7()),
		Role:           models.Role{Name: string(dto.RoleMember)}, // Global RoleMember, but PM in project
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
	if existingTask.Status != "Completed" {
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
	if existingTask.Status != "Blocked" {
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
		RoleID:         uuid.Must(uuid.NewV7()),
		Role:           models.Role{Name: string(dto.RoleMember)},
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
		RoleID:         uuid.Must(uuid.NewV7()),
		Role:           models.Role{Name: string(dto.RoleSuperAdmin)},
	}
	authRepo.user = superAdminUser
	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:    taskID,
		ProjectID: projectID,
		UserID:    superAdminUser.ID,
	})
	if err == nil {
		t.Fatal("expected super_admin task update to be rejected")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for super_admin, got %d", err.StatusCode)
	}
}

func TestTaskService_BulkUpdateTasks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{
		user: models.User{
			ID:             userID,
			OrganizationID: &orgID,
			RoleID:         uuid.Must(uuid.NewV7()),
			Role:           models.Role{Name: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:     models.Project{ID: projectID, OrganizationID: orgID, Name: "WorkPilot Backend"},
		isMember:    true,
		projectRole: string(dto.ProjectRoleProjectManager),
	}

	taskID1 := uuid.Must(uuid.NewV4())
	taskID2 := uuid.Must(uuid.NewV4())
	taskID3 := uuid.Must(uuid.NewV4())

	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{
			taskID1: {
				ID:        taskID1,
				ProjectID: projectID,
				Key:       "WB-1",
				Status:    "todo",
			},
			taskID2: {
				ID:        taskID2,
				ProjectID: projectID,
				Key:       "WB-2",
				Status:    "in_progress",
			},
		},
		validSprints: make(map[uuid.UUID]bool),
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Test 1: Non-PM/Admin user (Developer) gets 403 Forbidden
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	authRepo.user.Role.Name = string(dto.RoleMember)
	req := dto.BulkUpdateTasksRequest{
		Tasks: []dto.BulkUpdateTaskItem{
			{
				TaskID: taskID1,
				Status: pointer("completed"),
			},
		},
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}
	_, err := service.BulkUpdateTasks(req)
	if err == nil {
		t.Fatal("expected non-PM/Admin user to be rejected")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", err.StatusCode)
	}

	// Reset role to Project Manager
	projectRepo.projectRole = string(dto.ProjectRoleProjectManager)

	// Test 2: Bulk Update with mixed valid and invalid tasks
	sprintID := uuid.Must(uuid.NewV4())
	taskRepo.validSprints[sprintID] = true

	invalidSprintID := uuid.Must(uuid.NewV4())

	req = dto.BulkUpdateTasksRequest{
		Tasks: []dto.BulkUpdateTaskItem{
			// 1. Valid update
			{
				TaskID:   taskID1,
				Status:   pointer("completed"),
				SprintID: &sprintID,
			},
			// 2. Task not found
			{
				TaskID: taskID3,
				Status: pointer("completed"),
			},
			// 3. Sprint not in project
			{
				TaskID:   taskID2,
				SprintID: &invalidSprintID,
			},
			// 4. Blocked status without reason
			{
				TaskID: taskID2,
				Status: pointer("blocked"),
			},
		},
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}

	res, err := service.BulkUpdateTasks(req)
	if err != nil {
		t.Fatalf("expected no general error, got %v", err)
	}

	if res.UpdatedCount != 1 {
		t.Fatalf("expected 1 task to be updated, got %d", res.UpdatedCount)
	}

	if len(res.FailedTaskIDs) != 3 {
		t.Fatalf("expected 3 task failures, got %d", len(res.FailedTaskIDs))
	}

	// Verify failure reasons
	reasonNotFound, exists := res.FailureReasons[taskID3.String()]
	if !exists || reasonNotFound != "Task not found" {
		t.Fatalf("expected 'Task not found' for task3, got '%s'", reasonNotFound)
	}

	reasonBlocked, exists := res.FailureReasons[taskID2.String()]
	if !exists {
		t.Fatalf("expected failure reason for task2")
	}
	if reasonBlocked != "Sprint must belong to the project" && reasonBlocked != "Moving to Blocked requires a blocked reason" {
		t.Fatalf("expected failure reason related to sprint/blocked, got '%s'", reasonBlocked)
	}

	// Verify task1 actually updated
	updatedTask1 := taskRepo.tasks[taskID1]
	if updatedTask1.Status != "Completed" {
		t.Fatalf("expected task1 status to be Completed, got %s", updatedTask1.Status)
	}
	if updatedTask1.SprintID == nil || *updatedTask1.SprintID != sprintID {
		t.Fatalf("expected task1 sprint ID to be updated")
	}
}

func pointer[T any](v T) *T {
	return &v
}

func TestTaskService_CreateAndUpdateTask_WithLabels(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember: true,
	}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Create Task with Labels
	labelID := uuid.Must(uuid.NewV4())
	createReq := dto.CreateTaskRequest{
		Title:          "Task with label",
		Type:           "task",
		Priority:       "medium",
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		LabelIDs:       []uuid.UUID{labelID},
	}

	_, res, err := service.CreateTask(createReq)
	if err != nil {
		t.Fatalf("expected create task with labels to succeed, got: %v", err)
	}

	if len(res.Labels) != 1 || res.Labels[0].ID != labelID {
		t.Errorf("expected task to have 1 label with ID %s, got: %+v", labelID, res.Labels)
	}

	// Update Task with Labels
	newLabelID := uuid.Must(uuid.NewV4())
	updateLabelIDs := []uuid.UUID{newLabelID}
	updateReq := dto.UpdateTaskRequest{
		TaskID:         res.ID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		LabelIDs:       &updateLabelIDs,
	}

	resUpdate, err := service.UpdateTask(updateReq)
	if err != nil {
		t.Fatalf("expected update task labels to succeed, got: %v", err)
	}

	if len(resUpdate.Labels) != 1 || resUpdate.Labels[0].ID != newLabelID {
		t.Errorf("expected updated task to have 1 label with ID %s, got: %+v", newLabelID, resUpdate.Labels)
	}
}

func TestTaskService_GetTasks_LabelFiltering(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember: true,
	}

	label1 := models.Label{ID: uuid.Must(uuid.NewV4()), Name: "Frontend", Color: "#0000FF"}
	label2 := models.Label{ID: uuid.Must(uuid.NewV4()), Name: "Bug", Color: "#FF0000"}

	task1 := models.Task{
		ID:        uuid.Must(uuid.NewV4()),
		Title:     "Task 1",
		ProjectID: projectID,
		Labels:    []models.Label{label1},
	}
	task2 := models.Task{
		ID:        uuid.Must(uuid.NewV4()),
		Title:     "Task 2",
		ProjectID: projectID,
		Labels:    []models.Label{label2},
	}
	task3 := models.Task{
		ID:        uuid.Must(uuid.NewV4()),
		Title:     "Task 3",
		ProjectID: projectID,
		Labels:    []models.Label{label1, label2},
	}

	taskRepo := &stubTaskRepo{tasks: map[uuid.UUID]*models.Task{
		task1.ID: &task1,
		task2.ID: &task2,
		task3.ID: &task3,
	}}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// 1. Filter by label1 ID
	res, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
		Labels: []string{label1.ID.String()},
	})
	if err != nil {
		t.Fatalf("expected GetTasks to succeed, got: %v", err)
	}
	if len(res) != 2 {
		t.Errorf("expected 2 tasks for label1, got: %d", len(res))
	}

	// 2. Filter by label2 Name
	res, _, err = service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
		Labels: []string{"Bug"},
	})
	if err != nil {
		t.Fatalf("expected GetTasks to succeed, got: %v", err)
	}
	if len(res) != 2 {
		t.Errorf("expected 2 tasks for label 'Bug', got: %d", len(res))
	}

	// 3. Filter by both label1 and label2
	res, _, err = service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
		Labels: []string{"Frontend", "Bug"},
	})
	if err != nil {
		t.Fatalf("expected GetTasks to succeed, got: %v", err)
	}
	if len(res) != 3 {
		t.Errorf("expected 3 tasks for 'Frontend' or 'Bug', got: %d", len(res))
	}

	// 4. AND filter: Filter by both label1 and label2 with match=all
	res, _, err = service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
		Labels: []string{"Frontend", "Bug"},
		Match:  "all",
	})
	if err != nil {
		t.Fatalf("expected GetTasks to succeed, got: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 task matching both 'Frontend' and 'Bug', got: %d", len(res))
	}
}

func TestTaskService_AttachAndRemoveLabel(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember: true,
	}

	labelID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	task := models.Task{
		ID:        taskID,
		Title:     "Task 1",
		ProjectID: projectID,
		Labels:    []models.Label{},
	}

	taskRepo := &stubTaskRepo{tasks: map[uuid.UUID]*models.Task{
		taskID: &task,
	}}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Attach Label (should succeed)
	err := service.AttachLabelToTask(projectID, taskID, labelID, userID, orgID)
	if err != nil {
		t.Fatalf("expected AttachLabelToTask to succeed, got: %v", err)
	}
	if len(task.Labels) != 1 || task.Labels[0].ID != labelID {
		t.Errorf("expected 1 label with ID %s attached to task, got: %+v", labelID, task.Labels)
	}

	// Idempotency: Attach same label again (should be no-op/succeed)
	err = service.AttachLabelToTask(projectID, taskID, labelID, userID, orgID)
	if err != nil {
		t.Fatalf("expected AttachLabelToTask to succeed (idempotent), got: %v", err)
	}
	if len(task.Labels) != 1 {
		t.Errorf("expected task to still have exactly 1 label, got: %d", len(task.Labels))
	}

	// Remove Label (should succeed)
	err = service.RemoveLabelFromTask(projectID, taskID, labelID, userID, orgID)
	if err != nil {
		t.Fatalf("expected RemoveLabelFromTask to succeed, got: %v", err)
	}
	if len(task.Labels) != 0 {
		t.Errorf("expected task to have 0 labels, got: %d", len(task.Labels))
	}
}

func TestTaskService_ValidationAndBusinessRules(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	// Stubs
	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}, IsActive: true}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}
	taskRepo := &stubTaskRepo{
		tasks:          make(map[uuid.UUID]*models.Task),
		sprintStatuses: make(map[uuid.UUID]string),
	}
	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, logger)

	// Case 1: Title too short
	_, _, err := service.CreateTask(dto.CreateTaskRequest{
		Title:          "ab",
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation for short title")
	}

	// Case 2: Title too long
	_, _, err = service.CreateTask(dto.CreateTaskRequest{
		Title:          string(make([]byte, 201)),
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation for long title")
	}

	// Case 3: Story points not Fibonacci
	_, _, err = service.CreateTask(dto.CreateTaskRequest{
		Title:          "Valid Title",
		StoryPoints:    4,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation for non-Fibonacci story points")
	}

	// Case 4: Backdated due date for non-PM/Admin
	backdated := time.Now().Add(-5 * 24 * time.Hour)
	_, _, err = service.CreateTask(dto.CreateTaskRequest{
		Title:          "Valid Title",
		DueDate:        &backdated,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation for backdated due date")
	}

	// Case 5: Backdated due date allowed for PM/Admin
	authRepo.user.Role.Name = string(dto.RoleOrgAdmin)
	_, createdTask, err := service.CreateTask(dto.CreateTaskRequest{
		Title:          "Valid Title",
		DueDate:        &backdated,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("expected PM/Admin to be allowed to backdate, got %v", err)
	}

	// Case 6: Actual Hours update on completed task
	taskID := createdTask.ID
	taskRepo.tasks[taskID].Status = string(dto.TaskStatusCompleted)
	authRepo.user.Role.Name = string(dto.RoleMember)

	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:         taskID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		ActualHours:    pointer(5.0),
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation when updating actual hours for a completed task")
	}

	// Case 7: Changing sprint of task in completed sprint
	completedSprintID := uuid.Must(uuid.NewV4())
	newSprintID := uuid.Must(uuid.NewV4())
	taskRepo.sprintStatuses[completedSprintID] = "completed"
	taskRepo.sprintStatuses[newSprintID] = "active"

	taskRepo.tasks[taskID].Status = string(dto.TaskStatusInProgress)
	taskRepo.tasks[taskID].SprintID = &completedSprintID
	taskRepo.tasks[taskID].Sprint = &models.Sprint{ID: completedSprintID, Status: "completed"}

	_, err = service.UpdateTask(dto.UpdateTaskRequest{
		TaskID:         taskID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
		SprintID:       &newSprintID,
	})
	if err == nil || err.Code != response.ErrValidation {
		t.Fatal("expected ErrValidation when changing sprint of a task in a completed sprint")
	}
}

func TestTaskService_BulkDeleteTasks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	taskID1 := uuid.Must(uuid.NewV4())
	taskID2 := uuid.Must(uuid.NewV4())
	taskID3 := uuid.Must(uuid.NewV4()) // already deleted
	taskID4 := uuid.Must(uuid.NewV4()) // not found / invalid

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	task1 := &models.Task{ID: taskID1, ProjectID: projectID, Key: "T-1", Title: "Task 1"}
	task2 := &models.Task{ID: taskID2, ProjectID: projectID, Key: "T-2", Title: "Task 2"}
	task3 := &models.Task{ID: taskID3, ProjectID: projectID, Key: "T-3", Title: "Task 3", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}

	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{
			taskID1: task1,
			taskID2: task2,
			taskID3: task3,
		},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	req := dto.BulkDeleteTasksRequest{
		TaskIDs:        []uuid.UUID{taskID1, taskID2, taskID3, taskID4},
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}

	res, err := service.BulkDeleteTasks(req)
	if err != nil {
		t.Fatalf("expected no bulk error, got %v", err)
	}

	if res.DeletedCount != 2 {
		t.Errorf("expected 2 deleted tasks, got %d", res.DeletedCount)
	}

	if len(res.DeletedTaskIDs) != 2 {
		t.Errorf("expected 2 deleted task IDs, got %d", len(res.DeletedTaskIDs))
	}

	if !task1.DeletedAt.Valid {
		t.Error("expected task1 to be soft deleted")
	}
	if !task2.DeletedAt.Valid {
		t.Error("expected task2 to be soft deleted")
	}

	if len(res.FailedTaskIDs) != 2 {
		t.Errorf("expected 2 failed task IDs, got %d", len(res.FailedTaskIDs))
	}

	// task3 is already deleted
	reason3, ok3 := res.FailureReasons[taskID3.String()]
	if !ok3 || reason3 != "Task is already deleted" {
		t.Errorf("expected task3 failure reason to be 'Task is already deleted', got: %s", reason3)
	}

	// task4 is not found
	reason4, ok4 := res.FailureReasons[taskID4.String()]
	if !ok4 || reason4 != "Task not found" {
		t.Errorf("expected task4 failure reason to be 'Task not found', got: %s", reason4)
	}
}

func TestTaskService_CreateTask_WithCrossProjectUserStory(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	otherStoryID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}, IsActive: true},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember: true,
	}
	taskRepo := &stubTaskRepo{
		validUserStories: map[uuid.UUID]bool{
			otherStoryID: false, // Inactive or from a different project
		},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	req := dto.CreateTaskRequest{
		Title:          "Task with invalid story",
		Type:           "task",
		Priority:       "medium",
		UserStoryID:    &otherStoryID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}

	_, _, err := service.CreateTask(req)
	if err == nil {
		t.Fatal("expected error due to cross-project User Story, got nil")
	}
	if err.Message != "User story must belong to the same project" {
		t.Errorf("expected error message 'User story must belong to the same project', got '%s'", err.Message)
	}
}

func TestTaskService_GetTasks_StatusValidation(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, RoleID: uuid.Must(uuid.NewV7()), Role: models.Role{Name: string(dto.RoleMember)}}}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember: true,
	}

	statusTodo := models.CustomStatus{ID: uuid.Must(uuid.NewV4()), ProjectID: projectID, Name: "Todo"}
	statusInProgress := models.CustomStatus{ID: uuid.Must(uuid.NewV4()), ProjectID: projectID, Name: "In Progress"}

	statusRepo := &stubCustomStatusRepo{
		statuses: map[uuid.UUID]map[string]*models.CustomStatus{
			projectID: {
				"todo":        &statusTodo,
				"in_progress": &statusInProgress,
			},
		},
	}

	taskRepo := &stubTaskRepo{tasks: map[uuid.UUID]*models.Task{}}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, statusRepo, &stubFavoriteRepo{}, zap.NewNop())

	t.Run("status=valid UUID resolves status ID correctly", func(t *testing.T) {
		res, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
			StatusID: []string{statusTodo.ID.String()},
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		_ = res
	})

	t.Run("status=UUID of another project returns 422", func(t *testing.T) {
		otherProjectStatusID := uuid.Must(uuid.NewV4())
		_, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
			StatusID: []string{otherProjectStatusID.String()},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != 422 {
			t.Errorf("expected status code 422, got: %d", err.StatusCode)
		}
		if err.Message != "Invalid task status_id: status does not exist or does not belong to this project" {
			t.Errorf("unexpected error message: %s", err.Message)
		}
	})

	t.Run("status=invalid UUID/name returns 422", func(t *testing.T) {
		_, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
			StatusID: []string{"nonexistent-status-name"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != 422 {
			t.Errorf("expected status code 422, got: %d", err.StatusCode)
		}
		if err.Message != "Invalid task status value: status name not found in this project" {
			t.Errorf("unexpected error message: %s", err.Message)
		}
	})

	t.Run("status=TODO case-insensitive resolves to todo", func(t *testing.T) {
		res, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
			StatusID: []string{"TODO"},
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		_ = res
	})

	t.Run("status=In Progress normalizes and resolves to in_progress", func(t *testing.T) {
		res, _, err := service.GetTasks(projectID, userID, orgID, dto.TaskFilter{
			StatusID: []string{"In Progress"},
		})
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		_ = res
	})
}

func TestTaskService_AssignTaskToMe(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{
		user: models.User{
			ID:             userID,
			OrganizationID: &orgID,
			IsActive:       true,
			RoleID:         uuid.Must(uuid.NewV7()),
			Role: models.Role{
				Name: string(dto.RoleMember),
			},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Key:       "WP-1",
		Title:     "Task Title",
		Status:    string(dto.TaskStatusTodo),
		Project:   models.Project{ID: projectID, OrganizationID: orgID},
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	t.Run("successfully assigns task to self", func(t *testing.T) {
		existingTask.AssigneeID = nil
		existingTask.Assignee = nil

		updated, err := service.AssignTaskToMe(taskID, userID, orgID, projectID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.AssigneeID == nil || *updated.AssigneeID != userID {
			t.Fatalf("expected assignee to be %v, got %v", userID, updated.AssigneeID)
		}
	})

	t.Run("forbidden when user is not active member of project", func(t *testing.T) {
		inactiveUserRepo := &sprintAuthRepoStub{
			user: models.User{
				ID:             userID,
				OrganizationID: &orgID,
				IsActive:       false,
				RoleID:         uuid.Must(uuid.NewV7()),
				Role: models.Role{
					Name: string(dto.RoleMember),
				},
			},
		}
		inactiveService := services.InitTaskService(inactiveUserRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

		_, err := inactiveService.AssignTaskToMe(taskID, userID, orgID, projectID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected status code 403, got: %d", err.StatusCode)
		}
	})

	t.Run("forbidden when user is not project member", func(t *testing.T) {
		nonMemberProjectRepo := &stubProjectRepo{
			project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
			isMember: false,
		}
		nonMemberService := services.InitTaskService(authRepo, nonMemberProjectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

		_, err := nonMemberService.AssignTaskToMe(taskID, userID, orgID, projectID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected status code 403, got: %d", err.StatusCode)
		}
	})

	t.Run("returns not found for non-existent task", func(t *testing.T) {
		nonExistentTaskID := uuid.Must(uuid.NewV4())
		_, err := service.AssignTaskToMe(nonExistentTaskID, userID, orgID, projectID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != http.StatusNotFound {
			t.Errorf("expected status code 404, got: %d", err.StatusCode)
		}
	})

	t.Run("forbidden when task does not belong to specified project", func(t *testing.T) {
		anotherProjectID := uuid.Must(uuid.NewV4())
		_, err := service.AssignTaskToMe(taskID, userID, orgID, anotherProjectID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected status code 403, got: %d", err.StatusCode)
		}
	})
}

func TestTaskService_UpdateTask_AssigneeAuditLogUsesUsername(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	oldAssigneeID := uuid.Must(uuid.NewV4())
	newAssigneeID := uuid.Must(uuid.NewV4())

	oldUser := models.User{
		ID:             oldAssigneeID,
		OrganizationID: &orgID,
		UserName:       "john_doe",
		IsActive:       true,
		Role:           models.Role{Name: string(dto.RoleMember)},
	}
	newUser := models.User{
		ID:             newAssigneeID,
		OrganizationID: &orgID,
		UserName:       "jane_doe",
		IsActive:       true,
		Role:           models.Role{Name: string(dto.RoleMember)},
	}
	callingUser := models.User{
		ID:             userID,
		OrganizationID: &orgID,
		UserName:       "caller",
		IsActive:       true,
		Role:           models.Role{Name: string(dto.RoleMember)},
	}

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID:        callingUser,
			oldAssigneeID: oldUser,
			newAssigneeID: newUser,
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Work Pilot"},
		isMember: true,
	}

	existingTask := &models.Task{
		ID:         taskID,
		ProjectID:  projectID,
		Key:        "WP-1",
		Title:      "Task 1",
		Status:     string(dto.TaskStatusTodo),
		AssigneeID: &oldAssigneeID,
		Assignee:   &oldUser,
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: existingTask},
	}

	auditRepo := &stubAuditLogRepo{}
	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, auditRepo, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	req := dto.UpdateTaskRequest{
		TaskID:     taskID,
		ProjectID:  projectID,
		UserID:     userID,
		AssigneeID: &newAssigneeID,
	}

	_, err := service.UpdateTask(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(auditRepo.createdLogs) == 0 {
		t.Fatalf("expected audit log created")
	}

	details := auditRepo.createdLogs[0].Details
	expectedSubstring := "assignee changed from john_doe to jane_doe"
	if !strings.Contains(details, expectedSubstring) {
		t.Errorf("expected audit details to contain %q, got %q", expectedSubstring, details)
	}
}

func TestTaskService_GetTaskByID_WithIDAndKey(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	user := models.User{
		ID:             userID,
		OrganizationID: &orgID,
		UserName:       "tester",
		IsActive:       true,
		Role:           models.Role{Name: "member"},
	}
	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: user,
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Test Project", Slug: "test-project"},
		isMember: true,
	}

	task := &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Key:       "PROJ-101",
		Title:     "Sample Task Key Test",
		Status:    "todo",
	}

	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{taskID: task},
	}

	service := services.InitTaskService(authRepo, projectRepo, taskRepo, &stubUserStoryRepo{}, &stubAuditLogRepo{}, &stubCustomStatusRepo{}, &stubFavoriteRepo{}, zap.NewNop())

	// Test 1: Fetch by Task UUID string and Project UUID string
	respByID, err := service.GetTaskByID(taskID.String(), projectID.String(), userID, orgID)
	if err != nil {
		t.Fatalf("expected no error fetching by task ID UUID, got %v", err)
	}
	if respByID.ID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, respByID.ID)
	}
	if respByID.Key != "PROJ-101" {
		t.Errorf("expected task key PROJ-101, got %s", respByID.Key)
	}

	// Test 2: Fetch by Task Key string ("PROJ-101")
	respByKey, err := service.GetTaskByID("PROJ-101", projectID.String(), userID, orgID)
	if err != nil {
		t.Fatalf("expected no error fetching by task key, got %v", err)
	}
	if respByKey.ID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, respByKey.ID)
	}
	if respByKey.Title != "Sample Task Key Test" {
		t.Errorf("expected task title 'Sample Task Key Test', got %s", respByKey.Title)
	}

	// Test 3: Fetch by lowercase Task Key string ("proj-101")
	respByLowerKey, err := service.GetTaskByID("proj-101", projectID.String(), userID, orgID)
	if err != nil {
		t.Fatalf("expected no error fetching by lowercase task key, got %v", err)
	}
	if respByLowerKey.ID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, respByLowerKey.ID)
	}

	// Test 4: Fetch by Project Slug ("test-project")
	respBySlug, err := service.GetTaskByID("PROJ-101", "test-project", userID, orgID)
	if err != nil {
		t.Fatalf("expected no error fetching by project slug, got %v", err)
	}
	if respBySlug.ID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, respBySlug.ID)
	}
}



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
)

type userStoryAuthRepoStub struct {
	dummyAuthRepo
	users map[uuid.UUID]models.User
	err   *response.Error
}

func (s *userStoryAuthRepoStub) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	if s.err != nil {
		return models.User{}, s.err
	}
	user, ok := s.users[id]
	if !ok {
		return models.User{}, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "User not found"}
	}
	return user, nil
}
type stubUserStoryRepo struct {
	stories        map[uuid.UUID]*models.UserStory
	validSprints   map[uuid.UUID]bool
	storyTaskStats map[uuid.UUID]models.StoryTaskStats
	createErr      *response.Error
	getErr         *response.Error
	updateErr      *response.Error
	deleteErr      *response.Error
}

func (s *stubUserStoryRepo) CreateUserStory(userStory *models.UserStory) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.stories == nil {
		s.stories = make(map[uuid.UUID]*models.UserStory)
	}
	s.stories[userStory.ID] = userStory
	return nil
}

func (s *stubUserStoryRepo) GetUserStoryByID(id uuid.UUID, projectID uuid.UUID) (*models.UserStory, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	story, ok := s.stories[id]
	if !ok || story.DeletedAt.Valid {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "User story not found"}
	}
	return story, nil
}

func (s *stubUserStoryRepo) UpdateUserStory(userStoryID uuid.UUID, updates map[string]interface{}) *response.Error {
	if s.updateErr != nil {
		return s.updateErr
	}
	story, ok := s.stories[userStoryID]
	if ok {
		if val, ok := updates["title"].(string); ok {
			story.Title = val
		}
		if val, ok := updates["description"].(string); ok {
			story.Description = val
		}
		if val, ok := updates["priority"].(string); ok {
			story.Priority = val
		}
		if val, ok := updates["status"].(string); ok {
			story.Status = val
		}
		if val, ok := updates["status_id"].(uuid.UUID); ok {
			story.StatusID = val
		}
		if val, ok := updates["story_points"].(int); ok {
			story.StoryPoints = val
		}
		if val, ok := updates["assignee_id"]; ok {
			if val == nil {
				story.AssigneeID = nil
				story.Assignee = nil
			} else if uid, ok := val.(uuid.UUID); ok {
				story.AssigneeID = &uid
			}
		}
		if val, ok := updates["sprint_id"]; ok {
			if val == nil {
				story.SprintID = nil
				story.Sprint = nil
			} else if uid, ok := val.(uuid.UUID); ok {
				story.SprintID = &uid
			}
		}
	}
	return nil
}

func (s *stubUserStoryRepo) DeleteUserStory(id uuid.UUID, projectID uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	story, ok := s.stories[id]
	if ok {
		story.DeletedAt.Time = time.Now()
		story.DeletedAt.Valid = true
	}
	return nil
}

func (s *stubUserStoryRepo) GetUserStories(projectID uuid.UUID, filter dto.UserStoryFilter) ([]models.UserStory, response.Pagination, *response.Error) {
	var res []models.UserStory
	for _, story := range s.stories {
		if story.DeletedAt.Valid {
			continue
		}
		if filter.Status != "" && story.Status != filter.Status {
			continue
		}
		if filter.Priority != "" && story.Priority != filter.Priority {
			continue
		}
		res = append(res, *story)
	}
	return res, response.Pagination{}, nil
}

func (s *stubUserStoryRepo) IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error) {
	if s.validSprints == nil {
		return true, nil
	}
	return s.validSprints[sprintID], nil
}

func (s *stubUserStoryRepo) GetMaxBacklogOrder(projectID uuid.UUID) (int, *response.Error) {
	return len(s.stories), nil
}

func (s *stubUserStoryRepo) ReorderUserStories(projectID uuid.UUID, storyIDs []uuid.UUID) *response.Error {
	for idx, id := range storyIDs {
		if story, ok := s.stories[id]; ok {
			story.BacklogOrder = idx + 1
		}
	}
	return nil
}

func (s *stubUserStoryRepo) GetStoryTaskStats(projectID uuid.UUID) (map[uuid.UUID]models.StoryTaskStats, *response.Error) {
	if s.storyTaskStats == nil {
		return make(map[uuid.UUID]models.StoryTaskStats), nil
	}
	return s.storyTaskStats, nil
}

func (s *stubUserStoryRepo) GetUserStoryAccessContext(id uuid.UUID) (*models.UserStoryAccessContext, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	story, ok := s.stories[id]
	if !ok || story.DeletedAt.Valid {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "User story not found"}
	}
	return &models.UserStoryAccessContext{
		UserStoryID:    story.ID,
		Title:          story.Title,
		ProjectID:      story.ProjectID,
		OrganizationID: story.Project.OrganizationID,
	}, nil
}

func TestUserStoryService_CreateUserStory_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember), FullName: "John Doe"},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "WorkPilot Backend"},
		isMember: true,
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	req := dto.CreateUserStoryRequest{
		Title:       "User Story Title",
		Description: "Detailed description of the user story.",
		Priority:    "high",
		StoryPoints: 5,
		ProjectID:   projectID,
		ReporterID:  userID,
	}

	res, err := service.CreateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Title != req.Title || res.Priority != req.Priority || res.StoryPoints != req.StoryPoints {
		t.Errorf("mismatched return fields: %+v", res)
	}

	if res.SprintID != nil {
		t.Errorf("expected sprint ID to be nil (Product Backlog by default), got %v", res.SprintID)
	}

	if strings.ToLower(res.Status) != "todo" {
		t.Errorf("expected default status to be 'todo', got %s", res.Status)
	}
}

func TestUserStoryService_CreateUserStory_ForbiddenIfNoAccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: false, // not a project member
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	req := dto.CreateUserStoryRequest{
		Title:      "Some title",
		Priority:   "medium",
		ProjectID:  projectID,
		ReporterID: userID,
	}

	_, err := service.CreateUserStory(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", err.StatusCode)
	}
}

func TestUserStoryService_CreateUserStory_InvalidSprintID(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}
	userStoryRepo := &stubUserStoryRepo{
		stories:      make(map[uuid.UUID]*models.UserStory),
		validSprints: map[uuid.UUID]bool{sprintID: false}, // Sprint does not belong to the project
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	req := dto.CreateUserStoryRequest{
		Title:      "Title",
		Priority:   "low",
		ProjectID:  projectID,
		ReporterID: userID,
		SprintID:   &sprintID,
	}

	_, err := service.CreateUserStory(req)
	if err == nil {
		t.Fatal("expected error for sprint not in project, got nil")
	}
	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
}

func TestUserStoryService_CreateUserStory_InvalidAssigneeID(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	assigneeID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID:     {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
			assigneeID: {ID: assigneeID, OrganizationID: &orgID, Role: string(dto.RoleMember), IsActive: false}, // Assignee is inactive
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
		// Mock assignee is not a member of the project either
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	req := dto.CreateUserStoryRequest{
		Title:      "Title",
		Priority:   "medium",
		ProjectID:  projectID,
		ReporterID: userID,
		AssigneeID: &assigneeID,
	}

	_, err := service.CreateUserStory(req)
	if err == nil {
		t.Fatal("expected error for inactive/non-member assignee, got nil")
	}
	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", err.StatusCode)
	}
}

func TestUserStoryService_GetUserStoryByID_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	existingStory := &models.UserStory{
		ID:        storyID,
		ProjectID: projectID,
		Title:     "Get Me By ID",
		Priority:  "low",
		Status:    "todo",
	}
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: existingStory},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	res, err := service.GetUserStoryByID(storyID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.ID != storyID || res.Title != "Get Me By ID" {
		t.Errorf("mismatched retrieved user story: %+v", res)
	}
}

func TestUserStoryService_UpdateUserStory_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	existingStory := &models.UserStory{
		ID:        storyID,
		ProjectID: projectID,
		Title:     "Old Title",
		Priority:  "low",
		Status:    "todo",
	}
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: existingStory},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	newTitle := "New Title"
	newPriority := "critical"
	newPoints := 8
	newStatus := "in_progress"

	req := dto.UpdateUserStoryRequest{
		UserStoryID: storyID,
		ProjectID:   projectID,
		UserID:      userID,
		Title:       &newTitle,
		Priority:    &newPriority,
		StoryPoints: &newPoints,
		Status:      &newStatus,
	}

	res, err := service.UpdateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Title != newTitle || res.Priority != newPriority || res.StoryPoints != newPoints || models.NormalizeTaskStatus(res.Status) != models.NormalizeTaskStatus(newStatus) {
		t.Errorf("fields did not update correctly: %+v", res)
	}
}

func TestUserStoryService_UpdateUserStory_SprintID(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	sprintID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	existingStory := &models.UserStory{
		ID:        storyID,
		ProjectID: projectID,
		Title:     "Title",
		SprintID:  &sprintID,
	}
	userStoryRepo := &stubUserStoryRepo{
		stories:      map[uuid.UUID]*models.UserStory{storyID: existingStory},
		validSprints: map[uuid.UUID]bool{sprintID: true},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	// 1. Update to a new Sprint ID
	newSprintID := uuid.Must(uuid.NewV4())
	userStoryRepo.validSprints[newSprintID] = true

	req := dto.UpdateUserStoryRequest{
		UserStoryID: storyID,
		ProjectID:   projectID,
		UserID:      userID,
		SprintID:    &newSprintID,
	}

	res, err := service.UpdateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.SprintID == nil || *res.SprintID != newSprintID {
		t.Errorf("sprint_id did not update to new sprint: %v", res.SprintID)
	}

	// 2. Update Sprint ID to null (explicitly null field in JSON)
	req = dto.UpdateUserStoryRequest{
		UserStoryID:  storyID,
		ProjectID:    projectID,
		UserID:       userID,
		SprintID:     nil,
		IsNullFields: map[string]bool{"sprint_id": true},
	}

	res, err = service.UpdateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.SprintID != nil {
		t.Errorf("sprint_id was not cleared (should be nil): %v", res.SprintID)
	}

	// 3. Update Sprint ID to null using uuid.Nil (sentinel value)
	existingStory.SprintID = &sprintID
	uuidNilVal := uuid.Nil

	req = dto.UpdateUserStoryRequest{
		UserStoryID: storyID,
		ProjectID:   projectID,
		UserID:      userID,
		SprintID:    &uuidNilVal,
	}

	res, err = service.UpdateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.SprintID != nil {
		t.Errorf("sprint_id was not cleared when using uuid.Nil (should be nil): %v", res.SprintID)
	}
}

func TestUpdateUserStoryRequest_UnmarshalJSON(t *testing.T) {
	// Case 1: sprint_id is omitted
	jsonDataOmitted := []byte(`{"title": "New Title"}`)
	var reqOmitted dto.UpdateUserStoryRequest
	if err := json.Unmarshal(jsonDataOmitted, &reqOmitted); err != nil {
		t.Fatalf("unmarshal omitted failed: %v", err)
	}
	if reqOmitted.SprintID != nil {
		t.Errorf("expected SprintID to be nil when omitted")
	}
	if reqOmitted.IsSprintIDNull() {
		t.Errorf("expected IsSprintIDNull to be false when omitted")
	}

	// Case 2: sprint_id is explicitly null
	jsonDataNull := []byte(`{"title": "New Title", "sprint_id": null}`)
	var reqNull dto.UpdateUserStoryRequest
	if err := json.Unmarshal(jsonDataNull, &reqNull); err != nil {
		t.Fatalf("unmarshal null failed: %v", err)
	}
	if reqNull.SprintID != nil {
		t.Errorf("expected SprintID to be nil when null")
	}
	if !reqNull.IsSprintIDNull() {
		t.Errorf("expected IsSprintIDNull to be true when null")
	}

	// Case 3: sprint_id is a valid UUID
	sprintUUID := uuid.Must(uuid.NewV4())
	jsonDataValid := []byte(`{"title": "New Title", "sprint_id": "` + sprintUUID.String() + `"}`)
	var reqValid dto.UpdateUserStoryRequest
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

func TestUserStoryService_DeleteUserStory_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	existingStory := &models.UserStory{
		ID:        storyID,
		ProjectID: projectID,
		Title:     "To Delete",
		Priority:  "medium",
	}
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: existingStory},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	err := service.DeleteUserStory(storyID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it's marked as deleted
	deletedStory := userStoryRepo.stories[storyID]
	if !deletedStory.DeletedAt.Valid {
		t.Errorf("expected story to be soft-deleted")
	}
}

func TestUserStoryService_ReorderUserStories_Success(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	story1ID := uuid.Must(uuid.NewV4())
	story2ID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	story1 := &models.UserStory{ID: story1ID, ProjectID: projectID, Title: "Story 1", BacklogOrder: 1}
	story2 := &models.UserStory{ID: story2ID, ProjectID: projectID, Title: "Story 2", BacklogOrder: 2}

	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{
			story1ID: story1,
			story2ID: story2,
		},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	// Reorder them: story2 first, then story1
	err := service.ReorderUserStories(projectID, userID, orgID, []uuid.UUID{story2ID, story1ID})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if story2.BacklogOrder != 1 || story1.BacklogOrder != 2 {
		t.Errorf("reordering did not persist: story1 order=%d, story2 order=%d", story1.BacklogOrder, story2.BacklogOrder)
	}
}

func TestUserStoryService_ProgressCalculation(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}

	story := &models.UserStory{ID: storyID, ProjectID: projectID, Title: "User Story with tasks"}

	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: story},
		storyTaskStats: map[uuid.UUID]models.StoryTaskStats{
			storyID: {
				UserStoryID: storyID,
				TotalTasks:  4,
				Completed:   2,
			},
		},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, &stubCustomStatusRepo{}, zap.NewNop())

	res, err := service.GetUserStoryByID(storyID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 2 completed out of 4 total tasks should result in 50% progress
	if res.Progress != 50.0 {
		t.Errorf("expected progress to be 50.0, got %.2f", res.Progress)
	}
	if res.TotalTasks != 4 {
		t.Errorf("expected total tasks to be 4, got %d", res.TotalTasks)
	}
	if res.CompletedTasks != 2 {
		t.Errorf("expected completed tasks to be 2, got %d", res.CompletedTasks)
	}
}

func TestUserStoryService_GetUserStoryByID_IncludesTasks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}
	story := &models.UserStory{ID: storyID, ProjectID: projectID, Title: "User Story with tasks"}
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: story},
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{
			taskID: {
				ID:          taskID,
				ProjectID:   projectID,
				UserStoryID: &storyID,
				Title:       "Sub-task 1",
				Status:      "todo",
			},
		},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, taskRepo, &stubCustomStatusRepo{}, zap.NewNop())

	res, err := service.GetUserStoryByID(storyID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(res.Tasks) != 1 {
		t.Fatalf("expected 1 task in response, got %d", len(res.Tasks))
	}
	if res.Tasks[0].ID != taskID || res.Tasks[0].Title != "Sub-task 1" {
		t.Errorf("unexpected task response: %+v", res.Tasks[0])
	}
}

func TestUserStoryService_GetUserStories_IncludesTasks(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}
	story := &models.UserStory{ID: storyID, ProjectID: projectID, Title: "Story in list"}
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{storyID: story},
	}
	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{
			taskID: {
				ID:          taskID,
				ProjectID:   projectID,
				UserStoryID: &storyID,
				Title:       "Task in story list",
				Status:      "todo",
			},
		},
	}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, taskRepo, &stubCustomStatusRepo{}, zap.NewNop())

	stories, _, err := service.GetUserStories(projectID, userID, orgID, dto.UserStoryFilter{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stories) != 1 {
		t.Fatalf("expected 1 user story, got %d", len(stories))
	}
	if len(stories[0].Tasks) != 1 {
		t.Fatalf("expected 1 task in user story response, got %d", len(stories[0].Tasks))
	}
	if stories[0].Tasks[0].ID != taskID || stories[0].Tasks[0].Title != "Task in story list" {
		t.Errorf("unexpected task in story list: %+v", stories[0].Tasks[0])
	}
}

func TestUserStoryService_CreateUserStory_CustomStatus(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember), FullName: "John Doe"},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project Name"},
		isMember: true,
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	customStatusRepo := &stubCustomStatusRepo{}

	// Add a specific custom status to project
	customStatusID := uuid.Must(uuid.NewV4())
	customStatusRepo.CreateStatus(&models.CustomStatus{
		ID:        customStatusID,
		ProjectID: projectID,
		Name:      "Ready For Dev",
		Color:     "#FFFF00",
	})

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, customStatusRepo, zap.NewNop())

	// 1. Create with custom status ID
	req := dto.CreateUserStoryRequest{
		Title:      "User Story with Status ID",
		Priority:   "medium",
		ProjectID:  projectID,
		ReporterID: userID,
		StatusID:   &customStatusID,
	}

	res, err := service.CreateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.StatusID != customStatusID || res.Status != "Ready For Dev" || res.StatusColor != "#FFFF00" {
		t.Errorf("mismatched resolved status fields: %+v", res)
	}

	// 2. Create with custom status Name
	customStatusName := "Ready For Dev"
	req2 := dto.CreateUserStoryRequest{
		Title:      "User Story with Status Name",
		Priority:   "medium",
		ProjectID:  projectID,
		ReporterID: userID,
		Status:     customStatusName,
	}

	res2, err2 := service.CreateUserStory(req2)
	if err2 != nil {
		t.Fatalf("expected no error, got %v", err2)
	}
	if res2.StatusID != customStatusID || res2.Status != "Ready For Dev" || res2.StatusColor != "#FFFF00" {
		t.Errorf("mismatched resolved status fields by name: %+v", res2)
	}
}

func TestUserStoryService_CreateUserStory_StatusPrecedence(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember), FullName: "John Doe"},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Project Name"},
		isMember: true,
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	customStatusRepo := &stubCustomStatusRepo{}

	// Create custom status 1
	statusID1 := uuid.Must(uuid.NewV4())
	customStatusRepo.CreateStatus(&models.CustomStatus{
		ID:        statusID1,
		ProjectID: projectID,
		Name:      "Design Phase",
		Color:     "#00FF00",
	})

	// Create custom status 2
	statusID2 := uuid.Must(uuid.NewV4())
	customStatusRepo.CreateStatus(&models.CustomStatus{
		ID:        statusID2,
		ProjectID: projectID,
		Name:      "Build Phase",
		Color:     "#0000FF",
	})

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, customStatusRepo, zap.NewNop())

	// StatusID (statusID1) should take precedence over Status Name ("Build Phase")
	req := dto.CreateUserStoryRequest{
		Title:      "Precedence Test Story",
		Priority:   "low",
		ProjectID:  projectID,
		ReporterID: userID,
		StatusID:   &statusID1,
		Status:     "Build Phase",
	}

	res, err := service.CreateUserStory(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.StatusID != statusID1 || res.Status != "Design Phase" || res.StatusColor != "#00FF00" {
		t.Errorf("StatusID did not take precedence over Status name: %+v", res)
	}
}

func TestUserStoryService_CreateUserStory_InvalidStatus(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &userStoryAuthRepoStub{
		users: map[uuid.UUID]models.User{
			userID: {ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)},
		},
	}
	projectRepo := &stubProjectRepo{
		project:  models.Project{ID: projectID, OrganizationID: orgID},
		isMember: true,
	}
	userStoryRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	customStatusRepo := &stubCustomStatusRepo{}

	service := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, &stubTaskRepo{}, customStatusRepo, zap.NewNop())

	// 1. Invalid status ID
	invalidStatusID := uuid.Must(uuid.NewV4())
	req := dto.CreateUserStoryRequest{
		Title:      "Invalid Status ID Story",
		Priority:   "low",
		ProjectID:  projectID,
		ReporterID: userID,
		StatusID:   &invalidStatusID,
	}

	_, err := service.CreateUserStory(req)
	if err == nil {
		t.Fatal("expected error for invalid StatusID, got nil")
	}
	if err.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", err.StatusCode)
	}

	// 2. Invalid status Name
	req2 := dto.CreateUserStoryRequest{
		Title:      "Invalid Status Name Story",
		Priority:   "low",
		ProjectID:  projectID,
		ReporterID: userID,
		Status:     "Non-existent Status",
	}

	_, err2 := service.CreateUserStory(req2)
	if err2 == nil {
		t.Fatal("expected error for invalid status name, got nil")
	}
	if err2.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", err2.StatusCode)
	}
}




package services_test

import (
	"errors"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type mockSearchRepo struct {
	tasks       []models.Task
	stories     []models.UserStory
	projects    []models.Project
	users       []models.User
	sprints     []models.Sprint
	err         error
	lastTsQuery string
	lastRaw     string
}

func (m *mockSearchRepo) SearchTasks(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Task, error) {
	m.lastTsQuery = tsQuery
	m.lastRaw = rawQuery
	return m.tasks, m.err
}

func (m *mockSearchRepo) SearchUserStories(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.UserStory, error) {
	return m.stories, m.err
}

func (m *mockSearchRepo) SearchProjects(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Project, error) {
	return m.projects, m.err
}

func (m *mockSearchRepo) SearchMembers(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.User, error) {
	return m.users, m.err
}

func (m *mockSearchRepo) SearchSprints(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Sprint, error) {
	return m.sprints, m.err
}

func TestGlobalSearchSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())

	projectID := uuid.Must(uuid.NewV7())
	project := models.Project{
		ID:   projectID,
		Name: "Project 1",
		Slug: "proj-1",
	}
	mockRepo := &mockSearchRepo{
		tasks: []models.Task{
			{Title: "Task 1", Key: "T-1", ProjectID: projectID, Project: project},
		},
		stories: []models.UserStory{
			{Title: "Story 1", Key: "US-1", ProjectID: projectID, Project: project},
		},
		projects: []models.Project{
			project,
		},
		users: []models.User{
			{FullName: "User 1", UserName: "user1"},
		},
		sprints: []models.Sprint{
			{Name: "Sprint 1", Goal: "Goal 1", ProjectID: projectID, Project: project},
		},
	}

	service := services.InitSearchService(mockRepo, &stubFavoriteAuthRepo{}, &stubFavoriteProjectRepo{}, zap.NewNop())

	resp, err := service.GlobalSearch(userID, orgID, "web development")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mockRepo.lastTsQuery != "web:* & development:*" {
		t.Errorf("expected tsQuery to be 'web:* & development:*', got '%s'", mockRepo.lastTsQuery)
	}

	if mockRepo.lastRaw != "web development" {
		t.Errorf("expected rawQuery to be 'web development', got '%s'", mockRepo.lastRaw)
	}

	if len(resp.Tasks) != 1 || resp.Tasks[0].Title != "Task 1" {
		t.Errorf("expected 1 task with Title 'Task 1', got %v", resp.Tasks)
	}
	if resp.Tasks[0].ProjectSlug != "proj-1" || resp.Tasks[0].ProjectName != "Project 1" {
		t.Errorf("expected task project details to be set, got %+v", resp.Tasks[0])
	}

	if len(resp.UserStories) != 1 || resp.UserStories[0].Title != "Story 1" {
		t.Errorf("expected 1 user story with Title 'Story 1', got %v", resp.UserStories)
	}
	if resp.UserStories[0].ProjectSlug != "proj-1" || resp.UserStories[0].ProjectName != "Project 1" {
		t.Errorf("expected story project details to be set, got %+v", resp.UserStories[0])
	}

	if len(resp.Projects) != 1 || resp.Projects[0].Title != "Project 1" {
		t.Errorf("expected 1 project with Title 'Project 1', got %v", resp.Projects)
	}
	if resp.Projects[0].ProjectSlug != "proj-1" || resp.Projects[0].ProjectName != "Project 1" {
		t.Errorf("expected project project details to be set, got %+v", resp.Projects[0])
	}

	if len(resp.Members) != 1 || resp.Members[0].Title != "User 1" {
		t.Errorf("expected 1 member with Title 'User 1', got %v", resp.Members)
	}

	if len(resp.Sprints) != 1 || resp.Sprints[0].Title != "Sprint 1" {
		t.Errorf("expected 1 sprint with Title 'Sprint 1', got %v", resp.Sprints)
	}
	if resp.Sprints[0].ProjectSlug != "proj-1" || resp.Sprints[0].ProjectName != "Project 1" {
		t.Errorf("expected sprint project details to be set, got %+v", resp.Sprints[0])
	}
}

func TestGlobalSearchEmptyQuery(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	mockRepo := &mockSearchRepo{}

	service := services.InitSearchService(mockRepo, &stubFavoriteAuthRepo{}, &stubFavoriteProjectRepo{}, zap.NewNop())

	resp, err := service.GlobalSearch(userID, orgID, "   ")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Tasks) != 0 || len(resp.UserStories) != 0 || len(resp.Projects) != 0 || len(resp.Members) != 0 || len(resp.Sprints) != 0 {
		t.Errorf("expected empty search results for empty query, got %+v", resp)
	}
}

func TestGlobalSearchError(t *testing.T) {
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	mockRepo := &mockSearchRepo{
		err: errors.New("database failure"),
	}

	service := services.InitSearchService(mockRepo, &stubFavoriteAuthRepo{}, &stubFavoriteProjectRepo{}, zap.NewNop())

	_, err := service.GlobalSearch(userID, orgID, "web")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

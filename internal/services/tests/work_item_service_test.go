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
	"gorm.io/gorm"
)

type stubWorkItemAuthRepo struct {
	dummyAuthRepo
	users map[uuid.UUID]models.User
}

func (s *stubWorkItemAuthRepo) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	user, ok := s.users[id]
	if !ok {
		return models.User{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User not found"}
	}
	return user, nil
}

type stubWorkItemProjectRepo struct {
	projects map[uuid.UUID]models.Project
	members  map[string]bool // key: projectID_userID
}

func (s *stubWorkItemProjectRepo) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	p, ok := s.projects[id]
	if !ok {
		return models.Project{}, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Project not found"}
	}
	return p, nil
}

func (s *stubWorkItemProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	key := projectID.String() + "_" + userID.String()
	return s.members[key], nil
}

func (s *stubWorkItemProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) UpdateProject(projectID uuid.UUID, req map[string]interface{}) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) GetProjectsByOrganizationID(organizationID uuid.UUID, filter requestdto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubWorkItemProjectRepo) GetAllProjects(filter requestdto.GlobalProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubWorkItemProjectRepo) GetMemberCountsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error) {
	return make(map[uuid.UUID]int64), nil
}
func (s *stubWorkItemProjectRepo) GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error) {
	return nil, nil
}
func (s *stubWorkItemProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	return &models.ProjectMember{UserID: userID, ProjectID: projectID, ProjectRole: "developer"}, nil
}
func (s *stubWorkItemProjectRepo) DeleteProject(projectID, organizationID uuid.UUID) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) CreateProjectMember(row *models.ProjectMember) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) GetProjectsMembersByProjectID(projectID uuid.UUID, filter requestdto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubWorkItemProjectRepo) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {
	return nil
}
func (s *stubWorkItemProjectRepo) GetProjectActivity(projectID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

type stubWorkItemRepo struct {
	tasks   map[int64]*models.Task
	stories map[int64]*models.UserStory
}

func (s *stubWorkItemRepo) GetTaskBySerialNumber(serialNumber int64) (*models.Task, *response.Error) {
	task, ok := s.tasks[serialNumber]
	if !ok || !task.DeletedAt.Time.IsZero() {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Task not found"}
	}
	return task, nil
}

func (s *stubWorkItemRepo) GetUserStoryBySerialNumber(serialNumber int64) (*models.UserStory, *response.Error) {
	story, ok := s.stories[serialNumber]
	if !ok || !story.DeletedAt.Time.IsZero() {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User story not found"}
	}
	return story, nil
}

func TestGetWorkItemBySerialNumber(t *testing.T) {
	logger := zap.NewNop()

	projectID1 := uuid.Must(uuid.NewV4())
	projectID2 := uuid.Must(uuid.NewV4())
	orgID := uuid.Must(uuid.NewV4())

	userIDMember := uuid.Must(uuid.NewV4())
	userIDNonMember := uuid.Must(uuid.NewV4())

	authRepo := &stubWorkItemAuthRepo{
		users: map[uuid.UUID]models.User{
			userIDMember:    {ID: userIDMember, Role: "user", OrganizationID: &orgID},
			userIDNonMember: {ID: userIDNonMember, Role: "user", OrganizationID: &orgID},
		},
	}

	projectRepo := &stubWorkItemProjectRepo{
		projects: map[uuid.UUID]models.Project{
			projectID1: {ID: projectID1, OrganizationID: orgID, Name: "Project 1"},
			projectID2: {ID: projectID2, OrganizationID: orgID, Name: "Project 2"},
		},
		members: map[string]bool{
			projectID1.String() + "_" + userIDMember.String(): true,
			projectID2.String() + "_" + userIDMember.String(): true,
		},
	}

	task101 := &models.Task{
		ID:           uuid.Must(uuid.NewV4()),
		ProjectID:    projectID1,
		SerialNumber: 101,
		Title:        "Implement login UI",
		Type:         "task",
		Priority:     "high",
		Status:       "in_progress",
	}

	story202 := &models.UserStory{
		ID:           uuid.Must(uuid.NewV4()),
		ProjectID:    projectID1,
		SerialNumber: 202,
		Title:        "As a user I want login",
		Priority:     "medium",
		Status:       "todo",
	}

	taskInOtherProject := &models.Task{
		ID:           uuid.Must(uuid.NewV4()),
		ProjectID:    projectID2,
		SerialNumber: 303,
		Title:        "Project 2 Task",
		Type:         "task",
		Priority:     "low",
		Status:       "todo",
	}

	deletedTask := &models.Task{
		ID:           uuid.Must(uuid.NewV4()),
		ProjectID:    projectID1,
		SerialNumber: 404,
		Title:        "Deleted Task",
		DeletedAt:    gorm.DeletedAt{Time: time.Now(), Valid: true},
	}

	workItemRepo := &stubWorkItemRepo{
		tasks: map[int64]*models.Task{
			101: task101,
			303: taskInOtherProject,
			404: deletedTask,
		},
		stories: map[int64]*models.UserStory{
			202: story202,
		},
	}

	service := services.InitWorkItemService(authRepo, projectRepo, workItemRepo, nil, nil, nil, nil, logger)

	t.Run("Successfully retrieve Task by serial ID", func(t *testing.T) {
		res, err := service.GetWorkItemBySerialNumber(projectID1, 101, userIDMember)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.WorkItemType != "task" {
			t.Errorf("Expected WorkItemType 'task', got %s", res.WorkItemType)
		}
		if res.SerialNumber != 101 {
			t.Errorf("Expected SerialNumber 101, got %d", res.SerialNumber)
		}
		if res.Title != "Implement login UI" {
			t.Errorf("Expected Title 'Implement login UI', got %s", res.Title)
		}
		if res.TaskDetails == nil {
			t.Errorf("Expected TaskDetails to be non-nil")
		}
	})

	t.Run("Successfully retrieve User Story by serial ID", func(t *testing.T) {
		res, err := service.GetWorkItemBySerialNumber(projectID1, 202, userIDMember)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.WorkItemType != "user_story" {
			t.Errorf("Expected WorkItemType 'user_story', got %s", res.WorkItemType)
		}
		if res.SerialNumber != 202 {
			t.Errorf("Expected SerialNumber 202, got %d", res.SerialNumber)
		}
		if res.Title != "As a user I want login" {
			t.Errorf("Expected Title 'As a user I want login', got %s", res.Title)
		}
		if res.UserStoryDetails == nil {
			t.Errorf("Expected UserStoryDetails to be non-nil")
		}
	})

	t.Run("Return 404 when serial ID does not exist", func(t *testing.T) {
		_, err := service.GetWorkItemBySerialNumber(projectID1, 999, userIDMember)
		if err == nil {
			t.Fatalf("Expected error for non-existent serial ID, got nil")
		}
		if err.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", err.StatusCode)
		}
	})

	t.Run("Return 404 when serial ID exists in another project", func(t *testing.T) {
		// Serial 303 belongs to projectID2, but requested on projectID1
		_, err := service.GetWorkItemBySerialNumber(projectID1, 303, userIDMember)
		if err == nil {
			t.Fatalf("Expected error for serial ID belonging to another project, got nil")
		}
		if err.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", err.StatusCode)
		}
	})

	t.Run("Return 404 when item is soft-deleted", func(t *testing.T) {
		_, err := service.GetWorkItemBySerialNumber(projectID1, 404, userIDMember)
		if err == nil {
			t.Fatalf("Expected error for soft-deleted item, got nil")
		}
		if err.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", err.StatusCode)
		}
	})

	t.Run("Return 403 when user is not authorized for the project", func(t *testing.T) {
		_, err := service.GetWorkItemBySerialNumber(projectID1, 101, userIDNonMember)
		if err == nil {
			t.Fatalf("Expected error for unauthorized user, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", err.StatusCode)
		}
	})
}

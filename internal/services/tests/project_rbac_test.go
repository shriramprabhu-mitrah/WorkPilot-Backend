package services_test

import (
	"net/http"
	"testing"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubProjectRbacAuthRepo struct {
	authrepo.AuthRepository
	roleName string
	orgID    uuid.UUID
}

func (s *stubProjectRbacAuthRepo) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	return models.User{
		ID:             id,
		OrganizationID: &s.orgID,
		Role:           models.Role{Name: s.roleName},
	}, nil
}

type stubProjectRbacProjectRepo struct {
	projectrepo.ProjectRepository
	roleName string
	project  models.Project
	isMember bool
}

func (s *stubProjectRbacProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	if !s.isMember {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404}
	}
	return &models.ProjectMember{
		UserID:    userID,
		ProjectID: projectID,
		Role:      models.Role{Name: s.roleName},
	}, nil
}

func (s *stubProjectRbacProjectRepo) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	return s.project, nil
}

func (s *stubProjectRbacProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {
	return nil
}

func (s *stubProjectRbacProjectRepo) DeleteProject(projectID, organizationID uuid.UUID) *response.Error {
	return nil
}

func (s *stubProjectRbacProjectRepo) IsSlugExists(slug string, excludeProjectID *uuid.UUID) (bool, *response.Error) {
	return false, nil
}

func TestProjectService_CreateProject_RBAC(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())

	t.Run("Org Admin can create project", func(t *testing.T) {
		authRepo := &stubProjectRbacAuthRepo{roleName: "org_admin", orgID: orgID}
		projectRepo := &stubProjectRbacProjectRepo{}
		service := services.InitProjectService(projectRepo, authRepo, &dummySprintRepo{}, &stubTaskRepo{}, &stubAuditLogRepo{}, logger)

		req := requestdto.CreateProjectRequest{
			UserID:         userID,
			OrganizationID: orgID,
			Name:           "New Project",
		}

		_, err := service.CreateProject(req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("Developer cannot create project", func(t *testing.T) {
		authRepo := &stubProjectRbacAuthRepo{roleName: "developer", orgID: orgID}
		projectRepo := &stubProjectRbacProjectRepo{}
		service := services.InitProjectService(projectRepo, authRepo, &dummySprintRepo{}, &stubTaskRepo{}, &stubAuditLogRepo{}, logger)

		req := requestdto.CreateProjectRequest{
			UserID:         userID,
			OrganizationID: orgID,
			Name:           "New Project",
		}

		_, err := service.CreateProject(req)
		if err == nil {
			t.Fatal("expected forbidden error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", err.StatusCode)
		}
	})
}

func TestProjectService_DeleteProject_RBAC(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())

	project := models.Project{
		ID:             projectID,
		OrganizationID: orgID,
		Name:           "Project to Delete",
	}

	t.Run("Org Admin can delete project", func(t *testing.T) {
		authRepo := &stubProjectRbacAuthRepo{roleName: "org_admin", orgID: orgID}
		projectRepo := &stubProjectRbacProjectRepo{
			roleName: "org_admin",
			project:  project,
			isMember: true,
		}
		service := services.InitProjectService(projectRepo, authRepo, &dummySprintRepo{}, &stubTaskRepo{}, &stubAuditLogRepo{}, logger)

		req := requestdto.DeleteProject{
			UserID:         userID,
			OrganizationID: orgID,
			ProjectID:      projectID,
		}

		err := service.DeleteProject(req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("Developer cannot delete project", func(t *testing.T) {
		authRepo := &stubProjectRbacAuthRepo{roleName: "developer", orgID: orgID}
		projectRepo := &stubProjectRbacProjectRepo{
			roleName: "developer",
			project:  project,
			isMember: true,
		}
		service := services.InitProjectService(projectRepo, authRepo, &dummySprintRepo{}, &stubTaskRepo{}, &stubAuditLogRepo{}, logger)

		req := requestdto.DeleteProject{
			UserID:         userID,
			OrganizationID: orgID,
			ProjectID:      projectID,
		}

		err := service.DeleteProject(req)
		if err == nil {
			t.Fatal("expected forbidden error, got nil")
		}
		if err.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", err.StatusCode)
		}
	})
}

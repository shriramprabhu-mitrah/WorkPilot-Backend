package services

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(req dto.CreateProjectRequest) *response.Error
	UpdateProject(req dto.UpdateProjectRequest) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error)
	CreateProjectMemeber(req dto.CreateProjectMemberRequest) *response.Error
	GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error)
	RemoveProjectMember(projectID, userID uuid.UUID) *response.Error
}

func InitProjectService(projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, logger *zap.Logger) ProjectService {
	return &projectService{
		authRepo:    authRepo,
		projectRepo: projectRepo,
		logger:      logger,
	}
}

type projectService struct {
	authRepo    authrepo.AuthRepository
	projectRepo projectrepo.ProjectRepository
	logger      *zap.Logger
}

func (s *projectService) CreateProject(req dto.CreateProjectRequest) *response.Error {

	projectPayload := models.Project{
		Name:           req.Name,
		Description:    req.Description,
		Status:         string(dto.ProjectStatusPlanning),
		OrganizationID: req.OrganizationID,
		CreatedBy:      req.UserID,
	}

	projectMemberPayload := models.ProjectMember{
		UserID:    req.UserID,
		AddedByID: req.UserID,
		JoinedAt:  time.Now(),
	}

	err := s.projectRepo.CreateProjectWithMember(projectPayload, projectMemberPayload)
	if err != nil {
		return err
	}

	return nil
}

func (s *projectService) UpdateProject(req dto.UpdateProjectRequest) *response.Error {

	result, err := s.authRepo.GetByID(req.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	payload := models.Project{
		Name:        req.Name,
		Description: req.Description,
	}

	if req.Status != "" {
		if err := req.Status.Validate(); err != nil {
			s.logger.Error("Invalid project status", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid status. Allowed values: active, archived, on_hold, completed, cancelled, planning",
			}
		}
		payload.Status = string(req.Status)
	}

	return s.projectRepo.UpdateProject(req.ProjectID, payload)
}
func (s *projectService) GetProjectsByOrganizationID(organizationID uuid.UUID, filterPayload dto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error) {

	if filterPayload.Status != "" {
		if err := filterPayload.Status.Validate(); err != nil {
			s.logger.Error("Invalid project status", zap.Error(err))
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid status. Allowed values: active, archived, on_hold, completed, cancelled, planning",
			}
		}
	}

	filter := dto.ProjectFilter{
		Page:     filterPayload.Page,
		PageSize: filterPayload.PageSize,
		Name:     filterPayload.Name,
		Status:   string(filterPayload.Status),
	}

	return s.projectRepo.GetProjectsByOrganizationID(organizationID, filter)
}

func (s *projectService) CreateProjectMemeber(req dto.CreateProjectMemberRequest) *response.Error {

	for _, userID := range req.UserIDs {

		result, err := s.authRepo.GetByID(userID)
		if err != nil {
			return err
		}

		if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to perform this action",
			}
		}

		if *result.OrganizationID != req.OrganizationID {
			s.logger.Error("Unauthorized Access",
				zap.String("Organization ID", req.OrganizationID.String()),
				zap.String("User Organization ID", result.OrganizationID.String()),
				zap.String("User ID", userID.String()),
			)

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to perform this action",
			}
		}

		projectMember := models.ProjectMember{
			ProjectID: req.ProjectID,
			UserID:    userID,
			AddedByID: req.AddedByID,
			JoinedAt:  time.Now(),
		}

		if err := s.projectRepo.CreateProjectMember(projectMember); err != nil {
			return err
		}
	}

	return nil
}

func (s *projectService) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {

	return s.projectRepo.GetProjectsMembersByProjectID(projectID, filter)
}

func (s *projectService) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {

	return s.projectRepo.RemoveProjectMember(projectID, userID)
}

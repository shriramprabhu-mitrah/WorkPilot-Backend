package services

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(row dto.CreateProjectRequest) *response.Error
	UpdateProject(req dto.UpdateProjectRequest) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error)
	ArchiveProject(payload dto.ArchiveProjectRequest) *response.Error
}

func InitProjectService(projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, logger *zap.Logger) ProjectService {
	return &projectService{
		AuthRepo:    authRepo,
		ProjectRepo: projectRepo,
		logger:      logger,
	}
}

type projectService struct {
	AuthRepo    authrepo.AuthRepository
	ProjectRepo projectrepo.ProjectRepository
	logger      *zap.Logger
}

func (s *projectService) CreateProject(row dto.CreateProjectRequest) *response.Error {

	if !utils.ValidateKey(row.Key) {
		s.logger.Error("Failed validation in key",
			zap.String("User ID", row.UserID.String()))
		return &response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Key does not meet the requirement",
		}
	}

	startDate, startErr := utils.StringToTime(row.StartDate)
	endDate, endErr := utils.StringToTime(row.EndDate)
	if startErr != nil && endErr != nil {
		s.logger.Error("Failed to parse the string into time",
			zap.Error(startErr),
			zap.Error(endErr))
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid time, Expected format: YYYY-MM-DD ",
		}
	}

	payload := models.Project{
		Name:           row.Name,
		Key:            row.Key,
		Description:    row.Description,
		StartDate:      startDate,
		EndDate:        endDate,
		OrganizationID: row.OrganizationID,
		CreatedBy:      row.UserID,
	}
	err := s.ProjectRepo.CreateProject(payload)
	if err != nil {
		return err
	}

	return nil
}

func (s *projectService) UpdateProject(req dto.UpdateProjectRequest) *response.Error {

	result, err := s.AuthRepo.GetByID(req.UserID)
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

	if req.StartDate != "" {
		t, err := utils.StringToTime(req.StartDate)
		if err != nil {
			s.logger.Error("Failed to parse start date", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
		payload.StartDate = t
	}

	if req.EndDate != "" {
		t, err := utils.StringToTime(req.EndDate)
		if err != nil {
			s.logger.Error("Failed to parse end date", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}
		payload.EndDate = t
	}

	return s.ProjectRepo.UpdateProject(req.ProjectID, payload)
}
func (s *projectService) GetProjectsByOrganizationID(organizationID uuid.UUID, filterPayload dto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error) {

	if filterPayload.Status != "" {
		if err := filterPayload.Status.Validate(); err != nil {
			s.logger.Error("Invalid project status", zap.Error(err))
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid status. Allowed values: active, archived",
			}
		}
	}

	filter := dto.ProjectFilter{
		Page:     filterPayload.Page,
		PageSize: filterPayload.PageSize,
		Name:     filterPayload.Name,
		Key:      filterPayload.Key,
		Status:   string(filterPayload.Status),
	}

	if filterPayload.StartDate != "" {
		t, err := utils.StringToTime(filterPayload.StartDate)
		if err != nil {
			s.logger.Error("Failed to parse start date", zap.Error(err))
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
		filter.StartDate = t
	}

	if filterPayload.EndDate != "" {
		t, err := utils.StringToTime(filterPayload.EndDate)
		if err != nil {
			s.logger.Error("Failed to parse end date", zap.Error(err))
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}
		filter.EndDate = t
	}
	return s.ProjectRepo.GetProjectsByOrganizationID(organizationID, filter)
}

func (s *projectService) ArchiveProject(payload dto.ArchiveProjectRequest) *response.Error {

	result, err := s.ProjectRepo.GetProjectByID(payload.ProjectID)
	if err != nil {
		return err
	}

	if result.OrganizationID == uuid.Nil || payload.OrganizationID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if result.OrganizationID != payload.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Payload Project Id", payload.ProjectID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	request := result

	if *payload.Archived {
		request.ArchiveStatus = models.ProjectStatusArchived
	} else {
		request.ArchiveStatus = models.ProjectStatusActive
	}

	err = s.ProjectRepo.UpdateProject(payload.ProjectID, request)
	if err != nil {
		return err
	}

	return nil
}

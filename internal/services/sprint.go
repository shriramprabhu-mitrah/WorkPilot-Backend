package services

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"go.uber.org/zap"
)

type SprintService interface {
	CreateSprint(req dto.CreateSprintRequest) *response.Error
	DeleteSprint(req dto.DeleteSprint) *response.Error
	UpdateSprint(req dto.UpdateSprintRequest) *response.Error
	GetSprints(req dto.GetSprint, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error)
	GetSprintByID(req dto.GetSprint) (*models.Sprint, *response.Error)
}

func InitSprintService(sprintRepo sprintrepo.SprintRepository, projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, logger *zap.Logger) SprintService {
	return &sprintService{
		sprintRepo:  sprintRepo,
		projectRepo: projectRepo,
		authRepo:    authRepo,
		logger:      logger,
	}
}

type sprintService struct {
	sprintRepo  sprintrepo.SprintRepository
	projectRepo projectrepo.ProjectRepository
	authRepo    authrepo.AuthRepository
	logger      *zap.Logger
}

func (s *sprintService) CreateSprint(req dto.CreateSprintRequest) *response.Error {

	result, errResp := s.authRepo.GetUserByID(req.UserID)
	if errResp != nil {
		return errResp
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

	for _, spr := range req.Sprints {

		startDate, err := utils.StringToTime(spr.StartDate)
		if err != nil {
			s.logger.Error("Invalid start_date", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}

		endDate, err := utils.StringToTime(spr.EndDate)
		if err != nil {
			s.logger.Error("Invalid end_date", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}

		if endDate.Before(*startDate) {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "end_date cannot be before start_date",
			}
		}

		sprint := models.Sprint{
			Name:        spr.Name,
			Goal:        spr.Goal,
			StartDate:   *startDate,
			EndDate:     *endDate,
			ProjectID:   req.ProjectID,
			CreatedByID: req.UserID,
		}

		if err := s.sprintRepo.CreateSprint(sprint); err != nil {
			return err
		}
	}

	return nil
}

func (s *sprintService) DeleteSprint(req dto.DeleteSprint) *response.Error {

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return errorResponse
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

	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	return s.sprintRepo.DeleteSprint(req.SprintID)
}

func (s *sprintService) UpdateSprint(req dto.UpdateSprintRequest) *response.Error {

	var startDate, endDate *time.Time

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return errorResponse
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

	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	existingSprint, errResp := s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
	if errResp != nil {
		return errResp
	}

	startDate = &existingSprint.StartDate
	endDate = &existingSprint.EndDate

	if req.StartDate != "" {
		d, err := utils.StringToTime(req.StartDate)
		startDate = d
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
	}

	if req.EndDate != "" {
		d, err := utils.StringToTime(req.EndDate)
		startDate = d
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
	}

	if startDate.After(*endDate) {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Sprint start date cannot be after end date",
		}
	}

	payload := models.Sprint{
		Name:      req.Name,
		Goal:      req.Goal,
		StartDate: *startDate,
		EndDate:   *endDate,
		Status:    string(req.Status),
	}

	return s.sprintRepo.UpdateSprint(req.ProjectID, req.SprintID, payload)
}

func (s *sprintService) GetSprints(req dto.GetSprint, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return nil, response.Pagination{}, errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if req.ProjectID == uuid.Nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
	}

	return s.sprintRepo.GetSprints(req.ProjectID, filter)
}

func (s *sprintService) GetSprintByID(req dto.GetSprint) (*models.Sprint, *response.Error) {

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return nil, errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if req.ProjectID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
	}

	if req.SprintID == uuid.Nil && req.ProjectID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid Sprint/Project id",
		}
	}

	return s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
}

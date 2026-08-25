package services

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	dashboardrepo "github.com/ms-kanban-server/internal/repository/dashboard-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type DashboardService interface {
	GetOverview(projectID uuid.UUID, userID uuid.UUID) (responsedto.DashboardOverview, *response.Error)
	GetTaskStatus(projectID uuid.UUID, userID uuid.UUID) (map[string]int64, *response.Error)
	GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardSprintBurndownResponse, *response.Error)
	GetWeeklyProgress(projectID uuid.UUID, startDate time.Time, endDate time.Time, userID uuid.UUID) ([]responsedto.WeeklyProgress, *response.Error)
	GetTeamWorkload(projectID uuid.UUID, userID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error)
	GetDashboardData(projectID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardResponse, *response.Error)
}

func InitDashboardService(
	dashboardRepo dashboardrepo.DashboardRepository,
	projectRepo projectrepo.ProjectRepository,
	authRepo authrepo.AuthRepository,
	sprintRepo sprintrepo.SprintRepository,
	taskRepo taskrepo.TaskRepository,
	auditRepo auditrepo.AuditLogRepository,
	logger *zap.Logger,
) DashboardService {
	return &dashboardService{
		dashboardRepo: dashboardRepo,
		projectRepo:   projectRepo,
		authRepo:      authRepo,
		sprintRepo:    sprintRepo,
		taskRepo:      taskRepo,
		auditRepo:     auditRepo,
		logger:        logger,
	}
}

type dashboardService struct {
	dashboardRepo dashboardrepo.DashboardRepository
	projectRepo   projectrepo.ProjectRepository
	authRepo      authrepo.AuthRepository
	sprintRepo    sprintrepo.SprintRepository
	taskRepo      taskrepo.TaskRepository
	auditRepo     auditrepo.AuditLogRepository
	logger        *zap.Logger
}

func (s *dashboardService) checkAuthorization(projectID uuid.UUID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		s.logger.Error("Failed to get project", zap.Error(fmt.Errorf("%v", err)))
		return models.Project{}, models.User{}, false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(fmt.Errorf("%v", err)))
		return models.Project{}, models.User{}, false, err
	}

	if user.Role.Name == string(dto.RoleSuperAdmin) {
		return models.Project{}, models.User{}, false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "view")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

// GetOverview returns dashboard overview with task counts
func (s *dashboardService) GetOverview(projectID uuid.UUID, userID uuid.UUID) (responsedto.DashboardOverview, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error("Authorization check failed", zap.Error(fmt.Errorf("%v", err)))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check authorization",
		}
	}

	if !authorized {
		s.logger.Warn("User not authorized for project", zap.String("projectID", projectID.String()), zap.String("userID", userID.String()))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view dashboard in this project",
		}
	}

	overview, err := s.dashboardRepo.GetOverview(projectID)
	if err != nil {
		s.logger.Error("Failed to get overview", zap.Error(fmt.Errorf("%v", err)))
		return responsedto.DashboardOverview{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get dashboard overview",
		}
	}

	s.logger.Info("Dashboard overview fetched successfully", zap.String("projectID", projectID.String()))
	return overview, nil
}

// GetTaskStatus returns task counts grouped by status
func (s *dashboardService) GetTaskStatus(projectID uuid.UUID, userID uuid.UUID) (map[string]int64, *response.Error) {

	// Check authorization
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error(
			"Authorization check failed",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, err
	}

	// Check project access
	if !authorized {
		s.logger.Warn(
			"User not authorized for project",
			zap.String("projectID", projectID.String()),
			zap.String("userID", userID.String()),
		)

		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view task status in this project",
		}
	}

	// Get task status from repository
	taskStatus, err := s.dashboardRepo.GetTaskStatus(projectID)
	if err != nil {
		s.logger.Error(
			"Failed to get task status",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, err
	}

	s.logger.Info(
		"Task status fetched successfully",
		zap.String("projectID", projectID.String()),
	)

	return taskStatus, nil
}

// GetTeamWorkload returns workload metrics for team members
func (s *dashboardService) GetTeamWorkload(projectID uuid.UUID, userID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error("Authorization check failed", zap.Error(fmt.Errorf("%v", err)))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check authorization",
		}
	}

	if !authorized {
		s.logger.Warn("User not authorized for project", zap.String("projectID", projectID.String()), zap.String("userID", userID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view team workload in this project",
		}
	}

	teamWorkload, err := s.dashboardRepo.GetTeamWorkload(projectID)
	if err != nil {
		s.logger.Error("Failed to get team workload", zap.Error(fmt.Errorf("%v", err)))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get team workload",
		}
	}

	s.logger.Info("Team workload fetched successfully", zap.String("projectID", projectID.String()))
	return teamWorkload, nil
}

// GetWeeklyProgress returns weekly progress statistics
func (s *dashboardService) GetWeeklyProgress(projectID uuid.UUID, startDate time.Time, endDate time.Time, userID uuid.UUID) ([]responsedto.WeeklyProgress, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error("Authorization check failed", zap.Error(fmt.Errorf("%v", err)))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check authorization",
		}
	}

	if !authorized {
		s.logger.Warn("User not authorized for project", zap.String("projectID", projectID.String()), zap.String("userID", userID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view weekly progress in this project",
		}
	}

	if startDate.After(endDate) {
		s.logger.Warn("Invalid date range", zap.Time("startDate", startDate), zap.Time("endDate", endDate))
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Start date must be before end date",
		}
	}

	weeklyProgress, err := s.dashboardRepo.GetWeeklyProgress(projectID, startDate, endDate)
	if err != nil {
		s.logger.Error("Failed to get weekly progress", zap.Error(fmt.Errorf("%v", err)))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get weekly progress",
		}
	}

	s.logger.Info("Weekly progress fetched successfully", zap.String("projectID", projectID.String()))
	return weeklyProgress, nil
}

// GetSprintBurndown returns sprint burndown chart data
func (s *dashboardService) GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardSprintBurndownResponse, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error("Authorization check failed", zap.Error(fmt.Errorf("%v", err)))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check authorization",
		}
	}

	if !authorized {
		s.logger.Warn("User not authorized for project", zap.String("projectID", projectID.String()), zap.String("userID", userID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view sprint burndown in this project",
		}
	}

	var responseSprints []responsedto.SprintBurndownData

	if sprintID != uuid.Nil {
		// Verify sprint belongs to project
		sprint, err := s.sprintRepo.GetSprintByID(sprintID, projectID)
		if err != nil {
			s.logger.Error("Failed to get sprint", zap.Error(fmt.Errorf("%v", err)))
			return nil, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to get sprint",
			}
		}

		if sprint.ProjectID != projectID {
			s.logger.Warn("Sprint does not belong to project", zap.String("projectID", projectID.String()), zap.String("sprintID", sprintID.String()))
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Sprint not found in this project",
			}
		}

		sprintBurndown, err := s.dashboardRepo.GetSprintBurndown(projectID, sprintID)
		if err != nil {
			s.logger.Error("Failed to get sprint burndown", zap.Error(fmt.Errorf("%v", err)))
			return nil, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to get sprint burndown",
			}
		}

		responseSprints = append(responseSprints, responsedto.SprintBurndownData{
			SprintID:   sprint.ID,
			SprintName: sprint.Name,
			Burndown:   sprintBurndown,
		})
	} else {
		// Fetch all active sprints belonging to the project
		activeSprints, err := s.sprintRepo.GetActiveSprintsByProjectID(projectID)
		if err != nil {
			return nil, err
		}

		for _, sprint := range activeSprints {
			// Skip sprints with missing or invalid dates to avoid failing the entire dashboard view
			if sprint.StartDate == nil || sprint.EndDate == nil {
				s.logger.Warn("Skipping active sprint in burndown calculation due to missing start or end date",
					zap.String("sprintID", sprint.ID.String()),
					zap.String("projectID", projectID.String()),
				)
				continue
			}
			if sprint.EndDate.Before(*sprint.StartDate) {
				s.logger.Warn("Skipping active sprint in burndown calculation because end date is before start date",
					zap.String("sprintID", sprint.ID.String()),
					zap.String("projectID", projectID.String()),
					zap.Time("startDate", *sprint.StartDate),
					zap.Time("endDate", *sprint.EndDate),
				)
				continue
			}

			sprintBurndown, err := s.dashboardRepo.GetSprintBurndown(projectID, sprint.ID)
			if err != nil {
				s.logger.Error("Failed to get sprint burndown for active sprint", zap.String("sprintID", sprint.ID.String()), zap.Error(fmt.Errorf("%v", err)))
				// Skip this active sprint rather than failing the whole request
				continue
			}

			responseSprints = append(responseSprints, responsedto.SprintBurndownData{
				SprintID:   sprint.ID,
				SprintName: sprint.Name,
				Burndown:   sprintBurndown,
			})
		}
	}

	return &responsedto.DashboardSprintBurndownResponse{
		Sprints: responseSprints,
	}, nil
}

func (s *dashboardService) GetDashboardData(projectID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardResponse, *response.Error) {

	// 1. Check authorization
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		s.logger.Error(
			"Authorization check failed",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check authorization",
		}
	}

	if !authorized {
		s.logger.Warn(
			"User not authorized for project",
			zap.String("projectID", projectID.String()),
			zap.String("userID", userID.String()),
		)

		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view dashboard in this project",
		}
	}

	// 2. Get overview
	overview, err := s.dashboardRepo.GetOverview(projectID)
	if err != nil {
		s.logger.Error(
			"Failed to get dashboard overview",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get dashboard overview",
		}
	}

	// 3. Get task status
	taskStatus, err := s.dashboardRepo.GetTaskStatus(projectID)
	if err != nil {
		s.logger.Error(
			"Failed to get task status",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get task status",
		}
	}

	// 4. Get team workload
	teamWorkload, err := s.dashboardRepo.GetTeamWorkload(projectID)
	if err != nil {
		s.logger.Error(
			"Failed to get team workload",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get team workload",
		}
	}

	var sprintBurndown []responsedto.SprintBurndown
	var activeSprintID uuid.UUID

	// Fetch all active sprints belonging to the project and choose the first valid one
	activeSprints, err := s.sprintRepo.GetActiveSprintsByProjectID(projectID)
	if err == nil && len(activeSprints) > 0 {
		for _, sprint := range activeSprints {
			// Skip sprints with missing or invalid dates
			if sprint.StartDate == nil || sprint.EndDate == nil || sprint.EndDate.Before(*sprint.StartDate) {
				continue
			}

			burndown, err := s.dashboardRepo.GetSprintBurndown(projectID, sprint.ID)
			if err == nil {
				sprintBurndown = burndown
				activeSprintID = sprint.ID
				break
			}
		}
	}

	// 8. Build final response
	result := &responsedto.DashboardResponse{
		Overview:       overview,
		TaskStatus:     taskStatus,
		TeamWorkload:   teamWorkload,
		SprintBurndown: sprintBurndown,
	}

	s.logger.Info(
		"Dashboard data fetched successfully",
		zap.String("projectID", projectID.String()),
		zap.String("activeSprintID", activeSprintID.String()),
	)

	return result, nil
}

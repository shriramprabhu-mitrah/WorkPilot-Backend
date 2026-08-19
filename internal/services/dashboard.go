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
	GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) ([]responsedto.SprintBurndown, *response.Error)
	GetWeeklyProgress(projectID uuid.UUID, startDate time.Time, endDate time.Time, userID uuid.UUID) ([]responsedto.WeeklyProgress, *response.Error)
	GetTeamWorkload(projectID uuid.UUID, userID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error)
	GetDashboardData(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardResponse, *response.Error)
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

// checkAuthorization verifies if the user has access to the project
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

	if user.Role == string(dto.RoleSuperAdmin) {
		return models.Project{}, models.User{}, false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return project, user, true, nil
	}

	isMember, err := s.projectRepo.IsUserProjectMember(projectID, userID)
	if err != nil {
		s.logger.Error("Failed to check project membership", zap.Error(fmt.Errorf("%v", err)))
		return models.Project{}, models.User{}, false, err
	}

	return project, user, isMember, nil
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
func (s *dashboardService) GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) ([]responsedto.SprintBurndown, *response.Error) {
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

	s.logger.Info("Sprint burndown fetched successfully", zap.String("projectID", projectID.String()), zap.String("sprintID", sprintID.String()))
	return sprintBurndown, nil
}

func (s *dashboardService) GetDashboardData(projectID uuid.UUID, sprintID uuid.UUID, userID uuid.UUID) (*responsedto.DashboardResponse, *response.Error) {

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

	// 5. Get sprint
	sprint, err := s.sprintRepo.GetSprintByID(sprintID, projectID)
	if err != nil {
		s.logger.Error(
			"Failed to get sprint",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get sprint",
		}
	}

	// 6. Validate sprint
	if sprint == nil {
		s.logger.Warn(
			"Sprint not found",
			zap.String("projectID", projectID.String()),
			zap.String("sprintID", sprintID.String()),
		)

		return nil, &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found",
		}
	}

	if sprint.ProjectID != projectID {
		s.logger.Warn(
			"Sprint does not belong to project",
			zap.String("projectID", projectID.String()),
			zap.String("sprintID", sprintID.String()),
		)

		return nil, &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found in this project",
		}
	}

	// 7. Get sprint burndown
	sprintBurndown, err := s.dashboardRepo.GetSprintBurndown(
		projectID,
		sprintID,
	)
	if err != nil {
		s.logger.Error(
			"Failed to get sprint burndown",
			zap.Error(fmt.Errorf("%v", err)),
		)

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get sprint burndown",
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
		zap.String("sprintID", sprintID.String()),
	)

	return result, nil
}

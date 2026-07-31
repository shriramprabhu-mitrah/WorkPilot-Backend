package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(req dto.CreateProjectRequest) *response.Error
	UpdateProject(req dto.UpdateProjectRequest) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error)
	CreateProjectMemeber(req dto.CreateProjectMemberRequest) *response.Error
	GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error)
	RemoveProjectMember(projectID, userID, performingUserID, organizationID uuid.UUID) *response.Error
	GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, req dto.ProjectActivityFilterRequest) ([]dto.ProjectActivityResponse, response.Pagination, *response.Error)
	GetProjectDetails(req dto.GetProjectDetails) (*dto.ProjectResponse, *response.Error)
	DeleteProject(req dto.DeleteProject) *response.Error
}

func InitProjectService(projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, sprintRepo sprintrepo.SprintRepository, logger *zap.Logger) ProjectService {
	return &projectService{
		authRepo:    authRepo,
		projectRepo: projectRepo,
		sprintRepo:  sprintRepo,
		logger:      logger,
	}
}

type projectService struct {
	authRepo    authrepo.AuthRepository
	projectRepo projectrepo.ProjectRepository
	sprintRepo  sprintrepo.SprintRepository
	logger      *zap.Logger
}

func (s *projectService) CreateProject(req dto.CreateProjectRequest) *response.Error {

	result, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
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

	projectPayload := &models.Project{
		Name:           req.Name,
		Description:    req.Description,
		Status:         string(dto.ProjectStatusPlanning),
		OrganizationID: req.OrganizationID,
		CreatedBy:      req.UserID,
	}

	projectMemberPayload := &models.ProjectMember{
		UserID:    req.UserID,
		AddedByID: req.UserID,
		JoinedAt:  time.Now(),
	}

	err = s.projectRepo.CreateProjectWithMember(projectPayload, projectMemberPayload)
	if err != nil {
		return err
	}

	// get the project details
	project, err := s.projectRepo.GetProjectByID(projectPayload.ID)
	if err != nil {
		return err
	}

	// Log project creation audit event
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &project.ID,
		Action:         "project_created",
		ResourceType:   "project",
		ResourceID:     project.ID.String(),
		Details:        fmt.Sprintf("Project '%s' created", req.Name),
		CreatedAt:      time.Now(),
	}

	// if error occurred just warn it do not return error
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) UpdateProject(req dto.UpdateProjectRequest) *response.Error {

	result, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return err
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
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

	updateErr := s.projectRepo.UpdateProject(req.ProjectID, payload)
	if updateErr != nil {
		return updateErr
	}

	// Log project update audit event
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "project_updated",
		ResourceType:   "project",
		ResourceID:     req.ProjectID.String(),
		Details:        fmt.Sprintf("Project updated: %s", req.Name),
		CreatedAt:      time.Now(),
	}
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
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
		Page:      filterPayload.Page,
		PageSize:  filterPayload.PageSize,
		Name:      filterPayload.Name,
		Status:    string(filterPayload.Status),
		SortBy:    filterPayload.SortBy,
		SortOrder: filterPayload.SortOrder,
	}

	return s.projectRepo.GetProjectsByOrganizationID(organizationID, filter)
}

func (s *projectService) CreateProjectMemeber(req dto.CreateProjectMemberRequest) *response.Error {

	var existingUsers []string

	for _, userID := range req.UserIDs {

		result, err := s.authRepo.GetUserByID(userID)
		if err != nil {
			return err
		}

		if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
			s.logger.Error("Unauthorized Access",
				zap.String("Organization ID", req.OrganizationID.String()),
				zap.String("User Organization ID", result.OrganizationID.String()))

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

		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, userID)
		if err != nil {
			return err
		}

		if isMember {
			existingUsers = append(existingUsers, result.UserName)
			continue
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

		auditLog := models.AuditLog{
			UserID:         &req.AddedByID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			Action:         "member_added",
			ResourceType:   "project_member",
			ResourceID:     userID.String(),
			Details:        fmt.Sprintf("User %s added to project", userID.String()),
			CreatedAt:      time.Now(),
		}

		if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}
	}

	if len(existingUsers) > 0 {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("The following users are already members of the project: %s", strings.Join(existingUsers, ", ")),
		}
	}

	return nil
}

func (s *projectService) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {

	return s.projectRepo.GetProjectsMembersByProjectID(projectID, filter)
}

func (s *projectService) RemoveProjectMember(projectID, userID, performingUserID, organizationID uuid.UUID) *response.Error {

	err := s.projectRepo.RemoveProjectMember(projectID, userID)
	if err != nil {
		return err
	}

	// Log member removed audit event
	auditLog := models.AuditLog{
		UserID:         &performingUserID,
		OrganizationID: &organizationID,
		ProjectID:      &projectID,
		Action:         "member_removed",
		ResourceType:   "project_member",
		ResourceID:     userID.String(),
		Details:        fmt.Sprintf("User %s removed from project", userID.String()),
		CreatedAt:      time.Now(),
	}
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, filterReq dto.ProjectActivityFilterRequest) ([]dto.ProjectActivityResponse, response.Pagination, *response.Error) {

	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// Authorization check: super_admin, org_admin of project's org, or project member
	isAuthorized := false

	if userRole == string(models.RoleSuperAdmin) {
		isAuthorized = true
	} else if userRole == string(models.RoleOrgAdmin) && userOrgID != uuid.Nil && userOrgID == project.OrganizationID {
		isAuthorized = true
	} else {
		isMember, memberErr := s.projectRepo.IsUserProjectMember(projectID, userID)
		if memberErr != nil {
			return nil, response.Pagination{}, memberErr
		}
		if isMember {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		s.logger.Error("Unauthorized access to project activity log",
			zap.String("user_id", userID.String()),
			zap.String("project_id", projectID.String()),
			zap.String("user_role", userRole),
		)
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	var parsedUserID *uuid.UUID
	if filterReq.UserID != "" {
		uid, parseErr := uuid.FromString(filterReq.UserID)
		if parseErr != nil || uid == uuid.Nil {
			s.logger.Error("Invalid user ID in project activity filter", zap.String("user_id", filterReq.UserID))
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid UserID filter format",
			}
		}
		parsedUserID = &uid
	}

	filter := dto.ProjectActivityFilter{
		Page:         filterReq.Page,
		PageSize:     filterReq.PageSize,
		Action:       filterReq.Action,
		UserID:       parsedUserID,
		ResourceType: filterReq.ResourceType,
		StartDate:    filterReq.StartDate,
		EndDate:      filterReq.EndDate,
	}

	logs, pagination, repoErr := s.projectRepo.GetProjectActivity(projectID, filter)
	if repoErr != nil {
		return nil, response.Pagination{}, repoErr
	}

	var responseDTOs []dto.ProjectActivityResponse
	for _, item := range logs {
		dtoItem := dto.ProjectActivityResponse{
			ID:             item.ID,
			ProjectID:      item.ProjectID,
			OrganizationID: item.OrganizationID,
			Action:         item.Action,
			ResourceType:   item.ResourceType,
			ResourceID:     item.ResourceID,
			Details:        item.Details,
			CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		}

		if item.User.ID != uuid.Nil {
			dtoItem.User = &dto.UserSummary{
				ID:        item.User.ID,
				FullName:  item.User.FullName,
				Email:     item.User.Email,
				AvatarURL: item.User.AvatarURL,
				Role:      item.User.Role,
			}
		}

		responseDTOs = append(responseDTOs, dtoItem)
	}

	return responseDTOs, pagination, nil
}

func (s *projectService) GetProjectDetails(req dto.GetProjectDetails) (*dto.ProjectResponse, *response.Error) {

	result, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return nil, err
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	project, err := s.projectRepo.GetProjectByID(req.ProjectID)
	if err != nil {
		return nil, err
	}

	memberFilter := dto.ProjectMemberFilter{
		Page:     1,
		PageSize: 1000,
	}

	projectMembers, _, err := s.projectRepo.GetProjectsMembersByProjectID(req.ProjectID, memberFilter)
	if err != nil {
		return nil, err
	}

	sprintFilter := dto.SprintFilter{
		Page:     1,
		PageSize: 1000,
	}

	sprints, _, err := s.sprintRepo.GetSprints(req.ProjectID, sprintFilter)
	if err != nil {
		return nil, err
	}

	payload := dto.ProjectResponse{
		ProjectID:      project.ID,
		OrganizationID: project.OrganizationID,
		Name:           project.Name,
		Description:    project.Description,
		Status:         project.Status,
		CreatedBy:      project.CreatedBy,
		Creator:        project.Creator.UserName,
		CreatedAt:      project.CreatedAt,
	}

	// Map members
	payload.Members = make([]dto.ProjectMemberResponse, 0, len(projectMembers))
	for _, member := range projectMembers {
		payload.Members = append(payload.Members, dto.ProjectMemberResponse{
			UserID:   member.UserID,
			Username: member.User.UserName,
			FullName: member.User.FullName,
			Role:     member.User.Role,
		})
	}

	// Map sprints
	payload.Sprints = make([]dto.SprintResponse, 0, len(sprints))
	for _, sprint := range sprints {
		payload.Sprints = append(payload.Sprints, dto.SprintResponse{
			ID:        sprint.ID,
			Name:      sprint.Name,
			Goal:      sprint.Goal,
			Status:    string(sprint.Status),
			StartDate: sprint.StartDate,
			EndDate:   sprint.EndDate,
		})
	}

	return &payload, nil
}

func (s *projectService) DeleteProject(req dto.DeleteProject) *response.Error {

	if req.ProjectID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
	}

	return s.projectRepo.DeleteProject(req.ProjectID, req.OrganizationID)
}

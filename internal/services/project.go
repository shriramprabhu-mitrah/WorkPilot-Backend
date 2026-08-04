package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
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
	RemoveProjectMember(req requestdto.RemoveProjectMember) *response.Error
	GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, req dto.ProjectActivityFilterRequest) ([]responsedto.ProjectActivityResponse, response.Pagination, *response.Error)
	GetProjectDetails(req dto.GetProjectDetails) (*responsedto.ProjectDetail, *response.Error)
	DeleteProject(req dto.DeleteProject) *response.Error
	GetProjectsByUserID(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error)
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

	user, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return err
	}

	if user.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", user.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *user.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", user.OrganizationID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
	if err != nil {
		return err
	}

	if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
		member.ProjectRole != string(requestdto.ProjectRoleProjectManager) {

		s.logger.Error("Unauthorized project update attempt",
			zap.String("User ID", req.UserID.String()),
			zap.String("Project ID", req.ProjectID.String()),
			zap.String("Project Role", string(member.ProjectRole)))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update this project",
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
		PaginationQuery: filterPayload.PaginationQuery,
		SortQuery:       filterPayload.SortQuery,
		Name:            filterPayload.Name,
		Status:          string(filterPayload.Status),
	}

	return s.projectRepo.GetProjectsByOrganizationID(organizationID, filter)
}

func (s *projectService) CreateProjectMemeber(req dto.CreateProjectMemberRequest) *response.Error {

	var existingUsers []string

	for _, member := range req.Members {

		result, err := s.authRepo.GetUserByID(member.UserID)
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
				zap.String("User ID", member.UserID.String()),
			)

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to perform this action",
			}
		}

		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, member.UserID)
		if err != nil {
			return err
		}

		if isMember {
			existingUsers = append(existingUsers, result.UserName)
			continue
		}

		projectMember := models.ProjectMember{
			ProjectID:   req.ProjectID,
			UserID:      member.UserID,
			ProjectRole: string(member.ProjectRole),
			AddedByID:   req.AddedByID,
			JoinedAt:    time.Now(),
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
			ResourceID:     member.UserID.String(),
			Details:        fmt.Sprintf("User %s added to project", member.UserID.String()),
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

func (s *projectService) RemoveProjectMember(req requestdto.RemoveProjectMember) *response.Error {

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.PerformingUserID, req.ProjectID)
	if err != nil {
		return err
	}

	if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
		member.ProjectRole != string(requestdto.ProjectRoleProjectManager) {

		s.logger.Error("Unauthorized project update attempt",
			zap.String("User ID", req.PerformingUserID.String()),
			zap.String("Project ID", req.ProjectID.String()),
			zap.String("Project Role", string(member.ProjectRole)))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update this project",
		}
	}

	err = s.projectRepo.RemoveProjectMember(req.ProjectID, req.TargetUserID)
	if err != nil {
		return err
	}

	// Log member removed audit event
	auditLog := models.AuditLog{
		UserID:         &req.PerformingUserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "member_removed",
		ResourceType:   "project_member",
		ResourceID:     req.PerformingUserID.String(),
		Details:        fmt.Sprintf("User %s removed from project", req.PerformingUserID.String()),
		CreatedAt:      time.Now(),
	}
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, filterReq dto.ProjectActivityFilterRequest) ([]responsedto.ProjectActivityResponse, response.Pagination, *response.Error) {

	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// Authorization check: super_admin, org_admin of project's org, or project member
	isAuthorized := false

	if userRole == "super_admin" {
		isAuthorized = true
	} else if userRole == "org_admin" && userOrgID != uuid.Nil && userOrgID == project.OrganizationID {
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
		PaginationQuery: filterReq.PaginationQuery,
		Action:          filterReq.Action,
		UserID:          parsedUserID,
		ResourceType:    filterReq.ResourceType,
		StartDate:       filterReq.StartDate,
		EndDate:         filterReq.EndDate,
	}

	logs, pagination, repoErr := s.projectRepo.GetProjectActivity(projectID, filter)
	if repoErr != nil {
		return nil, response.Pagination{}, repoErr
	}

	var responseDTOs []responsedto.ProjectActivityResponse
	for _, item := range logs {
		dtoItem := responsedto.ProjectActivityResponse{
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
			dtoItem.User = &responsedto.UserSummary{
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

func (s *projectService) GetProjectDetails(req dto.GetProjectDetails) (*responsedto.ProjectDetail, *response.Error) {

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
		PaginationQuery: response.PaginationQuery{Page: 1, PageSize: 1000},
	}

	projectMembers, _, err := s.projectRepo.GetProjectsMembersByProjectID(req.ProjectID, memberFilter)
	if err != nil {
		return nil, err
	}

	sprintFilter := dto.SprintFilter{
		PaginationQuery: response.PaginationQuery{Page: 1, PageSize: 1000},
	}

	sprints, _, err := s.sprintRepo.GetSprints(req.ProjectID, sprintFilter)
	if err != nil {
		return nil, err
	}

	payload := responsedto.ProjectDetail{
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
	payload.Members = make([]responsedto.ProjectMember, 0, len(projectMembers))
	for _, member := range projectMembers {
		payload.Members = append(payload.Members, responsedto.ProjectMember{
			UserID:   member.UserID,
			Username: member.User.UserName,
			FullName: member.User.FullName,
			Role:     member.User.Role,
		})
	}

	// Map sprints
	payload.Sprints = make([]responsedto.Sprint, 0, len(sprints))
	for _, sprint := range sprints {
		payload.Sprints = append(payload.Sprints, responsedto.Sprint{
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

func (s *projectService) GetProjectsByUserID(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error) {

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

	projectMembers, err := s.projectRepo.GetProjectsByUserID(req.UserID)
	if err != nil {
		return nil, err
	}

	resp := &responsedto.GetProjectByUserIDResponse{
		UserID:  req.UserID,
		Project: make([]responsedto.ProjectResponse, 0, len(projectMembers)),
	}

	for _, member := range projectMembers {
		projectID := member.ProjectID

		resp.Project = append(resp.Project, responsedto.ProjectResponse{
			ProjectID: &projectID,
			Role:      member.ProjectRole,
		})
	}

	return resp, nil
}

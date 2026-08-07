package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
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
	CreateProject(req requestdto.CreateProjectRequest) *response.Error
	UpdateProject(req requestdto.UpdateProjectRequest) *response.Error
	GetProjectsByOrganizationID(organizationID uuid.UUID, filter requestdto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error)
	CreateProjectMemeber(req requestdto.CreateProjectMemberRequest) *response.Error
	GetProjectsMembersByProjectID(projectID uuid.UUID, filter requestdto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error)
	RemoveProjectMember(req requestdto.RemoveProjectMember) *response.Error
	GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, req requestdto.ProjectActivityFilterRequest) ([]responsedto.ProjectActivityResponse, response.Pagination, *response.Error)
	GetProjectDetails(req requestdto.GetProjectDetails) (*responsedto.ProjectDetail, *response.Error)
	DeleteProject(req requestdto.DeleteProject) *response.Error
	GetProjectsByUserID(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error)
	UpdateProjectMember(req requestdto.UpdateProjectMemberRequest) *response.Error
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

func (s *projectService) checkAuthorization(projectID, userID uuid.UUID) (bool, *response.Error) {

	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	if user.Role == string(requestdto.RoleSuperAdmin) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}
	if user.Role == string(requestdto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}
	isMember, err := s.projectRepo.IsUserProjectMember(projectID, userID)
	if err != nil {
		return false, err
	}
	return isMember, nil
}

func (s *projectService) CreateProject(req requestdto.CreateProjectRequest) *response.Error {

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
		Status:         string(requestdto.ProjectStatusPlanning),
		OrganizationID: req.OrganizationID,
		CreatedBy:      req.UserID,
	}

	projectMemberPayload := &models.ProjectMember{
		UserID:      req.UserID,
		AddedByID:   req.UserID,
		ProjectRole: string(requestdto.ProjectRoleOrgAdmin),
		JoinedAt:    time.Now(),
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

func (s *projectService) UpdateProject(req requestdto.UpdateProjectRequest) *response.Error {

	authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update project",
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

func (s *projectService) GetProjectsByOrganizationID(organizationID uuid.UUID, filterPayload requestdto.ProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error) {

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

	filter := requestdto.ProjectFilter{
		PaginationQuery: filterPayload.PaginationQuery,
		SortQuery:       filterPayload.SortQuery,
		Name:            filterPayload.Name,
		Status:          string(filterPayload.Status),
	}

	return s.projectRepo.GetProjectsByOrganizationID(organizationID, filter)
}

func (s *projectService) CreateProjectMemeber(req requestdto.CreateProjectMemberRequest) *response.Error {

	authorized, err := s.checkAuthorization(req.ProjectID, req.AddedByID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to add project members",
		}
	}

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.AddedByID, req.ProjectID)
	if err != nil {
		return err
	}

	if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
		member.ProjectRole != string(requestdto.ProjectRoleProjectManager) {

		s.logger.Error("Unauthorized project update attempt",
			zap.String("User ID", req.AddedByID.String()),
			zap.String("Project ID", req.ProjectID.String()),
			zap.String("Project Role", string(member.ProjectRole)))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update this project",
		}
	}

	var existingUsers []string

	for _, member := range req.Members {

		result, err := s.authRepo.GetUserByID(member.UserID)
		if err != nil {
			return err
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

func (s *projectService) GetProjectsMembersByProjectID(projectID uuid.UUID, filter requestdto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {

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
		Details:        fmt.Sprintf("User %s removed from project", req.TargetUserID.String()),
		CreatedAt:      time.Now(),
	}
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, filterReq requestdto.ProjectActivityFilterRequest) ([]responsedto.ProjectActivityResponse, response.Pagination, *response.Error) {

	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// Authorization check: org_admin of project's org, or project member
	isAuthorized := false

	if userRole == string(requestdto.RoleOrgAdmin) && userOrgID != uuid.Nil && userOrgID == project.OrganizationID {
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

	filter := requestdto.ProjectActivityFilter{
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

func (s *projectService) GetProjectDetails(req requestdto.GetProjectDetails) (*responsedto.ProjectDetail, *response.Error) {

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

	memberFilter := requestdto.ProjectMemberFilter{
		PaginationQuery: response.PaginationQuery{Page: 1, PageSize: 1000},
	}

	projectMembers, _, err := s.projectRepo.GetProjectsMembersByProjectID(req.ProjectID, memberFilter)
	if err != nil {
		return nil, err
	}

	sprintFilter := requestdto.SprintFilter{
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
			Role:     member.ProjectRole,
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

func (s *projectService) DeleteProject(req requestdto.DeleteProject) *response.Error {

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
		UserID:    req.UserID,
		FullName:  result.FullName,
		UserName:  result.UserName,
		AvatarURL: result.AvatarURL,
		Email:     result.Email,
		Role:      result.Role,
		Project:   make([]responsedto.ProjectResponse, 0, len(projectMembers)),
	}

	for _, member := range projectMembers {
		projectID := member.ProjectID

		resp.Project = append(resp.Project, responsedto.ProjectResponse{
			ProjectID:   projectID,
			Role:        member.ProjectRole,
			ProjectName: member.Project.Name,
			Status:      member.Project.Status,
		})
	}

	return resp, nil
}

func (s *projectService) UpdateProjectMember(req requestdto.UpdateProjectMemberRequest) *response.Error {

	authorized, err := s.checkAuthorization(req.ProjectID, req.UpdatedBy)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update project members",
		}
	}

	err = s.validateProjectMemberRoleUpdate(req.ProjectID, req.UpdatedBy, req.MemberID, req.ProjectRole)
	if err != nil {
		return err
	}

	updateErr := s.projectRepo.UpdateProjectMember(req.ProjectID, req.MemberID, string(req.ProjectRole))
	if updateErr != nil {
		return updateErr
	}

	auditLog := models.AuditLog{
		UserID:         &req.UpdatedBy,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "updated_project_member",
		ResourceType:   "project_member",
		ResourceID:     req.MemberID.String(),
		Details:        fmt.Sprintf("Changed user %s role  to %s", req.MemberID, req.ProjectRole),
		CreatedAt:      time.Now(),
	}
	err = s.projectRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) validateProjectMemberRoleUpdate(projectID, actorUserID, targetUserID uuid.UUID, newRole requestdto.ProjectRole) *response.Error {

	actor, err := s.projectRepo.GetProjectMemberByUserAndProjectID(actorUserID, projectID)
	if err != nil {
		return err
	}

	target, err := s.projectRepo.GetProjectMemberByUserAndProjectID(targetUserID, projectID)
	if err != nil {
		return err
	}

	// Prevent users from updating their own project role.
	if actorUserID == targetUserID {
		s.logger.Error("User attempted to update their own project role",
			zap.String("user_id", actorUserID.String()),
			zap.String("project_id", projectID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You cannot update your own project role",
		}
	}

	// Prevent Org Admin from modifying another Org Admin.
	if actor.ProjectRole == string(requestdto.ProjectRoleOrgAdmin) &&
		target.ProjectRole == string(requestdto.ProjectRoleOrgAdmin) {

		s.logger.Error("Org Admin cannot modify another Org Admin",
			zap.String("user_id", actorUserID.String()),
			zap.String("target_user_id", targetUserID.String()),
			zap.String("project_id", projectID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Org Admin cannot modify another Org Admin",
		}
	}

	// Project Manager restrictions.
	if actor.ProjectRole == string(requestdto.ProjectRoleProjectManager) {

		// Cannot modify Org Admin or another Project Manager.
		if target.ProjectRole == string(requestdto.ProjectRoleOrgAdmin) ||
			target.ProjectRole == string(requestdto.ProjectRoleProjectManager) {

			s.logger.Error("Project Manager cannot modify Org Admin or Project Manager",
				zap.String("user_id", actorUserID.String()),
				zap.String("target_user_id", targetUserID.String()),
				zap.String("project_id", projectID.String()))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Project Manager cannot modify Org Admin or Project Manager",
			}
		}

		// Cannot assign Org Admin or Project Manager role.
		if newRole == requestdto.ProjectRoleOrgAdmin ||
			newRole == requestdto.ProjectRoleProjectManager {

			s.logger.Error("Project Manager cannot assign Org Admin or Project Manager role",
				zap.String("user_id", actorUserID.String()),
				zap.String("project_id", projectID.String()))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Project Manager cannot assign Org Admin or Project Manager role",
			}
		}
	}

	return nil
}

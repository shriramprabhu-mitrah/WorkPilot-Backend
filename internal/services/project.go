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
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(req requestdto.CreateProjectRequest) (uuid.UUID, *response.Error)
	UpdateProject(req requestdto.UpdateProjectRequest) *response.Error
	GetProjectsByOrganizationID(filter requestdto.ProjectFilterRequest) ([]responsedto.ProjectSummary, response.Pagination, *response.Error)
	GetAllProjects(filterPayload requestdto.GlobalProjectFilterRequest) ([]responsedto.ProjectSummary, response.Pagination, *response.Error)
	CreateProjectMemeber(req requestdto.CreateProjectMemberRequest) *response.Error
	GetProjectsMembersByProjectID(projectID uuid.UUID, filter requestdto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error)
	RemoveProjectMember(req requestdto.RemoveProjectMember) *response.Error
	GetProjectActivity(userID uuid.UUID, userRole string, userOrgID uuid.UUID, projectID uuid.UUID, req requestdto.ProjectActivityFilterRequest) ([]responsedto.ProjectActivityResponse, response.Pagination, *response.Error)
	GetProjectDetails(req requestdto.GetProjectDetails) (*responsedto.ProjectDetail, *response.Error)
	DeleteProject(req requestdto.DeleteProject) *response.Error
	GetProjectsByUserID(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error)
	GetRecentProjects(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error)
	UpdateProjectMember(req requestdto.UpdateProjectMemberRequest) *response.Error
}

func InitProjectService(projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, sprintRepo sprintrepo.SprintRepository, taskRepo taskrepo.TaskRepository, auditRepo auditrepo.AuditLogRepository, logger *zap.Logger) ProjectService {
	return &projectService{
		authRepo:    authRepo,
		projectRepo: projectRepo,
		sprintRepo:  sprintRepo,
		taskRepo:    taskRepo,
		auditRepo:   auditRepo,
		logger:      logger,
	}
}

type projectService struct {
	authRepo    authrepo.AuthRepository
	projectRepo projectrepo.ProjectRepository
	sprintRepo  sprintrepo.SprintRepository
	auditRepo   auditrepo.AuditLogRepository
	taskRepo    taskrepo.TaskRepository
	logger      *zap.Logger
}

func (s *projectService) checkAuthorization(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {

	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}
	if user.Role.Name == string(requestdto.RoleSuperAdmin) {
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

func (s *projectService) CreateProject(req requestdto.CreateProjectRequest) (uuid.UUID, *response.Error) {

	result, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return uuid.Nil, err
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
		return uuid.Nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return uuid.Nil, &response.Error{
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
		UserID:    req.UserID,
		AddedByID: req.UserID,
		RoleID:    result.RoleID,
		JoinedAt:  time.Now(),
	}

	err = s.projectRepo.CreateProjectWithMember(projectPayload, projectMemberPayload)
	if err != nil {
		return uuid.Nil, err
	}

	// get the project details
	project, err := s.projectRepo.GetProjectByID(projectPayload.ID)
	if err != nil {
		return uuid.Nil, err
	}

	// Log project creation audit event
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &project.ID,
		Action:         "created",
		ResourceType:   "project",
		ResourceID:     project.ID.String(),
		Title:          project.Name,
		Details:        fmt.Sprintf("The project '%s' was created by %s", req.Name, result.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	// if error occurred just warn it do not return error
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return project.ID, nil
}

func (s *projectService) UpdateProject(req requestdto.UpdateProjectRequest) *response.Error {

	existingProject, user, _, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return err
	}
	hasModify, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "projects", "modify")
	if permErr != nil {
		return permErr
	}
	if !hasModify {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update this project",
		}
	}

	changedBy := req.UserID.String()
	if user.UserName != "" {
		changedBy = user.UserName
	} else if user.FullName != "" {
		changedBy = user.FullName
	} else if user.Email != "" {
		changedBy = user.Email
	}

	updates := make(map[string]interface{})
	var changes []string
	if req.Name != nil {
		newName := *req.Name
		updates["name"] = newName
		if newName != existingProject.Name {
			changes = append(changes, fmt.Sprintf("name changed from '%s' to '%s'", existingProject.Name, newName))
		}
	}
	if req.Description != nil {
		newDesc := *req.Description
		updates["description"] = newDesc
		if newDesc != existingProject.Description {
			changes = append(changes, fmt.Sprintf("description changed from '%s' to '%s'", existingProject.Description, newDesc))
		}
	}
	if req.Status != nil {
		if err := req.Status.Validate(); err != nil {
			s.logger.Error("Invalid project status", zap.Error(err))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid status. Allowed values: active, archived, on_hold, completed, cancelled, planning",
			}
		}
		newStatus := string(*req.Status)
		updates["status"] = newStatus
		if newStatus != existingProject.Status {
			changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", existingProject.Status, newStatus))
		}
	}

	if len(updates) == 0 {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "No changes to update",
		}
	}

	updateErr := s.projectRepo.UpdateProject(req.ProjectID, updates)
	if updateErr != nil {
		return updateErr
	}

	var detail string
	if len(changes) > 0 {
		detail = fmt.Sprintf("Project updated by %s: %s", changedBy, strings.Join(changes, ", "))
	} else {
		detail = fmt.Sprintf("Project details updated by %s", changedBy)
	}

	// Log project update audit event
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "updated",
		ResourceType:   "project",
		ResourceID:     req.ProjectID.String(),
		Title:          existingProject.Name,
		Details:        detail,
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) GetProjectsByOrganizationID(filterPayload requestdto.ProjectFilterRequest) ([]responsedto.ProjectSummary, response.Pagination, *response.Error) {
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
		IncludeSprints:  filterPayload.IncludeSprints,
		UserID:          filterPayload.UserID,
		UserRole:        filterPayload.UserRole,
	}

	projects, pagination, err := s.projectRepo.GetProjectsByOrganizationID(filterPayload.OrganizationID, filter)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	projectSummaries := make([]responsedto.ProjectSummary, 0, len(projects))
	if len(projects) > 0 {
		projectIDs := make([]uuid.UUID, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}

		sprintsByProjectID, _ := s.sprintRepo.GetSprintsByProjectIDs(projectIDs)
		sprintCounts, _ := s.sprintRepo.GetSprintCountByProjectIDs(projectIDs)
		taskCounts, _ := s.taskRepo.GetTaskCountsByProjectIDs(projectIDs)
		memberCounts, _ := s.projectRepo.GetMemberCountsByProjectIDs(projectIDs)

		for _, p := range projects {
			p.Sprints = sprintsByProjectID[p.ID]
			if p.Sprints == nil {
				p.Sprints = []models.Sprint{}
			}
			p.SprintCount = sprintCounts[p.ID]
			summary := responsedto.ProjectSummaryFromModel(p, int(taskCounts[p.ID]), int(memberCounts[p.ID]))
			projectSummaries = append(projectSummaries, summary)
		}
	}

	auditLog := models.AuditLog{
		UserID:         &filterPayload.UserID,
		OrganizationID: &filterPayload.OrganizationID,
		Action:         "viewed",
		ResourceType:   "project",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return projectSummaries, pagination, nil
}

func (s *projectService) GetAllProjects(filterPayload requestdto.GlobalProjectFilterRequest) ([]responsedto.ProjectSummary, response.Pagination, *response.Error) {
	if filterPayload.Status != "" {
		if err := filterPayload.Status.Validate(); err != nil {
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid status. Allowed values: active, archived, on_hold, completed, cancelled, planning",
			}
		}
	}

	filterPayload.PaginationQuery.Normalize(10)
	filterPayload.SortQuery.Normalize("created_at", "DESC")

	projects, pagination, err := s.projectRepo.GetAllProjects(filterPayload)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	projectSummaries := make([]responsedto.ProjectSummary, 0, len(projects))
	if len(projects) > 0 {
		projectIDs := make([]uuid.UUID, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}

		sprintsByProjectID, _ := s.sprintRepo.GetSprintsByProjectIDs(projectIDs)
		sprintCounts, _ := s.sprintRepo.GetSprintCountByProjectIDs(projectIDs)
		taskCounts, _ := s.taskRepo.GetTaskCountsByProjectIDs(projectIDs)
		memberCounts, _ := s.projectRepo.GetMemberCountsByProjectIDs(projectIDs)

		for _, p := range projects {
			p.Sprints = sprintsByProjectID[p.ID]
			if p.Sprints == nil {
				p.Sprints = []models.Sprint{}
			}
			p.SprintCount = sprintCounts[p.ID]
			summary := responsedto.ProjectSummaryFromModel(p, int(taskCounts[p.ID]), int(memberCounts[p.ID]))
			projectSummaries = append(projectSummaries, summary)
		}
	}

	auditLog := models.AuditLog{
		Action:       "viewed",
		ResourceType: "all_projects",
		Type:         models.AuditLogTypeAudit,
		CreatedAt:    time.Now(),
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return projectSummaries, pagination, nil
}

func (s *projectService) CreateProjectMemeber(req requestdto.CreateProjectMemberRequest) *response.Error {

	project, addedBy, authorized, err := s.checkAuthorization(req.ProjectID, req.AddedByID)
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

	hasModify, permErr := CheckPermission(s.authRepo, s.projectRepo, req.AddedByID, req.ProjectID, "projects", "modify")
	if permErr != nil {
		return permErr
	}
	if !hasModify {
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

		var roleID uuid.UUID
		if member.RoleID != nil && *member.RoleID != uuid.Nil {
			role, roleErr := s.authRepo.GetRoleByID(*member.RoleID)
			if roleErr != nil {
				return roleErr
			}
			if role.OrganizationID != nil && *role.OrganizationID != project.OrganizationID {
				return &response.Error{
					Code:       response.ErrForbidden,
					StatusCode: http.StatusForbidden,
					Message:    "The specified role does not belong to your organization",
				}
			}
			roleID = *member.RoleID
		} else {
			roleName := "developer"
			switch member.ProjectRole {
			case "org_admin":
				roleName = "org_admin"
			case "project_manager":
				roleName = "project_manager"
			case "developer":
				roleName = "developer"
			case "tester", "qa":
				roleName = "qa"
			case "viewer", "stakeholder":
				roleName = "stakeholder"
			}
			role, roleErr := s.authRepo.GetRoleByNameAndOrg(roleName, project.OrganizationID)
			if roleErr != nil {
				return roleErr
			}
			roleID = role.ID
		}

		projectMember := models.ProjectMember{
			ProjectID: req.ProjectID,
			UserID:    member.UserID,
			RoleID:    roleID,
			AddedByID: req.AddedByID,
			JoinedAt:  time.Now(),
		}

		if err := s.projectRepo.CreateProjectMember(&projectMember); err != nil {
			return err
		}

		auditLog := models.AuditLog{
			UserID:         &req.AddedByID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			Action:         "added",
			ResourceType:   "project_member",
			ResourceID:     member.UserID.String(),
			Details:        fmt.Sprintf("User %s added to project by %s", result.UserName, addedBy.UserName),
			CreatedAt:      time.Now(),
			Type:           models.AuditLogTypeActivity,
		}

		if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
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

	members, pagination, err := s.projectRepo.GetProjectsMembersByProjectID(projectID, filter)

	auditLog := models.AuditLog{
		UserID:         &filter.UserID,
		OrganizationID: &filter.OrganizationID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "project_member",
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeAudit,
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}
	return members, pagination, nil
}

func (s *projectService) RemoveProjectMember(req requestdto.RemoveProjectMember) *response.Error {

	hasModify, permErr := CheckPermission(s.authRepo, s.projectRepo, req.PerformingUserID, req.ProjectID, "projects", "modify")
	if permErr != nil {
		return permErr
	}
	if !hasModify {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to remove project members",
		}
	}

	performingUser, err := s.authRepo.GetUserByID(req.PerformingUserID)
	if err != nil {
		return err
	}

	targetUser, err := s.authRepo.GetUserByID(req.TargetUserID)
	if err != nil {
		return err
	}

	performingUserName := performingUser.UserName
	if performingUserName == "" {
		performingUserName = performingUser.FullName
	}
	if performingUserName == "" {
		performingUserName = req.PerformingUserID.String()
	}

	targetUserName := targetUser.UserName
	if targetUserName == "" {
		targetUserName = targetUser.FullName
	}
	if targetUserName == "" {
		targetUserName = req.TargetUserID.String()
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
		Action:         "removed",
		ResourceType:   "project_member",
		ResourceID:     req.TargetUserID.String(),
		Details:        fmt.Sprintf("User %s removed from project by %s", targetUserName, performingUserName),
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
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

	var parsedTaskID *uuid.UUID
	if filterReq.TaskID != "" {
		tid, parseErr := uuid.FromString(filterReq.TaskID)
		if parseErr == nil && tid != uuid.Nil {
			parsedTaskID = &tid
		}
	}

	var parsedUserStoryID *uuid.UUID
	if filterReq.UserStoryID != "" {
		sid, parseErr := uuid.FromString(filterReq.UserStoryID)
		if parseErr == nil && sid != uuid.Nil {
			parsedUserStoryID = &sid
		}
	}

	var parsedSprintID *uuid.UUID
	if filterReq.SprintID != "" {
		spid, parseErr := uuid.FromString(filterReq.SprintID)
		if parseErr == nil && spid != uuid.Nil {
			parsedSprintID = &spid
		}
	}

	resType := filterReq.ResourceType

	filter := requestdto.ProjectActivityFilter{
		PaginationQuery: filterReq.PaginationQuery,
		Action:          filterReq.Action,
		UserID:          parsedUserID,
		ResourceType:    resType,
		ResourceID:      filterReq.ResourceID,
		TaskID:          parsedTaskID,
		UserStoryID:     parsedUserStoryID,
		SprintID:        parsedSprintID,
		Type:            filterReq.Type,
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
			ProjectName:    item.ProjectName,
			OrganizationID: item.OrganizationID,
			Action:         item.Action,
			ResourceType:   item.ResourceType,
			ResourceID:     item.ResourceID,
			Details:        item.Details,
			CreatedAt:      item.CreatedAt.Format(time.RFC3339),
			Title:          item.Title,
			TaskName:       item.TaskName,
			UserStoryName:  item.UserStoryName,
			SprintName:     item.SprintName,
			TaskKey:        item.TaskKey,
		}

		if item.User.ID != uuid.Nil {
			avatarURL := &item.User.AvatarURL
			dtoItem.User = &responsedto.UserSummary{
				ID:        item.User.ID,
				FullName:  item.User.FullName,
				Email:     item.User.Email,
				AvatarURL: avatarURL,
				Color:     item.User.Color,
				Role:      item.User.Role.Name,
			}
		}

		if strings.ToLower(item.ResourceType) == "task" {
			dtoItem.TaskKey = item.TaskKey
		}

		responseDTOs = append(responseDTOs, dtoItem)
	}
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &userOrgID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "project",
		ResourceID:     projectID.String(),
		Details:        "Activity of the project viewed",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
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
		var avatarURL *string
		if member.User.AvatarURL != "" {
			avatarURL = &member.User.AvatarURL
		}
		payload.Members = append(payload.Members, responsedto.ProjectMember{
			UserID:    member.UserID,
			Username:  member.User.UserName,
			FullName:  member.User.FullName,
			Role:      member.Role.Name,
			AvatarURL: avatarURL,
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

	taskFilter := requestdto.TaskFilter{
		PaginationQuery: response.PaginationQuery{Page: 1, PageSize: 10000},
	}

	tasks, _, err := s.taskRepo.GetTasks(req.ProjectID, taskFilter)
	if err != nil {
		return nil, err
	}

	totalTasks := len(tasks)
	completedTasks := 0
	overdueTasks := 0
	now := time.Now()

	for _, task := range tasks {
		if task.Status == string(requestdto.TaskStatusCompleted) {
			completedTasks++
		}
		if task.Status != string(requestdto.TaskStatusCompleted) &&
			task.DueDate != nil && task.DueDate.Before(now) {
			overdueTasks++
		}
	}

	pendingTasks := totalTasks - completedTasks
	completedPercentage := 0
	if totalTasks > 0 {
		completedPercentage = int(float64(completedTasks) / float64(totalTasks) * 100)
	}

	totalMembers := len(projectMembers)
	totalSprints := len(sprints)
	activeSprints := 0
	completedSprints := 0
	for _, sprint := range sprints {
		switch sprint.Status {
		case string(requestdto.SprintStatusActive):
			activeSprints++
		case string(requestdto.SprintStatusCompleted):
			completedSprints++
		}
	}

	payload.Metrics = responsedto.ProjectMetrics{
		TotalTasks:               totalTasks,
		CompletedTasks:           completedTasks,
		PendingTasks:             pendingTasks,
		OverdueTasks:             overdueTasks,
		CompletedTasksPercentage: completedPercentage,
		TotalSprints:             totalSprints,
		ActiveSprints:            activeSprints,
		CompletedSprints:         completedSprints,
		TotalMembers:             totalMembers,
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "viewed",
		ResourceType:   "project",
		ResourceID:     req.ProjectID.String(),
		Details:        "Project details viewed",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &payload, nil
}

func (s *projectService) DeleteProject(req requestdto.DeleteProject) *response.Error {

	existingProject, user, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete project",
		}
	}

	if req.ProjectID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
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
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", user.OrganizationID.String()),
			zap.String("User ID", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	err = s.projectRepo.DeleteProject(req.ProjectID, req.OrganizationID)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "deleted",
		ResourceType:   "project",
		ResourceID:     req.ProjectID.String(),
		Details:        fmt.Sprintf("Project %s deleted by %s", existingProject.Name, user.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil

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

	var avatarURL *string
	if result.AvatarURL != "" {
		avatarURL = &result.AvatarURL
	}

	resp := &responsedto.GetProjectByUserIDResponse{
		UserID:    req.UserID,
		FullName:  result.FullName,
		UserName:  result.UserName,
		AvatarURL: avatarURL,
		Color:     result.Color,
		Email:     result.Email,
		Role:      result.Role.Name,
		Project:   make([]responsedto.ProjectResponse, 0, len(projectMembers)),
	}

	for _, member := range projectMembers {
		projectID := member.ProjectID

		resp.Project = append(resp.Project, responsedto.ProjectResponse{
			ProjectID:   projectID,
			Role:        member.Role.Name,
			ProjectName: member.Project.Name,
			Status:      member.Project.Status,
		})
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		Action:         "viewed",
		ResourceType:   "project",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return resp, nil
}

func (s *projectService) GetRecentProjects(req requestdto.GetProjectByUserID) (*responsedto.GetProjectByUserIDResponse, *response.Error) {

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

	projectIDs := make([]uuid.UUID, 0, len(projectMembers))
	for _, member := range projectMembers {
		projectIDs = append(projectIDs, member.ProjectID)
	}

	taskCounts, taskErr := s.taskRepo.GetTaskCountsByProjectIDs(projectIDs)
	if taskErr != nil {
		return nil, taskErr
	}

	var avatarURL *string
	if result.AvatarURL != "" {
		avatarURL = &result.AvatarURL
	}

	resp := &responsedto.GetProjectByUserIDResponse{
		UserID:    req.UserID,
		FullName:  result.FullName,
		UserName:  result.UserName,
		AvatarURL: avatarURL,
		Color:     result.Color,
		Email:     result.Email,
		Role:      result.Role.Name,
		Project:   make([]responsedto.ProjectResponse, 0, len(projectMembers)),
	}

	for _, member := range projectMembers {
		projectID := member.ProjectID

		if count, exists := taskCounts[projectID]; exists && count > 0 {
			resp.Project = append(resp.Project, responsedto.ProjectResponse{
				ProjectID:   projectID,
				Role:        member.Role.Name,
				ProjectName: member.Project.Name,
				Status:      member.Project.Status,
			})
		}
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		Action:         "viewed",
		ResourceType:   "project",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return resp, nil
}

func (s *projectService) UpdateProjectMember(req requestdto.UpdateProjectMemberRequest) *response.Error {

	project, updater, authorized, err := s.checkAuthorization(req.ProjectID, req.UpdatedBy)
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

	var targetRoleID uuid.UUID
	if req.RoleID != nil && *req.RoleID != uuid.Nil {
		role, roleErr := s.authRepo.GetRoleByID(*req.RoleID)
		if roleErr != nil {
			return roleErr
		}
		if role.OrganizationID != nil && *role.OrganizationID != project.OrganizationID {
			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "The specified role does not belong to your organization",
			}
		}
		targetRoleID = *req.RoleID
	} else {
		roleName := "developer"
		switch req.ProjectRole {
		case "org_admin":
			roleName = "org_admin"
		case "project_manager":
			roleName = "project_manager"
		case "developer":
			roleName = "developer"
		case "tester", "qa":
			roleName = "qa"
		case "viewer", "stakeholder":
			roleName = "stakeholder"
		default:
			return &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid project role",
			}
		}
		role, roleErr := s.authRepo.GetRoleByNameAndOrg(roleName, project.OrganizationID)
		if roleErr != nil {
			return roleErr
		}
		targetRoleID = role.ID
	}

	err = s.validateProjectMemberRoleUpdate(req.ProjectID, req.UpdatedBy, req.MemberID, targetRoleID)
	if err != nil {
		return err
	}

	existingMember, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.MemberID, req.ProjectID)
	if err != nil {
		return err
	}

	oldRole := existingMember.Role.Name
	if oldRole == "" {
		oldRole = "unknown"
	}

	targetUser, targetUserErr := s.authRepo.GetUserByID(req.MemberID)
	targetUserName := req.MemberID.String()
	if targetUserErr == nil {
		if targetUser.UserName != "" {
			targetUserName = targetUser.UserName
		} else if targetUser.FullName != "" {
			targetUserName = targetUser.FullName
		}
	}

	updaterName := updater.UserName
	if updaterName == "" {
		updaterName = updater.FullName
	}
	if updaterName == "" {
		updaterName = req.UpdatedBy.String()
	}

	updateErr := s.projectRepo.UpdateProjectMember(req.ProjectID, req.MemberID, targetRoleID)
	if updateErr != nil {
		return updateErr
	}

	// Fetch target role details to log name
	var newRole string = targetRoleID.String()
	if updatedMem, updatedMemErr := s.projectRepo.GetProjectMemberByUserAndProjectID(req.MemberID, req.ProjectID); updatedMemErr == nil {
		newRole = updatedMem.Role.Name
	}

	auditLog := models.AuditLog{
		UserID:         &req.UpdatedBy,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "updated",
		ResourceType:   "project_member",
		ResourceID:     req.MemberID.String(),
		Details:        fmt.Sprintf("User %s role changed from %s to %s by %s", targetUserName, oldRole, newRole, updaterName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *projectService) validateProjectMemberRoleUpdate(projectID, actorUserID, targetUserID, targetRoleID uuid.UUID) *response.Error {

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
	if actor.Role.Name == "org_admin" &&
		target.Role.Name == "org_admin" {

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
	if actor.Role.Name == "project_manager" {

		// Cannot modify Org Admin or another Project Manager.
		if target.Role.Name == "org_admin" ||
			target.Role.Name == "project_manager" {

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
		targetRole, roleErr := s.authRepo.GetRoleByID(targetRoleID)
		if roleErr == nil && (targetRole.Name == "org_admin" || targetRole.Name == "project_manager") {

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

package services

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	"go.uber.org/zap"
)

type UserStoryService interface {
	CreateUserStory(req dto.CreateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error)
	GetUserStoryByID(userStoryID, projectID, userID, orgID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error)
	UpdateUserStory(req dto.UpdateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error)
	DeleteUserStory(userStoryID, projectID, userID, orgID uuid.UUID) *response.Error
	GetUserStories(projectID, userID, orgID uuid.UUID, filter dto.UserStoryFilter) ([]responsedto.UserStoryResponse, response.Pagination, *response.Error)
	ReorderUserStories(projectID, userID, orgID uuid.UUID, storyIDs []uuid.UUID) *response.Error
}

type userStoryService struct {
	authRepo         authrepo.AuthRepository
	projectRepo      projectrepo.ProjectRepository
	userStoryRepo    userstoryrepo.UserStoryRepository
	taskRepo         taskrepo.TaskRepository
	customStatusRepo customstatusrepo.CustomStatusRepository
	logger           *zap.Logger
}

func InitUserStoryService(authRepo authrepo.AuthRepository, projectRepo projectrepo.ProjectRepository, userStoryRepo userstoryrepo.UserStoryRepository, taskRepo taskrepo.TaskRepository, customStatusRepo customstatusrepo.CustomStatusRepository, logger *zap.Logger) UserStoryService {
	return &userStoryService{
		authRepo:         authRepo,
		projectRepo:      projectRepo,
		userStoryRepo:    userStoryRepo,
		taskRepo:         taskRepo,
		customStatusRepo: customStatusRepo,
		logger:           logger,
	}
}

func (s *userStoryService) getStatusColorMap(projectID uuid.UUID) map[string]string {
	colorMap := make(map[string]string)
	for k, v := range models.DefaultStatusColors {
		colorMap[k] = v
	}
	if s.customStatusRepo != nil {
		customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
		if err == nil {
			for _, cs := range customStatuses {
				colorMap[models.NormalizeTaskStatus(cs.Name)] = cs.Color
			}
		}
	}
	return colorMap
}

func (s *userStoryService) resolveStatusIDAndName(projectID uuid.UUID, statusID *uuid.UUID, statusName *string) (uuid.UUID, string, *response.Error) {
	if statusID != nil && *statusID != uuid.Nil {
		if s.customStatusRepo == nil {
			return uuid.Nil, "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Custom status repository not initialized",
			}
		}
		cs, err := s.customStatusRepo.GetStatusByID(*statusID, projectID)
		if err != nil {
			if err.StatusCode == http.StatusNotFound {
				return uuid.Nil, "", &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusUnprocessableEntity,
					Message:    "Invalid user story status_id: status does not exist or does not belong to this project",
				}
			}
			return uuid.Nil, "", err
		}
		return cs.ID, cs.Name, nil
	}

	if statusName != nil && *statusName != "" {
		if s.customStatusRepo == nil {
			return uuid.Nil, "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Custom status repository not initialized",
			}
		}
		normalized := models.NormalizeTaskStatus(*statusName)
		cs, err := s.customStatusRepo.GetStatusByName(projectID, normalized)
		if err != nil {
			if err.StatusCode == http.StatusNotFound {
				return uuid.Nil, "", &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusUnprocessableEntity,
					Message:    "Invalid user story status value: status name not found in this project",
				}
			}
			return uuid.Nil, "", err
		}
		return cs.ID, cs.Name, nil
	}

	if s.customStatusRepo == nil {
		return uuid.Nil, "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Custom status repository not initialized",
		}
	}
	customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
	if err != nil {
		return uuid.Nil, "", err
	}

	var defaultStatus *models.CustomStatus
	for i := range customStatuses {
		if customStatuses[i].IsDefault {
			if defaultStatus == nil || customStatuses[i].DisplayOrder < defaultStatus.DisplayOrder {
				defaultStatus = &customStatuses[i]
			}
		}
	}

	if defaultStatus == nil && len(customStatuses) > 0 {
		sort.Slice(customStatuses, func(i, j int) bool {
			return customStatuses[i].DisplayOrder < customStatuses[j].DisplayOrder
		})
		defaultStatus = &customStatuses[0]
	}

	if defaultStatus == nil {
		return uuid.Nil, "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Project has no defined statuses",
		}
	}

	return defaultStatus.ID, defaultStatus.Name, nil
}

func (s *userStoryService) checkAuthorization(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
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
		return models.Project{}, models.User{}, false, err
	}
	return project, user, isMember, nil
}

func (s *userStoryService) CreateUserStory(req dto.CreateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(req.ProjectID, req.ReporterID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to create user stories in this project",
		}
	}

	// Validation: Title length
	if len([]rune(req.Title)) < 3 || len([]rune(req.Title)) > 250 {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "User story title must be between 3 and 250 characters",
		}
	}

	// Validation: Assignee must be active and a member of the project
	if req.AssigneeID != nil && *req.AssigneeID != uuid.Nil {
		assigneeUser, err := s.authRepo.GetUserByID(*req.AssigneeID)
		if err != nil {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee user not found",
			}
		}
		if !assigneeUser.IsActive {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be an active user",
			}
		}
		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, *req.AssigneeID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			if assigneeUser.Role == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
				isMember = true
			}
		}
		if !isMember {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be a member of the project",
			}
		}
	}

	// Validation: Sprint must belong to the same project
	if req.SprintID != nil && *req.SprintID != uuid.Nil {
		inProject, err := s.userStoryRepo.IsSprintInProject(*req.SprintID, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if !inProject {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Sprint must belong to the same project",
			}
		}
	}

	maxOrder, orderErr := s.userStoryRepo.GetMaxBacklogOrder(req.ProjectID)
	if orderErr != nil {
		return nil, orderErr
	}

	var story models.UserStory
	story.ProjectID = req.ProjectID
	story.SprintID = req.SprintID
	story.Title = req.Title
	story.Description = utils.SanitizeHTML(req.Description)
	story.Priority = req.Priority
	var statusNameArg *string
	if req.Status != "" {
		statusNameArg = &req.Status
	}
	resolvedStatusID, resolvedStatusName, valErr := s.resolveStatusIDAndName(req.ProjectID, req.StatusID, statusNameArg)
	if valErr != nil {
		return nil, valErr
	}
	story.StatusID = resolvedStatusID
	story.Status = resolvedStatusName
	story.StoryPoints = req.StoryPoints
	story.BacklogOrder = maxOrder + 1
	story.AssigneeID = req.AssigneeID
	story.ReporterID = req.ReporterID

	createErr := s.userStoryRepo.CreateUserStory(&story)
	if createErr != nil {
		return nil, createErr
	}

	// Fetch created story with preloaded fields
	createdStory, getErr := s.userStoryRepo.GetUserStoryByID(story.ID, req.ProjectID)
	if getErr != nil {
		return nil, getErr
	}

	var customStatuses []models.CustomStatus
	if s.customStatusRepo != nil {
		customStatuses, _ = s.customStatusRepo.GetStatusesByProjectID(req.ProjectID)
	}
	res := mapToUserStoryResponse(*createdStory, customStatuses, 0, 0, 0.0)
	return &res, nil
}

func (s *userStoryService) GetUserStoryByID(userStoryID, projectID, userID, orgID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view user stories in this project",
		}
	}

	story, err := s.userStoryRepo.GetUserStoryByID(userStoryID, projectID)
	if err != nil {
		return nil, err
	}

	statsMap, statErr := s.userStoryRepo.GetStoryTaskStats(projectID)
	if statErr != nil {
		return nil, statErr
	}
	var total, completed int64
	progress := 0.0
	if stat, ok := statsMap[userStoryID]; ok {
		total = stat.TotalTasks
		completed = stat.Completed
		if total > 0 {
			progress = (float64(completed) / float64(total)) * 100.0
		}
	}

	tasks, taskErr := s.taskRepo.GetTasksByUserStoryID(userStoryID)
	if taskErr != nil {
		return nil, taskErr
	}

	colorMap := s.getStatusColorMap(projectID)
	taskResponses := make([]responsedto.TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		taskResponses = append(taskResponses, mapToTaskResponse(t, colorMap))
	}

	var customStatuses []models.CustomStatus
	if s.customStatusRepo != nil {
		customStatuses, _ = s.customStatusRepo.GetStatusesByProjectID(projectID)
	}
	res := mapToUserStoryResponse(*story, customStatuses, total, completed, progress)
	res.Tasks = taskResponses
	return &res, nil
}

func (s *userStoryService) UpdateUserStory(req dto.UpdateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error) {
	project, user, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update user stories in this project",
		}
	}

	isPMOrAdmin := (user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID)
	if !isPMOrAdmin {
		member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager) {
			isPMOrAdmin = true
		} else if member.ProjectRole == string(dto.ProjectRoleViewer) {
			return nil, &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Viewers do not have permission to update user stories",
			}
		}
	}

	// Fetch existing user story
	_, err = s.userStoryRepo.GetUserStoryByID(req.UserStoryID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	// Title length validation
	if req.Title != nil {
		if len([]rune(*req.Title)) < 3 || len([]rune(*req.Title)) > 250 {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "User story title must be between 3 and 250 characters",
			}
		}
		updates["title"] = *req.Title
	}

	if req.Description != nil {
		sanitizedDescription := strings.TrimSpace(utils.SanitizeHTML(*req.Description))
		updates["description"] = sanitizedDescription
	}

	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}

	if req.StatusID != nil || req.Status != nil {
		var statusNameArg *string
		if req.Status != nil && *req.Status != "" {
			statusNameArg = req.Status
		}
		resolvedStatusID, resolvedStatusName, valErr := s.resolveStatusIDAndName(req.ProjectID, req.StatusID, statusNameArg)
		if valErr != nil {
			return nil, valErr
		}
		updates["status_id"] = resolvedStatusID
		updates["status"] = resolvedStatusName
	}

	if req.StoryPoints != nil {
		updates["story_points"] = *req.StoryPoints
	}

	// Assignee check
	if req.IsAssigneeIDNull() || (req.AssigneeID != nil && *req.AssigneeID == uuid.Nil) {
		updates["assignee_id"] = nil
	} else if req.AssigneeID != nil {
		assigneeUser, err := s.authRepo.GetUserByID(*req.AssigneeID)
		if err != nil {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee user not found",
			}
		}
		if !assigneeUser.IsActive {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be an active user",
			}
		}
		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, *req.AssigneeID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			if assigneeUser.Role == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
				isMember = true
			}
		}
		if !isMember {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be a member of the project",
			}
		}
		updates["assignee_id"] = *req.AssigneeID
	}

	// Sprint check
	if req.IsSprintIDNull() || (req.SprintID != nil && *req.SprintID == uuid.Nil) {
		updates["sprint_id"] = nil
	} else if req.SprintID != nil {
		inProject, err := s.userStoryRepo.IsSprintInProject(*req.SprintID, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if !inProject {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Sprint must belong to the same project",
			}
		}
		updates["sprint_id"] = *req.SprintID
	}

	updateErr := s.userStoryRepo.UpdateUserStory(req.UserStoryID, updates)
	if updateErr != nil {
		return nil, updateErr
	}

	// Fetch updated story
	updatedStory, getErr := s.userStoryRepo.GetUserStoryByID(req.UserStoryID, req.ProjectID)
	if getErr != nil {
		return nil, getErr
	}

	statsMap, statErr := s.userStoryRepo.GetStoryTaskStats(req.ProjectID)
	if statErr != nil {
		return nil, statErr
	}
	var total, completed int64
	progress := 0.0
	if stat, ok := statsMap[req.UserStoryID]; ok {
		total = stat.TotalTasks
		completed = stat.Completed
		if total > 0 {
			progress = (float64(completed) / float64(total)) * 100.0
		}
	}

	var customStatuses []models.CustomStatus
	if s.customStatusRepo != nil {
		customStatuses, _ = s.customStatusRepo.GetStatusesByProjectID(req.ProjectID)
	}
	res := mapToUserStoryResponse(*updatedStory, customStatuses, total, completed, progress)
	return &res, nil
}

func (s *userStoryService) DeleteUserStory(userStoryID, projectID, userID, orgID uuid.UUID) *response.Error {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete user stories in this project",
		}
	}

	// Fetch story first to verify existence
	_, getErr := s.userStoryRepo.GetUserStoryByID(userStoryID, projectID)
	if getErr != nil {
		return getErr
	}

	return s.userStoryRepo.DeleteUserStory(userStoryID, projectID)
}

func (s *userStoryService) GetUserStories(projectID, userID, orgID uuid.UUID, filter dto.UserStoryFilter) ([]responsedto.UserStoryResponse, response.Pagination, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return nil, response.Pagination{}, err
	}
	if !authorized {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view user stories in this project",
		}
	}

	stories, pagination, getErr := s.userStoryRepo.GetUserStories(projectID, filter)
	if getErr != nil {
		return nil, response.Pagination{}, getErr
	}

	statsMap, statErr := s.userStoryRepo.GetStoryTaskStats(projectID)
	if statErr != nil {
		return nil, response.Pagination{}, statErr
	}

	colorMap := s.getStatusColorMap(projectID)
	var customStatuses []models.CustomStatus
	if s.customStatusRepo != nil {
		customStatuses, _ = s.customStatusRepo.GetStatusesByProjectID(projectID)
	}
	resList := []responsedto.UserStoryResponse{}
	for _, story := range stories {
		var total, completed int64
		progress := 0.0
		if stat, ok := statsMap[story.ID]; ok {
			total = stat.TotalTasks
			completed = stat.Completed
			if total > 0 {
				progress = (float64(completed) / float64(total)) * 100.0
			}
		}
		tasks, taskErr := s.taskRepo.GetTasksByUserStoryID(story.ID)
		if taskErr != nil {
			return nil, response.Pagination{}, taskErr
		}

		taskResponses := make([]responsedto.TaskResponse, 0, len(tasks))
		for _, t := range tasks {
			taskResponses = append(taskResponses, mapToTaskResponse(t, colorMap))
		}

		storyRes := mapToUserStoryResponse(story, customStatuses, total, completed, progress)
		storyRes.Tasks = taskResponses
		resList = append(resList, storyRes)
	}

	return resList, pagination, nil
}

func mapToUserStoryResponse(story models.UserStory, customStatuses []models.CustomStatus, totalTasks, completedTasks int64, progress float64) responsedto.UserStoryResponse {
	var sprintName string
	if story.Sprint != nil {
		sprintName = story.Sprint.Name
	}
	var assigneeName string
	if story.Assignee != nil {
		assigneeName = story.Assignee.FullName
	}

	resolvedStatusID := story.StatusID
	resolvedStatusColor := ""
	normStatusName := models.NormalizeTaskStatus(story.Status)

	var found bool
	if story.StatusID != uuid.Nil {
		for _, cs := range customStatuses {
			if cs.ID == story.StatusID {
				resolvedStatusColor = cs.Color
				found = true
				break
			}
		}
	}

	if !found {
		for _, cs := range customStatuses {
			if models.NormalizeTaskStatus(cs.Name) == normStatusName {
				resolvedStatusID = cs.ID
				resolvedStatusColor = cs.Color
				found = true
				break
			}
		}
	}

	if resolvedStatusColor == "" {
		resolvedStatusColor = models.DefaultStatusColors[normStatusName]
		if resolvedStatusColor == "" {
			resolvedStatusColor = "#808080"
		}
	}

	res := responsedto.UserStoryResponse{
		ID:             story.ID,
		ProjectID:      story.ProjectID,
		SprintID:       story.SprintID,
		SprintName:     sprintName,
		Title:          story.Title,
		Description:    story.Description,
		Priority:       story.Priority,
		StatusID:       resolvedStatusID,
		Status:         story.Status,
		StatusColor:    resolvedStatusColor,
		StoryPoints:    story.StoryPoints,
		AssigneeID:     story.AssigneeID,
		AssigneeName:   assigneeName,
		ReporterID:     story.ReporterID,
		ReporterName:   story.Reporter.FullName,
		BacklogOrder:   story.BacklogOrder,
		TotalTasks:     totalTasks,
		CompletedTasks: completedTasks,
		Progress:       progress,
		CreatedAt:      story.CreatedAt,
		UpdatedAt:      story.UpdatedAt,
	}

	res.Reporter = &responsedto.UserSummary{
		ID:        story.Reporter.ID,
		FullName:  story.Reporter.FullName,
		Email:     story.Reporter.Email,
		AvatarURL: story.Reporter.AvatarURL,
		Role:      story.Reporter.Role,
	}

	if story.Assignee != nil {
		res.Assignee = &responsedto.UserSummary{
			ID:        story.Assignee.ID,
			FullName:  story.Assignee.FullName,
			Email:     story.Assignee.Email,
			AvatarURL: story.Assignee.AvatarURL,
			Role:      story.Assignee.Role,
		}
	}

	return res
}

func (s *userStoryService) ReorderUserStories(projectID, userID, orgID uuid.UUID, storyIDs []uuid.UUID) *response.Error {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to reorder user stories in this project",
		}
	}

	return s.userStoryRepo.ReorderUserStories(projectID, storyIDs)
}

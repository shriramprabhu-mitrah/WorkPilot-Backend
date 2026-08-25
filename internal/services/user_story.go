package services

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	"go.uber.org/zap"
)

type UserStoryService interface {
	CreateUserStory(req dto.CreateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error)
	GetUserStoryByID(userStoryID, projectID, userID, orgID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error)
	UpdateUserStory(req dto.UpdateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error)
	DeleteUserStory(userStoryID, projectID, userID, orgID uuid.UUID) *response.Error
	GetUserStories(projectID, userID, orgID uuid.UUID, filter dto.UserStoryFilter) ([]responsedto.UserStoryResponse, response.Pagination, *response.Error)
	ReorderUserStories(projectID, userID, orgID uuid.UUID, storyIDs []uuid.UUID) *response.Error
	UpdateUserStoryStatus(req dto.UpdateUserStoryStatusAssignmentRequest) (*responsedto.UserStoryResponse, *response.Error)
}

type userStoryService struct {
	authRepo            authrepo.AuthRepository
	projectRepo         projectrepo.ProjectRepository
	userStoryRepo       userstoryrepo.UserStoryRepository
	taskRepo            taskrepo.TaskRepository
	customStatusRepo    customstatusrepo.CustomStatusRepository
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository
	auditRepo           auditrepo.AuditLogRepository
	favoriteRepo        favoriterepo.FavoriteRepository
	logger              *zap.Logger
}

func InitUserStoryService(
	authRepo authrepo.AuthRepository,
	projectRepo projectrepo.ProjectRepository,
	userStoryRepo userstoryrepo.UserStoryRepository,
	taskRepo taskrepo.TaskRepository,
	customStatusRepo customstatusrepo.CustomStatusRepository,
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository,
	auditRepo auditrepo.AuditLogRepository,
	favoriteRepo favoriterepo.FavoriteRepository,
	logger *zap.Logger,
) UserStoryService {
	return &userStoryService{
		authRepo:            authRepo,
		projectRepo:         projectRepo,
		userStoryRepo:       userStoryRepo,
		taskRepo:            taskRepo,
		customStatusRepo:    customStatusRepo,
		userStoryStatusRepo: userStoryStatusRepo,
		auditRepo:           auditRepo,
		favoriteRepo:        favoriteRepo,
		logger:              logger,
	}
}

func (s *userStoryService) getFavoriteUserStoryMap(userID uuid.UUID) map[uuid.UUID]bool {
	favMap := make(map[uuid.UUID]bool)
	if s.favoriteRepo == nil || userID == uuid.Nil {
		return favMap
	}
	favs, err := s.favoriteRepo.GetFavoritesByUserID(userID, models.FavoriteItemTypeUserStory)
	if err == nil {
		for _, f := range favs {
			if f.UserStoryID != nil {
				favMap[*f.UserStoryID] = true
			}
		}
	}
	return favMap
}

func (s *userStoryService) getFavoriteTaskMap(userID uuid.UUID) map[uuid.UUID]bool {
	favMap := make(map[uuid.UUID]bool)
	if s.favoriteRepo == nil || userID == uuid.Nil {
		return favMap
	}
	favs, err := s.favoriteRepo.GetFavoritesByUserID(userID, models.FavoriteItemTypeTask)
	if err == nil {
		for _, f := range favs {
			if f.TaskID != nil {
				favMap[*f.TaskID] = true
			}
		}
	}
	return favMap
}

func (s *userStoryService) isUserStoryFavorited(userID, userStoryID uuid.UUID) bool {
	if s.favoriteRepo == nil || userID == uuid.Nil || userStoryID == uuid.Nil {
		return false
	}
	isFav, err := s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeUserStory, userStoryID)
	if err != nil {
		return false
	}
	return isFav
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

func (s *userStoryService) getStatusIsFinalMap(projectID uuid.UUID) map[string]bool {
	isFinalMap := make(map[string]bool)
	for k, v := range models.DefaultStatusIsFinal {
		isFinalMap[k] = v
	}
	if s.customStatusRepo != nil {
		customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
		if err == nil {
			for _, cs := range customStatuses {
				isFinalMap[models.NormalizeTaskStatus(cs.Name)] = cs.IsFinal
			}
		}
	}
	return isFinalMap
}

func (s *userStoryService) resolveStatusIDAndName(projectID uuid.UUID, statusID *uuid.UUID, statusName *string) (uuid.UUID, string, *response.Error) {
	if statusID != nil && *statusID != uuid.Nil {
		if s.userStoryStatusRepo == nil {
			return uuid.Nil, "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "User story status repository not initialized",
			}
		}
		cs, err := s.userStoryStatusRepo.GetStatusByID(*statusID, projectID)
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
		if s.userStoryStatusRepo == nil {
			return uuid.Nil, "", &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "User story status repository not initialized",
			}
		}
		normalized := models.NormalizeTaskStatus(*statusName)
		cs, err := s.userStoryStatusRepo.GetStatusByName(projectID, normalized)
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

	if s.userStoryStatusRepo == nil {
		return uuid.Nil, "", &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "User story status repository not initialized",
		}
	}
	customStatuses, err := s.userStoryStatusRepo.GetStatusesByProjectID(projectID)
	if err != nil {
		return uuid.Nil, "", err
	}

	var defaultStatus *models.UserStoryStatus
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
	if user.Role.Name == string(dto.RoleSuperAdmin) {
		return models.Project{}, models.User{}, false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "user_stories", "view")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

func (s *userStoryService) CreateUserStory(req dto.CreateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error) {
	_, user, authorized, err := s.checkAuthorization(req.ProjectID, req.ReporterID)
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

	hasAddPermission, permErr := CheckPermission(s.authRepo, s.projectRepo, req.ReporterID, req.ProjectID, "user_stories", "add")
	if permErr != nil {
		return nil, permErr
	}
	if !hasAddPermission {
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
			if assigneeUser.Role.Name == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
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

	recalcErr := s.userStoryRepo.RecalculateUserStoryIsClosed(story.ID)
	if recalcErr != nil {
		return nil, recalcErr
	}

	// Fetch created story with preloaded fields
	createdStory, getErr := s.userStoryRepo.GetUserStoryByID(story.ID, req.ProjectID)
	if getErr != nil {
		return nil, getErr
	}

	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(req.ProjectID)
	}
	res := mapToUserStoryResponse(*createdStory, userStoryStatuses, 0, 0, 0.0)

	auditLog := models.AuditLog{
		UserID:         &req.ReporterID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		UserStoryID:    &story.ID,
		Action:         "created",
		ResourceType:   "user_story",
		ResourceID:     story.ID.String(),
		Details:        fmt.Sprintf("User Story '%s' created by %s", story.Title, user.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &res, nil
}

func (s *userStoryService) GetUserStoryByID(userStoryID, projectID, userID, orgID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error) {
	_, user, authorized, err := s.checkAuthorization(projectID, userID)
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
	isFinalMap := s.getStatusIsFinalMap(projectID)
	favTaskMap := s.getFavoriteTaskMap(userID)
	taskResponses := make([]responsedto.TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		tR := mapToTaskResponse(t, colorMap, isFinalMap)
		tR.IsFavourite = favTaskMap[t.ID]
		taskResponses = append(taskResponses, tR)
	}

	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(projectID)
	}
	res := mapToUserStoryResponse(*story, userStoryStatuses, total, completed, progress)
	isFav := s.isUserStoryFavorited(userID, story.ID)
	res.IsFavourite = isFav
	res.Tasks = taskResponses

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		UserStoryID:    &story.ID,
		Action:         "viewed",
		ResourceType:   "user_story",
		ResourceID:     story.ID.String(),
		Details:        fmt.Sprintf("User Story '%s' viewed by %s", story.Title, user.UserName),
		Type:           models.AuditLogTypeView,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &res, nil
}

func (s *userStoryService) UpdateUserStory(req dto.UpdateUserStoryRequest) (*responsedto.UserStoryResponse, *response.Error) {
	_, user, _, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	authorizedUpdate, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "user_stories", "modify")
	if permErr != nil {
		return nil, permErr
	}
	if !authorizedUpdate {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update user stories in this project",
		}
	}

	// Fetch existing user story
	existingStory, err := s.userStoryRepo.GetUserStoryByID(req.UserStoryID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	changedBy := user.UserName
	if changedBy == "" {
		changedBy = user.FullName
	}
	if changedBy == "" {
		changedBy = user.Email
	}
	if changedBy == "" {
		changedBy = req.UserID.String()
	}

	updates := make(map[string]interface{})
	var changes []string

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
		if *req.Title != existingStory.Title {
			changes = append(changes, fmt.Sprintf("title changed from '%s' to '%s'", existingStory.Title, *req.Title))
		}
	}

	if req.Description != nil {
		sanitizedDescription := strings.TrimSpace(utils.SanitizeHTML(*req.Description))
		updates["description"] = sanitizedDescription
		if sanitizedDescription != existingStory.Description {
			changes = append(changes, "description changed")
		}
	}

	if req.Priority != nil {
		updates["priority"] = *req.Priority
		if *req.Priority != existingStory.Priority {
			changes = append(changes, fmt.Sprintf("priority changed from '%s' to '%s'", existingStory.Priority, *req.Priority))
		}
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
		if resolvedStatusName != existingStory.Status {
			changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", existingStory.Status, resolvedStatusName))
		}
	}

	if req.StoryPoints != nil {
		updates["story_points"] = *req.StoryPoints
		if *req.StoryPoints != existingStory.StoryPoints {
			changes = append(changes, fmt.Sprintf("story points changed from %d to %d", existingStory.StoryPoints, *req.StoryPoints))
		}
	}

	// Assignee check
	if req.IsAssigneeIDNull() || (req.AssigneeID != nil && *req.AssigneeID == uuid.Nil) {
		oldAssignee := "nil"
		if existingStory.AssigneeID != nil {
			oldAssignee = existingStory.AssigneeID.String()
		}
		if oldAssignee != "nil" {
			changes = append(changes, fmt.Sprintf("assignee changed from %s to nil", oldAssignee))
		}
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
			if assigneeUser.Role.Name == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
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
		oldAssignee := "nil"
		if existingStory.AssigneeID != nil {
			oldAssignee = existingStory.AssigneeID.String()
		}
		newAssignee := req.AssigneeID.String()
		if oldAssignee != newAssignee {
			changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))
		}
		updates["assignee_id"] = *req.AssigneeID
	}

	if req.IsClosed != nil {
		updates["is_closed"] = *req.IsClosed
		if *req.IsClosed != existingStory.IsClosed {
			changes = append(changes, fmt.Sprintf("is_closed changed from %t to %t", existingStory.IsClosed, *req.IsClosed))
		}
	}

	// Sprint check
	if req.IsSprintIDNull() || (req.SprintID != nil && *req.SprintID == uuid.Nil) {
		oldSprint := "nil"
		if existingStory.SprintID != nil {
			oldSprint = existingStory.SprintID.String()
		}
		if oldSprint != "nil" {
			changes = append(changes, fmt.Sprintf("sprint changed from %s to nil", oldSprint))
		}
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
		oldSprint := "nil"
		if existingStory.SprintID != nil {
			oldSprint = existingStory.SprintID.String()
		}
		newSprint := req.SprintID.String()
		if oldSprint != newSprint {
			changes = append(changes, fmt.Sprintf("sprint changed from %s to %s", oldSprint, newSprint))
		}
		updates["sprint_id"] = *req.SprintID
	}

	updateErr := s.userStoryRepo.UpdateUserStory(req.UserStoryID, updates)
	if updateErr != nil {
		return nil, updateErr
	}

	recalcErr := s.userStoryRepo.RecalculateUserStoryIsClosed(req.UserStoryID)
	if recalcErr != nil {
		return nil, recalcErr
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

	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(req.ProjectID)
	}
	res := mapToUserStoryResponse(*updatedStory, userStoryStatuses, total, completed, progress)

	var detail string
	if len(changes) > 0 {
		detail = fmt.Sprintf("User Story '%s' updated by %s: %s", updatedStory.Title, changedBy, strings.Join(changes, ", "))
	} else {
		detail = fmt.Sprintf("User Story '%s' details updated by %s", updatedStory.Title, changedBy)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		UserStoryID:    &updatedStory.ID,
		Action:         "updated",
		ResourceType:   "user_story",
		ResourceID:     updatedStory.ID.String(),
		Title:          updatedStory.Title,
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &res, nil
}

func (s *userStoryService) DeleteUserStory(userStoryID, projectID, userID, orgID uuid.UUID) *response.Error {
	_, user, authorized, err := s.checkAuthorization(projectID, userID)
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

	hasDeletePermission, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "user_stories", "delete")
	if permErr != nil {
		return permErr
	}
	if !hasDeletePermission {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete user stories in this project",
		}
	}

	// Fetch existingStory first to verify existence
	existingStory, getErr := s.userStoryRepo.GetUserStoryByID(userStoryID, projectID)
	if getErr != nil {
		return getErr
	}

	err = s.userStoryRepo.DeleteUserStory(userStoryID, projectID)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		UserStoryID:    &userStoryID,
		Action:         "deleted",
		ResourceType:   "user_story",
		ResourceID:     userStoryID.String(),
		Details:        fmt.Sprintf("User Story '%s' deleted by %s", existingStory.Title, user.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
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
	isFinalMap := s.getStatusIsFinalMap(projectID)
	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(projectID)
	}
	favStoryMap := s.getFavoriteUserStoryMap(userID)
	favTaskMap := s.getFavoriteTaskMap(userID)
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
			tR := mapToTaskResponse(t, colorMap, isFinalMap)
			tR.IsFavourite = favTaskMap[t.ID]
			taskResponses = append(taskResponses, tR)
		}

		storyRes := mapToUserStoryResponse(story, userStoryStatuses, total, completed, progress)
		storyRes.IsFavourite = favStoryMap[story.ID]
		storyRes.Tasks = taskResponses
		resList = append(resList, storyRes)
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "user_story",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return resList, pagination, nil
}

func mapToUserStoryResponse(story models.UserStory, customStatuses []models.UserStoryStatus, totalTasks, completedTasks int64, progress float64) responsedto.UserStoryResponse {
	var projectName string
	if story.Project.Name != "" {
		projectName = story.Project.Name
	}
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
		ID:                    story.ID,
		ProjectID:             story.ProjectID,
		ProjectName:           projectName,
		SprintID:              story.SprintID,
		SprintName:            sprintName,
		SerialNumber:          story.SerialNumber,
		FormattedSerialNumber: story.FormattedSerialNumber(),
		Title:                 story.Title,
		Description:           story.Description,
		Priority:              story.Priority,
		StatusID:              resolvedStatusID,
		Status:                story.Status,
		StatusColor:           resolvedStatusColor,
		IsClosed:              story.IsClosed,
		StoryPoints:           story.StoryPoints,
		AssigneeID:            story.AssigneeID,
		AssigneeName:          assigneeName,
		ReporterID:            story.ReporterID,
		ReporterName:          story.Reporter.FullName,
		BacklogOrder:          story.BacklogOrder,
		TotalTasks:            totalTasks,
		CompletedTasks:        completedTasks,
		Progress:              progress,
		CreatedAt:             story.CreatedAt,
		UpdatedAt:             story.UpdatedAt,
	}

	var reporterAvatarURL *string
	if story.Reporter.AvatarURL != "" {
		reporterAvatarURL = &story.Reporter.AvatarURL
	}
	res.Reporter = &responsedto.UserSummary{
		ID:        story.Reporter.ID,
		FullName:  story.Reporter.FullName,
		Email:     story.Reporter.Email,
		AvatarURL: reporterAvatarURL,
		Color:     story.Reporter.Color,
		Role:      story.Reporter.Role.Name,
	}

	if story.Assignee != nil {
		var assigneeAvatarURL *string
		if story.Assignee.AvatarURL != "" {
			assigneeAvatarURL = &story.Assignee.AvatarURL
		}
		res.Assignee = &responsedto.UserSummary{
			ID:        story.Assignee.ID,
			FullName:  story.Assignee.FullName,
			Email:     story.Assignee.Email,
			AvatarURL: assigneeAvatarURL,
			Color:     story.Assignee.Color,
			Role:      story.Assignee.Role.Name,
		}
	}

	return res
}

func (s *userStoryService) ReorderUserStories(projectID, userID, orgID uuid.UUID, storyIDs []uuid.UUID) *response.Error {
	_, user, authorized, err := s.checkAuthorization(projectID, userID)
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

	err = s.userStoryRepo.ReorderUserStories(projectID, storyIDs)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "reordered",
		ResourceType:   "user_story",
		ResourceID:     storyIDs[0].String(),
		Details:        fmt.Sprintf("User Story was reordered by %s", user.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *userStoryService) UpdateUserStoryStatus(req dto.UpdateUserStoryStatusAssignmentRequest) (*responsedto.UserStoryResponse, *response.Error) {
	// 1. Authorization Check
	_, _, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
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

	// 2. Fetch User Story
	_, err = s.userStoryRepo.GetUserStoryByID(req.UserStoryID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	// 3. Fetch User Story Status and verify it belongs to the same project
	status, err := s.userStoryStatusRepo.GetStatusByID(req.StatusID, req.ProjectID)
	if err != nil {
		if err.StatusCode == http.StatusNotFound {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Invalid user story status: status does not exist or does not belong to this project",
			}
		}
		return nil, err
	}

	// 4. Perform update
	updates := map[string]interface{}{
		"status_id": status.ID,
	}
	updateErr := s.userStoryRepo.UpdateUserStory(req.UserStoryID, updates)
	if updateErr != nil {
		return nil, updateErr
	}

	recalcErr := s.userStoryRepo.RecalculateUserStoryIsClosed(req.UserStoryID)
	if recalcErr != nil {
		return nil, recalcErr
	}

	// 5. Fetch updated User Story
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

	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(req.ProjectID)
	}
	res := mapToUserStoryResponse(*updatedStory, userStoryStatuses, total, completed, progress)

	// 6. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "updated",
		ResourceType:   "user_story",
		ResourceID:     updatedStory.ID.String(),
		Details:        fmt.Sprintf("User Story '%s' status updated to '%s'", updatedStory.Title, status.Name),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if logErr := s.auditRepo.CreateAuditLog(auditLog); logErr != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", logErr))
	}

	return &res, nil
}

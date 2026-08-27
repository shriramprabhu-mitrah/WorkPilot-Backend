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
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	"go.uber.org/zap"
)

type TaskService interface {
	CreateTask(req dto.CreateTaskRequest) (uuid.UUID, *responsedto.TaskResponse, *response.Error)
	GetTaskByID(taskIDOrKey string, projectIDOrSlug string, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error)
	UpdateTask(req dto.UpdateTaskRequest) (*responsedto.TaskResponse, *response.Error)
	RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error
	CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error)
	GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, response.Pagination, *response.Error)
	BulkUpdateTasks(req dto.BulkUpdateTasksRequest) (*responsedto.BulkUpdateTasksResponse, *response.Error)
	BulkDeleteTasks(req dto.BulkDeleteTasksRequest) (*responsedto.BulkDeleteTasksResponse, *response.Error)
	AttachLabelToTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
	RemoveLabelFromTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
	AssignTaskToMe(taskID, userID, organizationID, projectID uuid.UUID) (*responsedto.TaskResponse, *response.Error)
}

type taskService struct {
	authRepo         authrepo.AuthRepository
	projectRepo      projectrepo.ProjectRepository
	taskRepo         taskrepo.TaskRepository
	userStoryRepo    userstoryrepo.UserStoryRepository
	auditRepo        auditrepo.AuditLogRepository
	customStatusRepo customstatusrepo.CustomStatusRepository
	favoriteRepo     favoriterepo.FavoriteRepository
	logger           *zap.Logger
}

func InitTaskService(
	authRepo authrepo.AuthRepository,
	projectRepo projectrepo.ProjectRepository,
	taskRepo taskrepo.TaskRepository,
	userStoryRepo userstoryrepo.UserStoryRepository,
	auditRepo auditrepo.AuditLogRepository,
	customStatusRepo customstatusrepo.CustomStatusRepository,
	favoriteRepo favoriterepo.FavoriteRepository,
	logger *zap.Logger,
) TaskService {
	return &taskService{
		authRepo:         authRepo,
		projectRepo:      projectRepo,
		taskRepo:         taskRepo,
		userStoryRepo:    userStoryRepo,
		auditRepo:        auditRepo,
		customStatusRepo: customStatusRepo,
		favoriteRepo:     favoriteRepo,
		logger:           logger,
	}
}

func (s *taskService) getFavoriteTaskMap(userID uuid.UUID) map[uuid.UUID]bool {
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

func (s *taskService) isTaskFavorited(userID, taskID uuid.UUID) bool {
	if s.favoriteRepo == nil || userID == uuid.Nil || taskID == uuid.Nil {
		return false
	}
	isFav, err := s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeTask, taskID)
	if err != nil {
		return false
	}
	return isFav
}

func (s *taskService) getAssigneeUsername(assigneeID *uuid.UUID, preloadedUser *models.User) string {
	if assigneeID == nil || *assigneeID == uuid.Nil {
		return "nil"
	}
	if preloadedUser != nil && preloadedUser.ID == *assigneeID && preloadedUser.UserName != "" {
		return preloadedUser.UserName
	}
	if u, err := s.authRepo.GetUserByID(*assigneeID); err == nil && u.UserName != "" {
		return u.UserName
	}
	return assigneeID.String()
}

func (s *taskService) checkAuthorization(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
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

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "tasks", "view")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

func GenerateProjectPrefix(name string) string {
	return utils.GenerateProjectPrefix(name)
}

func mapToTaskResponse(task models.Task, colorMap map[string]string, isFinalMap map[string]bool) responsedto.TaskResponse {
	var projectName string
	if task.Project.Name != "" {
		projectName = task.Project.Name
	}
	var sprintName string
	if task.Sprint != nil {
		sprintName = task.Sprint.Name
	}
	var userStoryTitle string
	if task.UserStory != nil {
		userStoryTitle = task.UserStory.Title
	}
	var assigneeName string
	if task.Assignee != nil {
		assigneeName = task.Assignee.FullName
	}
	var reporterName string
	if task.Reporter != nil {
		reporterName = task.Reporter.FullName
	}

	labelsRes := []responsedto.LabelResponse{}
	for _, l := range task.Labels {
		labelsRes = append(labelsRes, responsedto.LabelFromModel(l))
	}

	statusKey := models.NormalizeTaskStatus(task.Status)
	statusColor := colorMap[statusKey]
	if statusColor == "" {
		statusColor = "#808080"
	}
	isFinal, exists := isFinalMap[statusKey]
	if !exists {
		isFinal = models.DefaultStatusIsFinal[statusKey]
	}

	response := responsedto.TaskResponse{
		ID:                    task.ID,
		ProjectID:             task.ProjectID,
		ProjectName:           projectName,
		SprintID:              task.SprintID,
		SprintName:            sprintName,
		UserStoryID:           task.UserStoryID,
		UserStoryTitle:        userStoryTitle,
		Key:                   task.Key,
		SerialNumber:          task.SerialNumber,
		FormattedSerialNumber: task.FormattedSerialNumber(),
		Title:                 task.Title,
		Description:           task.Description,
		Type:                  task.Type,
		Priority:              task.Priority,
		StatusID:              task.StatusID,
		Status:                task.Status,
		StatusColor:           statusColor,
		IsFinal:               isFinal,
		AssigneeID:            task.AssigneeID,
		ReporterID:            task.ReporterID,
		AssigneeName:          assigneeName,
		ReporterName:          reporterName,
		StoryPoints:           task.StoryPoints,
		DueDate:               task.DueDate,
		EstimatedHours:        task.EstimatedHours,
		ActualHours:           task.ActualHours,
		BlockedReason:         task.BlockedReason,
		CreatedAt:             task.CreatedAt,
		UpdatedAt:             task.UpdatedAt,
		Labels:                labelsRes,
	}

	if task.Reporter != nil {
		var avatarURL *string
		if task.Reporter.AvatarURL != "" {
			avatarURL = &task.Reporter.AvatarURL
		}
		response.User = &responsedto.UserSummary{
			ID:        task.Reporter.ID,
			FullName:  task.Reporter.FullName,
			Email:     task.Reporter.Email,
			AvatarURL: avatarURL,
			Color:     task.Reporter.Color,
			Role:      task.Reporter.Role.Name,
		}
	}

	if task.Assignee != nil {
		var avatarURL *string
		if task.Assignee.AvatarURL != "" {
			avatarURL = &task.Assignee.AvatarURL
		}
		response.Assignee = &responsedto.UserSummary{
			ID:        task.Assignee.ID,
			FullName:  task.Assignee.FullName,
			Email:     task.Assignee.Email,
			AvatarURL: avatarURL,
			Color:     task.Assignee.Color,
			Role:      task.Assignee.Role.Name,
		}
	}
	return response
}

func (s *taskService) getStatusColorMap(projectID uuid.UUID) map[string]string {
	colorMap := make(map[string]string)
	for k, v := range models.DefaultStatusColors {
		colorMap[k] = v
	}
	customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
	if err == nil {
		for _, cs := range customStatuses {
			colorMap[models.NormalizeTaskStatus(cs.Name)] = cs.Color
		}
	}
	return colorMap
}

func (s *taskService) getStatusIsFinalMap(projectID uuid.UUID) map[string]bool {
	isFinalMap := make(map[string]bool)
	for k, v := range models.DefaultStatusIsFinal {
		isFinalMap[k] = v
	}
	customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
	if err == nil {
		for _, cs := range customStatuses {
			isFinalMap[models.NormalizeTaskStatus(cs.Name)] = cs.IsFinal
		}
	}
	return isFinalMap
}

func (s *taskService) resolveStatusIDAndName(projectID uuid.UUID, statusID *uuid.UUID, statusName *string) (uuid.UUID, string, *response.Error) {
	// Rule 1 (Precedence): If statusID is provided, fetch the status by ID
	if statusID != nil && *statusID != uuid.Nil {
		cs, err := s.customStatusRepo.GetStatusByID(*statusID, projectID)
		if err != nil {
			if err.StatusCode == http.StatusNotFound {
				return uuid.Nil, "", &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusUnprocessableEntity,
					Message:    "Invalid task status_id: status does not exist or does not belong to this project",
				}
			}
			return uuid.Nil, "", err
		}
		return cs.ID, cs.Name, nil
	}

	// Rule 2: If statusID is nil/empty but statusName is provided
	if statusName != nil && *statusName != "" {
		normalized := models.NormalizeTaskStatus(*statusName)
		cs, err := s.customStatusRepo.GetStatusByName(projectID, normalized)
		if err != nil {
			if err.StatusCode == http.StatusNotFound {
				return uuid.Nil, "", &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusUnprocessableEntity,
					Message:    "Invalid task status value: status name not found in this project",
				}
			}
			return uuid.Nil, "", err
		}
		return cs.ID, cs.Name, nil
	}

	// Rule 3: If both are nil/empty, default to the project's default status
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

func (s *taskService) CreateTask(req dto.CreateTaskRequest) (uuid.UUID, *responsedto.TaskResponse, *response.Error) {
	project, result, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if !authorized {
		return uuid.Nil, nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	hasAddPermission, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "tasks", "add")
	if permErr != nil {
		return uuid.Nil, nil, permErr
	}
	if !hasAddPermission {
		return uuid.Nil, nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to create tasks in this project",
		}
	}

	// project is returned from checkAuthorization to avoid extra DB fetch

	// Validation: Title length
	if len([]rune(req.Title)) < 3 || len([]rune(req.Title)) > 200 {
		return uuid.Nil, nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Task title must be between 3 and 200 characters",
		}
	}

	// Validation: Story Points (Fibonacci)
	if !isFibonacci(req.StoryPoints) {
		return uuid.Nil, nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Story points must follow the Fibonacci scale",
		}
	}

	// Validation: Assignee must be active and a member of the project
	if req.AssigneeID != nil && *req.AssigneeID != uuid.Nil {
		assigneeUser, err := s.authRepo.GetUserByID(*req.AssigneeID)
		if err != nil {
			return uuid.Nil, nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee user not found",
			}
		}
		if !assigneeUser.IsActive {
			return uuid.Nil, nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be an active user",
			}
		}
		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, *req.AssigneeID)
		if err != nil {
			return uuid.Nil, nil, err
		}
		if !isMember {
			if assigneeUser.Role.Name == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
				isMember = true
			}
		}
		if !isMember {
			return uuid.Nil, nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Assignee must be a member of the project",
			}
		}
	}

	// Validation: Due date cannot be backdated unless overridden by PM/Admin
	if req.DueDate != nil && !req.DueDate.IsZero() {
		if isBackdated(*req.DueDate) {
			isPMOrAdmin, checkErr := s.checkIsPMOrAdmin(req.ProjectID, req.UserID)
			if checkErr != nil {
				return uuid.Nil, nil, checkErr
			}
			if !isPMOrAdmin {
				return uuid.Nil, nil, &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusBadRequest,
					Message:    "Due date cannot be backdated unless set by a PM or Admin",
				}
			}
		}
	}

	// Validation: Target sprint cannot be completed
	if req.SprintID != nil && *req.SprintID != uuid.Nil {
		sprintStatus, err := s.taskRepo.GetSprintStatus(*req.SprintID)
		if err != nil {
			return uuid.Nil, nil, err
		}
		if sprintStatus == string(dto.SprintStatusCompleted) {
			return uuid.Nil, nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Cannot assign a task to a completed sprint",
			}
		}
	}

	userStoryID := req.UserStoryID

	// Validation: User Story must belong to the same project
	if userStoryID != nil && *userStoryID != uuid.Nil {
		inProject, err := s.taskRepo.IsUserStoryInProject(*userStoryID, req.ProjectID)
		if err != nil {
			return uuid.Nil, nil, err
		}
		if !inProject {
			return uuid.Nil, nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "User story must belong to the same project",
			}
		}
	}

	req.ReporterID = &req.UserID

	projectKey := GenerateProjectPrefix(project.Name)

	var task models.Task
	task.ProjectID = req.ProjectID
	task.SprintID = req.SprintID
	task.UserStoryID = userStoryID
	task.Title = req.Title
	task.Description = utils.SanitizeHTML(req.Description)
	task.Type = req.Type
	task.Priority = req.Priority
	var statusNameArg *string
	if req.Status != "" {
		statusNameArg = &req.Status
	}
	resolvedStatusID, resolvedStatusName, valErr := s.resolveStatusIDAndName(req.ProjectID, req.StatusID, statusNameArg)
	if valErr != nil {
		return uuid.Nil, nil, valErr
	}

	task.StatusID = resolvedStatusID
	task.Status = resolvedStatusName
	task.AssigneeID = req.AssigneeID
	task.ReporterID = req.ReporterID
	task.StoryPoints = req.StoryPoints
	task.DueDate = req.DueDate
	task.EstimatedHours = req.EstimatedHours
	task.ActualHours = req.ActualHours

	var labels []models.Label
	if len(req.LabelIDs) > 0 {
		var verifyErr *response.Error
		labels, verifyErr = s.taskRepo.VerifyLabelIDs(req.ProjectID, req.LabelIDs)
		if verifyErr != nil {
			return uuid.Nil, nil, verifyErr
		}
	}
	task.Labels = labels

	var lastErr *response.Error
	for attempt := 0; attempt < 3; attempt++ {
		seq, err := s.taskRepo.GetNextSequenceNumber(req.ProjectID)
		if err != nil {
			return uuid.Nil, nil, err
		}
		task.SequenceNumber = seq
		task.Key = fmt.Sprintf("%s-%d", projectKey, seq)

		lastErr = s.taskRepo.CreateTask(&task)
		if lastErr == nil {
			break
		}
		if lastErr.Code != response.ErrConflict {
			return uuid.Nil, nil, lastErr
		}
	}
	if lastErr != nil {
		return uuid.Nil, nil, lastErr
	}

	if s.userStoryRepo != nil && task.UserStoryID != nil && *task.UserStoryID != uuid.Nil {
		_ = s.userStoryRepo.RecalculateUserStoryIsClosed(*task.UserStoryID)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		TaskID:         &task.ID,
		Action:         "created",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Title:          task.Title,
		Details:        fmt.Sprintf("The task '%s' was created by %s", task.Title, result.UserName),
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	isFinalMap := s.getStatusIsFinalMap(req.ProjectID)
	res := mapToTaskResponse(task, colorMap, isFinalMap)
	return task.ID, &res, nil
}

func (s *taskService) GetTaskByID(taskIDOrKey string, projectIDOrSlug string, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error) {
	var project models.Project
	var getProjErr *response.Error
	projectUUID, parseErr := uuid.FromString(projectIDOrSlug)
	if parseErr == nil {
		project, getProjErr = s.projectRepo.GetProjectByID(projectUUID)
	} else {
		project, getProjErr = s.projectRepo.GetProjectBySlug(projectIDOrSlug)
	}
	if getProjErr != nil {
		return nil, getProjErr
	}

	_, result, authorized, err := s.checkAuthorization(project.ID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	var task *models.Task
	taskUUID, parseErr := uuid.FromString(taskIDOrKey)
	if parseErr == nil {
		task, err = s.taskRepo.GetTaskByID(taskUUID, project.ID)
	} else {
		task, err = s.taskRepo.GetTaskByKey(taskIDOrKey, project.ID)
	}
	if err != nil {
		return nil, err
	}

	colorMap := s.getStatusColorMap(project.ID)
	isFinalMap := s.getStatusIsFinalMap(project.ID)
	res := mapToTaskResponse(*task, colorMap, isFinalMap)
	isFav := s.isTaskFavorited(userID, task.ID)
	res.IsFavourite = isFav

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &project.ID,
		TaskID:         &task.ID,
		Action:         "viewed",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Title:          task.Title,
		Details:        fmt.Sprintf("The task '%s' was viewed by %s", task.Title, result.UserName),
		Type:           models.AuditLogTypeView,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &res, nil
}

func (s *taskService) UpdateTask(req dto.UpdateTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	_, user, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	hasModifyPermission, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "tasks", "modify")
	if permErr != nil {
		return nil, permErr
	}
	if !hasModifyPermission {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update tasks in this project",
		}
	}

	isPMOrAdmin, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "projects", "modify")
	if permErr != nil {
		return nil, permErr
	}

	task, err := s.taskRepo.GetTaskByID(req.TaskID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	// Validation: Title length
	if req.Title != nil && (len([]rune(*req.Title)) < 3 || len([]rune(*req.Title)) > 200) {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Task title must be between 3 and 200 characters",
		}
	}

	// Validation: Story Points (Fibonacci)
	if req.StoryPoints != nil && !isFibonacci(*req.StoryPoints) {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Story points must follow the Fibonacci scale",
		}
	}

	// 1. Validate Assignee Membership
	if !req.IsAssigneeIDNull() && req.AssigneeID != nil && *req.AssigneeID != uuid.Nil {
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

	userStoryID := req.UserStoryID

	// Validation: User Story must belong to the same project
	if !req.IsUserStoryIDNull() && userStoryID != nil && *userStoryID != uuid.Nil {
		inProject, err := s.taskRepo.IsUserStoryInProject(*userStoryID, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if !inProject {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "User story must belong to the same project",
			}
		}
	}

	// Validation: Due date cannot be backdated unless overridden by PM/Admin
	if !req.IsDueDateNull() && req.DueDate != nil && !req.DueDate.IsZero() {
		if isBackdated(*req.DueDate) {
			isPMOrAdmin, checkErr := s.checkIsPMOrAdmin(req.ProjectID, req.UserID)
			if checkErr != nil {
				return nil, checkErr
			}
			if !isPMOrAdmin {
				return nil, &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusBadRequest,
					Message:    "Due date cannot be backdated unless set by a PM or Admin",
				}
			}
		}
	}

	// 2. Validate Actual Hours Update
	if req.ActualHours != nil {
		isStatusChangingToActive := req.Status != nil && *req.Status != string(dto.TaskStatusCompleted)
		if task.Status == string(dto.TaskStatusCompleted) && !isStatusChangingToActive {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Actual hours can only be updated while the task is not completed",
			}
		}
		if !isPMOrAdmin {
			if task.AssigneeID == nil || *task.AssigneeID != req.UserID {
				return nil, &response.Error{
					Code:       response.ErrForbidden,
					StatusCode: http.StatusForbidden,
					Message:    "Only the task assignee or a PM/Admin can update actual hours",
				}
			}
			if task.ActualHours != nil && *req.ActualHours <= *task.ActualHours {
				return nil, &response.Error{
					Code:       response.ErrBadRequest,
					StatusCode: http.StatusBadRequest,
					Message:    "Actual hours can only be incremented",
				}
			}
			if task.ActualHours == nil && *req.ActualHours <= 0 {
				return nil, &response.Error{
					Code:       response.ErrBadRequest,
					StatusCode: http.StatusBadRequest,
					Message:    "Actual hours must be greater than 0",
				}
			}
		} else {
			if *req.ActualHours < 0 {
				return nil, &response.Error{
					Code:       response.ErrBadRequest,
					StatusCode: http.StatusBadRequest,
					Message:    "Actual hours cannot be negative",
				}
			}
		}
	}

	// Validation: Sprint completed constraints
	isSprintChanging := false
	var targetSprintID uuid.UUID = uuid.Nil
	if req.IsSprintIDNull() || (req.SprintID != nil && *req.SprintID == uuid.Nil) {
		targetSprintID = uuid.Nil
	} else if req.SprintID != nil {
		targetSprintID = *req.SprintID
	}

	if req.IsSprintIDNull() || req.SprintID != nil {
		currentSprintID := uuid.Nil
		if task.SprintID != nil {
			currentSprintID = *task.SprintID
		}
		if targetSprintID != currentSprintID {
			isSprintChanging = true
		}
	}

	if isSprintChanging {
		if task.Sprint != nil && task.Sprint.Status == string(dto.SprintStatusCompleted) {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Changing the sprint of a task in a completed sprint is blocked",
			}
		}
		if targetSprintID != uuid.Nil {
			targetStatus, err := s.taskRepo.GetSprintStatus(targetSprintID)
			if err != nil {
				return nil, err
			}
			if targetStatus == string(dto.SprintStatusCompleted) {
				return nil, &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusBadRequest,
					Message:    "Cannot assign a task to a completed sprint",
				}
			}
		}
	}

	// 3. Validate Status Transition
	var newNormalizedStatus string
	var newStatusID uuid.UUID
	isStatusChanging := false

	if req.StatusID != nil && *req.StatusID != task.StatusID {
		isStatusChanging = true
	} else if req.Status != nil && *req.Status != task.Status {
		isStatusChanging = true
	}

	if isStatusChanging {
		resolvedStatusID, resolvedStatusName, valErr := s.resolveStatusIDAndName(req.ProjectID, req.StatusID, req.Status)
		if valErr != nil {
			return nil, valErr
		}

		if resolvedStatusID != task.StatusID {
			newStatusID = resolvedStatusID
			newNormalizedStatus = resolvedStatusName

			newStatus := newNormalizedStatus
			oldStatus := models.NormalizeTaskStatus(task.Status)

			if models.NormalizeTaskStatus(newStatus) == string(dto.TaskStatusBlocked) {
				if req.BlockedReason == nil || strings.TrimSpace(*req.BlockedReason) == "" {
					return nil, &response.Error{
						Code:       response.ErrBadRequest,
						StatusCode: http.StatusBadRequest,
						Message:    "Moving to Blocked requires a blocked reason",
					}
				}
			}

			if !isPMOrAdmin {
				allowedTransition := false
				if models.IsDefaultTaskStatus(oldStatus) && models.IsDefaultTaskStatus(newStatus) {
					normOld := models.NormalizeTaskStatus(oldStatus)
					normNew := models.NormalizeTaskStatus(newStatus)
					switch normOld {
					case string(dto.TaskStatusTodo):
						allowedTransition = normNew == string(dto.TaskStatusInProgress) || normNew == string(dto.TaskStatusBlocked)
					case string(dto.TaskStatusInProgress):
						allowedTransition = normNew == string(dto.TaskStatusInReview) || normNew == string(dto.TaskStatusBlocked)
					case string(dto.TaskStatusInReview):
						allowedTransition = normNew == string(dto.TaskStatusTesting) || normNew == string(dto.TaskStatusBlocked)
					case string(dto.TaskStatusTesting):
						allowedTransition = normNew == string(dto.TaskStatusCompleted) || normNew == string(dto.TaskStatusBlocked)
					case string(dto.TaskStatusCompleted):
						allowedTransition = normNew == string(dto.TaskStatusBlocked)
					case string(dto.TaskStatusBlocked):
						allowedTransition = normNew == string(dto.TaskStatusInProgress) || normNew == string(dto.TaskStatusTodo)
					}
				} else {
					// Transition involving at least one custom status is allowed
					allowedTransition = true
				}

				if !allowedTransition {
					return nil, &response.Error{
						Code:       response.ErrInvalidStatusTransition,
						StatusCode: http.StatusBadRequest,
						Message:    fmt.Sprintf("Invalid status transition from %s to %s for developers", oldStatus, newStatus),
					}
				}
			}
		} else {
			isStatusChanging = false
		}
	}

	// 4. Track Changes for Audit Log and Apply Updates
	var changes []string

	if req.Title != nil && *req.Title != task.Title {
		changes = append(changes, fmt.Sprintf("title changed from '%s' to '%s'", task.Title, *req.Title))
		task.Title = *req.Title
	}
	if req.Description != nil && *req.Description != task.Description {
		changes = append(changes, "description changed")
		sanitizedDescription := utils.SanitizeHTML(*req.Description)
		task.Description = sanitizedDescription
	}
	if req.Type != nil && *req.Type != task.Type {
		changes = append(changes, fmt.Sprintf("type changed from '%s' to '%s'", task.Type, *req.Type))
		task.Type = *req.Type
	}
	if req.Priority != nil && *req.Priority != task.Priority {
		changes = append(changes, fmt.Sprintf("priority changed from '%s' to '%s'", task.Priority, *req.Priority))
		task.Priority = *req.Priority
	}
	if isStatusChanging {
		changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", task.Status, newNormalizedStatus))
		task.StatusID = newStatusID
		task.Status = newNormalizedStatus
		if models.NormalizeTaskStatus(newNormalizedStatus) == string(dto.TaskStatusBlocked) {
			if req.BlockedReason != nil {
				task.BlockedReason = *req.BlockedReason
			}
		} else {
			task.BlockedReason = ""
		}
	}
	if req.IsAssigneeIDNull() || req.AssigneeID != nil {
		oldAssigneeIDStr := "nil"
		if task.AssigneeID != nil {
			oldAssigneeIDStr = task.AssigneeID.String()
		}
		newAssigneeIDStr := "nil"
		var targetAssignee *uuid.UUID = nil
		if !req.IsAssigneeIDNull() && *req.AssigneeID != uuid.Nil {
			newAssigneeIDStr = req.AssigneeID.String()
			targetAssignee = req.AssigneeID
		}
		if oldAssigneeIDStr != newAssigneeIDStr {
			oldAssignee := s.getAssigneeUsername(task.AssigneeID, task.Assignee)
			newAssignee := s.getAssigneeUsername(targetAssignee, nil)
			changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))
			task.AssigneeID = targetAssignee
		}
	}
	if req.IsSprintIDNull() || req.SprintID != nil {
		oldSprint := "nil"
		if task.SprintID != nil {
			oldSprint = task.SprintID.String()
		}
		newSprint := "nil"
		var targetSprint *uuid.UUID = nil
		if !req.IsSprintIDNull() && *req.SprintID != uuid.Nil {
			newSprint = req.SprintID.String()
			targetSprint = req.SprintID
		}
		if oldSprint != newSprint {
			changes = append(changes, fmt.Sprintf("sprint changed from %s to %s", oldSprint, newSprint))
			task.SprintID = targetSprint
		}
	}
	if req.IsUserStoryIDNull() || userStoryID != nil {
		oldStory := "nil"
		if task.UserStoryID != nil {
			oldStory = task.UserStoryID.String()
		}
		newStory := "nil"
		var targetStory *uuid.UUID = nil
		if !req.IsUserStoryIDNull() && userStoryID != nil && *userStoryID != uuid.Nil {
			newStory = userStoryID.String()
			targetStory = userStoryID
		}
		if oldStory != newStory {
			changes = append(changes, fmt.Sprintf("user story changed from %s to %s", oldStory, newStory))
			task.UserStoryID = targetStory
		}
	}
	if req.StoryPoints != nil && *req.StoryPoints != task.StoryPoints {
		changes = append(changes, fmt.Sprintf("story points changed from %d to %d", task.StoryPoints, *req.StoryPoints))
		task.StoryPoints = *req.StoryPoints
	}
	if req.IsDueDateNull() || req.DueDate != nil {
		oldDue := "nil"
		if task.DueDate != nil {
			oldDue = task.DueDate.Format(time.RFC3339)
		}
		newDue := "nil"
		var targetDue *time.Time = nil
		if !req.IsDueDateNull() && !req.DueDate.IsZero() {
			newDue = req.DueDate.Format(time.RFC3339)
			targetDue = req.DueDate
		}
		if oldDue != newDue {
			changes = append(changes, fmt.Sprintf("due date changed from %s to %s", oldDue, newDue))
			task.DueDate = targetDue
		}
	}
	if req.IsEstimatedHoursNull() || req.EstimatedHours != nil {
		oldEst := "nil"
		if task.EstimatedHours != nil {
			oldEst = fmt.Sprintf("%.2f", *task.EstimatedHours)
		}
		newEst := "nil"
		var targetEst *float64 = nil
		if !req.IsEstimatedHoursNull() && *req.EstimatedHours >= 0 {
			newEst = fmt.Sprintf("%.2f", *req.EstimatedHours)
			targetEst = req.EstimatedHours
		}
		if oldEst != newEst {
			changes = append(changes, fmt.Sprintf("estimated hours changed from %s to %s", oldEst, newEst))
			task.EstimatedHours = targetEst
		}
	}
	if req.IsActualHoursNull() || req.ActualHours != nil {
		oldAct := "nil"
		if task.ActualHours != nil {
			oldAct = fmt.Sprintf("%.2f", *task.ActualHours)
		}
		newAct := "nil"
		var targetAct *float64 = nil
		if !req.IsActualHoursNull() {
			newAct = fmt.Sprintf("%.2f", *req.ActualHours)
			targetAct = req.ActualHours
		}
		if oldAct != newAct {
			changes = append(changes, fmt.Sprintf("actual hours changed from %s to %s", oldAct, newAct))
			task.ActualHours = targetAct
		}
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if isStatusChanging {
		updates["status_id"] = newStatusID
		updates["status"] = newNormalizedStatus
		if models.NormalizeTaskStatus(newNormalizedStatus) == string(dto.TaskStatusBlocked) {
			if req.BlockedReason != nil {
				updates["blocked_reason"] = *req.BlockedReason
			}
		} else {
			updates["blocked_reason"] = ""
		}
	}
	if req.IsAssigneeIDNull() || req.AssigneeID != nil {
		if !req.IsAssigneeIDNull() && *req.AssigneeID != uuid.Nil {
			updates["assignee_id"] = *req.AssigneeID
		} else {
			updates["assignee_id"] = nil
		}
	}
	if req.IsSprintIDNull() || req.SprintID != nil {
		if !req.IsSprintIDNull() && *req.SprintID != uuid.Nil {
			updates["sprint_id"] = *req.SprintID
		} else {
			updates["sprint_id"] = nil
		}
	}
	if req.IsUserStoryIDNull() || userStoryID != nil {
		if !req.IsUserStoryIDNull() && userStoryID != nil && *userStoryID != uuid.Nil {
			updates["user_story_id"] = *userStoryID
		} else {
			updates["user_story_id"] = nil
		}
	}
	if req.StoryPoints != nil {
		updates["story_points"] = *req.StoryPoints
	}
	if req.IsDueDateNull() || req.DueDate != nil {
		if !req.IsDueDateNull() && !req.DueDate.IsZero() {
			updates["due_date"] = *req.DueDate
		} else {
			updates["due_date"] = nil
		}
	}
	if req.IsEstimatedHoursNull() || req.EstimatedHours != nil {
		if !req.IsEstimatedHoursNull() {
			updates["estimated_hours"] = *req.EstimatedHours
		} else {
			updates["estimated_hours"] = nil
		}
	}
	if req.IsActualHoursNull() || req.ActualHours != nil {
		if !req.IsActualHoursNull() {
			updates["actual_hours"] = *req.ActualHours
		} else {
			updates["actual_hours"] = nil
		}
	}

	if req.ReporterID != nil {
		updates["reporter_id"] = *req.ReporterID
	}

	if req.LabelIDs != nil {
		verifiedLabels, verifyErr := s.taskRepo.VerifyLabelIDs(req.ProjectID, *req.LabelIDs)
		if verifyErr != nil {
			return nil, verifyErr
		}

		existingMap := make(map[uuid.UUID]string)
		for _, l := range task.Labels {
			existingMap[l.ID] = l.Name
		}

		newMap := make(map[uuid.UUID]string)
		for _, l := range verifiedLabels {
			newMap[l.ID] = l.Name
		}

		var added []string
		var removed []string
		for id, name := range newMap {
			if _, exists := existingMap[id]; !exists {
				added = append(added, fmt.Sprintf("'%s'", name))
			}
		}
		for id, name := range existingMap {
			if _, exists := newMap[id]; !exists {
				removed = append(removed, fmt.Sprintf("'%s'", name))
			}
		}

		var labelChanges []string
		if len(added) > 0 {
			labelChanges = append(labelChanges, fmt.Sprintf("attached %s", strings.Join(added, ", ")))
		}
		if len(removed) > 0 {
			labelChanges = append(labelChanges, fmt.Sprintf("removed %s", strings.Join(removed, ", ")))
		}
		if len(labelChanges) > 0 {
			changes = append(changes, fmt.Sprintf("labels changed (%s)", strings.Join(labelChanges, " and ")))
		}

		err = s.taskRepo.UpdateTaskWithLabels(task.ID, updates, verifiedLabels)
	} else {
		if len(updates) == 0 {
			colorMap := s.getStatusColorMap(req.ProjectID)
			isFinalMap := s.getStatusIsFinalMap(req.ProjectID)
			res := mapToTaskResponse(*task, colorMap, isFinalMap)
			return &res, nil
		}
		err = s.taskRepo.UpdateTask(task.ID, updates)
	}
	if err != nil {
		return nil, err
	}

	// Refetch to ensure GORM relations are preloaded
	updatedTask, err := s.taskRepo.GetTaskByID(task.ID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	// 5. Create Audit Log
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

	var detail string
	if len(changes) > 0 {
		detail = fmt.Sprintf("Task '%s' updated by %s: %s", task.Title, changedBy, strings.Join(changes, ", "))
	} else {
		detail = fmt.Sprintf("Task '%s' details updated by %s", task.Title, changedBy)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		TaskID:         &task.ID,
		Action:         "updated",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Title:          task.Title,
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	isFinalMap := s.getStatusIsFinalMap(req.ProjectID)
	res := mapToTaskResponse(*updatedTask, colorMap, isFinalMap)

	if s.userStoryRepo != nil {
		if task.UserStoryID != nil && *task.UserStoryID != uuid.Nil {
			err = s.userStoryRepo.RecalculateUserStoryIsClosed(*task.UserStoryID)
			if err != nil {
				return nil, err
			}
		}
		if updatedTask.UserStoryID != nil && *updatedTask.UserStoryID != uuid.Nil {
			err = s.userStoryRepo.RecalculateUserStoryIsClosed(*updatedTask.UserStoryID)
			if err != nil {
				return nil, err
			}
		}
	}

	return &res, nil
}

func (s *taskService) RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error {
	_, user, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to restore tasks in this project",
		}
	}

	task, err := s.taskRepo.GetTaskByIDUnscoped(taskID, projectID)
	if err != nil {
		if err.Code == response.ErrNotFound {
			return &response.Error{
				Code:       response.ErrTaskPermanentlyDeleted,
				StatusCode: http.StatusGone,
				Message:    "Task is permanently deleted and cannot be restored",
			}
		}
		return err
	}

	if task.DeletedAt.Valid && time.Since(task.DeletedAt.Time) > 30*24*time.Hour {
		return &response.Error{
			Code:       response.ErrTaskPermanentlyDeleted,
			StatusCode: http.StatusGone,
			Message:    "Task is permanently deleted and cannot be restored",
		}
	}

	err = s.taskRepo.RestoreTask(taskID, projectID)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		TaskID:         &taskID,
		Action:         "restored",
		ResourceType:   "task",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("Task '%s' restored by %s", task.Title, user.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	if s.userStoryRepo != nil && task.UserStoryID != nil && *task.UserStoryID != uuid.Nil {
		_ = s.userStoryRepo.RecalculateUserStoryIsClosed(*task.UserStoryID)
	}

	return nil
}

func (s *taskService) CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	project, user, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	hasAddPermission, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "tasks", "add")
	if permErr != nil {
		return nil, permErr
	}
	if !hasAddPermission {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to clone tasks in this project",
		}
	}

	origTask, err := s.taskRepo.GetTaskByIDUnscoped(req.TaskID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	projectKey := GenerateProjectPrefix(project.Name)

	statusNameArg := string(dto.TaskStatusTodo)
	resolvedStatusID, resolvedStatusName, valErr := s.resolveStatusIDAndName(req.ProjectID, nil, &statusNameArg)
	if valErr != nil {
		// Fallback to project's default status if "todo" is not defined/found
		resolvedStatusID, resolvedStatusName, valErr = s.resolveStatusIDAndName(req.ProjectID, nil, nil)
		if valErr != nil {
			return nil, valErr
		}
	}

	var clonedTask models.Task
	clonedTask.ProjectID = origTask.ProjectID
	clonedTask.SprintID = origTask.SprintID
	clonedTask.UserStoryID = origTask.UserStoryID
	clonedTitle := origTask.Title + " (Cloned)"
	if len([]rune(clonedTitle)) > 200 {
		clonedTitle = string([]rune(clonedTitle)[:200])
	}
	clonedTask.Title = clonedTitle
	clonedTask.Description = origTask.Description
	clonedTask.Type = origTask.Type
	clonedTask.Priority = origTask.Priority
	clonedTask.StatusID = resolvedStatusID
	clonedTask.Status = resolvedStatusName
	if req.KeepAssignee {
		clonedTask.AssigneeID = origTask.AssigneeID
	}
	clonedTask.ReporterID = origTask.ReporterID
	clonedTask.StoryPoints = origTask.StoryPoints
	clonedTask.DueDate = origTask.DueDate
	clonedTask.EstimatedHours = origTask.EstimatedHours
	clonedTask.ActualHours = origTask.ActualHours

	var lastErr *response.Error
	for attempt := 0; attempt < 3; attempt++ {
		seq, err := s.taskRepo.GetNextSequenceNumber(req.ProjectID)
		if err != nil {
			return nil, err
		}
		clonedTask.SequenceNumber = seq
		clonedTask.Key = fmt.Sprintf("%s-%d", projectKey, seq)

		lastErr = s.taskRepo.CreateTask(&clonedTask)
		if lastErr == nil {
			break
		}
		if lastErr.Code != response.ErrConflict {
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		TaskID:         &clonedTask.ID,
		Action:         "cloned",
		ResourceType:   "task",
		ResourceID:     clonedTask.ID.String(),
		Details:        fmt.Sprintf("Task '%s'-'%s' cloned from '%s'-'%s' by %s", clonedTask.Title, clonedTask.Key, origTask.Title, origTask.Key, user.UserName),
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	isFinalMap := s.getStatusIsFinalMap(req.ProjectID)
	res := mapToTaskResponse(clonedTask, colorMap, isFinalMap)

	if s.userStoryRepo != nil && clonedTask.UserStoryID != nil && *clonedTask.UserStoryID != uuid.Nil {
		_ = s.userStoryRepo.RecalculateUserStoryIsClosed(*clonedTask.UserStoryID)
	}

	return &res, nil
}

func (s *taskService) GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, response.Pagination, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return nil, response.Pagination{}, err
	}
	if !authorized {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	if len(filter.StatusID) > 0 {
		customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
		if err != nil {
			return nil, response.Pagination{}, err
		}

		statusByName := make(map[string]models.CustomStatus)
		statusByID := make(map[uuid.UUID]models.CustomStatus)
		for _, cs := range customStatuses {
			statusByName[models.NormalizeTaskStatus(cs.Name)] = cs
			statusByID[cs.ID] = cs
		}

		seenIDs := make(map[uuid.UUID]bool)
		for _, statusVal := range filter.StatusID {
			if statusVal == "" {
				continue
			}
			if statusUUID, parseErr := uuid.FromString(statusVal); parseErr == nil {
				cs, exists := statusByID[statusUUID]
				if !exists {
					return nil, response.Pagination{}, &response.Error{
						Code:       response.ErrValidation,
						StatusCode: http.StatusUnprocessableEntity,
						Message:    "Invalid task status_id: status does not exist or does not belong to this project",
					}
				}
				if !seenIDs[cs.ID] {
					seenIDs[cs.ID] = true
					filter.StatusIDs = append(filter.StatusIDs, cs.ID)
				}
			} else {
				normalizedVal := models.NormalizeTaskStatus(statusVal)
				cs, exists := statusByName[normalizedVal]
				if !exists {
					return nil, response.Pagination{}, &response.Error{
						Code:       response.ErrValidation,
						StatusCode: http.StatusUnprocessableEntity,
						Message:    "Invalid task status value: status name not found in this project",
					}
				}
				if !seenIDs[cs.ID] {
					seenIDs[cs.ID] = true
					filter.StatusIDs = append(filter.StatusIDs, cs.ID)
				}
			}
		}
	}

	tasks, pagination, err := s.taskRepo.GetTasks(projectID, filter)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	colorMap := s.getStatusColorMap(projectID)
	isFinalMap := s.getStatusIsFinalMap(projectID)
	favTaskMap := s.getFavoriteTaskMap(userID)
	resList := []responsedto.TaskResponse{}
	for _, t := range tasks {
		tR := mapToTaskResponse(t, colorMap, isFinalMap)
		tR.IsFavourite = favTaskMap[t.ID]
		resList = append(resList, tR)
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "task",
		ResourceID:     projectID.String(),
		Details:        "tasks viewed",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return resList, pagination, nil
}

func (s *taskService) checkIsPMOrAdmin(projectID, userID uuid.UUID) (bool, *response.Error) {
	return CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "modify")
}

func (s *taskService) BulkUpdateTasks(req dto.BulkUpdateTasksRequest) (*responsedto.BulkUpdateTasksResponse, *response.Error) {
	isPMOrAdmin, checkErr := s.checkIsPMOrAdmin(req.ProjectID, req.UserID)
	if checkErr != nil {
		return nil, checkErr
	}
	if !isPMOrAdmin {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to bulk update tasks in this project",
		}
	}

	validStatuses := make(map[string]struct{})
	for status := range models.DefaultStatusColors {
		validStatuses[status] = struct{}{}
	}
	customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(req.ProjectID)
	if err == nil {
		for _, cs := range customStatuses {
			validStatuses[models.NormalizeTaskStatus(cs.Name)] = struct{}{}
		}
	}

	updatedCount := 0
	failedTaskIDs := []uuid.UUID{}
	failureReasons := make(map[string]string)

	for _, item := range req.Tasks {
		// 1. Fetch Task
		task, getErr := s.taskRepo.GetTaskByID(item.TaskID, req.ProjectID)
		if getErr != nil {
			failedTaskIDs = append(failedTaskIDs, item.TaskID)
			failureReasons[item.TaskID.String()] = getErr.Message
			continue
		}

		// 2. Validate Assignee
		if item.AssigneeID != nil && *item.AssigneeID != uuid.Nil {
			assigneeUser, err := s.authRepo.GetUserByID(*item.AssigneeID)
			if err != nil {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Assignee user not found"
				continue
			}
			if !assigneeUser.IsActive {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Assignee must be an active user"
				continue
			}
			isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, *item.AssigneeID)
			if err != nil {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Failed to validate assignee membership"
				continue
			}
			if !isMember {
				if assigneeUser.Role.Name == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
					isMember = true
				}
			}
			if !isMember {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Assignee must be a member of the project"
				continue
			}
		}

		// 3. Validate Sprint & completed sprint constraints
		isSprintChanging := false
		if item.SprintID != nil {
			currentSprintID := uuid.Nil
			if task.SprintID != nil {
				currentSprintID = *task.SprintID
			}
			if *item.SprintID != currentSprintID {
				isSprintChanging = true
			}
		}

		if isSprintChanging {
			if task.Sprint != nil && task.Sprint.Status == string(dto.SprintStatusCompleted) {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Changing the sprint of a task in a completed sprint is blocked"
				continue
			}
			if *item.SprintID != uuid.Nil {
				exists, err := s.taskRepo.IsSprintInProject(*item.SprintID, req.ProjectID)
				if err != nil {
					failedTaskIDs = append(failedTaskIDs, item.TaskID)
					failureReasons[item.TaskID.String()] = "Failed to validate sprint"
					continue
				}
				if !exists {
					failedTaskIDs = append(failedTaskIDs, item.TaskID)
					failureReasons[item.TaskID.String()] = "Sprint must belong to the project"
					continue
				}

				targetStatus, err := s.taskRepo.GetSprintStatus(*item.SprintID)
				if err != nil {
					failedTaskIDs = append(failedTaskIDs, item.TaskID)
					failureReasons[item.TaskID.String()] = "Failed to validate sprint status"
					continue
				}
				if targetStatus == string(dto.SprintStatusCompleted) {
					failedTaskIDs = append(failedTaskIDs, item.TaskID)
					failureReasons[item.TaskID.String()] = "Cannot assign a task to a completed sprint"
					continue
				}
			}
		}

		// 4. Validate Status Value
		isStatusChanging := false
		if item.StatusID != nil && *item.StatusID != task.StatusID {
			isStatusChanging = true
		} else if item.Status != nil && *item.Status != task.Status {
			isStatusChanging = true
		}

		var resolvedStatusID uuid.UUID
		var resolvedStatusName string

		if isStatusChanging {
			resID, resName, valErr := s.resolveStatusIDAndName(req.ProjectID, item.StatusID, item.Status)
			if valErr != nil {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = valErr.Message
				continue
			}

			if resID != task.StatusID {
				resolvedStatusID = resID
				resolvedStatusName = resName
			} else {
				isStatusChanging = false
			}
		}

		// 5. Validate Status Transition to Blocked
		if isStatusChanging && models.NormalizeTaskStatus(resolvedStatusName) == string(dto.TaskStatusBlocked) {
			if item.BlockedReason == nil || strings.TrimSpace(*item.BlockedReason) == "" {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = "Moving to Blocked requires a blocked reason"
				continue
			}
		}

		// 6. Track Changes and Update Task
		var changes []string
		updates := make(map[string]interface{})
		if isStatusChanging {
			changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", task.Status, resolvedStatusName))
			task.StatusID = resolvedStatusID
			task.Status = resolvedStatusName
			updates["status_id"] = resolvedStatusID
			updates["status"] = resolvedStatusName
			if models.NormalizeTaskStatus(resolvedStatusName) == string(dto.TaskStatusBlocked) {
				task.BlockedReason = *item.BlockedReason
				updates["blocked_reason"] = *item.BlockedReason
			} else {
				task.BlockedReason = ""
				updates["blocked_reason"] = ""
			}
		}
		if item.AssigneeID != nil {
			oldAssigneeIDStr := "nil"
			if task.AssigneeID != nil {
				oldAssigneeIDStr = task.AssigneeID.String()
			}
			newAssigneeIDStr := "nil"
			var targetAssignee *uuid.UUID = nil
			if *item.AssigneeID != uuid.Nil {
				newAssigneeIDStr = item.AssigneeID.String()
				targetAssignee = item.AssigneeID
			}
			if oldAssigneeIDStr != newAssigneeIDStr {
				oldAssignee := s.getAssigneeUsername(task.AssigneeID, task.Assignee)
				newAssignee := s.getAssigneeUsername(targetAssignee, nil)
				changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))
				task.AssigneeID = targetAssignee
				updates["assignee_id"] = targetAssignee
			}
		}
		if item.SprintID != nil {
			oldSprint := "nil"
			if task.SprintID != nil {
				oldSprint = task.SprintID.String()
			}
			newSprint := "nil"
			if *item.SprintID != uuid.Nil {
				newSprint = item.SprintID.String()
			}
			if oldSprint != newSprint {
				changes = append(changes, fmt.Sprintf("sprint changed from %s to %s", oldSprint, newSprint))
				if *item.SprintID == uuid.Nil {
					task.SprintID = nil
					updates["sprint_id"] = nil
				} else {
					task.SprintID = item.SprintID
					updates["sprint_id"] = item.SprintID
				}
			}
		}

		if len(changes) > 0 {
			updateErr := s.taskRepo.UpdateTask(task.ID, updates)
			if updateErr != nil {
				failedTaskIDs = append(failedTaskIDs, item.TaskID)
				failureReasons[item.TaskID.String()] = updateErr.Message
				continue
			}

			// Create individual task audit log
			detail := fmt.Sprintf("Task %s updated in bulk: %s", task.Key, strings.Join(changes, ", "))
			auditLog := models.AuditLog{
				UserID:         &req.UserID,
				OrganizationID: &req.OrganizationID,
				ProjectID:      &req.ProjectID,
				TaskID:         &task.ID,
				Action:         "updated",
				ResourceType:   "task",
				ResourceID:     task.ID.String(),
				Details:        detail,
				CreatedAt:      time.Now(),
				Type:           models.AuditLogTypeAudit,
			}
			if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
				s.logger.Warn("Failed to create audit log", zap.Any("error", err))
			}
		}
		updatedCount++
	}

	// Create bulk operation audit log
	var bulkDetails string
	if updatedCount > 0 {
		bulkDetails = fmt.Sprintf("Bulk update completed. Successfully updated %d tasks. Failed tasks: %d.", updatedCount, len(failedTaskIDs))
	} else {
		bulkDetails = fmt.Sprintf("Bulk update executed but 0 tasks were updated. Failed tasks: %d.", len(failedTaskIDs))
	}
	bulkAuditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "bulk_updated",
		ResourceType:   "task",
		ResourceID:     req.ProjectID.String(),
		Details:        bulkDetails,
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(bulkAuditLog); err != nil {
		s.logger.Warn("Failed to create bulk audit log", zap.Any("error", err))
	}

	if s.userStoryRepo != nil {
		affectedStoryIDs := make(map[uuid.UUID]bool)
		for _, item := range req.Tasks {
			if item.TaskID != uuid.Nil {
				if t, err := s.taskRepo.GetTaskByID(item.TaskID, req.ProjectID); err == nil && t != nil && t.UserStoryID != nil {
					affectedStoryIDs[*t.UserStoryID] = true
				}
			}
		}
		for storyID := range affectedStoryIDs {
			_ = s.userStoryRepo.RecalculateUserStoryIsClosed(storyID)
		}
	}

	return &responsedto.BulkUpdateTasksResponse{
		UpdatedCount:   updatedCount,
		FailedTaskIDs:  failedTaskIDs,
		FailureReasons: failureReasons,
	}, nil
}

func (s *taskService) checkProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	authorized, err := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "tasks", "modify")
	if err != nil {
		return false, err
	}
	return authorized, nil
}

func (s *taskService) AttachLabelToTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error {
	authorized, err := s.checkProjectMember(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to modify task labels in this project",
		}
	}

	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return err
	}

	labels, verifyErr := s.taskRepo.VerifyLabelIDs(projectID, []uuid.UUID{labelID})
	if verifyErr != nil {
		return verifyErr
	}
	label := labels[0]

	for _, l := range task.Labels {
		if l.ID == labelID {
			return nil
		}
	}

	err = s.taskRepo.AttachLabel(taskID, &label)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		TaskID:         &taskID,
		Action:         "updated",
		ResourceType:   "label",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("Task %s updated: labels changed (attached '%s')", task.Key, label.Name),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *taskService) RemoveLabelFromTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error {
	authorized, err := s.checkProjectMember(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to modify task labels in this project",
		}
	}

	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return err
	}

	labels, verifyErr := s.taskRepo.VerifyLabelIDs(projectID, []uuid.UUID{labelID})
	if verifyErr != nil {
		return verifyErr
	}
	label := labels[0]

	attached := false
	for _, l := range task.Labels {
		if l.ID == labelID {
			attached = true
			break
		}
	}
	if !attached {
		return nil
	}

	err = s.taskRepo.RemoveLabel(taskID, &label)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		TaskID:         &taskID,
		Action:         "removed",
		ResourceType:   "label",
		ResourceID:     taskID.String(),
		Type:           models.AuditLogTypeActivity,
		Details:        fmt.Sprintf("Task %s updated: labels changed (removed '%s')", task.Key, label.Name),
		CreatedAt:      time.Now(),
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *taskService) BulkDeleteTasks(req dto.BulkDeleteTasksRequest) (*responsedto.BulkDeleteTasksResponse, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	hasDeletePermission, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "tasks", "delete")
	if permErr != nil {
		return nil, permErr
	}
	if !hasDeletePermission {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete tasks in this project",
		}
	}

	deletedCount := 0
	deletedTaskIDs := []uuid.UUID{}
	failedTaskIDs := []uuid.UUID{}
	failureReasons := make(map[string]string)

	for _, taskID := range req.TaskIDs {
		// 1. Fetch task unscoped (to see if it exists at all, or if it is already soft deleted)
		task, getErr := s.taskRepo.GetTaskByIDUnscoped(taskID, req.ProjectID)
		if getErr != nil {
			failedTaskIDs = append(failedTaskIDs, taskID)
			failureReasons[taskID.String()] = getErr.Message
			continue
		}

		// 2. Check if already soft deleted
		if task.DeletedAt.Valid {
			failedTaskIDs = append(failedTaskIDs, taskID)
			failureReasons[taskID.String()] = "Task is already deleted"
			continue
		}

		// 3. Perform soft delete
		deleteErr := s.taskRepo.DeleteTask(taskID, req.ProjectID)
		if deleteErr != nil {
			failedTaskIDs = append(failedTaskIDs, taskID)
			failureReasons[taskID.String()] = deleteErr.Message
			continue
		}

		// 4. Create individual task audit log
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			TaskID:         &taskID,
			Action:         "deleted",
			ResourceType:   "task",
			ResourceID:     taskID.String(),
			Details:        fmt.Sprintf("Task %s soft deleted in bulk", task.Key),
			Type:           models.AuditLogTypeAudit,
			CreatedAt:      time.Now(),
		}
		err = s.auditRepo.CreateAuditLog(auditLog)
		if err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}

		deletedCount++
		deletedTaskIDs = append(deletedTaskIDs, taskID)
		if task.UserStoryID != nil && *task.UserStoryID != uuid.Nil && s.userStoryRepo != nil {
			_ = s.userStoryRepo.RecalculateUserStoryIsClosed(*task.UserStoryID)
		}
	}

	// Create bulk operation audit log
	var bulkDetails string
	if deletedCount > 0 {
		bulkDetails = fmt.Sprintf("Bulk deletion completed. Successfully deleted %d tasks. Failed tasks: %d.", deletedCount, len(failedTaskIDs))
	} else {
		bulkDetails = fmt.Sprintf("Bulk deletion executed but 0 tasks were deleted. Failed tasks: %d.", len(failedTaskIDs))
	}
	bulkAuditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "bulk_deleted",
		ResourceType:   "task",
		ResourceID:     req.ProjectID.String(),
		Details:        bulkDetails,
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	err = s.auditRepo.CreateAuditLog(bulkAuditLog)
	if err != nil {
		s.logger.Warn("Failed to create bulk audit log", zap.Any("error", err))
	}

	return &responsedto.BulkDeleteTasksResponse{
		DeletedCount:   deletedCount,
		DeletedTaskIDs: deletedTaskIDs,
		FailedTaskIDs:  failedTaskIDs,
		FailureReasons: failureReasons,
	}, nil
}

func isFibonacci(n int) bool {
	if n < 0 {
		return false
	}
	if n == 0 || n == 1 {
		return true
	}
	a, b := 1, 1
	for b < n {
		a, b = b, a+b
	}
	return b == n
}

func isBackdated(t time.Time) bool {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tLocal := t.In(now.Location())
	tDate := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day(), 0, 0, 0, 0, now.Location())
	return tDate.Before(today)
}

func (s *taskService) AssignTaskToMe(taskID, userID, organizationID, projectID uuid.UUID) (*responsedto.TaskResponse, *response.Error) {
	accessCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, err
	}

	if accessCtx.ProjectID != projectID {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Task does not belong to the specified project",
		}
	}

	project, user, authorized, err := s.checkAuthorization(accessCtx.ProjectID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view tasks in this project",
		}
	}

	hasModifyPermission, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, accessCtx.ProjectID, "tasks", "modify")
	if permErr != nil {
		return nil, permErr
	}
	if !hasModifyPermission {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update tasks in this project",
		}
	}

	isMember, projectErr := s.projectRepo.IsUserProjectMember(accessCtx.ProjectID, userID)
	if projectErr != nil {
		return nil, projectErr
	}
	if !isMember {
		if user.Role.Name == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
			isMember = true
		}
	}
	if !isMember || !user.IsActive {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You must be an active project member to assign tasks to yourself",
		}
	}

	task, err := s.taskRepo.GetTaskByID(taskID, accessCtx.ProjectID)
	if err != nil {
		return nil, err
	}

	var changes []string
	oldAssigneeIDStr := "nil"
	if task.AssigneeID != nil {
		oldAssigneeIDStr = task.AssigneeID.String()
	}
	newAssigneeIDStr := userID.String()

	if oldAssigneeIDStr != newAssigneeIDStr {
		oldAssignee := s.getAssigneeUsername(task.AssigneeID, task.Assignee)
		newAssignee := s.getAssigneeUsername(&userID, &user)
		changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))

		updates := map[string]interface{}{
			"assignee_id": userID,
		}

		err = s.taskRepo.UpdateTask(task.ID, updates)
		if err != nil {
			return nil, err
		}

		// Refetch task to ensure preloaded values are fresh
		task, err = s.taskRepo.GetTaskByID(task.ID, accessCtx.ProjectID)
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
			changedBy = userID.String()
		}

		var detail string
		if len(changes) > 0 {
			detail = fmt.Sprintf("Task '%s' updated by %s: %s", task.Title, changedBy, strings.Join(changes, ", "))
		} else {
			detail = fmt.Sprintf("Task '%s' details updated by %s", task.Title, changedBy)
		}

		auditLog := models.AuditLog{
			UserID:         &userID,
			OrganizationID: &organizationID,
			ProjectID:      &accessCtx.ProjectID,
			TaskID:         &task.ID,
			Action:         "updated",
			ResourceType:   "task",
			ResourceID:     task.ID.String(),
			Title:          task.Title,
			Details:        detail,
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now(),
		}
		if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}
	}

	colorMap := s.getStatusColorMap(accessCtx.ProjectID)
	isFinalMap := s.getStatusIsFinalMap(accessCtx.ProjectID)
	res := mapToTaskResponse(*task, colorMap, isFinalMap)
	return &res, nil
}

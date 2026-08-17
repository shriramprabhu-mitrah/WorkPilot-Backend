package services

import (
	"fmt"
	"maps"
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
	"go.uber.org/zap"
)

type TaskService interface {
	CreateTask(req dto.CreateTaskRequest) (uuid.UUID, *responsedto.TaskResponse, *response.Error)
	GetTaskByID(taskID, projectID, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error)
	UpdateTask(req dto.UpdateTaskRequest) (*responsedto.TaskResponse, *response.Error)
	RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error
	CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error)
	GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, response.Pagination, *response.Error)
	BulkUpdateTasks(req dto.BulkUpdateTasksRequest) (*responsedto.BulkUpdateTasksResponse, *response.Error)
	BulkDeleteTasks(req dto.BulkDeleteTasksRequest) (*responsedto.BulkDeleteTasksResponse, *response.Error)
	AttachLabelToTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
	RemoveLabelFromTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
}

type taskService struct {
	authRepo         authrepo.AuthRepository
	projectRepo      projectrepo.ProjectRepository
	taskRepo         taskrepo.TaskRepository
	auditRepo        auditrepo.AuditLogRepository
	customStatusRepo customstatusrepo.CustomStatusRepository
	logger           *zap.Logger
}

func InitTaskService(
	authRepo authrepo.AuthRepository,
	projectRepo projectrepo.ProjectRepository,
	taskRepo taskrepo.TaskRepository,
	auditRepo auditrepo.AuditLogRepository,
	customStatusRepo customstatusrepo.CustomStatusRepository,
	logger *zap.Logger,
) TaskService {
	return &taskService{
		authRepo:         authRepo,
		projectRepo:      projectRepo,
		taskRepo:         taskRepo,
		auditRepo:        auditRepo,
		customStatusRepo: customStatusRepo,
		logger:           logger,
	}
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

func GenerateProjectPrefix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "TASK"
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})

	var prefix string
	if len(parts) > 1 {
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) > 0 {
				prefix += strings.ToUpper(string(part[0]))
			}
		}
	} else {
		runes := []rune(strings.ToUpper(name))
		if len(runes) > 3 {
			prefix = string(runes[:3])
		} else {
			prefix = string(runes)
		}
	}

	cleaned := ""
	for _, r := range prefix {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			cleaned += string(r)
		}
	}

	if len(cleaned) < 2 {
		cleaned = "WP"
	}
	if len(cleaned) > 10 {
		cleaned = cleaned[:10]
	}
	return cleaned
}

func mapToTaskResponse(task models.Task, colorMaps ...map[string]string) responsedto.TaskResponse {
	var colorMap map[string]string
	if len(colorMaps) > 0 {
		colorMap = colorMaps[0]
	} else {
		colorMap = make(map[string]string)
		maps.Copy(colorMap, models.DefaultStatusColors)
	}

	var sprintName string
	if task.Sprint != nil {
		sprintName = task.Sprint.Name
	}
	var assigneeName string
	if task.Assignee != nil {
		assigneeName = task.Assignee.FullName
	}

	labelsRes := []responsedto.LabelResponse{}
	for _, l := range task.Labels {
		labelsRes = append(labelsRes, responsedto.LabelFromModel(l))
	}

	statusColor := colorMap[models.NormalizeTaskStatus(task.Status)]
	if statusColor == "" {
		statusColor = "#808080"
	}

	response := responsedto.TaskResponse{
		ID:             task.ID,
		ProjectID:      task.ProjectID,
		SprintID:       task.SprintID,
		SprintName:     sprintName,
		UserStoryID:    task.UserStoryID,
		Key:            task.Key,
		Title:          task.Title,
		Description:    task.Description,
		Type:           task.Type,
		Priority:       task.Priority,
		StatusID:       task.StatusID,
		Status:         task.Status,
		StatusColor:    statusColor,
		AssigneeID:     task.AssigneeID,
		ReporterID:     task.ReporterID,
		AssigneeName:   assigneeName,
		StoryPoints:    task.StoryPoints,
		DueDate:        task.DueDate,
		EstimatedHours: task.EstimatedHours,
		ActualHours:    task.ActualHours,
		BlockedReason:  task.BlockedReason,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
		Labels:         labelsRes,
	}

	if task.Reporter != nil {
		response.User = &responsedto.UserSummary{
			ID:        task.Reporter.ID,
			FullName:  task.Reporter.FullName,
			Email:     task.Reporter.Email,
			AvatarURL: task.Reporter.AvatarURL,
			Role:      task.Reporter.Role,
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
	project, _, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if !authorized {
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
			if assigneeUser.Role == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
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

	// Validation: User Story must belong to the same project
	if req.UserStoryID != nil && *req.UserStoryID != uuid.Nil {
		inProject, err := s.taskRepo.IsUserStoryInProject(*req.UserStoryID, req.ProjectID)
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
	task.UserStoryID = req.UserStoryID
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

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "task_created",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Details:        fmt.Sprintf("Task %s created", task.Key),
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	res := mapToTaskResponse(task, colorMap)
	return task.ID, &res, nil
}

func (s *taskService) GetTaskByID(taskID, projectID, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error) {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
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

	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, err
	}

	colorMap := s.getStatusColorMap(projectID)
	res := mapToTaskResponse(*task, colorMap)

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "task_viewed",
		ResourceType:   "task",
		ResourceID:     taskID.String(),
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
	project, user, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update tasks in this project",
		}
	}

	isPMOrAdmin := (user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID)

	var member *models.ProjectMember
	if !isPMOrAdmin {
		member, err = s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
		if err != nil {
			return nil, err
		}

		if member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager) {
			isPMOrAdmin = true
		} else if member.ProjectRole == string(dto.ProjectRoleViewer) {
			return nil, &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Viewers do not have permission to update tasks",
			}
		}
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

	// Validation: User Story must belong to the same project
	if req.UserStoryID != nil && *req.UserStoryID != uuid.Nil {
		inProject, err := s.taskRepo.IsUserStoryInProject(*req.UserStoryID, req.ProjectID)
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
	if req.DueDate != nil && !req.DueDate.IsZero() {
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
	if req.SprintID != nil {
		currentSprintID := uuid.Nil
		if task.SprintID != nil {
			currentSprintID = *task.SprintID
		}
		if *req.SprintID != currentSprintID {
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
		if *req.SprintID != uuid.Nil {
			targetStatus, err := s.taskRepo.GetSprintStatus(*req.SprintID)
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
	if req.AssigneeID != nil {
		oldAssignee := "nil"
		if task.AssigneeID != nil {
			oldAssignee = task.AssigneeID.String()
		}
		newAssignee := "nil"
		if *req.AssigneeID != uuid.Nil {
			newAssignee = req.AssigneeID.String()
		}
		if oldAssignee != newAssignee {
			changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))
			if *req.AssigneeID == uuid.Nil {
				task.AssigneeID = nil
			} else {
				task.AssigneeID = req.AssigneeID
			}
		}
	}
	if req.SprintID != nil {
		oldSprint := "nil"
		if task.SprintID != nil {
			oldSprint = task.SprintID.String()
		}
		newSprint := "nil"
		if *req.SprintID != uuid.Nil {
			newSprint = req.SprintID.String()
		}
		if oldSprint != newSprint {
			changes = append(changes, fmt.Sprintf("sprint changed from %s to %s", oldSprint, newSprint))
			if *req.SprintID == uuid.Nil {
				task.SprintID = nil
			} else {
				task.SprintID = req.SprintID
			}
		}
	}
	if req.UserStoryID != nil {
		oldStory := "nil"
		if task.UserStoryID != nil {
			oldStory = task.UserStoryID.String()
		}
		newStory := "nil"
		if *req.UserStoryID != uuid.Nil {
			newStory = req.UserStoryID.String()
		}
		if oldStory != newStory {
			changes = append(changes, fmt.Sprintf("user story changed from %s to %s", oldStory, newStory))
			if *req.UserStoryID == uuid.Nil {
				task.UserStoryID = nil
			} else {
				task.UserStoryID = req.UserStoryID
			}
		}
	}
	if req.StoryPoints != nil && *req.StoryPoints != task.StoryPoints {
		changes = append(changes, fmt.Sprintf("story points changed from %d to %d", task.StoryPoints, *req.StoryPoints))
		task.StoryPoints = *req.StoryPoints
	}
	if req.DueDate != nil {
		oldDue := "nil"
		if task.DueDate != nil {
			oldDue = task.DueDate.Format(time.RFC3339)
		}
		newDue := "nil"
		if !req.DueDate.IsZero() {
			newDue = req.DueDate.Format(time.RFC3339)
		}
		if oldDue != newDue {
			changes = append(changes, fmt.Sprintf("due date changed from %s to %s", oldDue, newDue))
			if req.DueDate.IsZero() {
				task.DueDate = nil
			} else {
				task.DueDate = req.DueDate
			}
		}
	}
	if req.EstimatedHours != nil {
		oldEst := "nil"
		if task.EstimatedHours != nil {
			oldEst = fmt.Sprintf("%.2f", *task.EstimatedHours)
		}
		newEst := "nil"
		if *req.EstimatedHours >= 0 {
			newEst = fmt.Sprintf("%.2f", *req.EstimatedHours)
		}
		if oldEst != newEst {
			changes = append(changes, fmt.Sprintf("estimated hours changed from %s to %s", oldEst, newEst))
			task.EstimatedHours = req.EstimatedHours
		}
	}
	if req.ActualHours != nil {
		oldAct := "nil"
		if task.ActualHours != nil {
			oldAct = fmt.Sprintf("%.2f", *task.ActualHours)
		}
		newAct := fmt.Sprintf("%.2f", *req.ActualHours)
		if oldAct != newAct {
			changes = append(changes, fmt.Sprintf("actual hours changed from %s to %s", oldAct, newAct))
			task.ActualHours = req.ActualHours
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
	if req.AssigneeID != nil {
		if *req.AssigneeID == uuid.Nil {
			updates["assignee_id"] = nil
		} else {
			updates["assignee_id"] = *req.AssigneeID
		}
	}
	if req.SprintID != nil {
		if *req.SprintID == uuid.Nil {
			updates["sprint_id"] = nil
		} else {
			updates["sprint_id"] = *req.SprintID
		}
	}
	if req.UserStoryID != nil {
		if *req.UserStoryID == uuid.Nil {
			updates["user_story_id"] = nil
		} else {
			updates["user_story_id"] = *req.UserStoryID
		}
	}
	if req.StoryPoints != nil {
		updates["story_points"] = *req.StoryPoints
	}
	if req.DueDate != nil {
		if req.DueDate.IsZero() {
			updates["due_date"] = nil
		} else {
			updates["due_date"] = *req.DueDate
		}
	}
	if req.EstimatedHours != nil {
		updates["estimated_hours"] = *req.EstimatedHours
	}
	if req.ActualHours != nil {
		updates["actual_hours"] = *req.ActualHours
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
			res := mapToTaskResponse(*task, colorMap)
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
	var detail string
	if len(changes) > 0 {
		detail = fmt.Sprintf("Task %s updated: %s", task.Key, strings.Join(changes, ", "))
	} else {
		detail = fmt.Sprintf("Task %s updated", task.Key)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "task_updated",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	res := mapToTaskResponse(*updatedTask, colorMap)
	return &res, nil
}

func (s *taskService) RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error {
	_, _, authorized, err := s.checkAuthorization(projectID, userID)
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
		Action:         "task_restored",
		ResourceType:   "task",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("Task %s restored", task.Key),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *taskService) CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	project, _, authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
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

	var clonedTask models.Task
	clonedTask.ProjectID = origTask.ProjectID
	clonedTask.SprintID = origTask.SprintID
	clonedTitle := origTask.Title + " (Cloned)"
	if len([]rune(clonedTitle)) > 200 {
		clonedTitle = string([]rune(clonedTitle)[:200])
	}
	clonedTask.Title = clonedTitle
	clonedTask.Description = origTask.Description
	clonedTask.Type = origTask.Type
	clonedTask.Priority = origTask.Priority
	clonedTask.Status = string(dto.TaskStatusTodo)
	if req.KeepAssignee {
		clonedTask.AssigneeID = origTask.AssigneeID
	}
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
		Action:         "task_cloned",
		ResourceType:   "task",
		ResourceID:     clonedTask.ID.String(),
		Details:        fmt.Sprintf("Task %s cloned from %s", clonedTask.Key, origTask.Key),
		CreatedAt:      time.Now(),
		Type:           models.AuditLogTypeActivity,
	}
	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	colorMap := s.getStatusColorMap(req.ProjectID)
	res := mapToTaskResponse(clonedTask, colorMap)
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

	tasks, pagination, err := s.taskRepo.GetTasks(projectID, filter)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	colorMap := s.getStatusColorMap(projectID)
	resList := []responsedto.TaskResponse{}
	for _, t := range tasks {
		resList = append(resList, mapToTaskResponse(t, colorMap))
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "tasks_viewed",
		ResourceType:   "task",
		ResourceID:     projectID.String(),
		Type:           models.AuditLogTypeView,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return resList, pagination, nil
}

func (s *taskService) checkIsPMOrAdmin(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	if user.Role == string(dto.RoleSuperAdmin) {
		return false, nil
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}
	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(userID, projectID)
	if err != nil {
		return false, nil
	}
	if member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager) {
		return true, nil
	}
	return false, nil
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
				if assigneeUser.Role == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
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
			oldAssignee := "nil"
			if task.AssigneeID != nil {
				oldAssignee = task.AssigneeID.String()
			}
			newAssignee := "nil"
			if *item.AssigneeID != uuid.Nil {
				newAssignee = item.AssigneeID.String()
			}
			if oldAssignee != newAssignee {
				changes = append(changes, fmt.Sprintf("assignee changed from %s to %s", oldAssignee, newAssignee))
				if *item.AssigneeID == uuid.Nil {
					task.AssigneeID = nil
					updates["assignee_id"] = nil
				} else {
					task.AssigneeID = item.AssigneeID
					updates["assignee_id"] = item.AssigneeID
				}
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
				Action:         "task_updated",
				ResourceType:   "task",
				ResourceID:     task.ID.String(),
				Details:        detail,
				CreatedAt:      time.Now(),
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
		Action:         "tasks_bulk_updated",
		ResourceType:   "task",
		ResourceID:     req.ProjectID.String(),
		Details:        bulkDetails,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(bulkAuditLog); err != nil {
		s.logger.Warn("Failed to create bulk audit log", zap.Any("error", err))
	}

	return &responsedto.BulkUpdateTasksResponse{
		UpdatedCount:   updatedCount,
		FailedTaskIDs:  failedTaskIDs,
		FailureReasons: failureReasons,
	}, nil
}

func (s *taskService) checkProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(userID, projectID)
	if err != nil {
		return false, err
	}

	if member.ProjectRole == string(dto.ProjectRoleViewer) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Viewers do not have permission to modify task labels",
		}
	}

	return true, nil
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
		Action:         "task_updated",
		ResourceType:   "task",
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
		Action:         "task_updated",
		ResourceType:   "task",
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
			Action:         "task_deleted",
			ResourceType:   "task",
			ResourceID:     taskID.String(),
			Details:        fmt.Sprintf("Task %s soft deleted in bulk", task.Key),
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now(),
		}
		err = s.auditRepo.CreateAuditLog(auditLog)
		if err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}

		deletedCount++
		deletedTaskIDs = append(deletedTaskIDs, taskID)
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
		Action:         "tasks_bulk_deleted",
		ResourceType:   "task",
		ResourceID:     req.ProjectID.String(),
		Details:        bulkDetails,
		Type:           models.AuditLogTypeActivity,
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

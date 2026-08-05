package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type TaskService interface {
	CreateTask(req dto.CreateTaskRequest) (*responsedto.TaskResponse, *response.Error)
	GetTaskByID(taskID, projectID, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error)
	UpdateTask(req dto.UpdateTaskRequest) (*responsedto.TaskResponse, *response.Error)
	DeleteTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error
	RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error
	CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error)
	GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, response.Pagination, *response.Error)
	AttachLabelToTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
	RemoveLabelFromTask(projectID, taskID, labelID, userID, orgID uuid.UUID) *response.Error
}

type taskService struct {
	authRepo    authrepo.AuthRepository
	projectRepo projectrepo.ProjectRepository
	taskRepo    taskrepo.TaskRepository
	logger      *zap.Logger
}

func InitTaskService(authRepo authrepo.AuthRepository, projectRepo projectrepo.ProjectRepository, taskRepo taskrepo.TaskRepository, logger *zap.Logger) TaskService {
	return &taskService{
		authRepo:    authRepo,
		projectRepo: projectRepo,
		taskRepo:    taskRepo,
		logger:      logger,
	}
}

func (s *taskService) checkAuthorization(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	if user.Role == string(dto.RoleSuperAdmin) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}
	isMember, err := s.projectRepo.IsUserProjectMember(projectID, userID)
	if err != nil {
		return false, err
	}
	return isMember, nil
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

func mapToTaskResponse(task models.Task) responsedto.TaskResponse {
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

	return responsedto.TaskResponse{
		ID:             task.ID,
		ProjectID:      task.ProjectID,
		SprintID:       task.SprintID,
		SprintName:     sprintName,
		Key:            task.Key,
		Title:          task.Title,
		Description:    task.Description,
		Type:           task.Type,
		Priority:       task.Priority,
		Status:         task.Status,
		AssigneeID:     task.AssigneeID,
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
}

func (s *taskService) CreateTask(req dto.CreateTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to create tasks in this project",
		}
	}

	project, err := s.projectRepo.GetProjectByID(req.ProjectID)
	if err != nil {
		return nil, err
	}
	projectKey := GenerateProjectPrefix(project.Name)

	var task models.Task
	task.ProjectID = req.ProjectID
	task.SprintID = req.SprintID
	task.Title = req.Title
	task.Description = req.Description
	task.Type = req.Type
	task.Priority = req.Priority
	if req.Status != "" {
		task.Status = req.Status
	} else {
		task.Status = string(dto.TaskStatusTodo)
	}
	task.AssigneeID = req.AssigneeID
	task.StoryPoints = req.StoryPoints
	task.DueDate = req.DueDate
	task.EstimatedHours = req.EstimatedHours
	task.ActualHours = req.ActualHours

	var labels []models.Label
	if len(req.LabelIDs) > 0 {
		var verifyErr *response.Error
		labels, verifyErr = s.taskRepo.VerifyLabelIDs(req.ProjectID, req.LabelIDs)
		if verifyErr != nil {
			return nil, verifyErr
		}
	}
	task.Labels = labels

	var lastErr *response.Error
	for attempt := 0; attempt < 3; attempt++ {
		seq, err := s.taskRepo.GetNextSequenceNumber(req.ProjectID)
		if err != nil {
			return nil, err
		}
		task.SequenceNumber = seq
		task.Key = fmt.Sprintf("%s-%d", projectKey, seq)

		lastErr = s.taskRepo.CreateTask(&task)
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
		Action:         "task_created",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Details:        fmt.Sprintf("Task %s created", task.Key),
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	res := mapToTaskResponse(task)
	return &res, nil
}

func (s *taskService) GetTaskByID(taskID, projectID, userID, orgID uuid.UUID) (*responsedto.TaskResponse, *response.Error) {
	authorized, err := s.checkAuthorization(projectID, userID)
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

	res := mapToTaskResponse(*task)
	return &res, nil
}

func (s *taskService) UpdateTask(req dto.UpdateTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
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

	project, err := s.projectRepo.GetProjectByID(req.ProjectID)
	if err != nil {
		return nil, err
	}

	user, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		return nil, err
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

	// 1. Validate Assignee Membership
	if req.AssigneeID != nil && *req.AssigneeID != uuid.Nil {
		isMember, err := s.projectRepo.IsUserProjectMember(req.ProjectID, *req.AssigneeID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			assigneeUser, err := s.authRepo.GetUserByID(*req.AssigneeID)
			if err == nil {
				if assigneeUser.Role == string(dto.RoleOrgAdmin) && assigneeUser.OrganizationID != nil && *assigneeUser.OrganizationID == req.OrganizationID {
					isMember = true
				}
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

	// 2. Validate Actual Hours Update
	if req.ActualHours != nil {
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

	// 3. Validate Status Transition
	if req.Status != nil && *req.Status != task.Status {
		newStatus := *req.Status
		oldStatus := task.Status

		if newStatus == string(dto.TaskStatusBlocked) {
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
			switch oldStatus {
			case string(dto.TaskStatusTodo):
				allowedTransition = newStatus == string(dto.TaskStatusInProgress) || newStatus == string(dto.TaskStatusBlocked)
			case string(dto.TaskStatusInProgress):
				allowedTransition = newStatus == string(dto.TaskStatusInReview) || newStatus == string(dto.TaskStatusBlocked)
			case string(dto.TaskStatusInReview):
				allowedTransition = newStatus == string(dto.TaskStatusTesting) || newStatus == string(dto.TaskStatusBlocked)
			case string(dto.TaskStatusTesting):
				allowedTransition = newStatus == string(dto.TaskStatusCompleted) || newStatus == string(dto.TaskStatusBlocked)
			case string(dto.TaskStatusCompleted):
				allowedTransition = newStatus == string(dto.TaskStatusBlocked)
			case string(dto.TaskStatusBlocked):
				allowedTransition = newStatus == string(dto.TaskStatusInProgress) || newStatus == string(dto.TaskStatusTodo)
			default:
				allowedTransition = false
			}

			if !allowedTransition {
				return nil, &response.Error{
					Code:       response.ErrInvalidStatusTransition,
					StatusCode: http.StatusBadRequest,
					Message:    fmt.Sprintf("Invalid status transition from %s to %s for developers", oldStatus, newStatus),
				}
			}
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
		task.Description = *req.Description
	}
	if req.Type != nil && *req.Type != task.Type {
		changes = append(changes, fmt.Sprintf("type changed from '%s' to '%s'", task.Type, *req.Type))
		task.Type = *req.Type
	}
	if req.Priority != nil && *req.Priority != task.Priority {
		changes = append(changes, fmt.Sprintf("priority changed from '%s' to '%s'", task.Priority, *req.Priority))
		task.Priority = *req.Priority
	}
	if req.Status != nil && *req.Status != task.Status {
		changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", task.Status, *req.Status))
		task.Status = *req.Status
		if *req.Status == string(dto.TaskStatusBlocked) {
			task.BlockedReason = *req.BlockedReason
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

		assocErr := s.taskRepo.UpdateTaskLabels(task.ID, verifiedLabels)
		if assocErr != nil {
			return nil, assocErr
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
	}

	err = s.taskRepo.UpdateTask(task)
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
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	res := mapToTaskResponse(*updatedTask)
	return &res, nil
}

func (s *taskService) DeleteTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error {
	authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete tasks in this project",
		}
	}

	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return err
	}

	err = s.taskRepo.DeleteTask(taskID, projectID)
	if err != nil {
		return err
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "task_deleted",
		ResourceType:   "task",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("Task %s soft deleted", task.Key),
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *taskService) RestoreTask(taskID, projectID, userID, orgID uuid.UUID) *response.Error {
	authorized, err := s.checkAuthorization(projectID, userID)
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
		return err
	}

	if task.DeletedAt.Valid && time.Since(task.DeletedAt.Time) > 30*24*time.Hour {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Task retention period has expired and cannot be restored",
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
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *taskService) CloneTask(req dto.CloneTaskRequest) (*responsedto.TaskResponse, *response.Error) {
	authorized, err := s.checkAuthorization(req.ProjectID, req.UserID)
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

	project, err := s.projectRepo.GetProjectByID(req.ProjectID)
	if err != nil {
		return nil, err
	}
	projectKey := GenerateProjectPrefix(project.Name)

	var clonedTask models.Task
	clonedTask.ProjectID = origTask.ProjectID
	clonedTask.SprintID = origTask.SprintID
	clonedTask.Title = origTask.Title + " (Cloned)"
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
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	res := mapToTaskResponse(clonedTask)
	return &res, nil
}

func (s *taskService) GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, response.Pagination, *response.Error) {
	authorized, err := s.checkAuthorization(projectID, userID)
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

	resList := []responsedto.TaskResponse{}
	for _, t := range tasks {
		resList = append(resList, mapToTaskResponse(t))
	}
	return resList, pagination, nil
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
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
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
		Details:        fmt.Sprintf("Task %s updated: labels changed (removed '%s')", task.Key, label.Name),
		CreatedAt:      time.Now(),
	}
	if err := s.projectRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

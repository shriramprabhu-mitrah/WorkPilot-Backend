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
	GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, *response.Error)
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
	if user.Role == string(models.RoleSuperAdmin) {
		return true, nil
	}
	if user.Role == string(models.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
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
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
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
		task.Status = "todo"
	}
	task.AssigneeID = req.AssigneeID
	task.StoryPoints = req.StoryPoints
	task.DueDate = req.DueDate
	task.EstimatedHours = req.EstimatedHours
	task.ActualHours = req.ActualHours

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

	task, err := s.taskRepo.GetTaskByID(req.TaskID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Type != nil {
		task.Type = *req.Type
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	task.AssigneeID = req.AssigneeID
	task.SprintID = req.SprintID
	if req.StoryPoints != nil {
		task.StoryPoints = *req.StoryPoints
	}
	task.DueDate = req.DueDate
	task.EstimatedHours = req.EstimatedHours
	task.ActualHours = req.ActualHours

	err = s.taskRepo.UpdateTask(task)
	if err != nil {
		return nil, err
	}

	// Refetch to ensure GORM relations are preloaded
	updatedTask, err := s.taskRepo.GetTaskByID(task.ID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "task_updated",
		ResourceType:   "task",
		ResourceID:     task.ID.String(),
		Details:        fmt.Sprintf("Task %s updated", task.Key),
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
	clonedTask.Status = "todo"
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

func (s *taskService) GetTasks(projectID, userID, orgID uuid.UUID, filter dto.TaskFilter) ([]responsedto.TaskResponse, *response.Error) {
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

	tasks, err := s.taskRepo.GetTasks(projectID, filter)
	if err != nil {
		return nil, err
	}

	resList := []responsedto.TaskResponse{}
	for _, t := range tasks {
		resList = append(resList, mapToTaskResponse(t))
	}
	return resList, nil
}

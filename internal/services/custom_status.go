package services

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type CustomStatusService interface {
	CreateStatus(req requestdto.CreateCustomStatusRequest) (*responsedto.CustomStatusResponse, *response.Error)
	GetStatuses(projectID, userID, orgID uuid.UUID) ([]responsedto.CustomStatusResponse, *response.Error)
	UpdateStatus(req requestdto.UpdateCustomStatusRequest) (*responsedto.CustomStatusResponse, *response.Error)
	DeleteStatus(statusID, projectID, userID, orgID uuid.UUID) *response.Error
}

type customStatusService struct {
	statusRepo  customstatusrepo.CustomStatusRepository
	projectRepo projectrepo.ProjectRepository
	authRepo    authrepo.AuthRepository
	auditRepo   auditrepo.AuditLogRepository
	taskRepo    taskrepo.TaskRepository
	logger      *zap.Logger
}

func InitCustomStatusService(
	statusRepo customstatusrepo.CustomStatusRepository,
	projectRepo projectrepo.ProjectRepository,
	authRepo authrepo.AuthRepository,
	auditRepo auditrepo.AuditLogRepository,
	taskRepo taskrepo.TaskRepository,
	logger *zap.Logger,
) CustomStatusService {
	return &customStatusService{
		statusRepo:  statusRepo,
		projectRepo: projectRepo,
		authRepo:    authRepo,
		auditRepo:   auditRepo,
		taskRepo:    taskRepo,
		logger:      logger,
	}
}

func (s *customStatusService) checkAdminOrPM(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	isPMOrAdmin := (user.Role == string(requestdto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID)

	if !isPMOrAdmin {
		member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(userID, projectID)
		if err != nil {
			return false, err
		}
		if member.ProjectRole == string(requestdto.ProjectRoleOrgAdmin) || member.ProjectRole == string(requestdto.ProjectRoleProjectManager) {
			isPMOrAdmin = true
		}
	}

	return isPMOrAdmin, nil
}

func (s *customStatusService) checkProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
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

func (s *customStatusService) CreateStatus(req requestdto.CreateCustomStatusRequest) (*responsedto.CustomStatusResponse, *response.Error) {
	// 1. Authorization
	authorized, err := s.checkAdminOrPM(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage custom statuses in this project",
		}
	}

	// 2. Validation & Normalization
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 50 {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    "Status name must be between 1 and 50 characters",
		}
	}

	normalizedName := models.NormalizeTaskStatus(req.Name)

	if !requestdto.ValidateColor(req.Color) {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    "Status color must be a valid hexadecimal color code (#RRGGBB)",
		}
	}

	if req.DisplayOrder < 0 {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusUnprocessableEntity,
			Message:    "Display order must be greater than or equal to 0",
		}
	}

	// 3. Uniqueness Check
	exists, err := s.statusRepo.IsStatusNameExists(req.ProjectID, normalizedName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Status name already exists in this project",
		}
	}

	// 4. Create status (casing is preserved)
	status := models.CustomStatus{
		ProjectID:    req.ProjectID,
		Name:         req.Name,
		Color:        req.Color,
		DisplayOrder: req.DisplayOrder,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.statusRepo.CreateStatus(&status)
	if err != nil {
		return nil, err
	}

	// 5. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "created",
		ResourceType:   "custom_status",
		ResourceID:     status.ID.String(),
		Details:        fmt.Sprintf("Custom Status '%s' created", status.Name),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	res := responsedto.CustomStatusFromModel(status)
	return &res, nil
}

func (s *customStatusService) GetStatuses(projectID, userID, orgID uuid.UUID) ([]responsedto.CustomStatusResponse, *response.Error) {
	// 1. Authorization
	authorized, err := s.checkProjectMember(projectID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view custom statuses in this project",
		}
	}

	// 2. Get custom statuses from repository
	customStatuses, err := s.statusRepo.GetStatusesByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	// 3. Map all statuses to response objects
	res := make([]responsedto.CustomStatusResponse, 0, len(customStatuses))
	for _, cs := range customStatuses {
		res = append(res, responsedto.CustomStatusFromModel(cs))
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].DisplayOrder < res[j].DisplayOrder
	})

	// Re-assign display_order dynamically to be strictly sequential (0 to N-1)
	for idx := range res {
		res[idx].DisplayOrder = idx
	}

	// 4. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "custom_status",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return res, nil
}

func (s *customStatusService) UpdateStatus(req requestdto.UpdateCustomStatusRequest) (*responsedto.CustomStatusResponse, *response.Error) {
	// 1. Authorization
	authorized, err := s.checkAdminOrPM(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage custom statuses in this project",
		}
	}

	// 2. Get existing status
	status, err := s.statusRepo.GetStatusByID(req.StatusID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	oldName := status.Name

	// 3. Validation and updates
	updated := false
	if req.Name != nil {
		trimmedName := strings.TrimSpace(*req.Name)
		if trimmedName == "" || len(trimmedName) > 50 {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Status name must be between 1 and 50 characters",
			}
		}

		normalizedName := models.NormalizeTaskStatus(trimmedName)

		// Check uniqueness if name is changing
		if normalizedName != models.NormalizeTaskStatus(status.Name) {
			exists, err := s.statusRepo.IsStatusNameExists(req.ProjectID, normalizedName)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, &response.Error{
					Code:       response.ErrConflict,
					StatusCode: http.StatusConflict,
					Message:    "Status name already exists in this project",
				}
			}
		}

		if trimmedName != status.Name {
			status.Name = trimmedName
			updated = true
		}
	}

	if req.Color != nil {
		if !requestdto.ValidateColor(*req.Color) {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Status color must be a valid hexadecimal color code (#RRGGBB)",
			}
		}
		if *req.Color != status.Color {
			status.Color = *req.Color
			updated = true
		}
	}

	if req.DisplayOrder != nil {
		if *req.DisplayOrder < 0 {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusUnprocessableEntity,
				Message:    "Display order must be greater than or equal to 0",
			}
		}
		if *req.DisplayOrder != status.DisplayOrder {
			status.DisplayOrder = *req.DisplayOrder
			updated = true
		}
	}

	if updated {
		status.UpdatedAt = time.Now()
		err = s.statusRepo.UpdateStatus(status)
		if err != nil {
			return nil, err
		}

		if oldName != status.Name {
			taskErr := s.taskRepo.UpdateTaskStatusName(req.ProjectID, oldName, status.Name)
			if taskErr != nil {
				return nil, taskErr
			}
		}

		// 4. Audit Logging
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			Action:         "updated",
			ResourceType:   "custom_status",
			ResourceID:     status.ID.String(),
			Details:        fmt.Sprintf("Custom Status '%s' updated", status.Name),
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now(),
		}
		if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}
	}

	res := responsedto.CustomStatusFromModel(*status)
	return &res, nil
}

func (s *customStatusService) DeleteStatus(statusID, projectID, userID, orgID uuid.UUID) *response.Error {
	// 1. Authorization
	authorized, err := s.checkAdminOrPM(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage custom statuses in this project",
		}
	}

	// 2. Fetch status to get its name for audit logging
	status, err := s.statusRepo.GetStatusByID(statusID, projectID)
	if err != nil {
		return err
	}

	// 4. Validation: status cannot be deleted while assigned to active (non-soft-deleted) tasks
	count, taskErr := s.taskRepo.CountTasksByStatus(projectID, status.Name)
	if taskErr != nil {
		return taskErr
	}
	if count > 0 {
		return &response.Error{
			Code:       response.ErrBusinessRule,
			StatusCode: http.StatusBadRequest,
			Message:    "A status cannot be deleted while it is assigned to existing Tasks",
		}
	}

	// 4. Delete status
	err = s.statusRepo.DeleteStatus(statusID, projectID)
	if err != nil {
		return err
	}

	// 5. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "deleted",
		ResourceType:   "custom_status",
		ResourceID:     statusID.String(),
		Details:        fmt.Sprintf("Custom Status '%s' deleted", status.Name),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

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
	labelrepo "github.com/ms-kanban-server/internal/repository/label-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"go.uber.org/zap"
)

type LabelService interface {
	CreateLabel(req requestdto.CreateLabelRequest) (*responsedto.LabelResponse, *response.Error)
	GetLabels(projectID, userID, orgID uuid.UUID) ([]responsedto.LabelResponse, *response.Error)
	UpdateLabel(req requestdto.UpdateLabelRequest) (*responsedto.LabelResponse, *response.Error)
	DeleteLabel(labelID, projectID, userID, orgID uuid.UUID) *response.Error
}

type labelService struct {
	labelRepo   labelrepo.LabelRepository
	projectRepo projectrepo.ProjectRepository
	authRepo    authrepo.AuthRepository
	auditRepo   auditrepo.AuditLogRepository
	logger      *zap.Logger
}

func InitLabelService(
	labelRepo labelrepo.LabelRepository,
	projectRepo projectrepo.ProjectRepository,
	authRepo authrepo.AuthRepository,
	auditRepo auditrepo.AuditLogRepository,
	logger *zap.Logger,
) LabelService {
	return &labelService{
		labelRepo:   labelRepo,
		projectRepo: projectRepo,
		authRepo:    authRepo,
		auditRepo:   auditRepo,
		logger:      logger,
	}
}

func (s *labelService) checkAdminOrPM(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "modify")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

func (s *labelService) checkProjectMember(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return models.Project{}, models.User{}, false, err
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "view")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

func (s *labelService) CreateLabel(req requestdto.CreateLabelRequest) (*responsedto.LabelResponse, *response.Error) {
	// 1. Authorization
	project, user, authorized, err := s.checkAdminOrPM(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage labels in this project",
		}
	}

	// 2. Validation & Normalization
	req.Name = strings.TrimSpace(req.Name)
	req.Name = strings.ToLower(req.Name)

	if len(req.Name) < 1 || len(req.Name) > 30 {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Label name must be between 1 and 30 characters",
		}
	}

	if !requestdto.ValidateColor(req.Color) {
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Label color must be a valid hexadecimal color code (#RRGGBB)",
		}
	}

	// 3. Uniqueness Check
	exists, err := s.labelRepo.IsLabelNameExists(req.ProjectID, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Label name already exists in this project",
		}
	}

	// 4. Create label
	label := models.Label{
		ProjectID: req.ProjectID,
		Name:      req.Name,
		Color:     req.Color,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.labelRepo.CreateLabel(&label)
	if err != nil {
		return nil, err
	}

	projectName := resolveProjectName(project, req.ProjectID)
	userName := resolveUserName(user, req.UserID)

	// 5. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "created",
		ResourceType:   "label",
		ResourceID:     label.ID.String(),
		Title:          label.Name,
		Details:        fmt.Sprintf("Label '%s' created for project '%s' by %s", label.Name, projectName, userName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	res := responsedto.LabelFromModel(label)
	return &res, nil
}

func (s *labelService) GetLabels(projectID, userID, orgID uuid.UUID) ([]responsedto.LabelResponse, *response.Error) {
	// 1. Authorization
	project, user, authorized, err := s.checkProjectMember(projectID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view labels in this project",
		}
	}

	// 2. Get labels
	labels, err := s.labelRepo.GetLabelsByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	res := make([]responsedto.LabelResponse, 0, len(labels))
	for _, l := range labels {
		res = append(res, responsedto.LabelFromModel(l))
	}

	projectName := resolveProjectName(project, projectID)
	userName := resolveUserName(user, userID)

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "viewed",
		ResourceType:   "label",
		Title:          projectName,
		Details:        fmt.Sprintf("Labels for project '%s' viewed by %s", projectName, userName),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return res, nil
}

func (s *labelService) UpdateLabel(req requestdto.UpdateLabelRequest) (*responsedto.LabelResponse, *response.Error) {
	// 1. Authorization
	project, user, authorized, err := s.checkAdminOrPM(req.ProjectID, req.UserID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage labels in this project",
		}
	}

	// 2. Get existing label
	label, err := s.labelRepo.GetLabelByID(req.LabelID, req.ProjectID)
	if err != nil {
		return nil, err
	}

	oldName := label.Name
	oldColor := label.Color
	var changes []string

	// 3. Validation and updates
	updated := false
	if req.Name != nil && *req.Name != "" {
		normalizedName := strings.TrimSpace(*req.Name)
		normalizedName = strings.ToLower(normalizedName)
		if normalizedName != label.Name {
			if len(normalizedName) < 1 || len(normalizedName) > 30 {
				return nil, &response.Error{
					Code:       response.ErrValidation,
					StatusCode: http.StatusBadRequest,
					Message:    "Label name must be between 1 and 30 characters",
				}
			}

			// check duplicate name
			exists, err := s.labelRepo.IsLabelNameExists(req.ProjectID, normalizedName)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, &response.Error{
					Code:       response.ErrConflict,
					StatusCode: http.StatusConflict,
					Message:    "Label name already exists in this project",
				}
			}

			changes = append(changes, fmt.Sprintf("name changed from '%s' to '%s'", oldName, normalizedName))
			label.Name = normalizedName
			updated = true
		}
	}

	if req.Color != nil && *req.Color != "" && *req.Color != label.Color {
		if !requestdto.ValidateColor(*req.Color) {
			return nil, &response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Label color must be a valid hexadecimal color code (#RRGGBB)",
			}
		}
		changes = append(changes, fmt.Sprintf("color changed from '%s' to '%s'", oldColor, *req.Color))
		label.Color = *req.Color
		updated = true
	}

	projectName := resolveProjectName(project, req.ProjectID)
	userName := resolveUserName(user, req.UserID)

	if updated {
		label.UpdatedAt = time.Now()
		err = s.labelRepo.UpdateLabel(label)
		if err != nil {
			return nil, err
		}

		var detail string
		if len(changes) > 0 {
			detail = fmt.Sprintf("Label '%s' updated for project '%s' by %s: %s", label.Name, projectName, userName, strings.Join(changes, ", "))
		} else {
			detail = fmt.Sprintf("Label '%s' updated for project '%s' by %s", label.Name, projectName, userName)
		}

		// 4. Audit Logging
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			Action:         "updated",
			ResourceType:   "label",
			ResourceID:     label.ID.String(),
			Title:          label.Name,
			Details:        detail,
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now(),
		}
		if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}
	}

	res := responsedto.LabelFromModel(*label)
	return &res, nil
}

func (s *labelService) DeleteLabel(labelID, projectID, userID, orgID uuid.UUID) *response.Error {
	// 1. Authorization
	project, user, authorized, err := s.checkAdminOrPM(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to manage labels in this project",
		}
	}

	// 2. Fetch label to get its name for audit logging
	label, err := s.labelRepo.GetLabelByID(labelID, projectID)
	if err != nil {
		return err
	}

	// 3. Delete label
	err = s.labelRepo.DeleteLabel(labelID, projectID)
	if err != nil {
		return err
	}

	projectName := resolveProjectName(project, projectID)
	userName := resolveUserName(user, userID)

	// 4. Audit Logging
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "deleted",
		ResourceType:   "label",
		ResourceID:     labelID.String(),
		Title:          label.Name,
		Details:        fmt.Sprintf("Label '%s' deleted for project '%s' by %s", label.Name, projectName, userName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if err := s.auditRepo.CreateAuditLog(auditLog); err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

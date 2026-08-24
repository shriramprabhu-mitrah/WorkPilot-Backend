package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"go.uber.org/zap"
)

type SprintService interface {
	CreateSprint(req dto.CreateSprintRequest) (uuid.UUID, *response.Error)
	DeleteSprint(req dto.DeleteSprint) *response.Error
	UpdateSprint(req dto.UpdateSprintRequest) *response.Error
	GetSprints(req dto.GetSprint, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error)
	GetSprintByID(req dto.GetSprint) (*models.Sprint, *response.Error)
	GetSprintBurndown(sprintID, projectID, userID, orgID uuid.UUID) (*dto.SprintBurndownResponse, *response.Error)
	TriggerDailySnapshots(projectUUID, userUUID, orgUUID uuid.UUID) *response.Error
}

func InitSprintService(sprintRepo sprintrepo.SprintRepository, projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, auditRepo auditrepo.AuditLogRepository, logger *zap.Logger) SprintService {
	return &sprintService{
		sprintRepo:  sprintRepo,
		projectRepo: projectRepo,
		authRepo:    authRepo,
		auditRepo:   auditRepo,
		logger:      logger,
	}
}

type sprintService struct {
	sprintRepo  sprintrepo.SprintRepository
	projectRepo projectrepo.ProjectRepository
	authRepo    authrepo.AuthRepository
	auditRepo   auditrepo.AuditLogRepository
	logger      *zap.Logger
}

func (s *sprintService) CreateSprint(req dto.CreateSprintRequest) (uuid.UUID, *response.Error) {
	var err *response.Error
	result, errResp := s.authRepo.GetUserByID(req.UserID)
	if errResp != nil {
		return uuid.Nil, errResp
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.Any("User Organization ID", result.OrganizationID))

		return uuid.Nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()))

		return uuid.Nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "sprints", "add")
	if permErr != nil {
		return uuid.Nil, permErr
	}
	if !authorized {
		s.logger.Error("Unauthorized sprint creation attempt",
			zap.String("User ID", req.UserID.String()),
			zap.String("Project ID", req.ProjectID.String()))

		return uuid.Nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to create sprints for this project",
		}
	}

	type validatedSprint struct {
		Name      string
		Goal      string
		StartDate time.Time
		EndDate   time.Time
	}

	validatedList := make([]validatedSprint, 0, len(req.Sprints))

	for _, spr := range req.Sprints {

		startDate, startErr := utils.StringToTime(spr.StartDate)
		if startErr != nil {
			s.logger.Error("Invalid start_date",
				zap.Error(startErr))
			return uuid.Nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}

		endDate, endErr := utils.StringToTime(spr.EndDate)
		if endErr != nil {
			s.logger.Error("Invalid end_date",
				zap.Error(endErr))
			return uuid.Nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}

		if endDate.Before(*startDate) {
			return uuid.Nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "end_date cannot be before start_date",
			}
		}

		validatedList = append(validatedList, validatedSprint{
			Name:      spr.Name,
			Goal:      spr.Goal,
			StartDate: *startDate,
			EndDate:   *endDate,
		})
	}

	var sprint *models.Sprint

	for _, vSpr := range validatedList {
		sprint = &models.Sprint{
			Name:        vSpr.Name,
			Goal:        vSpr.Goal,
			StartDate:   vSpr.StartDate,
			EndDate:     vSpr.EndDate,
			ProjectID:   req.ProjectID,
			CreatedByID: req.UserID,
		}

		err = s.sprintRepo.CreateSprint(sprint)
		if err != nil {
			return uuid.Nil, err
		}

		// audit log creation
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      &req.ProjectID,
			SprintID:       &sprint.ID,
			Action:         "created",
			ResourceType:   "sprint",
			ResourceID:     sprint.ID.String(),
			Details:        fmt.Sprintf("The sprint '%s' was created by %s", sprint.Name, result.UserName),
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now(),
		}

		err = s.auditRepo.CreateAuditLog(auditLog)
		if err != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", err))
		}
	}

	return sprint.ID, nil
}

func (s *sprintService) DeleteSprint(req dto.DeleteSprint) *response.Error {
	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	var err *response.Error
	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
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

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "sprints", "delete")
	if permErr != nil {
		return permErr
	}
	if !authorized {
		s.logger.Error("Unauthorized sprint deletion attempt",
			zap.String("User ID", req.UserID.String()),
			zap.String("Project ID", req.ProjectID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete sprints from this project",
		}
	}

	sprintName := req.SprintID.String()
	sprint, _ := s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
	if sprint != nil && sprint.Name != "" {
		sprintName = sprint.Name
	}

	err = s.sprintRepo.DeleteSprint(req.SprintID)
	if err != nil {
		return err
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		SprintID:       &req.SprintID,
		Action:         "deleted",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
		Details:        fmt.Sprintf("The sprint '%s' was deleted by %s", sprintName, result.UserName),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *sprintService) UpdateSprint(req dto.UpdateSprintRequest) *response.Error {
	var err *response.Error
	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
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

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, req.ProjectID, "sprints", "modify")
	if permErr != nil {
		return permErr
	}
	if !authorized {
		s.logger.Error("Unauthorized sprint update attempt",
			zap.String("User ID", req.UserID.String()),
			zap.String("Project ID", req.ProjectID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to update sprints for this project",
		}
	}

	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	existingSprint, errResp := s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
	if errResp != nil {
		return errResp
	}

	changedBy := result.UserName
	if changedBy == "" {
		changedBy = result.FullName
	}
	if changedBy == "" {
		changedBy = result.Email
	}
	if changedBy == "" {
		changedBy = req.UserID.String()
	}

	startDate := existingSprint.StartDate
	endDate := existingSprint.EndDate
	updates := make(map[string]interface{})
	var changes []string

	if req.StartDate != nil {
		d, err := utils.StringToTime(*req.StartDate)
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
		startDate = *d
		updates["start_date"] = startDate
		if startDate.Format("2006-01-02") != existingSprint.StartDate.Format("2006-01-02") {
			changes = append(changes, fmt.Sprintf("start date changed from '%s' to '%s'", existingSprint.StartDate.Format("2006-01-02"), startDate.Format("2006-01-02")))
		}
	}

	if req.EndDate != nil {
		d, err := utils.StringToTime(*req.EndDate)
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}
		endDate = *d
		updates["end_date"] = endDate
		if endDate.Format("2006-01-02") != existingSprint.EndDate.Format("2006-01-02") {
			changes = append(changes, fmt.Sprintf("end date changed from '%s' to '%s'", existingSprint.EndDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
		}
	}

	if startDate.After(endDate) {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Sprint start date cannot be after end date",
		}
	}

	if req.Name != nil {
		updates["name"] = *req.Name
		if *req.Name != existingSprint.Name {
			changes = append(changes, fmt.Sprintf("name changed from '%s' to '%s'", existingSprint.Name, *req.Name))
		}
	}
	if req.Goal != nil {
		updates["goal"] = *req.Goal
		if *req.Goal != existingSprint.Goal {
			changes = append(changes, fmt.Sprintf("goal changed from '%s' to '%s'", existingSprint.Goal, *req.Goal))
		}
	}

	newStatus := existingSprint.Status
	if req.Status != nil {
		newStatus = string(*req.Status)
		updates["status"] = newStatus
		if newStatus != existingSprint.Status {
			changes = append(changes, fmt.Sprintf("status changed from '%s' to '%s'", existingSprint.Status, newStatus))
		}
	}

	if req.Status != nil && newStatus == "completed" && existingSprint.Status != "completed" {
		v, err := s.sprintRepo.GetCompletedTasksStoryPoints(req.SprintID)
		if err != nil {
			return err
		}
		updates["velocity"] = v

		if existingSprint.Status != "completed" {
			err := s.sprintRepo.MoveIncompleteTasksToBacklog(req.SprintID)
			if err != nil {
				return err
			}
		}
	}

	if len(updates) == 0 {
		return nil
	}

	var detail string
	if len(changes) > 0 {
		detail = fmt.Sprintf("Sprint updated by %s: %s", changedBy, strings.Join(changes, ", "))
	} else {
		detail = fmt.Sprintf("Sprint details updated by %s", changedBy)
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		SprintID:       &req.SprintID,
		Action:         "updated",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
		Title:          existingSprint.Name,
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return s.sprintRepo.UpdateSprint(req.ProjectID, req.SprintID, updates)
}

func (s *sprintService) GetSprints(req dto.GetSprint, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return nil, response.Pagination{}, errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if req.ProjectID == uuid.Nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
	}

	// Validate date range
	if !filter.StartDate.IsZero() && !filter.EndDate.IsZero() {
		if filter.StartDate.After(filter.EndDate) {
			return nil, response.Pagination{}, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Start date must be before or equal to end date",
			}
		}
	}

	sprints, pagination, err := s.sprintRepo.GetSprints(req.ProjectID, filter)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "viewed",
		ResourceType:   "sprint",
		Details:        "sprint viewed",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return sprints, pagination, nil
}

func (s *sprintService) GetSprintByID(req dto.GetSprint) (*models.Sprint, *response.Error) {

	result, errorResponse := s.authRepo.GetUserByID(req.UserID)
	if errorResponse != nil {
		return nil, errorResponse
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", req.OrganizationID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if req.ProjectID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid project id",
		}
	}

	if req.SprintID == uuid.Nil && req.ProjectID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid Sprint/Project id",
		}
	}

	sprint, errorResponse := s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
	if errorResponse != nil {
		return nil, errorResponse
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		SprintID:       &req.SprintID,
		Action:         "viewed",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
		Details:        fmt.Sprintf("Sprint details viewed by %s", result.UserName),
		Type:           models.AuditLogTypeView,
		CreatedAt:      time.Now(),
	}

	err := s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return sprint, nil
}

func (s *sprintService) GetSprintBurndown(sprintID, projectID, userID, orgID uuid.UUID) (*dto.SprintBurndownResponse, *response.Error) {
	// 1. Authorize
	result, errorResponse := s.authRepo.GetUserByID(userID)
	if errorResponse != nil {
		return nil, errorResponse
	}

	if result.OrganizationID == nil || orgID == uuid.Nil {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != orgID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization Id", orgID.String()),
			zap.String("User Organization Id", result.OrganizationID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	// 2. Fetch Sprint
	sprint, errResp := s.sprintRepo.GetSprintByID(sprintID, projectID)
	if errResp != nil {
		return nil, errResp
	}

	// 3. Fetch snapshots
	snapshots, errResp := s.sprintRepo.GetSprintSnapshots(sprintID)
	if errResp != nil {
		return nil, errResp
	}

	// 4. Calculate total points dynamically
	totalPoints, errResp := s.sprintRepo.GetTotalStoryPoints(sprintID)
	if errResp != nil {
		return nil, errResp
	}

	// Calculate remaining points dynamically
	remainingPointsNow, errResp := s.sprintRepo.GetRemainingStoryPoints(sprintID)
	if errResp != nil {
		return nil, errResp
	}

	// Create date to snapshot map
	snapshotMap := make(map[string]models.SprintSnapshot)
	for _, snap := range snapshots {
		dateStr := snap.Date.Format("2006-01-02")
		snapshotMap[dateStr] = snap
	}

	// 5. Generate daily data points from StartDate to EndDate
	var burndownData []dto.BurndownDataPoint
	startDate := time.Date(sprint.StartDate.Year(), sprint.StartDate.Month(), sprint.StartDate.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(sprint.EndDate.Year(), sprint.EndDate.Month(), sprint.EndDate.Day(), 0, 0, 0, 0, time.UTC)

	totalDays := int(endDate.Sub(startDate).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for i := 0; i <= totalDays; i++ {
		currentDate := startDate.AddDate(0, 0, i)
		dateStr := currentDate.Format("2006-01-02")

		// Calculate Ideal
		ideal := float64(totalPoints) * (1.0 - float64(i)/float64(totalDays))
		if ideal < 0 {
			ideal = 0
		}

		// Calculate remaining points
		var remaining *int
		if !currentDate.After(today) {
			if snap, exists := snapshotMap[dateStr]; exists {
				val := snap.RemainingStoryPoints
				remaining = &val
			} else {
				// Fallback: If it's today, use current remaining points.
				if currentDate.Equal(today) {
					remaining = &remainingPointsNow
				} else {
					// Fallback to previous day's snapshot or totalPoints
					var prevVal int = totalPoints
					if len(burndownData) > 0 && burndownData[len(burndownData)-1].RemainingPoints != nil {
						prevVal = *burndownData[len(burndownData)-1].RemainingPoints
					}
					remaining = &prevVal
				}
			}
		}

		burndownData = append(burndownData, dto.BurndownDataPoint{
			Date:            dateStr,
			RemainingPoints: remaining,
			IdealValue:      ideal,
		})
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		SprintID:       &sprintID,
		Action:         "viewed",
		ResourceType:   "sprint",
		ResourceID:     sprintID.String(),
		Details:        "Sprint burndown chart viewed",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err := s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return &dto.SprintBurndownResponse{
		SprintID:         sprint.ID.String(),
		SprintName:       sprint.Name,
		TotalStoryPoints: totalPoints,
		BurndownData:     burndownData,
	}, nil
}

func (s *sprintService) TriggerDailySnapshots(projectUUID, userUUID, orgUUID uuid.UUID) *response.Error {

	user, userErr := s.authRepo.GetUserByID(userUUID)
	if userErr != nil {
		return userErr
	}

	project, projErr := s.projectRepo.GetProjectByID(projectUUID)
	if projErr != nil {
		return projErr
	}

	if user.OrganizationID == nil || *user.OrganizationID != project.OrganizationID {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userUUID, projectUUID, "sprints", "modify")
	if permErr != nil {
		return permErr
	}
	if !authorized {
		s.logger.Error("Unauthorized sprint update/snapshot attempt",
			zap.String("User ID", userUUID.String()),
			zap.String("Project ID", projectUUID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to modify sprints for this project",
		}
	}

	sprints, err := s.sprintRepo.GetActiveSprints()
	if err != nil {
		s.logger.Error("Failed to fetch active sprints for manual snapshot", zap.Any("error", err))
		return err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, sprint := range sprints {
		totalPoints, err := s.sprintRepo.GetTotalStoryPoints(sprint.ID)
		if err != nil {
			s.logger.Error("Failed to calculate total points for sprint snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
			continue
		}

		remainingPoints, err := s.sprintRepo.GetRemainingStoryPoints(sprint.ID)
		if err != nil {
			s.logger.Error("Failed to calculate remaining points for sprint snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
			continue
		}

		snapshot := models.SprintSnapshot{
			SprintID:             sprint.ID,
			Date:                 today,
			TotalStoryPoints:     totalPoints,
			RemainingStoryPoints: remainingPoints,
			CreatedAt:            now,
		}

		err = s.sprintRepo.CreateSprintSnapshot(snapshot)
		if err != nil {
			s.logger.Error("Failed to save daily snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
		}
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &userUUID,
		OrganizationID: &orgUUID,
		ProjectID:      &projectUUID,
		Action:         "created",
		ResourceType:   "sprint",
		Details:        "Sprint snapshot triggered",
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}
	return nil
}

package services

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
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
	CreateSprint(req dto.CreateSprintRequest) *response.Error
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

func (s *sprintService) CreateSprint(req dto.CreateSprintRequest) *response.Error {

	result, errResp := s.authRepo.GetUserByID(req.UserID)
	if errResp != nil {
		return errResp
	}

	if result.OrganizationID == nil || req.OrganizationID == uuid.Nil {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.Any("User Organization ID", result.OrganizationID))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	if *result.OrganizationID != req.OrganizationID {
		s.logger.Error("Unauthorized Access",
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.String("User Organization ID", result.OrganizationID.String()))

		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
	isOrgAdmin := result.Role == string(dto.RoleOrgAdmin)

	if err != nil {
		if !isOrgAdmin {
			return err
		}
	} else {
		if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
			member.ProjectRole != string(requestdto.ProjectRoleProjectManager) &&
			!isOrgAdmin {

			s.logger.Error("Unauthorized project update attempt",
				zap.String("User ID", req.UserID.String()),
				zap.String("Project ID", req.ProjectID.String()),
				zap.String("Project Role", string(member.ProjectRole)))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to update this project",
			}
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
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}

		endDate, endErr := utils.StringToTime(spr.EndDate)
		if endErr != nil {
			s.logger.Error("Invalid end_date",
				zap.Error(endErr))
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid end_date. Expected format: YYYY-MM-DD",
			}
		}

		if endDate.Before(*startDate) {
			return &response.Error{
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

	for _, vSpr := range validatedList {
		sprint := models.Sprint{
			Name:        vSpr.Name,
			Goal:        vSpr.Goal,
			StartDate:   vSpr.StartDate,
			EndDate:     vSpr.EndDate,
			ProjectID:   req.ProjectID,
			CreatedByID: req.UserID,
		}

		if err := s.sprintRepo.CreateSprint(sprint); err != nil {
			return err
		}
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "sprint_created",
		ResourceType:   "sprint",
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return nil
}

func (s *sprintService) DeleteSprint(req dto.DeleteSprint) *response.Error {

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

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
	isOrgAdmin := result.Role == string(dto.RoleOrgAdmin)

	if err != nil {
		if !isOrgAdmin {
			return err
		}
	} else {
		if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
			member.ProjectRole != string(requestdto.ProjectRoleProjectManager) &&
			!isOrgAdmin {

			s.logger.Error("Unauthorized project update attempt",
				zap.String("User ID", req.UserID.String()),
				zap.String("Project ID", req.ProjectID.String()),
				zap.String("Project Role", string(member.ProjectRole)))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to update this project",
			}
		}
	}

	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "sprint_deleted",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}

	return s.sprintRepo.DeleteSprint(req.SprintID)
}

func (s *sprintService) UpdateSprint(req dto.UpdateSprintRequest) *response.Error {

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

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(req.UserID, req.ProjectID)
	isOrgAdmin := result.Role == string(dto.RoleOrgAdmin)

	if err != nil {
		if !isOrgAdmin {
			return err
		}
	} else {
		if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
			member.ProjectRole != string(requestdto.ProjectRoleProjectManager) &&
			!isOrgAdmin {

			s.logger.Error("Unauthorized project update attempt",
				zap.String("User ID", req.UserID.String()),
				zap.String("Project ID", req.ProjectID.String()),
				zap.String("Project Role", string(member.ProjectRole)))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to update this project",
			}
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

	startDate := existingSprint.StartDate
	endDate := existingSprint.EndDate
	updates := make(map[string]interface{})

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
	}
	if req.Goal != nil {
		updates["goal"] = *req.Goal
	}

	newStatus := existingSprint.Status
	if req.Status != nil {
		newStatus = string(*req.Status)
		updates["status"] = newStatus
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

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "sprint_updated",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
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

	sprints, pagination, err := s.sprintRepo.GetSprints(req.ProjectID, filter)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// audit log creation
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      &req.ProjectID,
		Action:         "sprint_created",
		ResourceType:   "sprint",
		Type:           models.AuditLogTypeActivity,
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
		Action:         "sprint_created",
		ResourceType:   "sprint",
		ResourceID:     req.SprintID.String(),
		Type:           models.AuditLogTypeActivity,
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
		Action:         "sprint_burndown",
		ResourceType:   "sprint",
		ResourceID:     sprintID.String(),
		Type:           models.AuditLogTypeView,
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
	isOrgAdmin := user.Role == string(dto.RoleOrgAdmin)

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

	member, err := s.projectRepo.GetProjectMemberByUserAndProjectID(userUUID, projectUUID)
	if err != nil {
		if !isOrgAdmin {
			return err
		}
	} else {
		if member.ProjectRole != string(requestdto.ProjectRoleOrgAdmin) &&
			member.ProjectRole != string(requestdto.ProjectRoleProjectManager) &&
			!isOrgAdmin {

			s.logger.Error("Unauthorized project update attempt",
				zap.String("User ID", userUUID.String()),
				zap.String("Project ID", projectUUID.String()),
				zap.String("Project Role", string(member.ProjectRole)))

			return &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to update this project",
			}
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
		Action:         "sprint_snapshot_triggered",
		ResourceType:   "sprint",
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	err = s.auditRepo.CreateAuditLog(auditLog)
	if err != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", err))
	}
	return nil
}

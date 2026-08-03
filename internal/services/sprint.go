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
	TriggerDailySnapshots() *response.Error
}

func InitSprintService(sprintRepo sprintrepo.SprintRepository, projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, logger *zap.Logger) SprintService {
	return &sprintService{
		sprintRepo:  sprintRepo,
		projectRepo: projectRepo,
		authRepo:    authRepo,
		logger:      logger,
	}
}

type sprintService struct {
	sprintRepo  sprintrepo.SprintRepository
	projectRepo projectrepo.ProjectRepository
	authRepo    authrepo.AuthRepository
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
			zap.String("User Organization ID", result.OrganizationID.String()))

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

	var existingSprints []string

	for _, spr := range req.Sprints {

		exists, err := s.sprintRepo.IsSprintExists(req.ProjectID, spr.Name)
		if err != nil {
			return err
		}

		if exists {
			existingSprints = append(existingSprints, spr.Name)
			continue
		}

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

		sprint := models.Sprint{
			Name:        spr.Name,
			Goal:        spr.Goal,
			StartDate:   *startDate,
			EndDate:     *endDate,
			ProjectID:   req.ProjectID,
			CreatedByID: req.UserID,
		}

		if err := s.sprintRepo.CreateSprint(sprint); err != nil {
			return err
		}
	}

	if len(existingSprints) > 0 {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message: fmt.Sprintf(
				"The following sprints already exist in the project: %s",
				strings.Join(existingSprints, ", "),
			),
		}
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

	if req.SprintID == uuid.Nil {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid sprint id",
		}
	}

	return s.sprintRepo.DeleteSprint(req.SprintID)
}

func (s *sprintService) UpdateSprint(req dto.UpdateSprintRequest) *response.Error {

	var startDate, endDate *time.Time

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

	startDate = &existingSprint.StartDate
	endDate = &existingSprint.EndDate

	if req.StartDate != "" {
		d, err := utils.StringToTime(req.StartDate)
		startDate = d
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
	}

	if req.EndDate != "" {
		d, err := utils.StringToTime(req.EndDate)
		startDate = d
		if err != nil {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid start_date. Expected format: YYYY-MM-DD",
			}
		}
	}

	if startDate.After(*endDate) {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Sprint start date cannot be after end date",
		}
	}

	name := req.Name
	if name == "" {
		name = existingSprint.Name
	}
	goal := req.Goal
	if goal == "" {
		goal = existingSprint.Goal
	}
	newStatus := string(req.Status)
	if newStatus == "" {
		newStatus = existingSprint.Status
	}

	var velocity *int
	if newStatus == "completed" {
		v, err := s.sprintRepo.GetCompletedTasksStoryPoints(req.SprintID)
		if err != nil {
			return err
		}
		velocity = &v
	}

	payload := models.Sprint{
		Name:      name,
		Goal:      goal,
		StartDate: *startDate,
		EndDate:   *endDate,
		Status:    newStatus,
		Velocity:  velocity,
	}

	return s.sprintRepo.UpdateSprint(req.ProjectID, req.SprintID, payload)
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

	return s.sprintRepo.GetSprints(req.ProjectID, filter)
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

	return s.sprintRepo.GetSprintByID(req.SprintID, req.ProjectID)
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

	return &dto.SprintBurndownResponse{
		SprintID:         sprint.ID.String(),
		SprintName:       sprint.Name,
		TotalStoryPoints: totalPoints,
		BurndownData:     burndownData,
	}, nil
}

func (s *sprintService) TriggerDailySnapshots() *response.Error {
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
	return nil
}

package services

import (
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	workitemrepo "github.com/ms-kanban-server/internal/repository/work-item-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	"go.uber.org/zap"
)

type WorkItemService interface {
	GetWorkItemBySerialNumber(projectIDOrSlug string, serialIDOrKey string, userID uuid.UUID) (*responsedto.WorkItemResponse, *response.Error)
	GetTaskByKey(projectIDOrSlug string, key string, userID uuid.UUID) (*responsedto.TaskResponse, *response.Error)
	GetUserStoryByKey(projectIDOrSlug string, key string, userID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error)
}

type workItemService struct {
	authRepo            authrepo.AuthRepository
	projectRepo         projectrepo.ProjectRepository
	workItemRepo        workitemrepo.WorkItemRepository
	customStatusRepo    customstatusrepo.CustomStatusRepository
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository
	taskRepo            taskrepo.TaskRepository
	userStoryRepo       userstoryrepo.UserStoryRepository
	favoriteRepo        favoriterepo.FavoriteRepository
	logger              *zap.Logger
}

func InitWorkItemService(
	authRepo authrepo.AuthRepository,
	projectRepo projectrepo.ProjectRepository,
	workItemRepo workitemrepo.WorkItemRepository,
	customStatusRepo customstatusrepo.CustomStatusRepository,
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository,
	taskRepo taskrepo.TaskRepository,
	userStoryRepo userstoryrepo.UserStoryRepository,
	favoriteRepo favoriterepo.FavoriteRepository,
	logger *zap.Logger,
) WorkItemService {
	return &workItemService{
		authRepo:            authRepo,
		projectRepo:         projectRepo,
		workItemRepo:        workItemRepo,
		customStatusRepo:    customStatusRepo,
		userStoryStatusRepo: userStoryStatusRepo,
		taskRepo:            taskRepo,
		userStoryRepo:       userStoryRepo,
		favoriteRepo:        favoriteRepo,
		logger:              logger,
	}
}

func (s *workItemService) checkAuthorization(projectID, userID uuid.UUID) (models.Project, models.User, bool, *response.Error) {
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

	authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "view")
	if permErr != nil {
		return models.Project{}, models.User{}, false, permErr
	}

	return project, user, authorized, nil
}

func (s *workItemService) GetWorkItemBySerialNumber(projectIDOrSlug string, serialIDOrKey string, userID uuid.UUID) (*responsedto.WorkItemResponse, *response.Error) {
	var project models.Project
	var err *response.Error

	projectUUID, parseErr := uuid.FromString(projectIDOrSlug)
	if parseErr == nil {
		project, err = s.projectRepo.GetProjectByID(projectUUID)
	} else {
		project, err = s.projectRepo.GetProjectBySlug(projectIDOrSlug)
	}
	if err != nil {
		return nil, err
	}

	_, _, authorized, err := s.checkAuthorization(project.ID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have access to this project",
		}
	}

	var task *models.Task
	var taskErr *response.Error = &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Task not found"}
	var story *models.UserStory
	var storyErr *response.Error = &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "User story not found"}

	serialID, intParseErr := strconv.ParseInt(serialIDOrKey, 10, 64)
	if intParseErr == nil {
		story, storyErr = s.workItemRepo.GetUserStoryBySerialNumber(serialID)
	} else {
		upperKey := strings.ToUpper(serialIDOrKey)
		if strings.HasPrefix(upperKey, "US-") {
			story, storyErr = s.userStoryRepo.GetUserStoryByKey(project.ID, upperKey)
		} else {
			task, taskErr = s.workItemRepo.GetTaskByKey(project.ID, upperKey)
		}
	}

	if taskErr == nil && task != nil {
		if task.ProjectID != project.ID {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Work item not found in this project",
			}
		}

		colorMap := make(map[string]string)
		isFinalMap := make(map[string]bool)
		maps.Copy(colorMap, models.DefaultStatusColors)
		maps.Copy(isFinalMap, models.DefaultStatusIsFinal)
		if s.customStatusRepo != nil {
			statuses, err := s.customStatusRepo.GetStatusesByProjectID(project.ID)
			if err == nil {
				for _, cs := range statuses {
					colorMap[models.NormalizeTaskStatus(cs.Name)] = cs.Color
					isFinalMap[models.NormalizeTaskStatus(cs.Name)] = cs.IsFinal
				}
			}
		}

		taskResp := mapToTaskResponse(*task, colorMap, isFinalMap)
		isFav := false
		if s.favoriteRepo != nil && userID != uuid.Nil {
			isFav, _ = s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeTask, task.ID)
		}
		taskResp.IsFavourite = isFav

		workItem := &responsedto.WorkItemResponse{
			WorkItemType:          "task",
			ID:                    task.ID,
			ProjectID:             task.ProjectID,
			SerialNumber:          task.SerialNumber,
			FormattedSerialNumber: task.FormattedSerialNumber(),
			Title:                 task.Title,
			Description:           task.Description,
			Priority:              task.Priority,
			StatusID:              task.StatusID,
			Status:                task.Status,
			StatusColor:           taskResp.StatusColor,
			IsFavourite:           isFav,
			StoryPoints:           task.StoryPoints,
			SprintID:              task.SprintID,
			SprintName:            taskResp.SprintName,
			AssigneeID:            task.AssigneeID,
			AssigneeName:          taskResp.AssigneeName,
			ReporterID:            task.ReporterID,
			ReporterName:          taskResp.ReporterName,
			CreatedAt:             task.CreatedAt,
			UpdatedAt:             task.UpdatedAt,
			TaskDetails:           &taskResp,
		}
		return workItem, nil
	}

	// 2. Try finding User Story
	if storyErr == nil && story != nil {
		if story.ProjectID != project.ID {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Work item not found in this project",
			}
		}

		var userStoryStatuses []models.UserStoryStatus
		if s.userStoryStatusRepo != nil {
			statuses, err := s.userStoryStatusRepo.GetStatusesByProjectID(project.ID)
			if err == nil {
				userStoryStatuses = statuses
			}
		}

		var totalTasks, completedTasks int64
		var progress float64
		if s.userStoryRepo != nil {
			statsMap, err := s.userStoryRepo.GetStoryTaskStats(project.ID)
			if err == nil {
				if stats, ok := statsMap[story.ID]; ok {
					totalTasks = stats.TotalTasks
					completedTasks = stats.Completed
					if totalTasks > 0 {
						progress = float64(completedTasks) / float64(totalTasks) * 100
					}
				}
			}
		}

		storyResp := mapToUserStoryResponse(*story, userStoryStatuses, totalTasks, completedTasks, progress)
		isFav := false
		if s.favoriteRepo != nil && userID != uuid.Nil {
			isFav, _ = s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeUserStory, story.ID)
		}
		storyResp.IsFavourite = isFav

		workItem := &responsedto.WorkItemResponse{
			WorkItemType:          "user_story",
			ID:                    story.ID,
			ProjectID:             story.ProjectID,
			SerialNumber:          story.SerialNumber,
			FormattedSerialNumber: story.FormattedSerialNumber(),
			Title:                 story.Title,
			Description:           story.Description,
			Priority:              story.Priority,
			StatusID:              storyResp.StatusID,
			Status:                story.Status,
			StatusColor:           storyResp.StatusColor,
			IsFavourite:           isFav,
			StoryPoints:           story.StoryPoints,
			SprintID:              story.SprintID,
			SprintName:            storyResp.SprintName,
			AssigneeID:            story.AssigneeID,
			AssigneeName:          storyResp.AssigneeName,
			ReporterID:            &story.ReporterID,
			ReporterName:          storyResp.ReporterName,
			CreatedAt:             story.CreatedAt,
			UpdatedAt:             story.UpdatedAt,
			UserStoryDetails:      &storyResp,
		}
		return workItem, nil
	}

	return nil, &response.Error{
		Code:       response.ErrNotFound,
		StatusCode: http.StatusNotFound,
		Message:    "Work item not found",
	}
}

func (s *workItemService) GetTaskByKey(projectIDOrSlug string, key string, userID uuid.UUID) (*responsedto.TaskResponse, *response.Error) {
	var project models.Project
	var err *response.Error

	projectUUID, parseErr := uuid.FromString(projectIDOrSlug)
	if parseErr == nil {
		project, err = s.projectRepo.GetProjectByID(projectUUID)
	} else {
		project, err = s.projectRepo.GetProjectBySlug(projectIDOrSlug)
	}
	if err != nil {
		return nil, err
	}

	_, _, authorized, err := s.checkAuthorization(project.ID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have access to this project",
		}
	}

	task, err := s.workItemRepo.GetTaskByKey(project.ID, strings.ToUpper(key))
	if err != nil {
		return nil, err
	}

	colorMap := make(map[string]string)
	isFinalMap := make(map[string]bool)
	maps.Copy(colorMap, models.DefaultStatusColors)
	maps.Copy(isFinalMap, models.DefaultStatusIsFinal)
	if s.customStatusRepo != nil {
		statuses, getStatusErr := s.customStatusRepo.GetStatusesByProjectID(project.ID)
		if getStatusErr == nil {
			for _, cs := range statuses {
				colorMap[models.NormalizeTaskStatus(cs.Name)] = cs.Color
				isFinalMap[models.NormalizeTaskStatus(cs.Name)] = cs.IsFinal
			}
		}
	}

	taskResp := mapToTaskResponse(*task, colorMap, isFinalMap)
	isFav := false
	if s.favoriteRepo != nil && userID != uuid.Nil {
		isFav, _ = s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeTask, task.ID)
	}
	taskResp.IsFavourite = isFav

	return &taskResp, nil
}

func (s *workItemService) GetUserStoryByKey(projectIDOrSlug string, key string, userID uuid.UUID) (*responsedto.UserStoryResponse, *response.Error) {
	var project models.Project
	var err *response.Error

	projectUUID, parseErr := uuid.FromString(projectIDOrSlug)
	if parseErr == nil {
		project, err = s.projectRepo.GetProjectByID(projectUUID)
	} else {
		project, err = s.projectRepo.GetProjectBySlug(projectIDOrSlug)
	}
	if err != nil {
		return nil, err
	}

	_, _, authorized, err := s.checkAuthorization(project.ID, userID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have access to this project",
		}
	}

	story, err := s.userStoryRepo.GetUserStoryByKey(project.ID, strings.ToUpper(key))
	if err != nil {
		return nil, err
	}

	var userStoryStatuses []models.UserStoryStatus
	if s.userStoryStatusRepo != nil {
		statuses, statusErr := s.userStoryStatusRepo.GetStatusesByProjectID(project.ID)
		if statusErr == nil {
			userStoryStatuses = statuses
		}
	}

	var totalTasks, completedTasks int64
	var progress float64
	if s.userStoryRepo != nil {
		statsMap, statsErr := s.userStoryRepo.GetStoryTaskStats(project.ID)
		if statsErr == nil {
			if stats, ok := statsMap[story.ID]; ok {
				totalTasks = stats.TotalTasks
				completedTasks = stats.Completed
				if totalTasks > 0 {
					progress = float64(completedTasks) / float64(totalTasks) * 100
				}
			}
		}
	}

	storyResp := mapToUserStoryResponse(*story, userStoryStatuses, totalTasks, completedTasks, progress)
	isFav := false
	if s.favoriteRepo != nil && userID != uuid.Nil {
		isFav, _ = s.favoriteRepo.IsFavorited(userID, models.FavoriteItemTypeUserStory, story.ID)
	}
	storyResp.IsFavourite = isFav

	return &storyResp, nil
}

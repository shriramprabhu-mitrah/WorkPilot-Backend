package services

import (
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	"go.uber.org/zap"
)

type FavoriteService interface {
	AddFavorite(req requestdto.AddFavoriteRequest) (*responsedto.FavoriteResponse, *response.Error)
	RemoveFavorite(userID uuid.UUID, itemType string, itemID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error)
	RemoveFavoriteByID(userID, favoriteID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error)
	GetFavorites(userID uuid.UUID, filter requestdto.GetFavoritesFilter) (*responsedto.FavoriteListResponse, response.Pagination, *response.Error)
	AddUserStoryFavorite(userID, projectID, userStoryID uuid.UUID) (*responsedto.FavoriteResponse, *response.Error)
	RemoveUserStoryFavorite(userID, projectID, userStoryID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error)
	AddTaskFavorite(userID, projectID, taskID uuid.UUID) (*responsedto.FavoriteResponse, *response.Error)
	RemoveTaskFavorite(userID, projectID, taskID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error)
}

type favoriteService struct {
	favoriteRepo        favoriterepo.FavoriteRepository
	userStoryRepo       userstoryrepo.UserStoryRepository
	taskRepo            taskrepo.TaskRepository
	projectRepo         projectrepo.ProjectRepository
	authRepo            authrepo.AuthRepository
	customStatusRepo    customstatusrepo.CustomStatusRepository
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository
	logger              *zap.Logger
}

func InitFavoriteService(
	favoriteRepo favoriterepo.FavoriteRepository,
	userStoryRepo userstoryrepo.UserStoryRepository,
	taskRepo taskrepo.TaskRepository,
	projectRepo projectrepo.ProjectRepository,
	authRepo authrepo.AuthRepository,
	customStatusRepo customstatusrepo.CustomStatusRepository,
	userStoryStatusRepo userstorystatusrepo.UserStoryStatusRepository,
	logger *zap.Logger,
) FavoriteService {
	return &favoriteService{
		favoriteRepo:        favoriteRepo,
		userStoryRepo:       userStoryRepo,
		taskRepo:            taskRepo,
		projectRepo:         projectRepo,
		authRepo:            authRepo,
		customStatusRepo:    customStatusRepo,
		userStoryStatusRepo: userStoryStatusRepo,
		logger:              logger,
	}
}

func (s *favoriteService) AddFavorite(req requestdto.AddFavoriteRequest) (*responsedto.FavoriteResponse, *response.Error) {
	isFav, _ := s.favoriteRepo.IsFavorited(req.UserID, req.ItemType, req.ItemID)
	if isFav {
		return nil, &response.Error{
			Code:       response.ErrConflict,
			StatusCode: http.StatusConflict,
			Message:    "Item is already added to favorites",
		}
	}

	favorite := models.Favorite{
		UserID:   req.UserID,
		ItemType: req.ItemType,
	}

	var userStoryResp *responsedto.UserStoryResponse
	var taskResp *responsedto.TaskResponse

	switch req.ItemType {
	case models.FavoriteItemTypeUserStory:
		ctx, err := s.userStoryRepo.GetUserStoryAccessContext(req.ItemID)
		if err != nil {
			return nil, err
		}
		authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, ctx.ProjectID, "projects", "view")
		if permErr != nil {
			return nil, permErr
		}
		if !authorized {
			return nil, &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to access this item",
			}
		}
		story, err := s.userStoryRepo.GetUserStoryByID(req.ItemID, ctx.ProjectID)
		if err != nil {
			return nil, err
		}

		favorite.UserStoryID = &req.ItemID

		// Build user story response payload
		var userStoryStatuses []models.UserStoryStatus
		if s.userStoryStatusRepo != nil {
			userStoryStatuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(ctx.ProjectID)
		}
		var total, completed int64
		var progress float64
		if statsMap, statErr := s.userStoryRepo.GetStoryTaskStats(ctx.ProjectID); statErr == nil {
			if stat, ok := statsMap[req.ItemID]; ok {
				total = stat.TotalTasks
				completed = stat.Completed
				if total > 0 {
					progress = (float64(completed) / float64(total)) * 100.0
				}
			}
		}
		mapped := mapToUserStoryResponse(*story, userStoryStatuses, total, completed, progress)
		userStoryResp = &mapped

	case models.FavoriteItemTypeTask:
		ctx, err := s.taskRepo.GetTaskAccessContext(req.ItemID)
		if err != nil {
			return nil, err
		}
		authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, req.UserID, ctx.ProjectID, "projects", "view")
		if permErr != nil {
			return nil, permErr
		}
		if !authorized {
			return nil, &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "You do not have permission to access this item",
			}
		}
		task, err := s.taskRepo.GetTaskByID(req.ItemID, ctx.ProjectID)
		if err != nil {
			return nil, err
		}

		favorite.TaskID = &req.ItemID

		// Build task response payload
		colorMap, isFinalMap := s.getStatusMaps(ctx.ProjectID)
		mapped := mapToTaskResponse(*task, colorMap, isFinalMap)
		taskResp = &mapped

	default:
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid item_type. Allowed values are 'user_story' and 'task'.",
		}
	}

	if err := s.favoriteRepo.AddFavorite(&favorite); err != nil {
		return nil, err
	}

	var userStoryName, userStoryTitle, taskName, taskTitle string
	var projectID *uuid.UUID
	var projectName string

	if userStoryResp != nil {
		userStoryName = userStoryResp.Title
		userStoryTitle = userStoryResp.Title
		pID := userStoryResp.ProjectID
		projectID = &pID
		projectName = s.getProjectName(pID, userStoryResp.ProjectName)
		if userStoryResp.ProjectName == "" {
			userStoryResp.ProjectName = projectName
		}
	}
	if taskResp != nil {
		taskName = taskResp.Title
		taskTitle = taskResp.Title
		if taskResp.UserStoryTitle != "" {
			userStoryName = taskResp.UserStoryTitle
			userStoryTitle = taskResp.UserStoryTitle
		}
		pID := taskResp.ProjectID
		projectID = &pID
		projectName = s.getProjectName(pID, taskResp.ProjectName)
		if taskResp.ProjectName == "" {
			taskResp.ProjectName = projectName
		}
	}

	return &responsedto.FavoriteResponse{
		ID:             favorite.ID,
		UserID:         favorite.UserID,
		ItemType:       favorite.ItemType,
		UserStoryID:    favorite.UserStoryID,
		TaskID:         favorite.TaskID,
		ProjectID:      projectID,
		ProjectName:    projectName,
		UserStoryName:  userStoryName,
		UserStoryTitle: userStoryTitle,
		TaskName:       taskName,
		TaskTitle:      taskTitle,
		UserStory:      userStoryResp,
		Task:           taskResp,
		CreatedAt:      favorite.CreatedAt,
	}, nil
}

func (s *favoriteService) getStatusMaps(projectID uuid.UUID) (map[string]string, map[string]bool) {
	colorMap := make(map[string]string)
	for k, v := range models.DefaultStatusColors {
		colorMap[k] = v
	}
	isFinalMap := make(map[string]bool)
	for k, v := range models.DefaultStatusIsFinal {
		isFinalMap[k] = v
	}

	if s.customStatusRepo != nil {
		customStatuses, err := s.customStatusRepo.GetStatusesByProjectID(projectID)
		if err == nil {
			for _, cs := range customStatuses {
				colorMap[models.NormalizeTaskStatus(cs.Name)] = cs.Color
				isFinalMap[models.NormalizeTaskStatus(cs.Name)] = cs.IsFinal
			}
		}
	}
	return colorMap, isFinalMap
}

func (s *favoriteService) getProjectName(projectID uuid.UUID, cachedName string) string {
	if cachedName != "" {
		return cachedName
	}
	if s.projectRepo != nil {
		project, err := s.projectRepo.GetProjectByID(projectID)
		if err == nil {
			return project.Name
		}
	}
	return ""
}

func (s *favoriteService) RemoveFavorite(userID uuid.UUID, itemType string, itemID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error) {
	fav, err := s.favoriteRepo.RemoveFavorite(userID, itemType, itemID)
	if err != nil {
		return nil, err
	}
	return &responsedto.RemoveFavoriteResponse{ID: fav.ID}, nil
}

func (s *favoriteService) RemoveFavoriteByID(userID, favoriteID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error) {
	fav, err := s.favoriteRepo.RemoveFavoriteByID(userID, favoriteID)
	if err != nil {
		return nil, err
	}
	return &responsedto.RemoveFavoriteResponse{ID: fav.ID}, nil
}

func (s *favoriteService) GetFavorites(userID uuid.UUID, filter requestdto.GetFavoritesFilter) (*responsedto.FavoriteListResponse, response.Pagination, *response.Error) {
	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	favorites, err := s.favoriteRepo.GetFavoritesByUserID(userID, filter.ItemType)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	// Cache maps for status styling across projects
	usStatusCache := make(map[uuid.UUID][]models.UserStoryStatus)
	statsCache := make(map[uuid.UUID]map[uuid.UUID]models.StoryTaskStats)
	taskColorCache := make(map[uuid.UUID]map[string]string)
	taskIsFinalCache := make(map[uuid.UUID]map[string]bool)

	favResponses := make([]responsedto.FavoriteResponse, 0, len(favorites))
	userStoryResponses := make([]responsedto.UserStoryResponse, 0)
	taskResponses := make([]responsedto.TaskResponse, 0)

	seenItems := make(map[string]bool)

	for _, fav := range favorites {
		key := fav.ItemType + ":"
		if fav.UserStoryID != nil {
			key += fav.UserStoryID.String()
		} else if fav.TaskID != nil {
			key += fav.TaskID.String()
		}
		if key != fav.ItemType+":" {
			if seenItems[key] {
				continue
			}
			seenItems[key] = true
		}

		if fav.ItemType == models.FavoriteItemTypeUserStory {
			if fav.UserStory == nil {
				continue
			}
			if filter.Search != "" {
				searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
				if !strings.Contains(strings.ToLower(fav.UserStory.Title), searchLower) && !strings.Contains(strings.ToLower(fav.UserStory.Description), searchLower) {
					continue
				}
			}

			projectID := fav.UserStory.ProjectID
			authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "view")
			if permErr != nil || !authorized {
				continue
			}
			statuses, ok := usStatusCache[projectID]
			if !ok && s.userStoryStatusRepo != nil {
				statuses, _ = s.userStoryStatusRepo.GetStatusesByProjectID(projectID)
				usStatusCache[projectID] = statuses
			}

			statsMap, ok := statsCache[projectID]
			if !ok {
				statsMap, _ = s.userStoryRepo.GetStoryTaskStats(projectID)
				statsCache[projectID] = statsMap
			}

			var total, completed int64
			var progress float64
			if stat, exists := statsMap[fav.UserStory.ID]; exists {
				total = stat.TotalTasks
				completed = stat.Completed
				if total > 0 {
					progress = (float64(completed) / float64(total)) * 100.0
				}
			}

			usResp := mapToUserStoryResponse(*fav.UserStory, statuses, total, completed, progress)
			pID := fav.UserStory.ProjectID
			pName := fav.UserStory.Project.Name
			if pName == "" {
				pName = s.getProjectName(pID, usResp.ProjectName)
			}
			usResp.ProjectName = pName
			userStoryResponses = append(userStoryResponses, usResp)

			favResponses = append(favResponses, responsedto.FavoriteResponse{
				ID:             fav.ID,
				UserID:         fav.UserID,
				ItemType:       fav.ItemType,
				UserStoryID:    fav.UserStoryID,
				ProjectID:      &pID,
				ProjectName:    pName,
				UserStoryName:  usResp.Title,
				UserStoryTitle: usResp.Title,
				UserStory:      &usResp,
				CreatedAt:      fav.CreatedAt,
			})

		} else if fav.ItemType == models.FavoriteItemTypeTask {
			if fav.Task == nil {
				continue
			}
			if filter.Search != "" {
				searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
				if !strings.Contains(strings.ToLower(fav.Task.Title), searchLower) && !strings.Contains(strings.ToLower(fav.Task.Description), searchLower) {
					continue
				}
			}

			projectID := fav.Task.ProjectID
			authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "projects", "view")
			if permErr != nil || !authorized {
				continue
			}
			colorMap, ok := taskColorCache[projectID]
			if !ok {
				cm, fm := s.getStatusMaps(projectID)
				taskColorCache[projectID] = cm
				taskIsFinalCache[projectID] = fm
				colorMap = cm
			}
			isFinalMap := taskIsFinalCache[projectID]

			tResp := mapToTaskResponse(*fav.Task, colorMap, isFinalMap)
			pID := fav.Task.ProjectID
			pName := fav.Task.Project.Name
			if pName == "" {
				pName = s.getProjectName(pID, tResp.ProjectName)
			}
			tResp.ProjectName = pName
			taskResponses = append(taskResponses, tResp)

			favResponses = append(favResponses, responsedto.FavoriteResponse{
				ID:             fav.ID,
				UserID:         fav.UserID,
				ItemType:       fav.ItemType,
				TaskID:         fav.TaskID,
				ProjectID:      &pID,
				ProjectName:    pName,
				TaskName:       tResp.Title,
				TaskTitle:      tResp.Title,
				UserStoryName:  tResp.UserStoryTitle,
				UserStoryTitle: tResp.UserStoryTitle,
				Task:           &tResp,
				CreatedAt:      fav.CreatedAt,
			})
		}
	}

	sortBy := strings.ToLower(strings.TrimSpace(filter.SortBy))
	isAsc := strings.ToUpper(strings.TrimSpace(filter.SortOrder)) == "ASC"

	if len(favResponses) > 0 {
		sort.Slice(favResponses, func(i, j int) bool {
			switch sortBy {
			case "title", "name":
				titleI := favResponses[i].UserStoryTitle
				if titleI == "" {
					titleI = favResponses[i].TaskTitle
				}
				titleJ := favResponses[j].UserStoryTitle
				if titleJ == "" {
					titleJ = favResponses[j].TaskTitle
				}
				if isAsc {
					return strings.ToLower(titleI) < strings.ToLower(titleJ)
				}
				return strings.ToLower(titleI) > strings.ToLower(titleJ)
			case "created_at":
				if isAsc {
					return favResponses[i].CreatedAt.Before(favResponses[j].CreatedAt)
				}
				return favResponses[i].CreatedAt.After(favResponses[j].CreatedAt)
			default:
				if isAsc {
					return favResponses[i].CreatedAt.Before(favResponses[j].CreatedAt)
				}
				return favResponses[i].CreatedAt.After(favResponses[j].CreatedAt)
			}
		})
	}

	if len(userStoryResponses) > 0 {
		sort.Slice(userStoryResponses, func(i, j int) bool {
			switch sortBy {
			case "title", "name":
				if isAsc {
					return strings.ToLower(userStoryResponses[i].Title) < strings.ToLower(userStoryResponses[j].Title)
				}
				return strings.ToLower(userStoryResponses[i].Title) > strings.ToLower(userStoryResponses[j].Title)
			case "created_at":
				if isAsc {
					return userStoryResponses[i].CreatedAt.Before(userStoryResponses[j].CreatedAt)
				}
				return userStoryResponses[i].CreatedAt.After(userStoryResponses[j].CreatedAt)
			default:
				if isAsc {
					return userStoryResponses[i].CreatedAt.Before(userStoryResponses[j].CreatedAt)
				}
				return userStoryResponses[i].CreatedAt.After(userStoryResponses[j].CreatedAt)
			}
		})
	}

	if len(taskResponses) > 0 {
		sort.Slice(taskResponses, func(i, j int) bool {
			switch sortBy {
			case "title", "name":
				if isAsc {
					return strings.ToLower(taskResponses[i].Title) < strings.ToLower(taskResponses[j].Title)
				}
				return strings.ToLower(taskResponses[i].Title) > strings.ToLower(taskResponses[j].Title)
			case "created_at":
				if isAsc {
					return taskResponses[i].CreatedAt.Before(taskResponses[j].CreatedAt)
				}
				return taskResponses[i].CreatedAt.After(taskResponses[j].CreatedAt)
			default:
				if isAsc {
					return taskResponses[i].CreatedAt.Before(taskResponses[j].CreatedAt)
				}
				return taskResponses[i].CreatedAt.After(taskResponses[j].CreatedAt)
			}
		})
	}

	totalItems := len(favResponses)
	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	startIndex := (filter.Page - 1) * filter.PageSize
	endIndex := startIndex + filter.PageSize

	if startIndex > totalItems {
		startIndex = totalItems
	}
	if endIndex > totalItems {
		endIndex = totalItems
	}

	paginatedFavs := favResponses[startIndex:endIndex]

	paginatedUserStories := make([]responsedto.UserStoryResponse, 0)
	paginatedTasks := make([]responsedto.TaskResponse, 0)
	for _, fav := range paginatedFavs {
		if fav.UserStory != nil {
			paginatedUserStories = append(paginatedUserStories, *fav.UserStory)
		}
		if fav.Task != nil {
			paginatedTasks = append(paginatedTasks, *fav.Task)
		}
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return &responsedto.FavoriteListResponse{
		Favorites:        paginatedFavs,
		Total:            int64(totalItems),
		TotalUserStories: int64(len(userStoryResponses)),
		TotalTasks:       int64(len(taskResponses)),
	}, pagination, nil
}

func (s *favoriteService) AddUserStoryFavorite(userID, projectID, userStoryID uuid.UUID) (*responsedto.FavoriteResponse, *response.Error) {
	_, err := s.userStoryRepo.GetUserStoryByID(userStoryID, projectID)
	if err != nil {
		return nil, err
	}

	return s.AddFavorite(requestdto.AddFavoriteRequest{
		ItemType: models.FavoriteItemTypeUserStory,
		ItemID:   userStoryID,
		UserID:   userID,
	})
}

func (s *favoriteService) RemoveUserStoryFavorite(userID, projectID, userStoryID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error) {
	_, err := s.userStoryRepo.GetUserStoryByID(userStoryID, projectID)
	if err != nil {
		return nil, err
	}

	return s.RemoveFavorite(userID, models.FavoriteItemTypeUserStory, userStoryID)
}

func (s *favoriteService) AddTaskFavorite(userID, projectID, taskID uuid.UUID) (*responsedto.FavoriteResponse, *response.Error) {
	_, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, err
	}

	return s.AddFavorite(requestdto.AddFavoriteRequest{
		ItemType: models.FavoriteItemTypeTask,
		ItemID:   taskID,
		UserID:   userID,
	})
}

func (s *favoriteService) RemoveTaskFavorite(userID, projectID, taskID uuid.UUID) (*responsedto.RemoveFavoriteResponse, *response.Error) {
	_, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, err
	}

	return s.RemoveFavorite(userID, models.FavoriteItemTypeTask, taskID)
}

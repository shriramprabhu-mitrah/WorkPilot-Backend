package services_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubFavoriteRepo struct {
	favorites map[uuid.UUID]*models.Favorite
	usRepo    *stubUserStoryRepo
	taskRepo  *stubTaskRepo
	addErr    *response.Error
	removeErr *response.Error
	getErr    *response.Error
}

func (s *stubFavoriteRepo) AddFavorite(favorite *models.Favorite) *response.Error {
	if s.addErr != nil {
		return s.addErr
	}
	if favorite.ID == uuid.Nil {
		favorite.ID, _ = uuid.NewV7()
	}
	favorite.CreatedAt = time.Now()
	if s.favorites == nil {
		s.favorites = make(map[uuid.UUID]*models.Favorite)
	}

	for _, existing := range s.favorites {
		if existing.UserID == favorite.UserID && existing.ItemType == favorite.ItemType {
			if favorite.ItemType == models.FavoriteItemTypeUserStory && existing.UserStoryID != nil && favorite.UserStoryID != nil && *existing.UserStoryID == *favorite.UserStoryID {
				return &response.Error{Code: response.ErrConflict, StatusCode: http.StatusConflict, Message: "Item is already added to favorites"}
			}
			if favorite.ItemType == models.FavoriteItemTypeTask && existing.TaskID != nil && favorite.TaskID != nil && *existing.TaskID == *favorite.TaskID {
				return &response.Error{Code: response.ErrConflict, StatusCode: http.StatusConflict, Message: "Item is already added to favorites"}
			}
		}
	}

	s.favorites[favorite.ID] = favorite
	return nil
}

func (s *stubFavoriteRepo) RemoveFavorite(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error) {
	if s.removeErr != nil {
		return nil, s.removeErr
	}
	var removed *models.Favorite
	for id, fav := range s.favorites {
		if fav.UserID == userID && fav.ItemType == itemType {
			if itemType == models.FavoriteItemTypeUserStory && fav.UserStoryID != nil && *fav.UserStoryID == itemID {
				removed = fav
				delete(s.favorites, id)
				break
			}
			if itemType == models.FavoriteItemTypeTask && fav.TaskID != nil && *fav.TaskID == itemID {
				removed = fav
				delete(s.favorites, id)
				break
			}
		}
	}
	if removed == nil {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Favorite record not found"}
	}
	return removed, nil
}

func (s *stubFavoriteRepo) RemoveFavoriteByID(userID uuid.UUID, favoriteID uuid.UUID) (*models.Favorite, *response.Error) {
	if s.removeErr != nil {
		return nil, s.removeErr
	}
	fav, ok := s.favorites[favoriteID]
	if !ok || fav.UserID != userID {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Favorite record not found"}
	}
	delete(s.favorites, favoriteID)
	return fav, nil
}

func (s *stubFavoriteRepo) GetFavoritesByUserID(userID uuid.UUID, itemType string) ([]models.Favorite, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	var res []models.Favorite
	for _, fav := range s.favorites {
		if fav.UserID == userID {
			if itemType == "" || fav.ItemType == itemType {
				itemCopy := *fav
				if itemCopy.ItemType == models.FavoriteItemTypeUserStory && itemCopy.UserStoryID != nil && s.usRepo != nil {
					if story, ok := s.usRepo.stories[*itemCopy.UserStoryID]; ok {
						itemCopy.UserStory = story
					}
				} else if itemCopy.ItemType == models.FavoriteItemTypeTask && itemCopy.TaskID != nil && s.taskRepo != nil {
					if task, ok := s.taskRepo.tasks[*itemCopy.TaskID]; ok {
						itemCopy.Task = task
					}
				}
				res = append(res, itemCopy)
			}
		}
	}
	return res, nil
}

func (s *stubFavoriteRepo) GetFavoriteByUserAndItem(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error) {
	for _, fav := range s.favorites {
		if fav.UserID == userID && fav.ItemType == itemType {
			if itemType == models.FavoriteItemTypeUserStory && fav.UserStoryID != nil && *fav.UserStoryID == itemID {
				return fav, nil
			}
			if itemType == models.FavoriteItemTypeTask && fav.TaskID != nil && *fav.TaskID == itemID {
				return fav, nil
			}
		}
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "Favorite record not found"}
}

func (s *stubFavoriteRepo) IsFavorited(userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, *response.Error) {
	fav, err := s.GetFavoriteByUserAndItem(userID, itemType, itemID)
	if err != nil || fav == nil {
		return false, nil
	}
	return true, nil
}

func TestFavoriteService_AddAndGetFavorites(t *testing.T) {
	usRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
	favRepo := &stubFavoriteRepo{
		favorites: make(map[uuid.UUID]*models.Favorite),
		usRepo:    usRepo,
		taskRepo:  taskRepo,
	}

	projectID, _ := uuid.NewV7()
	userID, _ := uuid.NewV7()
	userStoryID, _ := uuid.NewV7()
	taskID, _ := uuid.NewV7()

	usRepo.stories[userStoryID] = &models.UserStory{
		ID:        userStoryID,
		ProjectID: projectID,
		Title:     "Fav User Story",
	}

	taskRepo.tasks[taskID] = &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Fav Task",
	}

	service := services.InitFavoriteService(favRepo, usRepo, taskRepo, nil, nil, nil, zap.NewNop())

	// 1. Add User Story Favorite
	usFavResp, err := service.AddUserStoryFavorite(userID, projectID, userStoryID)
	if err != nil {
		t.Fatalf("Expected no error adding user story favorite, got: %v", err)
	}
	if usFavResp.UserStoryID == nil || *usFavResp.UserStoryID != userStoryID {
		t.Errorf("Expected UserStoryID %v, got %v", userStoryID, usFavResp.UserStoryID)
	}

	// 2. Add Task Favorite
	taskFavResp, err := service.AddTaskFavorite(userID, projectID, taskID)
	if err != nil {
		t.Fatalf("Expected no error adding task favorite, got: %v", err)
	}
	if taskFavResp.TaskID == nil || *taskFavResp.TaskID != taskID {
		t.Errorf("Expected TaskID %v, got %v", taskID, taskFavResp.TaskID)
	}

	// 3. Get All Favorites
	listResp, pageMeta, err := service.GetFavorites(userID, requestdto.GetFavoritesFilter{})
	if err != nil {
		t.Fatalf("Expected no error getting favorites, got: %v", err)
	}
	if listResp.Total != 2 {
		t.Errorf("Expected total favorites count 2, got %d", listResp.Total)
	}
	if listResp.TotalUserStories != 1 {
		t.Errorf("Expected total user stories 1, got %d", listResp.TotalUserStories)
	}
	if listResp.TotalTasks != 1 {
		t.Errorf("Expected total tasks 1, got %d", listResp.TotalTasks)
	}
	if pageMeta.TotalItems != 2 || pageMeta.Page != 1 || pageMeta.PageSize != 10 {
		t.Errorf("Unexpected pagination metadata: %+v", pageMeta)
	}

	// 4. Remove User Story Favorite
	_, err = service.RemoveUserStoryFavorite(userID, projectID, userStoryID)
	if err != nil {
		t.Fatalf("Expected no error removing user story favorite, got: %v", err)
	}

	// 5. Verify remaining favorites count is 1
	listRespAfter, _, err := service.GetFavorites(userID, requestdto.GetFavoritesFilter{})
	if err != nil {
		t.Fatalf("Expected no error getting favorites, got: %v", err)
	}
	if listRespAfter.Total != 1 {
		t.Errorf("Expected total favorites count 1, got %d", listRespAfter.Total)
	}
}

func TestFavoriteService_GetFavoritesWithFilters(t *testing.T) {
	usRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
	favRepo := &stubFavoriteRepo{
		favorites: make(map[uuid.UUID]*models.Favorite),
		usRepo:    usRepo,
		taskRepo:  taskRepo,
	}

	projectID1, _ := uuid.NewV7()
	projectID2, _ := uuid.NewV7()
	userID, _ := uuid.NewV7()
	userStoryID, _ := uuid.NewV7()
	taskID, _ := uuid.NewV7()

	usRepo.stories[userStoryID] = &models.UserStory{
		ID:        userStoryID,
		ProjectID: projectID1,
		Title:     "Frontend UI User Story",
	}

	taskRepo.tasks[taskID] = &models.Task{
		ID:        taskID,
		ProjectID: projectID2,
		Title:     "Backend API Task",
	}

	service := services.InitFavoriteService(favRepo, usRepo, taskRepo, nil, nil, nil, zap.NewNop())
	_, _ = service.AddUserStoryFavorite(userID, projectID1, userStoryID)
	_, _ = service.AddTaskFavorite(userID, projectID2, taskID)

	// Filter by search "Frontend"
	resSearch, _, err := service.GetFavorites(userID, requestdto.GetFavoritesFilter{Search: "Frontend"})
	if err != nil || resSearch.Total != 1 {
		t.Fatalf("Expected 1 result for search 'Frontend', got %d (err: %v)", resSearch.Total, err)
	}

	// Filter by item_type "user_story"
	resItemType, _, err := service.GetFavorites(userID, requestdto.GetFavoritesFilter{ItemType: "user_story"})
	if err != nil || resItemType.Total != 1 {
		t.Fatalf("Expected 1 result for item_type 'user_story', got %d (err: %v)", resItemType.Total, err)
	}

	// Sort by title ASC
	filterSort := requestdto.GetFavoritesFilter{}
	filterSort.SortBy = "title"
	filterSort.SortOrder = "ASC"
	resSort, _, err := service.GetFavorites(userID, filterSort)
	if err != nil || resSort.Total != 2 {
		t.Fatalf("Expected 2 results for sort by title, got %d (err: %v)", resSort.Total, err)
	}
	if getFavTitle(resSort.Favorites[0]) != "Backend API Task" {
		t.Errorf("Expected first sorted title 'Backend API Task', got '%s'", getFavTitle(resSort.Favorites[0]))
	}
}

func getFavTitle(f responsedto.FavoriteResponse) string {
	if f.UserStoryTitle != "" {
		return f.UserStoryTitle
	}
	return f.TaskTitle
}

func TestFavoriteService_GetFavoritesWithPagination(t *testing.T) {
	usRepo := &stubUserStoryRepo{stories: make(map[uuid.UUID]*models.UserStory)}
	taskRepo := &stubTaskRepo{tasks: make(map[uuid.UUID]*models.Task)}
	favRepo := &stubFavoriteRepo{
		favorites: make(map[uuid.UUID]*models.Favorite),
		usRepo:    usRepo,
		taskRepo:  taskRepo,
	}

	projectID, _ := uuid.NewV7()
	userID, _ := uuid.NewV7()
	storyID1, _ := uuid.NewV7()
	storyID2, _ := uuid.NewV7()
	taskID1, _ := uuid.NewV7()

	usRepo.stories[storyID1] = &models.UserStory{ID: storyID1, ProjectID: projectID, Title: "Alpha Story"}
	usRepo.stories[storyID2] = &models.UserStory{ID: storyID2, ProjectID: projectID, Title: "Beta Story"}
	taskRepo.tasks[taskID1] = &models.Task{ID: taskID1, ProjectID: projectID, Title: "Gamma Task"}

	service := services.InitFavoriteService(favRepo, usRepo, taskRepo, nil, nil, nil, zap.NewNop())
	_, _ = service.AddUserStoryFavorite(userID, projectID, storyID1)
	_, _ = service.AddUserStoryFavorite(userID, projectID, storyID2)
	_, _ = service.AddTaskFavorite(userID, projectID, taskID1)

	// Page 1, PageSize 2, Sorted by Title ASC
	filter1 := requestdto.GetFavoritesFilter{
		Search: "",
	}
	filter1.Page = 1
	filter1.PageSize = 2
	filter1.SortBy = "title"
	filter1.SortOrder = "ASC"

	res1, meta1, err := service.GetFavorites(userID, filter1)
	if err != nil {
		t.Fatalf("Expected no error for page 1, got %v", err)
	}
	if res1.Total != 3 {
		t.Errorf("Expected total 3, got %d", res1.Total)
	}
	if len(res1.Favorites) != 2 {
		t.Fatalf("Expected 2 favorites in page 1, got %d", len(res1.Favorites))
	}
	if getFavTitle(res1.Favorites[0]) != "Alpha Story" || getFavTitle(res1.Favorites[1]) != "Beta Story" {
		t.Errorf("Unexpected titles on page 1: %s, %s", getFavTitle(res1.Favorites[0]), getFavTitle(res1.Favorites[1]))
	}
	if meta1.Page != 1 || meta1.PageSize != 2 || meta1.TotalItems != 3 || meta1.TotalPages != 2 || !meta1.HasNext || meta1.HasPrevious {
		t.Errorf("Unexpected meta1: %+v", meta1)
	}

	// Page 2, PageSize 2
	filter2 := filter1
	filter2.Page = 2

	res2, meta2, err := service.GetFavorites(userID, filter2)
	if err != nil {
		t.Fatalf("Expected no error for page 2, got %v", err)
	}
	if len(res2.Favorites) != 1 {
		t.Fatalf("Expected 1 favorite in page 2, got %d", len(res2.Favorites))
	}
	if getFavTitle(res2.Favorites[0]) != "Gamma Task" {
		t.Errorf("Unexpected title on page 2: %s", getFavTitle(res2.Favorites[0]))
	}
	if meta2.Page != 2 || meta2.PageSize != 2 || meta2.TotalItems != 3 || meta2.TotalPages != 2 || meta2.HasNext || !meta2.HasPrevious {
		t.Errorf("Unexpected meta2: %+v", meta2)
	}
}

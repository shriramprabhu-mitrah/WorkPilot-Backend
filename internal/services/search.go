package services

import (
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	searchrepo "github.com/ms-kanban-server/internal/repository/search-repo"
	"go.uber.org/zap"
)

type SearchService interface {
	GlobalSearch(userID, orgID uuid.UUID, query string) (*responsedto.GlobalSearchResponse, *response.Error)
}

type searchService struct {
	searchRepo  searchrepo.SearchRepository
	authRepo    authrepo.AuthRepository
	projectRepo projectrepo.ProjectRepository
	logger      *zap.Logger
}

func InitSearchService(searchRepo searchrepo.SearchRepository, authRepo authrepo.AuthRepository, projectRepo projectrepo.ProjectRepository, logger *zap.Logger) SearchService {
	return &searchService{
		searchRepo:  searchRepo,
		authRepo:    authRepo,
		projectRepo: projectRepo,
		logger:      logger,
	}
}

func (s *searchService) GlobalSearch(userID, orgID uuid.UUID, query string) (*responsedto.GlobalSearchResponse, *response.Error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return &responsedto.GlobalSearchResponse{
			Tasks:       []responsedto.SearchResult{},
			UserStories: []responsedto.SearchResult{},
			Projects:    []responsedto.SearchResult{},
			Members:     []responsedto.SearchResult{},
		}, nil
	}

	tsQuery := buildSearchTsQuery(trimmedQuery)

	var (
		tasks                                           []models.Task
		stories                                         []models.UserStory
		projects                                        []models.Project
		users                                           []models.User
		sprints                                         []models.Sprint
		errTask, errStory, errProj, errUser, errSprints error
		wg                                              sync.WaitGroup
	)

	wg.Add(5)
	go func() {
		defer wg.Done()
		tasks, errTask = s.searchRepo.SearchTasks(orgID, tsQuery, trimmedQuery)
	}()
	go func() {
		defer wg.Done()
		stories, errStory = s.searchRepo.SearchUserStories(orgID, tsQuery, trimmedQuery)
	}()
	go func() {
		defer wg.Done()
		projects, errProj = s.searchRepo.SearchProjects(orgID, tsQuery, trimmedQuery)
	}()
	go func() {
		defer wg.Done()
		users, errUser = s.searchRepo.SearchMembers(orgID, tsQuery, trimmedQuery)
	}()
	go func() {
		defer wg.Done()
		sprints, errSprints = s.searchRepo.SearchSprints(orgID, tsQuery, trimmedQuery)
	}()
	wg.Wait()

	if errTask != nil || errStory != nil || errProj != nil || errUser != nil || errSprints != nil {
		s.logger.Error("one or more search queries failed",
			zap.Error(errTask),
			zap.Error(errStory),
			zap.Error(errProj),
			zap.Error(errUser),
			zap.Error(errSprints),
		)
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to execute search queries",
		}
	}

	res := &responsedto.GlobalSearchResponse{
		Tasks:       make([]responsedto.SearchResult, 0),
		UserStories: make([]responsedto.SearchResult, 0),
		Projects:    make([]responsedto.SearchResult, 0),
		Members:     make([]responsedto.SearchResult, 0),
		Sprints:     make([]responsedto.SearchResult, 0),
	}

	projectAccessCache := make(map[uuid.UUID]bool)
	hasAccess := func(pID uuid.UUID) bool {
		if pID == uuid.Nil {
			return false
		}
		if val, ok := projectAccessCache[pID]; ok {
			return val
		}
		authorized, permErr := CheckPermission(s.authRepo, s.projectRepo, userID, pID, "projects", "view")
		if permErr != nil || !authorized {
			projectAccessCache[pID] = false
			return false
		}
		projectAccessCache[pID] = true
		return true
	}

	for _, t := range tasks {
		projID := t.ProjectID
		if !hasAccess(projID) {
			continue
		}
		res.Tasks = append(res.Tasks, responsedto.SearchResult{
			ID:          t.ID,
			Type:        "task",
			Title:       t.Title,
			Key:         t.Key,
			Description: t.Description,
			Status:      t.Status,
			Priority:    t.Priority,
			ProjectID:   &projID,
			ProjectName: t.Project.Name,
		})
	}

	for _, us := range stories {
		projID := us.ProjectID
		if !hasAccess(projID) {
			continue
		}
		res.UserStories = append(res.UserStories, responsedto.SearchResult{
			ID:          us.ID,
			Type:        "user_story",
			Title:       us.Title,
			Key:         us.Key,
			Description: us.Description,
			Status:      us.Status,
			Priority:    us.Priority,
			ProjectID:   &projID,
			ProjectName: us.Project.Name,
		})
	}

	for _, p := range projects {
		if !hasAccess(p.ID) {
			continue
		}
		res.Projects = append(res.Projects, responsedto.SearchResult{
			ID:          p.ID,
			Type:        "project",
			Title:       p.Name,
			Key:         p.Slug,
			Description: p.Description,
			Status:      p.Status,
		})
	}

	for _, u := range users {
		res.Members = append(res.Members, responsedto.SearchResult{
			ID:          u.ID,
			Type:        "member",
			Title:       u.FullName,
			Key:         u.UserName,
			Description: u.Email,
			AvatarURL:   u.AvatarURL,
		})
	}

	for _, sp := range sprints {
		projID := sp.ProjectID
		if !hasAccess(projID) {
			continue
		}
		res.Sprints = append(res.Sprints, responsedto.SearchResult{
			ID:          sp.ID,
			Type:        "sprint",
			Title:       sp.Name,
			Description: sp.Goal,
			Status:      sp.Status,
			ProjectID:   &projID,
		})
	}

	return res, nil
}

func buildSearchTsQuery(query string) string {
	words := strings.Fields(query)
	var tsqueryParts []string
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	for _, word := range words {
		cleanWord := reg.ReplaceAllString(word, "")
		if cleanWord != "" {
			tsqueryParts = append(tsqueryParts, cleanWord+":*")
		}
	}
	return strings.Join(tsqueryParts, " & ")
}

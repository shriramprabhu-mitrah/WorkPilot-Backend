package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	"github.com/ms-kanban-server/internal/services"
)

func FavoriteRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	favoriteRepo := favoriterepo.InitFavoriteRepository(deps)
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	customStatusRepo := customstatusrepo.InitCustomStatusRepository(deps)
	userStoryStatusRepo := userstorystatusrepo.InitUserStoryStatusRepository(deps)

	// initialize service
	favoriteService := services.InitFavoriteService(
		favoriteRepo,
		userStoryRepo,
		taskRepo,
		projectRepo,
		authRepo,
		customStatusRepo,
		userStoryStatusRepo,
		deps.Logger,
	)

	// initialize handler
	favoriteHandler := handlers.InitFavoriteHandler(favoriteService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	fav := api.Group("/favorites")
	{
		fav.POST("", middleware.ValidateJWT(), favoriteHandler.AddFavorite)
		fav.DELETE("", middleware.ValidateJWT(), favoriteHandler.RemoveFavorite)
		fav.GET("", middleware.ValidateJWT(), favoriteHandler.GetFavorites)
	}
}

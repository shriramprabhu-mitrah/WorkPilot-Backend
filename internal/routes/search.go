package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	searchrepo "github.com/ms-kanban-server/internal/repository/search-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"github.com/ms-kanban-server/internal/services"
)

func SearchRoutes(deps models.Config, api *gin.RouterGroup) {
	// Initialize Repository
	searchRepo := searchrepo.InitSearchRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)

	// Initialize Service
	searchService := services.InitSearchService(searchRepo, authRepo, projectRepo, deps.Logger)

	// Initialize Handler
	searchHandler := handlers.InitSearchHandler(searchService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	// Register Routes
	api.GET("/search", middleware.ValidateJWT(), searchHandler.GlobalSearch)
}

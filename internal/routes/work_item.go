package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	workitemrepo "github.com/ms-kanban-server/internal/repository/work-item-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	"github.com/ms-kanban-server/internal/services"
)

func WorkItemRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	authRepo := authrepo.InitAuthRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	workItemRepo := workitemrepo.InitWorkItemRepository(deps)
	customStatusRepo := customstatusrepo.InitCustomStatusRepository(deps)
	userStoryStatusRepo := userstorystatusrepo.InitUserStoryStatusRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	favoriteRepo := favoriterepo.InitFavoriteRepository(deps)

	// initialize service
	workItemService := services.InitWorkItemService(authRepo, projectRepo, workItemRepo, customStatusRepo, userStoryStatusRepo, taskRepo, userStoryRepo, favoriteRepo, deps.Logger)

	// initialize handler
	workItemHandler := handlers.InitWorkItemHandler(workItemService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	wi := api.Group("/projects/:project_id/work-items")
	{
		wi.GET("/:serial_id", middleware.ValidateJWT(), workItemHandler.GetWorkItemBySerialNumber)
		wi.GET("/task/:key", middleware.ValidateJWT(), workItemHandler.GetTaskByKey)
		wi.GET("/us/:key", middleware.ValidateJWT(), workItemHandler.GetUserStoryByKey)
	}
}

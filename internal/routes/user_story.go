package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	"github.com/ms-kanban-server/internal/services"
)

func UserStoryRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)

	// initialize services
	userStoryService := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, taskRepo, deps.Logger)

	// initialize handlers
	userStoryHandler := handlers.InitUserStoryHandler(userStoryService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	us := api.Group("/projects/:project_id/user-stories")
	{
		us.POST("", middleware.ValidateJWT(), userStoryHandler.CreateUserStory)
		us.PATCH("/reorder", middleware.ValidateJWT(), userStoryHandler.ReorderUserStories)
		us.GET("", middleware.ValidateJWT(), userStoryHandler.GetUserStories)
		us.GET("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.GetUserStoryByID)
		us.PATCH("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.UpdateUserStory)
		us.DELETE("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.DeleteUserStory)
	}
}

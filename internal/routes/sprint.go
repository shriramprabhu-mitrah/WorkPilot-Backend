package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"github.com/ms-kanban-server/internal/services"
)

func SprintRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	sprintRepo := sprintrepo.InitSprintRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	sprintService := services.InitSprintService(sprintRepo, projectRepo, authRepo, deps.Logger)

	// initialize handlers
	sprintHandler := handlers.InitSprintHandler(sprintService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	spr := api.Group("/projects/:project_id/sprint")
	{
		spr.POST("", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.CreateSprint)
		spr.GET("", middleware.ValidateJWT(), sprintHandler.GetSprints)
		spr.GET("/:sprint_id", middleware.ValidateJWT(), sprintHandler.GetSprintByID)
		spr.PATCH("/:sprint_id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.UpdateSprint)
		spr.DELETE("/:sprint_id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.DeleteSprint)
		spr.GET("/:sprint_id/burndown", middleware.ValidateJWT(), sprintHandler.GetSprintBurndown)
		spr.POST("/:sprint_id/snapshot", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.TriggerSnapshot)
	}
}

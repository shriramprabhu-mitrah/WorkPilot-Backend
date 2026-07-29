package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"github.com/ms-kanban-server/internal/services"
)

func SprintRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	sprintRepo := sprintrepo.InitSprintRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)

	// initialize services
	sprintService := services.InitSprintService(sprintRepo, projectRepo, deps.Logger)

	// initialize handlers
	sprintHandler := handlers.InitSprintHandler(sprintService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	spr := api.Group("/projects/:project_id/sprint")
	{
		spr.POST("/create", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.CreateSprint)
		spr.GET("get-sprints", middleware.ValidateJWT(), sprintHandler.GetSprints)
		spr.GET("/:sprint_id", middleware.ValidateJWT(), sprintHandler.GetSprintByID)
		spr.PATCH("/:sprint_id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.UpdateSprint)
		spr.DELETE("/:sprint_id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), sprintHandler.DeleteSprint)
	}
}

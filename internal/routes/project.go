package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"github.com/ms-kanban-server/internal/services"
)

func ProjectRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	ProjectRepo := projectrepo.InitProjectRepository(deps)
	AuthRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	ProjectService := services.InitProjectService(ProjectRepo, AuthRepo, deps.Logger)

	// initialize handlers
	ProjectHandler := handlers.InitProjectHandler(ProjectService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	prj := api.Group("/project")
	{

		prj.POST("/create", middleware.ValidateJWT(), middleware.Authorize("org_admin"), ProjectHandler.CreateProject)
		prj.PATCH("/update/:id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), ProjectHandler.UpdateProject)
		prj.GET("/get", middleware.ValidateJWT(), ProjectHandler.GetProjects)
		prj.PATCH("/archive/:id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), ProjectHandler.ArchiveProject)

	}
}

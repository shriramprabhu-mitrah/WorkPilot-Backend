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
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	projectService := services.InitProjectService(projectRepo, authRepo, deps.Logger)

	// initialize handlers
	projectHandler := handlers.InitProjectHandler(projectService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	prj := api.Group("/project")
	{

		prj.POST("/create", middleware.ValidateJWT(), middleware.Authorize("org_admin"), projectHandler.CreateProject)
		prj.PATCH("/update/:id", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), projectHandler.UpdateProject)
		prj.GET("/get", middleware.ValidateJWT(), projectHandler.GetProjects)
		prj.POST("/add-members", middleware.ValidateJWT(), middleware.Authorize("org_admin", "project_manager"), projectHandler.CreateProjectMember)
		prj.GET("/members/:project_id", middleware.ValidateJWT(), projectHandler.GetProjectMembers)
		prj.DELETE("/:project_id/member/:user_id", middleware.Authorize("org_admin", "project_manager"), projectHandler.RemoveProjectMember)

	}
}

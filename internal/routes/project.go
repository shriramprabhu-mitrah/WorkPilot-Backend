package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"github.com/ms-kanban-server/internal/services"
)

func ProjectRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	sprintRepo := sprintrepo.InitSprintRepository(deps)
	auditRepo := auditrepo.InitAuditLogRepository(deps)

	// initialize services
	projectService := services.InitProjectService(projectRepo, authRepo, sprintRepo, auditRepo, deps.Logger)

	// initialize handlers
	projectHandler := handlers.InitProjectHandler(projectService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	prj := api.Group("/project")
	{

		prj.POST("/create", middleware.ValidateJWT(), middleware.Authorize("org_admin"), projectHandler.CreateProject)
		prj.PATCH("/update/:project_id", middleware.ValidateJWT(), projectHandler.UpdateProject)
		prj.GET("/get", middleware.ValidateJWT(), projectHandler.GetProjects)
		prj.POST("/add-members", middleware.ValidateJWT(), middleware.Authorize("org_admin"), projectHandler.CreateProjectMember)
		prj.GET("/members/:project_id", middleware.ValidateJWT(), projectHandler.GetProjectMembers)
		prj.DELETE("/:project_id/member/:user_id", middleware.ValidateJWT(), projectHandler.RemoveProjectMember)
		prj.GET("/:project_id/activity", middleware.ValidateJWT(), projectHandler.GetProjectActivity)
		prj.GET("/:project_id/detail", middleware.ValidateJWT(), projectHandler.GetProjectDetails)
		prj.DELETE("/:project_id", middleware.ValidateJWT(), middleware.Authorize("org_admin"), projectHandler.Deleteproject)
		prj.GET("/user/:user_id", middleware.ValidateJWT(), projectHandler.GetProjectByUser)
		prj.PATCH("/:project_id/member/:user_id", middleware.ValidateJWT(), projectHandler.UpdateProjectMember)
	}
}

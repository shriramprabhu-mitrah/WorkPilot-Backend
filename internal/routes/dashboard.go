package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	dashboardrepo "github.com/ms-kanban-server/internal/repository/dashboard-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"github.com/ms-kanban-server/internal/services"
)

func DashboardRoutes(deps models.Config, api *gin.RouterGroup) {

	dashboardrepo := dashboardrepo.InitDashboardRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	sprintRepo := sprintrepo.InitSprintRepository(deps)

	dashboardService := services.InitDashboardService(dashboardrepo, projectRepo, authRepo, sprintRepo, taskRepo, auditRepo, deps.Logger)

	dashboardhandler := handlers.InitDashboardHandler(dashboardService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	dashbrd := api.Group("/:project_id")

	{
		dashbrd.GET("/overview", middleware.ValidateJWT(), dashboardhandler.GetOverview)
		dashbrd.GET("/task-status", middleware.ValidateJWT(), dashboardhandler.GetTaskStatus)
		dashbrd.GET("/sprint-burndown", middleware.ValidateJWT(), dashboardhandler.GetSprintBurndown)
		dashbrd.GET("/weekly-progress", middleware.ValidateJWT(), dashboardhandler.GetWeeklyProgress)
		dashbrd.GET("/team-workload", middleware.ValidateJWT(), dashboardhandler.GetTeamWorkload)

		dashbrd.GET("/dashboard", middleware.ValidateJWT(), dashboardhandler.GetDashboardData)
	}

}

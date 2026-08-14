package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"github.com/ms-kanban-server/internal/services"
)

func CustomStatusRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	statusRepo := customstatusrepo.InitCustomStatusRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)

	// initialize services
	statusService := services.InitCustomStatusService(statusRepo, projectRepo, authRepo, auditRepo, taskRepo, deps.Logger)

	// initialize handlers
	statusHandler := handlers.InitCustomStatusHandler(statusService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	st := api.Group("/projects/:project_id/custom-statuses")
	{
		st.POST("", middleware.ValidateJWT(), statusHandler.CreateStatus)
		st.GET("", middleware.ValidateJWT(), statusHandler.GetStatuses)
		st.PATCH("/:status_id", middleware.ValidateJWT(), statusHandler.UpdateStatus)
		st.DELETE("/:status_id", middleware.ValidateJWT(), statusHandler.DeleteStatus)
	}
}

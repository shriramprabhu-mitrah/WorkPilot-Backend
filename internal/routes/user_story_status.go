package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	"github.com/ms-kanban-server/internal/services"
)

func UserStoryStatusRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	statusRepo := userstorystatusrepo.InitUserStoryStatusRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)

	// initialize services
	statusService := services.InitUserStoryStatusService(statusRepo, projectRepo, authRepo, auditRepo, userStoryRepo, deps.Logger)

	// initialize handlers
	statusHandler := handlers.InitUserStoryStatusHandler(statusService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	st := api.Group("/projects/:project_id/user-story-statuses")
	{
		st.POST("", middleware.ValidateJWT(), statusHandler.CreateStatus)
		st.GET("", middleware.ValidateJWT(), statusHandler.GetStatuses)
		st.PATCH("/:status_id", middleware.ValidateJWT(), statusHandler.UpdateStatus)
		st.DELETE("/:status_id", middleware.ValidateJWT(), statusHandler.DeleteStatus)
	}
}

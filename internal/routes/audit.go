package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"github.com/ms-kanban-server/internal/services"
)

func AuditRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	auditRepo := auditrepo.InitAuditRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	auditService := services.InitAuditService(auditRepo, authRepo, deps.Logger)

	// initialize handlers
	auditHandler := handlers.InitAuditHandler(auditService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	aud := api.Group("/audit")
	{
		aud.GET("", middleware.ValidateJWT(), auditHandler.GetAuditLogs)
	}
}

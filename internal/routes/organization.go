package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/repository"
	"github.com/ms-kanban-server/internal/services"
)

func OrganizationRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	OrganizationRepo := repository.InitOrganizationRepository(deps)
	AuthRepo := repository.InitAuthRepository(deps)

	// initialize services
	OrganizationService := services.InitOrganizationService(OrganizationRepo, AuthRepo, deps.Logger)

	// initialize handlers
	OrganizationHandler := handlers.InitOrganizationHandler(OrganizationService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	org := api.Group("/organization")
	{
		org.DELETE("/delete", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.DeleteOrganization)
		org.POST("/create", middleware.ValidateJWT(), OrganizationHandler.CreateOrganization)
		org.PATCH("/update", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateOrganization)
		org.GET("/get", middleware.ValidateJWT(), OrganizationHandler.GetOrganizationByID)
		org.PATCH("/user-status", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserStatus)
		org.PATCH("/user-role", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserRole)
		org.GET("/get-users", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.GetUserInOrganization)
	}
}

package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/storage"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	organizationrepo "github.com/ms-kanban-server/internal/repository/organization-repo"
	publicrepo "github.com/ms-kanban-server/internal/repository/public-repo"
	"github.com/ms-kanban-server/internal/services"
)

func OrganizationRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	OrganizationRepo := organizationrepo.InitOrganizationRepository(deps)
	AuthRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	OrganizationService := services.InitOrganizationService(OrganizationRepo, AuthRepo, deps.Logger)

	publicRepo := publicrepo.InitPublicRepository(deps)
	publicService := services.InitPublicService(publicRepo, deps.Logger)

	// initialize storage client
	storageClient := storage.NewS3Client(deps.Logger)

	// initialize handlers
	OrganizationHandler := handlers.InitOrganizationHandler(OrganizationService, publicService, storageClient, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	org := api.Group("/organization")
	{
		org.DELETE("/delete", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.DeleteOrganization)
		org.POST("/create", middleware.ValidateJWT(), OrganizationHandler.CreateOrganization)
		org.PATCH("/update", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateOrganization)
		org.GET("/get", middleware.ValidateJWT(), OrganizationHandler.GetOrganizationByID)
		org.PATCH("/user-status", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserStatus)
		org.PATCH("/user-role", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserRole)
		org.POST("/invite", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.InviteOrganizationMember)
		org.POST("/invitations/accept", middleware.ValidateJWT(), OrganizationHandler.AcceptInvitation)
		org.GET("/get-users", middleware.ValidateJWT(), OrganizationHandler.GetUserInOrganization)
		org.DELETE("/remove-user/:user_id", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.RemoveUser)
	}
}

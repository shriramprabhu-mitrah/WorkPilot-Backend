package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/storage"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	organizationrepo "github.com/ms-kanban-server/internal/repository/organization-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	publicrepo "github.com/ms-kanban-server/internal/repository/public-repo"
	rolerepo "github.com/ms-kanban-server/internal/repository/role-repo"
	"github.com/ms-kanban-server/internal/services"
)

func OrganizationRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	OrganizationRepo := organizationrepo.InitOrganizationRepository(deps)
	AuthRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	roleRepo := rolerepo.InitRoleRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)

	// initialize services
	OrganizationService := services.InitOrganizationService(OrganizationRepo, AuthRepo, auditRepo, deps.Logger)
	roleService := services.InitRoleService(roleRepo, deps.Logger)
	authService := services.InitAuthService(AuthRepo, auditRepo, deps.Logger)
	authService.SetProjectRepository(projectRepo)

	publicRepo := publicrepo.InitPublicRepository(deps)
	publicService := services.InitPublicService(publicRepo, deps.Logger)

	// initialize storage client
	storageClient := storage.NewS3Client(deps.Logger)

	// initialize handlers
	OrganizationHandler := handlers.InitOrganizationHandler(OrganizationService, publicService, storageClient, deps.Logger)
	roleHandler := handlers.InitRoleHandler(roleService, authService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	org := api.Group("/organization")
	{
		org.DELETE("/delete", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.DeleteOrganization)
		org.POST("/create", middleware.ValidateJWT(), OrganizationHandler.CreateOrganization)
		org.PATCH("/update", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateOrganization)
		org.GET("/get", middleware.ValidateJWT(), OrganizationHandler.GetOrganizationByID)
		org.GET("", middleware.ValidateJWT(), middleware.Authorize("super_admin"), OrganizationHandler.GetAllOrganizations)
		org.PATCH("/status/:organization_id", middleware.ValidateJWT(), middleware.Authorize("super_admin"), OrganizationHandler.UpdateOrganizationStatus)
		org.PATCH("/user-status", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserStatus)
		org.PATCH("/user-role", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.UpdateUserRole)
		org.POST("/invite", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.InviteOrganizationMember)
		org.GET("/invitations/accept", OrganizationHandler.AcceptInvitationPage)
		org.POST("/invitations/accept", middleware.ValidateJWT(), OrganizationHandler.AcceptInvitation)
		org.GET("/get-users", middleware.ValidateJWT(), OrganizationHandler.GetUserInOrganization)
		org.GET("/all-members", middleware.ValidateJWT(), middleware.Authorize("super_admin"), OrganizationHandler.GetAllMembers)
		org.DELETE("/remove-user/:user_id", middleware.ValidateJWT(), middleware.Authorize("org_admin"), OrganizationHandler.RemoveUser)

		// Role management endpoints
		org.POST("/roles", middleware.ValidateJWT(), roleHandler.CreateRole)
		org.GET("/roles", middleware.ValidateJWT(), roleHandler.GetRoles)
		org.GET("/roles/:role_id", middleware.ValidateJWT(), roleHandler.GetRole)
		org.PATCH("/roles/:role_id", middleware.ValidateJWT(), roleHandler.UpdateRole)
		org.DELETE("/roles/:role_id", middleware.ValidateJWT(), roleHandler.DeleteRole)
	}
}

package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/storage"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"github.com/ms-kanban-server/internal/services"
)

func AuthRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	authRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditLogRepository(deps)

	// initialize services
	authService := services.InitAuthService(authRepo, auditRepo, deps.Logger)

	// initialize storage client
	storageClient := storage.NewS3Client(deps.Logger)

	// initialize handlers
	authHandler := handlers.InitAuthHandler(authService, storageClient, deps.Logger)

	// initialize middleware
	middleware := middleware.InitMiddleware(deps.Logger)

	auth := api.Group("/auth")
	{
		auth.POST("/signin", authHandler.SignIn)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/logout", middleware.ValidateJWT(), authHandler.Logout)
		auth.POST("/signup", authHandler.SignUp)
		auth.POST("/verify-email", authHandler.VerifyEmail)
		auth.POST("/resend-verification-otp", authHandler.ResendVerificationOTP)
		auth.POST("/change-password", middleware.ValidateJWT(), authHandler.ChangePassword)
		auth.POST("/password-reset/request", authHandler.RequestPasswordReset)
		auth.POST("/password-reset/confirm", authHandler.ResetPassword)
		auth.PATCH("/update", middleware.ValidateJWT(), authHandler.UpdateUser)
		auth.GET("/me", middleware.ValidateJWT(), authHandler.GetUser)
		auth.GET("/validate", authHandler.Validate)
		auth.GET("/:user_id", middleware.ValidateJWT(), authHandler.GetUserByID)
	}
}

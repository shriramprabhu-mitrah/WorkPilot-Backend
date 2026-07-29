package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"github.com/ms-kanban-server/internal/services"
)

func AuthRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	AuthRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	AuthService := services.InitAuthService(AuthRepo, deps.Logger)

	// initialize handlers
	AuthHandler := handlers.InitAuthHandler(AuthService, deps.Logger)

	// initialize middleware
	middleware := middleware.InitMiddleware(deps.Logger)

	auth := api.Group("/auth")
	{
		auth.POST("/signin", AuthHandler.SignIn)
		auth.POST("/refresh", AuthHandler.RefreshToken)
		auth.POST("/logout", middleware.ValidateJWT(), AuthHandler.Logout)
		auth.POST("/signup", AuthHandler.SignUp)
		auth.POST("/verify-email", AuthHandler.VerifyEmail)
		auth.POST("/resend-verification-otp", AuthHandler.ResendVerificationOTP)
		auth.POST("/change-password", middleware.ValidateJWT(), AuthHandler.ChangePassword)
		auth.POST("/password-reset/request", AuthHandler.RequestPasswordReset)
		auth.POST("/password-reset/confirm", AuthHandler.ResetPassword)
		auth.PATCH("/update", middleware.ValidateJWT(), AuthHandler.UpdateUser)
		auth.GET("/me", middleware.ValidateJWT(), AuthHandler.GetUser)
		auth.GET("/validate", AuthHandler.Validate)
	}
}

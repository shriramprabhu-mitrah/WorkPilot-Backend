package routes

import (
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/pkg/models"
	publicrepo "github.com/ms-kanban-server/internal/repository/public-repo"
	"github.com/ms-kanban-server/internal/services"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// PublicRoutes registers publicly accessible routes (health checks, swagger, etc.)
func PublicRoutes(deps models.Config) {
	publicRepo := publicrepo.InitPublicRepository(deps)
	publicService := services.InitPublicService(publicRepo, deps.Logger)
	publicHandler := handlers.InitPublicHandler(deps.Logger, publicService)

	// Register public routes
	deps.Router.GET("/health", publicHandler.HealthHandler(deps))
	deps.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	deps.Router.GET("/countries", publicHandler.GetAllCountries)
}

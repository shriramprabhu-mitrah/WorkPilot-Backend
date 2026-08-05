package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	labelrepo "github.com/ms-kanban-server/internal/repository/label-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	"github.com/ms-kanban-server/internal/services"
)

func LabelRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	labelRepo := labelrepo.InitLabelRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	labelService := services.InitLabelService(labelRepo, projectRepo, authRepo, deps.Logger)

	// initialize handlers
	labelHandler := handlers.InitLabelHandler(labelService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	lbl := api.Group("/projects/:project_id/labels")
	{
		lbl.POST("/", middleware.ValidateJWT(), labelHandler.CreateLabel)
		lbl.GET("/", middleware.ValidateJWT(), labelHandler.GetLabels)
		lbl.PATCH("/:label_id", middleware.ValidateJWT(), labelHandler.UpdateLabel)
		lbl.DELETE("/:label_id", middleware.ValidateJWT(), labelHandler.DeleteLabel)
	}
}

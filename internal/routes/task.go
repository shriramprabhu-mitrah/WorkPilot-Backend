package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"github.com/ms-kanban-server/internal/services"
)

func TaskRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	taskRepo := taskrepo.InitTaskRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)

	// initialize services
	taskService := services.InitTaskService(authRepo, projectRepo, taskRepo, deps.Logger)

	// initialize handlers
	taskHandler := handlers.InitTaskHandler(taskService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	tsk := api.Group("/projects/:project_id/tasks")
	{
		tsk.POST("/", middleware.ValidateJWT(), taskHandler.CreateTask)
		tsk.GET("/", middleware.ValidateJWT(), taskHandler.GetTasks)
		tsk.PATCH("/bulk", middleware.ValidateJWT(), taskHandler.BulkUpdateTasks)
		tsk.GET("/:task_id", middleware.ValidateJWT(), taskHandler.GetTaskByID)
		tsk.PATCH("/:task_id", middleware.ValidateJWT(), taskHandler.UpdateTask)
		tsk.DELETE("/:task_id", middleware.ValidateJWT(), taskHandler.DeleteTask)
		tsk.POST("/:task_id/restore", middleware.ValidateJWT(), taskHandler.RestoreTask)
		tsk.POST("/:task_id/clone", middleware.ValidateJWT(), taskHandler.CloneTask)
		tsk.POST("/:task_id/labels/:label_id", middleware.ValidateJWT(), taskHandler.AttachLabelToTask)
		tsk.DELETE("/:task_id/labels/:label_id", middleware.ValidateJWT(), taskHandler.RemoveLabelFromTask)
	}
}

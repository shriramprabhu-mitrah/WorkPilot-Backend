package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"github.com/ms-kanban-server/internal/services"
)

func CommentsRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	commentsRepo := commentsrepo.InitCommentsRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)

	// initialize services
	commentsService := services.InitCommentsService(commentsRepo, taskRepo, projectRepo, authRepo, auditRepo, deps.Logger)

	// initialize handlers
	commentsHandler := handlers.InitCommentsHandler(commentsService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	cmt := api.Group("/task/:task_id/comments")
	{
		cmt.POST("", middleware.ValidateJWT(), commentsHandler.CreateComments)
		cmt.GET("/:comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentByID)
		cmt.GET("/replies/:parent_comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentsByParentID)
		cmt.GET("", middleware.ValidateJWT(), commentsHandler.GetCommentsByTaskID)
		cmt.PATCH("/:comment_id", middleware.ValidateJWT(), commentsHandler.UpdateComments)
		cmt.DELETE("/:comment_id", middleware.ValidateJWT(), commentsHandler.DeleteComments)
	}
}

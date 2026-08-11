package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/storage"
	attachmentrepo "github.com/ms-kanban-server/internal/repository/attachment-repo"
	commentattachmentrepo "github.com/ms-kanban-server/internal/repository/comment-attachment-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"

	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
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
	auditRepo := auditrepo.InitAuditLogRepository(deps)
	attachmentRepo := attachmentrepo.InitAttachmentRepository(deps)
	commentsRepo := commentsrepo.InitCommentsRepository(deps)
	commentAttachmentRepo := commentattachmentrepo.InitCommentAttachmentRepository(deps)
	// initialize services
	taskService := services.InitTaskService(authRepo, projectRepo, taskRepo, auditRepo, deps.Logger)
	storageClient := storage.NewS3Client(deps.Logger)
	attachmentService := services.InitAttachmentService(attachmentRepo, commentAttachmentRepo, commentsRepo, taskRepo, projectRepo, authRepo, auditRepo, storageClient, deps.Logger)

	// initialize handlers
	taskHandler := handlers.InitTaskHandler(taskService, deps.Logger)
	attachmentHandler := handlers.InitAttachmentHandler(attachmentService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	tsk := api.Group("/projects/:project_id/tasks")
	{
		tsk.POST("", middleware.ValidateJWT(), taskHandler.CreateTask)
		tsk.GET("", middleware.ValidateJWT(), taskHandler.GetTasks)
		tsk.PATCH("/bulk", middleware.ValidateJWT(), taskHandler.BulkUpdateTasks)
		tsk.GET("/:task_id", middleware.ValidateJWT(), taskHandler.GetTaskByID)
		tsk.PATCH("/:task_id", middleware.ValidateJWT(), taskHandler.UpdateTask)
		tsk.DELETE("", middleware.ValidateJWT(), taskHandler.DeleteTasks)
		tsk.POST("/:task_id/restore", middleware.ValidateJWT(), taskHandler.RestoreTask)
		tsk.POST("/:task_id/clone", middleware.ValidateJWT(), taskHandler.CloneTask)
		tsk.PUT("/:task_id/labels/:label_id", middleware.ValidateJWT(), taskHandler.AttachLabelToTask)
		tsk.DELETE("/:task_id/labels/:label_id", middleware.ValidateJWT(), taskHandler.RemoveLabelFromTask)

		// Attachment routes
		tsk.POST("/:task_id/attachments", middleware.ValidateJWT(), attachmentHandler.UploadAttachment)
		tsk.GET("/:task_id/attachments", middleware.ValidateJWT(), attachmentHandler.GetAttachments)
		tsk.GET("/:task_id/attachments/:attachment_id/download", middleware.ValidateJWT(), attachmentHandler.DownloadAttachment)
		tsk.DELETE("/:task_id/attachments/:attachment_id", middleware.ValidateJWT(), attachmentHandler.DeleteAttachment)
	}
}

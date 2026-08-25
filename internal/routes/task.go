package routes

import (
	"github.com/gin-gonic/gin"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/middleware"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/storage"
	attachmentrepo "github.com/ms-kanban-server/internal/repository/attachment-repo"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	commentattachmentrepo "github.com/ms-kanban-server/internal/repository/comment-attachment-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"
	customstatusrepo "github.com/ms-kanban-server/internal/repository/custom-status-repo"
	filecleanuprepo "github.com/ms-kanban-server/internal/repository/file-cleanup-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryattachmentrepo "github.com/ms-kanban-server/internal/repository/user-story-attachment-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	userstorystatusrepo "github.com/ms-kanban-server/internal/repository/user-story-status-repo"
	favoriterepo "github.com/ms-kanban-server/internal/repository/favorite-repo"
	"github.com/ms-kanban-server/internal/services"
)

func TaskRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	taskRepo := taskrepo.InitTaskRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	attachmentRepo := attachmentrepo.InitAttachmentRepository(deps)
	commentsRepo := commentsrepo.InitCommentsRepository(deps)
	commentAttachmentRepo := commentattachmentrepo.InitCommentAttachmentRepository(deps)
	cleanupRepo := filecleanuprepo.InitFileCleanupRepository(deps)
	customStatusRepo := customstatusrepo.InitCustomStatusRepository(deps)
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	userStoryAttachmentRepo := userstoryattachmentrepo.InitUserStoryAttachmentRepository(deps)
	userStoryStatusRepo := userstorystatusrepo.InitUserStoryStatusRepository(deps)
	favoriteRepo := favoriterepo.InitFavoriteRepository(deps)

	// initialize services
	taskService := services.InitTaskService(authRepo, projectRepo, taskRepo, userStoryRepo, auditRepo, customStatusRepo, favoriteRepo, deps.Logger)
	storageClient := storage.NewS3Client(deps.Logger)
	attachmentService := services.InitAttachmentService(attachmentRepo, commentAttachmentRepo, userStoryAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, userStoryRepo, projectRepo, authRepo, auditRepo, storageClient, deps.Logger, deps.Context)
	favoriteService := services.InitFavoriteService(favoriteRepo, userStoryRepo, taskRepo, projectRepo, customStatusRepo, userStoryStatusRepo, deps.Logger)

	// initialize handlers
	taskHandler := handlers.InitTaskHandler(taskService, deps.Logger)
	attachmentHandler := handlers.InitAttachmentHandler(attachmentService, deps.Logger)
	favoriteHandler := handlers.InitFavoriteHandler(favoriteService, deps.Logger)

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

		// Favorite routes
		tsk.POST("/:task_id/favorite", middleware.ValidateJWT(), favoriteHandler.AddTaskFavorite)
		tsk.DELETE("/:task_id/favorite", middleware.ValidateJWT(), favoriteHandler.RemoveTaskFavorite)

		// Attachment routes
		tsk.POST("/:task_id/attachments", middleware.ValidateJWT(), attachmentHandler.UploadAttachment)
		tsk.GET("/:task_id/attachments", middleware.ValidateJWT(), attachmentHandler.GetAttachments)
		tsk.GET("/:task_id/attachments/:attachment_id/download", middleware.ValidateJWT(), attachmentHandler.DownloadAttachment)
		tsk.DELETE("/:task_id/attachments/:attachment_id", middleware.ValidateJWT(), attachmentHandler.DeleteAttachment)
	}
}

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
	filecleanuprepo "github.com/ms-kanban-server/internal/repository/file-cleanup-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryattachmentrepo "github.com/ms-kanban-server/internal/repository/user-story-attachment-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	"github.com/ms-kanban-server/internal/services"
)

func CommentsRoutes(deps models.Config, api *gin.RouterGroup) {

	// initialize repositories
	commentsRepo := commentsrepo.InitCommentsRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	attachmentRepo := attachmentrepo.InitAttachmentRepository(deps)
	commentAttachmentRepo := commentattachmentrepo.InitCommentAttachmentRepository(deps)
	cleanupRepo := filecleanuprepo.InitFileCleanupRepository(deps)
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	userStoryAttachmentRepo := userstoryattachmentrepo.InitUserStoryAttachmentRepository(deps)

	// initialize services
	commentsService := services.InitCommentsService(commentsRepo, taskRepo, userStoryRepo, projectRepo, authRepo, auditRepo, deps.Logger)
	storageClient := storage.NewS3Client(deps.Logger)
	attachmentService := services.InitAttachmentService(attachmentRepo, commentAttachmentRepo, userStoryAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, userStoryRepo, projectRepo, authRepo, auditRepo, storageClient, deps.Logger, deps.Context)

	// initialize handlers
	commentsHandler := handlers.InitCommentsHandler(commentsService, deps.Logger)
	attachmentHandler := handlers.InitAttachmentHandler(attachmentService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	cmt := api.Group("/task/:task_id/comments")
	{
		cmt.POST("", middleware.ValidateJWT(), commentsHandler.CreateComments)
		cmt.GET("/:comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentByID)
		cmt.GET("/replies/:parent_comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentsByParentID)
		cmt.GET("", middleware.ValidateJWT(), commentsHandler.GetCommentsByTaskID)
		cmt.PATCH("/:comment_id", middleware.ValidateJWT(), commentsHandler.UpdateComments)
		cmt.DELETE("/:comment_id", middleware.ValidateJWT(), commentsHandler.DeleteComments)

		// Attachment routes
		cmt.POST("/:comment_id/attachments", middleware.ValidateJWT(), attachmentHandler.UploadCommentAttachment)
		cmt.GET("/:comment_id/attachments", middleware.ValidateJWT(), attachmentHandler.GetCommentAttachments)
		cmt.GET("/:comment_id/attachments/:attachment_id/download", middleware.ValidateJWT(), attachmentHandler.DownloadCommentAttachment)
		cmt.DELETE("/:comment_id/attachments/:attachment_id", middleware.ValidateJWT(), attachmentHandler.DeleteCommentAttachment)
	}
}

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

func UserStoryRoutes(deps models.Config, api *gin.RouterGroup) {
	// initialize repositories
	userStoryRepo := userstoryrepo.InitUserStoryRepository(deps)
	projectRepo := projectrepo.InitProjectRepository(deps)
	authRepo := authrepo.InitAuthRepository(deps)
	taskRepo := taskrepo.InitTaskRepository(deps)
	customStatusRepo := customstatusrepo.InitCustomStatusRepository(deps)
	userStoryStatusRepo := userstorystatusrepo.InitUserStoryStatusRepository(deps)
	auditRepo := auditrepo.InitAuditRepository(deps)
	attachmentRepo := attachmentrepo.InitAttachmentRepository(deps)
	commentsRepo := commentsrepo.InitCommentsRepository(deps)
	commentAttachmentRepo := commentattachmentrepo.InitCommentAttachmentRepository(deps)
	cleanupRepo := filecleanuprepo.InitFileCleanupRepository(deps)
	userStoryAttachmentRepo := userstoryattachmentrepo.InitUserStoryAttachmentRepository(deps)
	favoriteRepo := favoriterepo.InitFavoriteRepository(deps)

	// initialize services
	userStoryService := services.InitUserStoryService(authRepo, projectRepo, userStoryRepo, taskRepo, customStatusRepo, userStoryStatusRepo, auditRepo, favoriteRepo, deps.Logger)
	storageClient := storage.NewS3Client(deps.Logger)
	attachmentService := services.InitAttachmentService(attachmentRepo, commentAttachmentRepo, userStoryAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, userStoryRepo, projectRepo, authRepo, auditRepo, storageClient, deps.Logger, deps.Context)
	commentsService := services.InitCommentsService(commentsRepo, taskRepo, userStoryRepo, projectRepo, authRepo, auditRepo, deps.Logger)
	favoriteService := services.InitFavoriteService(favoriteRepo, userStoryRepo, taskRepo, projectRepo, authRepo, customStatusRepo, userStoryStatusRepo, deps.Logger)

	// initialize handlers
	userStoryHandler := handlers.InitUserStoryHandler(userStoryService, deps.Logger)
	attachmentHandler := handlers.InitAttachmentHandler(attachmentService, deps.Logger)
	commentsHandler := handlers.InitCommentsHandler(commentsService, deps.Logger)
	favoriteHandler := handlers.InitFavoriteHandler(favoriteService, deps.Logger)

	middleware := middleware.InitMiddleware(deps.Logger)

	us := api.Group("/projects/:project_id/user-stories")
	{
		us.POST("", middleware.ValidateJWT(), userStoryHandler.CreateUserStory)
		us.PATCH("/reorder", middleware.ValidateJWT(), userStoryHandler.ReorderUserStories)
		us.GET("", middleware.ValidateJWT(), userStoryHandler.GetUserStories)
		us.GET("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.GetUserStoryByID)
		us.PATCH("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.UpdateUserStory)
		us.PATCH("/:user_story_id/status", middleware.ValidateJWT(), userStoryHandler.UpdateUserStoryStatus)
		us.DELETE("/:user_story_id", middleware.ValidateJWT(), userStoryHandler.DeleteUserStory)

		// Favorite routes
		us.POST("/:user_story_id/favorite", middleware.ValidateJWT(), favoriteHandler.AddUserStoryFavorite)
		us.DELETE("/:user_story_id/favorite", middleware.ValidateJWT(), favoriteHandler.RemoveUserStoryFavorite)

		// Attachment routes
		us.POST("/:user_story_id/attachments", middleware.ValidateJWT(), attachmentHandler.UploadUserStoryAttachment)
		us.GET("/:user_story_id/attachments", middleware.ValidateJWT(), attachmentHandler.GetUserStoryAttachments)
		us.GET("/:user_story_id/attachments/:attachment_id/download", middleware.ValidateJWT(), attachmentHandler.DownloadUserStoryAttachment)
		us.DELETE("/:user_story_id/attachments/:attachment_id", middleware.ValidateJWT(), attachmentHandler.DeleteUserStoryAttachment)

		// Comments routes
		us.POST("/:user_story_id/comments", middleware.ValidateJWT(), commentsHandler.CreateComments)
		us.GET("/:user_story_id/comments", middleware.ValidateJWT(), commentsHandler.GetCommentsByUserStoryID)
		us.GET("/:user_story_id/comments/:comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentByID)
		us.GET("/:user_story_id/comments/replies/:parent_comment_id", middleware.ValidateJWT(), commentsHandler.GetCommentsByParentID)
		us.PATCH("/:user_story_id/comments/:comment_id", middleware.ValidateJWT(), commentsHandler.UpdateComments)
		us.DELETE("/:user_story_id/comments/:comment_id", middleware.ValidateJWT(), commentsHandler.DeleteComments)
	}
}

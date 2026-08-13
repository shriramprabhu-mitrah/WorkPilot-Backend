package services

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/storage"
	attachmentrepo "github.com/ms-kanban-server/internal/repository/attachment-repo"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	commentattachmentrepo "github.com/ms-kanban-server/internal/repository/comment-attachment-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"
	filecleanuprepo "github.com/ms-kanban-server/internal/repository/file-cleanup-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type JitterSource interface {
	Int63n(n int64) int64
}

type productionJitterSource struct{}

func (p productionJitterSource) Int63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return rand.Int63n(n)
}

type AttachmentService interface {
	// Task attachments
	UploadAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error)
	GetAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error)
	DownloadAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error

	// Comment attachments
	UploadCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error)
	GetCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error)
	DownloadCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) *response.Error

	GetConfig() models.AttachmentConfig
}

type attachmentService struct {
	attachmentRepo        attachmentrepo.AttachmentRepository
	commentAttachmentRepo commentattachmentrepo.CommentAttachmentRepository
	cleanupRepo           filecleanuprepo.FileCleanupRepository
	commentsRepo          commentsrepo.CommentsRepository
	taskRepo              taskrepo.TaskRepository
	projectRepo           projectrepo.ProjectRepository
	authRepo              authrepo.AuthRepository
	auditRepo             auditrepo.AuditLogRepository
	storageClient         storage.StorageClient
	logger                *zap.Logger
	cfg                   models.AttachmentConfig
	claimTTL              time.Duration
	jitterSource          JitterSource
}

func InitAttachmentService(
	attachmentRepo attachmentrepo.AttachmentRepository,
	commentAttachmentRepo commentattachmentrepo.CommentAttachmentRepository,
	cleanupRepo filecleanuprepo.FileCleanupRepository,
	commentsRepo commentsrepo.CommentsRepository,
	taskRepo taskrepo.TaskRepository,
	projectRepo projectrepo.ProjectRepository,
	authRepo authrepo.AuthRepository,
	auditRepo auditrepo.AuditLogRepository,
	storageClient storage.StorageClient,
	logger *zap.Logger,
	appCtx context.Context,
) AttachmentService {
	maxFileSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxFileSizeMB = parsed
		}
	}

	maxFiles := 5
	if v := config.GetEnv("ATTACHMENT_MAX_FILES_COUNT", ""); v != "" {
		var parsed int
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxFiles = parsed
		}
	}

	cfg := models.AttachmentConfig{
		MaxFileSizeMB: maxFileSizeMB,
		MaxFiles:      maxFiles,
	}

	claimTTL := 2 * time.Minute
	if v := config.GetEnv("CLEANUP_CLAIM_TTL", ""); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil && parsed > 0 {
			claimTTL = parsed
		}
	}

	s := &attachmentService{
		attachmentRepo:        attachmentRepo,
		commentAttachmentRepo: commentAttachmentRepo,
		cleanupRepo:           cleanupRepo,
		commentsRepo:          commentsRepo,
		taskRepo:              taskRepo,
		projectRepo:           projectRepo,
		authRepo:              authRepo,
		auditRepo:             auditRepo,
		storageClient:         storageClient,
		logger:                logger,
		cfg:                   cfg,
		claimTTL:              claimTTL,
		jitterSource:          productionJitterSource{},
	}

	if appCtx != nil {
		go s.startCleanupWorker(appCtx)
	}

	return s
}

func (s *attachmentService) GetConfig() models.AttachmentConfig {
	return s.cfg
}

// Single Consolidated Authorization Policy helper methods using TaskAccessContext to prevent duplicate DB queries

func (s *attachmentService) CanAccessTask(user *models.User, taskCtx *models.TaskAccessContext) (bool, *response.Error) {
	if user.Role == string(dto.RoleSuperAdmin) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == taskCtx.OrganizationID {
		return true, nil
	}

	if user.OrganizationID == nil || *user.OrganizationID != taskCtx.OrganizationID {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You are not authorized to access this organization",
		}
	}

	isMember, err := s.projectRepo.IsUserProjectMember(taskCtx.ProjectID, user.ID)
	if err != nil {
		return false, err
	}
	return isMember, nil
}

func (s *attachmentService) CanAccessComment(user *models.User, comment *models.Comments, taskCtx *models.TaskAccessContext) (bool, *response.Error) {
	if comment.TaskID != taskCtx.TaskID {
		return false, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}
	return s.CanAccessTask(user, taskCtx)
}

func (s *attachmentService) CanDeleteAttachment(user *models.User, attachment *models.TaskAttachment, taskCtx *models.TaskAccessContext) (bool, *response.Error) {
	canAccess, err := s.CanAccessTask(user, taskCtx)
	if err != nil || !canAccess {
		return false, err
	}

	if attachment.UploadedBy == user.ID {
		return true, nil
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == taskCtx.OrganizationID {
		return true, nil
	}

	member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(user.ID, taskCtx.ProjectID)
	if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
		return true, nil
	}

	return false, nil
}

func (s *attachmentService) CanDeleteCommentAttachment(user *models.User, attachment *models.CommentAttachment, comment *models.Comments, taskCtx *models.TaskAccessContext) (bool, *response.Error) {
	canAccess, err := s.CanAccessComment(user, comment, taskCtx)
	if err != nil || !canAccess {
		return false, err
	}

	if attachment.UploadedBy == user.ID {
		return true, nil
	}

	if comment.UserID == user.ID {
		return true, nil
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == taskCtx.OrganizationID {
		return true, nil
	}

	member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(user.ID, taskCtx.ProjectID)
	if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
		return true, nil
	}

	return false, nil
}

package services

import (
	"context"
	"fmt"
	"io"
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
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

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
}

type attachmentService struct {
	attachmentRepo        attachmentrepo.AttachmentRepository
	commentAttachmentRepo commentattachmentrepo.CommentAttachmentRepository
	commentsRepo          commentsrepo.CommentsRepository
	taskRepo              taskrepo.TaskRepository
	projectRepo           projectrepo.ProjectRepository
	authRepo              authrepo.AuthRepository
	auditRepo             auditrepo.AuditLogRepository
	storageClient         storage.StorageClient
	logger                *zap.Logger
	cfg                   models.AttachmentConfig
}

func InitAttachmentService(
	attachmentRepo attachmentrepo.AttachmentRepository,
	commentAttachmentRepo commentattachmentrepo.CommentAttachmentRepository,
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

	s := &attachmentService{
		attachmentRepo:        attachmentRepo,
		commentAttachmentRepo: commentAttachmentRepo,
		commentsRepo:          commentsRepo,
		taskRepo:              taskRepo,
		projectRepo:           projectRepo,
		authRepo:              authRepo,
		auditRepo:             auditRepo,
		storageClient:         storageClient,
		logger:                logger,
		cfg:                   cfg,
	}

	if appCtx != nil {
		go s.startCleanupWorker(appCtx)
	}

	return s
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

func (s *attachmentService) startCleanupWorker(ctx context.Context) {
	s.logger.Info("Starting orphaned files cleanup outbox worker...")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Orphaned files cleanup outbox worker stopping...")
			return
		case <-ticker.C:
			s.processOrphanedFiles(ctx)
		}
	}
}

func (s *attachmentService) processOrphanedFiles(ctx context.Context) {
	now := time.Now()
	claimedUntil := now.Add(2 * time.Minute)

	files, err := s.attachmentRepo.ClaimOrphanedFiles(now, claimedUntil, 50)
	if err != nil {
		return
	}

	for _, file := range files {
		delErr := s.storageClient.DeleteObject(ctx, file.StoragePath)
		if delErr == nil {
			_ = s.attachmentRepo.DeleteOrphanedFile(file.ID)
		} else {
			lastErrStr := delErr.Error()
			if len(lastErrStr) > 500 {
				lastErrStr = lastErrStr[:500]
			}
			_ = s.attachmentRepo.ReleaseOrphanedFile(file.ID, lastErrStr, time.Now())
			s.logger.Error("Orphaned files cleanup worker failed to delete storage object, marked for retry",
				zap.String("path", file.StoragePath),
				zap.Error(delErr),
			)
		}
	}
}

func (s *attachmentService) UploadAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
	if len(files) > s.cfg.MaxFiles {
		return nil, &response.Error{
			Code:       response.ErrorCode("BAD_REQUEST"),
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("Maximum of %d files can be uploaded per request.", s.cfg.MaxFiles),
		}
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, err
	}

	// Enforce URL project hierarchy validation
	if taskCtx.ProjectID != projectID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Task does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessTask(&user, taskCtx)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.TaskAttachment
	var auditLogs []models.AuditLog

	// Retrieve task details for task key (used in audit details)
	task, detailsErr := s.taskRepo.GetTaskDetailsByID(taskID)
	var taskKey string
	if detailsErr == nil && task != nil {
		taskKey = task.Key
	}

	for _, header := range files {
		file, openErr := header.Open()
		if openErr != nil {
			s.rollbackUploads(uploadedKeys, createdIDs)
			s.logger.Error("Failed to open uploaded file", zap.Error(openErr))
			return nil, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to process uploaded files. Please try again.",
			}
		}

		key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadAttachment(ctx, file, header, taskID, s.cfg)
		file.Close()
		if uploadErr != nil {
			s.rollbackUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		attachment := models.TaskAttachment{
			TaskID:           taskID,
			OriginalFilename: header.Filename,
			StoredFilename:   sanitizedName,
			MIMEType:         detectedMIME,
			FileSize:         header.Size,
			StoragePath:      key,
			UploadedBy:       userID,
			UploadedAt:       time.Now(),
		}

		dbErr := s.attachmentRepo.CreateAttachment(&attachment)
		if dbErr != nil {
			s.rollbackUploads(uploadedKeys, createdIDs)
			return nil, dbErr
		}
		createdIDs = append(createdIDs, attachment.ID)
		createdAttachments = append(createdAttachments, attachment)

		auditLog := models.AuditLog{
			UserID:         &userID,
			OrganizationID: &taskCtx.OrganizationID,
			ProjectID:      &taskCtx.ProjectID,
			Action:         "attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to task %s", attachment.OriginalFilename, taskKey),
			CreatedAt:      time.Now(),
		}
		auditLogs = append(auditLogs, auditLog)
	}

	// Write audit logs only when the entire transaction has succeeded
	for _, log := range auditLogs {
		if auditErr := s.auditRepo.CreateAuditLog(log); auditErr != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
		}
	}

	res := make([]responsedto.AttachmentResponse, len(createdAttachments))
	for i, a := range createdAttachments {
		res[i] = responsedto.AttachmentFromModel(a)
	}

	return res, nil
}

// rollbackUploads performs best-effort cleanup of uploaded S3 objects and database metadata
// in case of batch upload failure. Since this is non-transactional across S3 and DB,
// failures during rollback are explicitly logged for manual cleanup or audit reconciliation.
func (s *attachmentService) rollbackUploads(keys []string, ids []uuid.UUID) {
	ctx := context.Background()
	for _, key := range keys {
		if err := s.storageClient.DeleteObject(ctx, key); err != nil {
			s.logger.Error("Rollback failed to delete storage object during batch recovery", zap.String("key", key), zap.Error(err))
		}
	}
	for _, id := range ids {
		if err := s.attachmentRepo.DeleteAttachment(id); err != nil {
			s.logger.Error("Rollback failed to delete database attachment during batch recovery", zap.String("id", id.String()), zap.Any("error", err))
		}
	}
}

func (s *attachmentService) GetAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, err
	}

	if taskCtx.ProjectID != projectID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Task does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessTask(&user, taskCtx)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	attachments, dbErr := s.attachmentRepo.GetAttachmentsByTaskID(taskID)
	if dbErr != nil {
		return nil, dbErr
	}

	res := make([]responsedto.AttachmentResponse, len(attachments))
	for i, a := range attachments {
		res[i] = responsedto.AttachmentFromModel(a)
	}

	return res, nil
}

func (s *attachmentService) DownloadAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, "", "", 0, err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(attachment.TaskID)
	if err != nil {
		return nil, "", "", 0, err
	}

	if taskCtx.ProjectID != projectID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessTask(&user, taskCtx)
	if err != nil {
		return nil, "", "", 0, err
	}
	if !authorized {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	stream, size, getErr := s.storageClient.GetObject(ctx, attachment.StoragePath)
	if getErr != nil {
		return nil, "", "", 0, getErr
	}

	return stream, attachment.OriginalFilename, attachment.MIMEType, size, nil
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(attachment.TaskID)
	if err != nil {
		return err
	}

	if taskCtx.ProjectID != projectID {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified project",
		}
	}

	allowed, authErr := s.CanDeleteAttachment(&user, attachment, taskCtx)
	if authErr != nil {
		return authErr
	}
	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	// 1. Transactional DB outbox pattern: delete metadata and record orphan path
	dbErr = s.attachmentRepo.DeleteAttachmentAndRecordOrphan(attachmentID, attachment.StoragePath)
	if dbErr != nil {
		return dbErr
	}

	// Retrieve task details for task key (used in audit details)
	task, detailsErr := s.taskRepo.GetTaskDetailsByID(attachment.TaskID)
	var taskKey string
	if detailsErr == nil && task != nil {
		taskKey = task.Key
	}

	// Log audit activity
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &taskCtx.OrganizationID,
		ProjectID:      &taskCtx.ProjectID,
		Action:         "attachment_deleted",
		ResourceType:   "attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from task %s", attachment.OriginalFilename, taskKey),
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
	}

	return nil
}

func (s *attachmentService) UploadCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	if len(files) > s.cfg.MaxFiles {
		return nil, &response.Error{
			Code:       response.ErrorCode("BAD_REQUEST"),
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("Maximum of %d files can be uploaded per request.", s.cfg.MaxFiles),
		}
	}

	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return nil, err
	}

	if comment.TaskID != taskID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessComment(&user, comment, taskCtx)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.CommentAttachment
	var auditLogs []models.AuditLog

	for _, header := range files {
		file, openErr := header.Open()
		if openErr != nil {
			s.rollbackCommentUploads(uploadedKeys, createdIDs)
			s.logger.Error("Failed to open uploaded file", zap.Error(openErr))
			return nil, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to process uploaded files. Please try again.",
			}
		}

		key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadCommentAttachment(ctx, file, header, commentID, s.cfg)
		file.Close()
		if uploadErr != nil {
			s.rollbackCommentUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		attachment := models.CommentAttachment{
			CommentID:        commentID,
			OriginalFilename: header.Filename,
			StoredFilename:   sanitizedName,
			MIMEType:         detectedMIME,
			FileSize:         header.Size,
			StoragePath:      key,
			UploadedBy:       userID,
			UploadedAt:       time.Now(),
		}

		dbErr := s.commentAttachmentRepo.CreateAttachment(&attachment)
		if dbErr != nil {
			s.rollbackCommentUploads(uploadedKeys, createdIDs)
			return nil, dbErr
		}
		createdIDs = append(createdIDs, attachment.ID)
		createdAttachments = append(createdAttachments, attachment)

		auditLog := models.AuditLog{
			UserID:         &userID,
			OrganizationID: &taskCtx.OrganizationID,
			ProjectID:      &taskCtx.ProjectID,
			Action:         "comment_attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to comment %s", attachment.OriginalFilename, commentID),
			CreatedAt:      time.Now(),
		}
		auditLogs = append(auditLogs, auditLog)
	}

	// Persist audit logs after batch success
	for _, log := range auditLogs {
		if auditErr := s.auditRepo.CreateAuditLog(log); auditErr != nil {
			s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
		}
	}

	res := make([]responsedto.CommentAttachmentResponse, len(createdAttachments))
	for i, a := range createdAttachments {
		res[i] = responsedto.CommentAttachmentFromModel(a)
	}

	return res, nil
}

func (s *attachmentService) rollbackCommentUploads(keys []string, ids []uuid.UUID) {
	ctx := context.Background()
	for _, key := range keys {
		if err := s.storageClient.DeleteObject(ctx, key); err != nil {
			s.logger.Error("Rollback failed to delete comment storage object during batch recovery", zap.String("key", key), zap.Error(err))
		}
	}
	for _, id := range ids {
		if err := s.commentAttachmentRepo.DeleteAttachment(id); err != nil {
			s.logger.Error("Rollback failed to delete database comment attachment during batch recovery", zap.String("id", id.String()), zap.Any("error", err))
		}
	}
}

func (s *attachmentService) GetCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return nil, err
	}

	if comment.TaskID != taskID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessComment(&user, comment, taskCtx)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	attachments, dbErr := s.commentAttachmentRepo.GetAttachmentsByCommentID(commentID)
	if dbErr != nil {
		return nil, dbErr
	}

	res := make([]responsedto.CommentAttachmentResponse, len(attachments))
	for i, a := range attachments {
		res[i] = responsedto.CommentAttachmentFromModel(a)
	}

	return res, nil
}

func (s *attachmentService) DownloadCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, "", "", 0, err
	}

	attachment, dbErr := s.commentAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	comment, err := s.commentsRepo.GetCommentByID(attachment.CommentID)
	if err != nil {
		return nil, "", "", 0, err
	}

	if comment.TaskID != taskID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified task",
		}
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return nil, "", "", 0, err
	}

	authorized, err := s.CanAccessComment(&user, comment, taskCtx)
	if err != nil {
		return nil, "", "", 0, err
	}
	if !authorized {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	stream, size, getErr := s.storageClient.GetObject(ctx, attachment.StoragePath)
	if getErr != nil {
		return nil, "", "", 0, getErr
	}

	return stream, attachment.OriginalFilename, attachment.MIMEType, size, nil
}

func (s *attachmentService) DeleteCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) *response.Error {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	attachment, dbErr := s.commentAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	comment, err := s.commentsRepo.GetCommentByID(attachment.CommentID)
	if err != nil {
		return err
	}

	if comment.TaskID != taskID {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified task",
		}
	}

	taskCtx, err := s.taskRepo.GetTaskAccessContext(taskID)
	if err != nil {
		return err
	}

	allowed, authErr := s.CanDeleteCommentAttachment(&user, attachment, comment, taskCtx)
	if authErr != nil {
		return authErr
	}
	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, comment author, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	// 1. Transactional DB outbox pattern: delete metadata and record orphan path
	dbErr = s.commentAttachmentRepo.DeleteAttachmentAndRecordOrphan(attachmentID, attachment.StoragePath)
	if dbErr != nil {
		return dbErr
	}

	// Log audit activity
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &taskCtx.OrganizationID,
		ProjectID:      &taskCtx.ProjectID,
		Action:         "comment_attachment_deleted",
		ResourceType:   "attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from comment %s", attachment.OriginalFilename, comment.ID),
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
	}

	return nil
}

package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
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
	UploadAttachments(ctx context.Context, taskID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error)
	GetAttachments(ctx context.Context, taskID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error)
	DownloadAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteAttachment(ctx context.Context, attachmentID, userID uuid.UUID) *response.Error

	// Comment attachments
	UploadCommentAttachments(ctx context.Context, commentID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error)
	GetCommentAttachments(ctx context.Context, commentID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error)
	DownloadCommentAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteCommentAttachment(ctx context.Context, attachmentID, userID uuid.UUID) *response.Error
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
) AttachmentService {
	return &attachmentService{
		attachmentRepo:        attachmentRepo,
		commentAttachmentRepo: commentAttachmentRepo,
		commentsRepo:          commentsRepo,
		taskRepo:              taskRepo,
		projectRepo:           projectRepo,
		authRepo:              authRepo,
		auditRepo:             auditRepo,
		storageClient:         storageClient,
		logger:                logger,
	}
}

// Single Consolidated Authorization Policy helper methods

func (s *attachmentService) CanAccessTask(user *models.User, task *models.Task) (bool, *response.Error) {
	if user.Role == string(dto.RoleSuperAdmin) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}

	project, err := s.projectRepo.GetProjectByID(task.ProjectID)
	if err != nil {
		return false, err
	}

	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}

	if user.OrganizationID == nil || *user.OrganizationID != project.OrganizationID {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You are not authorized to access this organization",
		}
	}

	isMember, err := s.projectRepo.IsUserProjectMember(task.ProjectID, user.ID)
	if err != nil {
		return false, err
	}
	return isMember, nil
}

func (s *attachmentService) CanAccessComment(user *models.User, comment *models.Comments, task *models.Task) (bool, *response.Error) {
	if comment.TaskID != task.ID {
		return false, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}
	return s.CanAccessTask(user, task)
}

func (s *attachmentService) CanDeleteAttachment(user *models.User, attachment *models.TaskAttachment, task *models.Task) (bool, *response.Error) {
	canAccess, err := s.CanAccessTask(user, task)
	if err != nil || !canAccess {
		return false, err
	}

	if attachment.UploadedBy == user.ID {
		return true, nil
	}

	project, err := s.projectRepo.GetProjectByID(task.ProjectID)
	if err != nil {
		return false, err
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}

	member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(user.ID, task.ProjectID)
	if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
		return true, nil
	}

	return false, nil
}

func (s *attachmentService) CanDeleteCommentAttachment(user *models.User, attachment *models.CommentAttachment, comment *models.Comments, task *models.Task) (bool, *response.Error) {
	canAccess, err := s.CanAccessComment(user, comment, task)
	if err != nil || !canAccess {
		return false, err
	}

	if attachment.UploadedBy == user.ID {
		return true, nil
	}

	// Note: Comment authors are allowed to delete attachments uploaded by other users to their comments because they own the comment resource.
	// This is a deliberate policy design difference from task attachments.
	if comment.UserID == user.ID {
		return true, nil
	}

	project, err := s.projectRepo.GetProjectByID(task.ProjectID)
	if err != nil {
		return false, err
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}

	member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(user.ID, task.ProjectID)
	if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
		return true, nil
	}

	return false, nil
}

var allowedAttachmentExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".pdf":  true,
	".docx": true,
	".xlsx": true,
	".zip":  true,
}

func (s *attachmentService) UploadAttachments(ctx context.Context, taskID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	task, err := s.taskRepo.GetTaskDetailsByID(taskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessTask(&user, task)
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

	project, err := s.projectRepo.GetProjectByID(task.ProjectID)
	if err != nil {
		return nil, err
	}
	orgID := project.OrganizationID

	maxSizeMB, valErr := s.validateFiles(files)
	if valErr != nil {
		return nil, valErr
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.TaskAttachment
	var auditLogs []models.AuditLog

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

		key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadAttachment(ctx, file, header, task.ID, maxSizeMB)
		file.Close()
		if uploadErr != nil {
			s.rollbackUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		attachment := models.TaskAttachment{
			TaskID:           task.ID,
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
			OrganizationID: &orgID,
			ProjectID:      &task.ProjectID,
			Action:         "attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to task %s", attachment.OriginalFilename, task.Key),
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

func (s *attachmentService) rollbackUploads(keys []string, ids []uuid.UUID) {
	ctx := context.Background()
	for _, key := range keys {
		_ = s.storageClient.DeleteObject(ctx, key)
	}
	for _, id := range ids {
		_ = s.attachmentRepo.DeleteAttachment(id)
	}
}

func (s *attachmentService) GetAttachments(ctx context.Context, taskID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	task, err := s.taskRepo.GetTaskDetailsByID(taskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessTask(&user, task)
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

func (s *attachmentService) DownloadAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, "", "", 0, err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	task, err := s.taskRepo.GetTaskDetailsByID(attachment.TaskID)
	if err != nil {
		return nil, "", "", 0, err
	}

	authorized, err := s.CanAccessTask(&user, task)
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

func (s *attachmentService) DeleteAttachment(ctx context.Context, attachmentID, userID uuid.UUID) *response.Error {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	task, err := s.taskRepo.GetTaskDetailsByID(attachment.TaskID)
	if err != nil {
		return err
	}

	allowed, authErr := s.CanDeleteAttachment(&user, attachment, task)
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

	// 1. Delete DB metadata first to ensure reverse consistency
	dbErr = s.attachmentRepo.DeleteAttachment(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	project, err := s.projectRepo.GetProjectByID(task.ProjectID)
	if err != nil {
		return err
	}
	orgID := project.OrganizationID

	// 2. Delete from storage asynchronously in a background goroutine
	go func() {
		err := s.storageClient.DeleteObject(context.Background(), attachment.StoragePath)
		if err != nil {
			s.logger.Error("Failed to delete attachment from storage during asynchronous cleanup",
				zap.Error(err),
				zap.String("storagePath", attachment.StoragePath),
				zap.String("attachmentID", attachmentID.String()),
			)
		}
	}()

	// Log audit activity
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &task.ProjectID,
		Action:         "attachment_deleted",
		ResourceType:   "attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from task %s", attachment.OriginalFilename, task.Key),
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
	}

	return nil
}

func (s *attachmentService) validateFiles(files []*multipart.FileHeader) (int64, *response.Error) {
	maxSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}

	for _, header := range files {
		if header.Size > maxSizeMB*1024*1024 {
			return 0, &response.Error{
				Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
				StatusCode: http.StatusRequestEntityTooLarge,
				Message:    fmt.Sprintf("File %s exceeds the maximum allowed size of %d MB.", header.Filename, maxSizeMB),
			}
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedAttachmentExtensions[ext] {
			return 0, &response.Error{
				Code:       response.ErrorCode("UNSUPPORTED_MEDIA_TYPE"),
				StatusCode: http.StatusUnsupportedMediaType,
				Message:    fmt.Sprintf("File %s has an unsupported file type. Only PNG, JPG/JPEG, PDF, DOCX, XLSX, and ZIP files are accepted.", header.Filename),
			}
		}
	}
	return maxSizeMB, nil
}

func (s *attachmentService) UploadCommentAttachments(ctx context.Context, commentID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return nil, err
	}

	task, err := s.taskRepo.GetTaskDetailsByID(comment.TaskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessComment(&user, comment, task)
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

	maxSizeMB, valErr := s.validateFiles(files)
	if valErr != nil {
		return nil, valErr
	}

	orgID := task.Project.OrganizationID

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

		key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadCommentAttachment(ctx, file, header, comment.ID, maxSizeMB)
		file.Close()
		if uploadErr != nil {
			s.rollbackCommentUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		attachment := models.CommentAttachment{
			CommentID:        comment.ID,
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
			OrganizationID: &orgID,
			ProjectID:      &task.ProjectID,
			Action:         "comment_attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to comment %s", attachment.OriginalFilename, comment.ID),
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
		_ = s.storageClient.DeleteObject(ctx, key)
	}
	for _, id := range ids {
		_ = s.commentAttachmentRepo.DeleteAttachment(id)
	}
}

func (s *attachmentService) GetCommentAttachments(ctx context.Context, commentID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return nil, err
	}

	task, err := s.taskRepo.GetTaskDetailsByID(comment.TaskID)
	if err != nil {
		return nil, err
	}

	authorized, err := s.CanAccessComment(&user, comment, task)
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

func (s *attachmentService) DownloadCommentAttachment(ctx context.Context, attachmentID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
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

	task, err := s.taskRepo.GetTaskDetailsByID(comment.TaskID)
	if err != nil {
		return nil, "", "", 0, err
	}

	authorized, err := s.CanAccessComment(&user, comment, task)
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

func (s *attachmentService) DeleteCommentAttachment(ctx context.Context, attachmentID, userID uuid.UUID) *response.Error {
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

	task, err := s.taskRepo.GetTaskDetailsByID(comment.TaskID)
	if err != nil {
		return err
	}

	allowed, authErr := s.CanDeleteCommentAttachment(&user, attachment, comment, task)
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

	// 1. Delete DB metadata first
	dbErr = s.commentAttachmentRepo.DeleteAttachment(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	orgID := task.Project.OrganizationID

	// 2. Delete S3 asynchronously using background context
	go func() {
		err := s.storageClient.DeleteObject(context.Background(), attachment.StoragePath)
		if err != nil {
			s.logger.Error("Failed to delete comment attachment from storage during asynchronous cleanup",
				zap.Error(err),
				zap.String("storagePath", attachment.StoragePath),
				zap.String("attachmentID", attachmentID.String()),
			)
		}
	}()

	// Log audit activity
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &task.ProjectID,
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

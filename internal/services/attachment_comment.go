package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

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
	// Note: Audit logging is best-effort by design.
	for _, log := range auditLogs {
		if auditErr := s.auditRepo.CreateAuditLog(log); auditErr != nil {
			s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
		}
	}

	res := make([]responsedto.CommentAttachmentResponse, len(createdAttachments))
	for i, a := range createdAttachments {
		res[i] = responsedto.CommentAttachmentFromModel(a)
	}

	return res, nil
}

// rollbackCommentUploads performs durable cleanup of uploaded comment S3 objects and database metadata
// in case of batch upload failure by leveraging the transactional outbox pattern.
func (s *attachmentService) rollbackCommentUploads(keys []string, ids []uuid.UUID) {
	for i, key := range keys {
		if i < len(ids) {
			if err := s.commentAttachmentRepo.DeleteAttachmentAndRecordOrphan(ids[i], key); err != nil {
				s.logger.Error("Rollback failed to delete comment database attachment and record orphan during batch recovery", zap.String("id", ids[i].String()), zap.Any("error", err))
			}
		} else {
			orphan := &models.OrphanedFile{
				StoragePath: key,
			}
			if err := s.cleanupRepo.CreateOrphanedFile(context.Background(), orphan); err != nil {
				s.logger.Error("Rollback failed to record comment orphaned file during batch recovery", zap.String("key", key), zap.Any("error", err))
			}
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
	// Note: Audit logging is best-effort by design.
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
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
	}

	return nil
}

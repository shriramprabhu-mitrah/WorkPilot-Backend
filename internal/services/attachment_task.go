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

	taskKey := taskCtx.TaskKey

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
	// Note: Audit logging is best-effort by design.
	for _, log := range auditLogs {
		if auditErr := s.auditRepo.CreateAuditLog(log); auditErr != nil {
			s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
		}
	}

	res := make([]responsedto.AttachmentResponse, len(createdAttachments))
	for i, a := range createdAttachments {
		res[i] = responsedto.AttachmentFromModel(a)
	}

	return res, nil
}

// rollbackUploads performs durable cleanup of uploaded S3 objects and database metadata
// in case of batch upload failure by leveraging the transactional outbox pattern.
func (s *attachmentService) rollbackUploads(keys []string, ids []uuid.UUID) {
	for i, key := range keys {
		if i < len(ids) {
			if err := s.attachmentRepo.DeleteAttachmentAndRecordOrphan(ids[i], key); err != nil {
				s.logger.Error("Rollback failed to delete database attachment and record orphan during batch recovery", zap.String("id", ids[i].String()), zap.Any("error", err))
			}
		} else {
			orphan := &models.OrphanedFile{
				StoragePath: key,
			}
			if err := s.cleanupRepo.CreateOrphanedFile(context.Background(), orphan); err != nil {
				s.logger.Error("Rollback failed to record orphaned file during batch recovery", zap.String("key", key), zap.Any("error", err))
			}
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

	taskKey := taskCtx.TaskKey

	// Log audit activity
	// Note: Audit logging is best-effort by design.
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
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
	}

	return nil
}

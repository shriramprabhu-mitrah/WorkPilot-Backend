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

func (s *attachmentService) UploadAttachments(ctx context.Context, taskID *uuid.UUID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
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

	var authorized bool
	var orgID uuid.UUID
	var taskKey string

	if taskID != nil && *taskID != uuid.Nil {
		taskCtx, err := s.taskRepo.GetTaskAccessContext(*taskID)
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

		authorized, err = s.CanAccessTask(&user, taskCtx)
		if err != nil {
			return nil, err
		}
		orgID = taskCtx.OrganizationID
		taskKey = taskCtx.TaskKey
	} else {
		authorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "tasks", "add")
		if err != nil {
			return nil, err
		}

		project, projErr := s.projectRepo.GetProjectByID(projectID)
		if projErr != nil {
			return nil, projErr
		}
		orgID = project.OrganizationID
		taskKey = "pending task creation"
	}

	if !authorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.TaskAttachment
	var auditLogs []models.AuditLog

	var folderID uuid.UUID
	if taskID != nil && *taskID != uuid.Nil {
		folderID = *taskID
	} else {
		folderID = projectID
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

		url, key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadAttachment(ctx, file, header, folderID, s.cfg)
		file.Close()
		if uploadErr != nil {
			s.rollbackUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		var dbTaskID *uuid.UUID
		if taskID != nil && *taskID != uuid.Nil {
			dbTaskID = taskID
		}

		attachment := models.TaskAttachment{
			TaskID:           dbTaskID,
			ProjectID:        projectID,
			OriginalFilename: header.Filename,
			StoredFilename:   sanitizedName,
			MIMEType:         detectedMIME,
			FileSize:         header.Size,
			StoragePath:      key,
			URL:              url,
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
			ProjectID:      &projectID,
			TaskID:         dbTaskID,
			Action:         "attachment_uploaded",
			ResourceType:   "task_attachment",
			Type:           models.AuditLogTypeActivity,
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("User %s uploaded attachment %s to task %s", user.Email, attachment.OriginalFilename, taskKey),
			CreatedAt:      time.Now(),
		}
		auditLogs = append(auditLogs, auditLog)
	}

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

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &taskCtx.OrganizationID,
		ProjectID:      &taskCtx.ProjectID,
		Action:         "viewed",
		ResourceType:   "task_attachment",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("User %s viewed attachments from task %s", user.Email, taskCtx.TaskKey),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
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

	var authorized bool
	var orgID uuid.UUID
	var resourceKey string

	if attachment.TaskID != nil && *attachment.TaskID != uuid.Nil {
		taskCtx, err := s.taskRepo.GetTaskAccessContext(*attachment.TaskID)
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

		authorized, err = s.CanAccessTask(&user, taskCtx)
		if err != nil {
			return nil, "", "", 0, err
		}
		orgID = taskCtx.OrganizationID
		resourceKey = taskCtx.TaskKey
	} else {
		if attachment.ProjectID != projectID {
			return nil, "", "", 0, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Attachment does not belong to the specified project",
			}
		}

		authorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "tasks", "view")
		if err != nil {
			return nil, "", "", 0, err
		}
		resourceKey = "pending attachment"

		project, projErr := s.projectRepo.GetProjectByID(projectID)
		if projErr != nil {
			return nil, "", "", 0, projErr
		}
		orgID = project.OrganizationID
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

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "downloaded",
		ResourceType:   "task_attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("User %s downloaded attachment %s from task %s", user.Email, attachment.OriginalFilename, resourceKey),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
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

	var allowed bool
	var orgID uuid.UUID
	var resourceKey string

	if attachment.TaskID != nil && *attachment.TaskID != uuid.Nil {
		taskCtx, err := s.taskRepo.GetTaskAccessContext(*attachment.TaskID)
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

		allowed, err = s.CanDeleteAttachment(&user, attachment, taskCtx)
		if err != nil {
			return err
		}
		orgID = taskCtx.OrganizationID
		resourceKey = taskCtx.TaskKey
	} else {
		if attachment.ProjectID != projectID {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Attachment does not belong to the specified project",
			}
		}

		isUploader := attachment.UploadedBy == userID
		isManager, err := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "tasks", "delete")
		if err != nil {
			return err
		}
		allowed = isUploader || isManager
		resourceKey = "pending attachment"

		project, projErr := s.projectRepo.GetProjectByID(projectID)
		if projErr != nil {
			return projErr
		}
		orgID = project.OrganizationID
	}

	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	dbErr = s.attachmentRepo.DeleteAttachmentAndRecordOrphan(attachmentID, attachment.StoragePath)
	if dbErr != nil {
		return dbErr
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "attachment_deleted",
		ResourceType:   "task_attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from task %s", attachment.OriginalFilename, resourceKey),
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
	}

	return nil
}

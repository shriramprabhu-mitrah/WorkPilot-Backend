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

func (s *attachmentService) UploadUserStoryAttachments(ctx context.Context, userStoryID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.UserStoryAttachmentResponse, *response.Error) {
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

	storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(userStoryID)
	if err != nil {
		return nil, err
	}

	// Enforce URL project hierarchy validation
	if storyCtx.ProjectID != projectID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "User story does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessUserStory(&user, storyCtx)
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
	var createdAttachments []models.UserStoryAttachment
	var auditLogs []models.AuditLog

	storyTitle := storyCtx.Title

	for _, header := range files {
		file, openErr := header.Open()
		if openErr != nil {
			s.rollbackUserStoryUploads(uploadedKeys, createdIDs)
			s.logger.Error("Failed to open uploaded file", zap.Error(openErr))
			return nil, &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to process uploaded files. Please try again.",
			}
		}

		_, key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadUserStoryAttachment(ctx, file, header, userStoryID, s.cfg)
		file.Close()
		if uploadErr != nil {
			s.rollbackUserStoryUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		attachment := models.UserStoryAttachment{
			UserStoryID:      userStoryID,
			OriginalFilename: header.Filename,
			StoredFilename:   sanitizedName,
			MIMEType:         detectedMIME,
			FileSize:         header.Size,
			StoragePath:      key,
			UploadedBy:       userID,
			UploadedAt:       time.Now(),
		}

		dbErr := s.userStoryAttachmentRepo.CreateAttachment(&attachment)
		if dbErr != nil {
			s.rollbackUserStoryUploads(uploadedKeys, createdIDs)
			return nil, dbErr
		}
		createdIDs = append(createdIDs, attachment.ID)
		createdAttachments = append(createdAttachments, attachment)

		auditLog := models.AuditLog{
			UserID:         &userID,
			OrganizationID: &storyCtx.OrganizationID,
			ProjectID:      &storyCtx.ProjectID,
			Action:         "user_story_attachment_uploaded",
			ResourceType:   "user_story_attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to user story %s", attachment.OriginalFilename, storyTitle),
			CreatedAt:      time.Now(),
		}
		auditLogs = append(auditLogs, auditLog)
	}

	// Write audit logs only when the entire transaction has succeeded
	for _, log := range auditLogs {
		if auditErr := s.auditRepo.CreateAuditLog(log); auditErr != nil {
			s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
		}
	}

	res := make([]responsedto.UserStoryAttachmentResponse, len(createdAttachments))
	for i, a := range createdAttachments {
		res[i] = responsedto.UserStoryAttachmentFromModel(a)
	}

	return res, nil
}

func (s *attachmentService) rollbackUserStoryUploads(keys []string, ids []uuid.UUID) {
	for i, key := range keys {
		if i < len(ids) {
			if err := s.userStoryAttachmentRepo.DeleteAttachmentAndRecordOrphan(ids[i], key); err != nil {
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

func (s *attachmentService) GetUserStoryAttachments(ctx context.Context, userStoryID, projectID, userID uuid.UUID) ([]responsedto.UserStoryAttachmentResponse, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(userStoryID)
	if err != nil {
		return nil, err
	}

	if storyCtx.ProjectID != projectID {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "User story does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessUserStory(&user, storyCtx)
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

	attachments, dbErr := s.userStoryAttachmentRepo.GetAttachmentsByUserStoryID(userStoryID)
	if dbErr != nil {
		return nil, dbErr
	}

	res := make([]responsedto.UserStoryAttachmentResponse, len(attachments))
	for i, a := range attachments {
		res[i] = responsedto.UserStoryAttachmentFromModel(a)
	}

	return res, nil
}

func (s *attachmentService) DownloadUserStoryAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, "", "", 0, err
	}

	attachment, dbErr := s.userStoryAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(attachment.UserStoryID)
	if err != nil {
		return nil, "", "", 0, err
	}

	if storyCtx.ProjectID != projectID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified project",
		}
	}

	authorized, err := s.CanAccessUserStory(&user, storyCtx)
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

func (s *attachmentService) DeleteUserStoryAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	attachment, dbErr := s.userStoryAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(attachment.UserStoryID)
	if err != nil {
		return err
	}

	if storyCtx.ProjectID != projectID {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Attachment does not belong to the specified project",
		}
	}

	allowed, authErr := s.CanDeleteUserStoryAttachment(&user, attachment, storyCtx)
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

	dbErr = s.userStoryAttachmentRepo.DeleteAttachmentAndRecordOrphan(attachmentID, attachment.StoragePath)
	if dbErr != nil {
		return dbErr
	}

	storyTitle := storyCtx.Title

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &storyCtx.OrganizationID,
		ProjectID:      &storyCtx.ProjectID,
		Action:         "user_story_attachment_deleted",
		ResourceType:   "user_story_attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from user story %s", attachment.OriginalFilename, storyTitle),
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
	}

	return nil
}

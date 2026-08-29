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

func (s *attachmentService) UploadCommentAttachments(ctx context.Context, commentID, taskID, userStoryID *uuid.UUID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
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

	var projectID uuid.UUID
	var isAuthorized bool
	var orgID uuid.UUID

	if commentID != nil && *commentID != uuid.Nil {
		comment, err := s.commentsRepo.GetCommentByID(*commentID)
		if err != nil {
			return nil, err
		}

		if taskID != nil && (comment.TaskID == nil || *comment.TaskID != *taskID) {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Comment does not belong to the specified task",
			}
		}
		if userStoryID != nil && (comment.UserStoryID == nil || *comment.UserStoryID != *userStoryID) {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Comment does not belong to the specified user story",
			}
		}

		if comment.TaskID != nil {
			taskCtx, err := s.taskRepo.GetTaskAccessContext(*comment.TaskID)
			if err != nil {
				return nil, err
			}
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			isAuthorized, err = s.CanAccessComment(&user, comment, taskCtx)
			if err != nil {
				return nil, err
			}
		} else if comment.UserStoryID != nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*comment.UserStoryID)
			if err != nil {
				return nil, err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "view")
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Comment doesn't exist yet (upload during creation)
		if taskID != nil && *taskID != uuid.Nil {
			taskCtx, err := s.taskRepo.GetTaskAccessContext(*taskID)
			if err != nil {
				return nil, err
			}
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "add")
			if err != nil {
				return nil, err
			}
		} else if userStoryID != nil && *userStoryID != uuid.Nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*userStoryID)
			if err != nil {
				return nil, err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "add")
			if err != nil {
				return nil, err
			}
		} else {
			return nil, &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Either Comment ID, Task ID, or User Story ID must be provided",
			}
		}
	}

	if !isAuthorized {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to perform this action",
		}
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.CommentAttachment
	var auditLogs []models.AuditLog

	var folderID uuid.UUID
	if commentID != nil && *commentID != uuid.Nil {
		folderID = *commentID
	} else if taskID != nil && *taskID != uuid.Nil {
		folderID = *taskID
	} else if userStoryID != nil && *userStoryID != uuid.Nil {
		folderID = *userStoryID
	}

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

		url, key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadCommentAttachment(ctx, file, header, folderID, s.cfg)
		file.Close()
		if uploadErr != nil {
			s.rollbackCommentUploads(uploadedKeys, createdIDs)
			return nil, uploadErr
		}
		uploadedKeys = append(uploadedKeys, key)

		var dbCommentID *uuid.UUID
		if commentID != nil && *commentID != uuid.Nil {
			dbCommentID = commentID
		}

		attachment := models.CommentAttachment{
			CommentID:        dbCommentID,
			TaskID:           taskID,
			UserStoryID:      userStoryID,
			OriginalFilename: header.Filename,
			StoredFilename:   sanitizedName,
			MIMEType:         detectedMIME,
			FileSize:         header.Size,
			StoragePath:      key,
			URL:              url,
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
			ProjectID:      &projectID,
			Action:         "uploaded",
			ResourceType:   "comment_attachment",
			Type:           models.AuditLogTypeAudit,
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded for comment (folder ID: %s)", attachment.OriginalFilename, folderID),
			CreatedAt:      time.Now(),
		}
		auditLogs = append(auditLogs, auditLog)
	}

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

	if comment.TaskID == nil || *comment.TaskID != taskID {
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

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &taskCtx.OrganizationID,
		ProjectID:      &taskCtx.ProjectID,
		Action:         "viewed",
		ResourceType:   "comment_attachment",
		ResourceID:     commentID.String(),
		Details:        fmt.Sprintf("User %s viewed attachments from comment %v", user.Email, comment.ID),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
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

	var projectID uuid.UUID
	var isAuthorized bool
	var orgID uuid.UUID
	var resourceKey string

	if attachment.CommentID != nil && *attachment.CommentID != uuid.Nil {
		comment, err := s.commentsRepo.GetCommentByID(*attachment.CommentID)
		if err != nil {
			return nil, "", "", 0, err
		}

		if comment.TaskID != nil {
			if *comment.TaskID != taskID {
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
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			resourceKey = taskCtx.TaskKey
			isAuthorized, err = s.CanAccessComment(&user, comment, taskCtx)
			if err != nil {
				return nil, "", "", 0, err
			}
		} else if comment.UserStoryID != nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*comment.UserStoryID)
			if err != nil {
				return nil, "", "", 0, err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			resourceKey = storyCtx.Title
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "view")
			if err != nil {
				return nil, "", "", 0, err
			}
		}
	} else {
		// Temporary/pending attachment before comment is created
		if attachment.TaskID != nil && *attachment.TaskID != uuid.Nil {
			if *attachment.TaskID != taskID {
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
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			resourceKey = taskCtx.TaskKey
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "view")
			if err != nil {
				return nil, "", "", 0, err
			}
		} else if attachment.UserStoryID != nil && *attachment.UserStoryID != uuid.Nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*attachment.UserStoryID)
			if err != nil {
				return nil, "", "", 0, err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			resourceKey = storyCtx.Title
			isAuthorized, err = CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "view")
			if err != nil {
				return nil, "", "", 0, err
			}
		}
	}

	if !isAuthorized {
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
		Action:         "download",
		ResourceType:   "comment_attachment",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("User %s downloaded attachment %s from resource %s", user.Email, attachment.OriginalFilename, resourceKey),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
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

	var projectID uuid.UUID
	var orgID uuid.UUID
	var allowed bool

	if attachment.CommentID != nil && *attachment.CommentID != uuid.Nil {
		comment, err := s.commentsRepo.GetCommentByID(*attachment.CommentID)
		if err != nil {
			return err
		}

		if comment.TaskID != nil {
			if *comment.TaskID != taskID {
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
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			allowed, err = s.CanDeleteCommentAttachment(&user, attachment, comment, taskCtx)
			if err != nil {
				return err
			}
		} else if comment.UserStoryID != nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*comment.UserStoryID)
			if err != nil {
				return err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			isUploader := attachment.UploadedBy == userID
			isManager, err := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "delete")
			if err != nil {
				return err
			}
			allowed = isUploader || isManager
		}
	} else {
		// Temporary/pending attachment before comment is created
		if attachment.TaskID != nil && *attachment.TaskID != uuid.Nil {
			if *attachment.TaskID != taskID {
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
			projectID = taskCtx.ProjectID
			orgID = taskCtx.OrganizationID
			isUploader := attachment.UploadedBy == userID
			isManager, err := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "delete")
			if err != nil {
				return err
			}
			allowed = isUploader || isManager
		} else if attachment.UserStoryID != nil && *attachment.UserStoryID != uuid.Nil {
			storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(*attachment.UserStoryID)
			if err != nil {
				return err
			}
			projectID = storyCtx.ProjectID
			orgID = storyCtx.OrganizationID
			isUploader := attachment.UploadedBy == userID
			isManager, err := CheckPermission(s.authRepo, s.projectRepo, userID, projectID, "comments", "delete")
			if err != nil {
				return err
			}
			allowed = isUploader || isManager
		}
	}

	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, comment author, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	dbErr = s.commentAttachmentRepo.DeleteAttachmentAndRecordOrphan(attachmentID, attachment.StoragePath)
	if dbErr != nil {
		return dbErr
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
		Action:         "deleted",
		ResourceType:   "comment_attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted", attachment.OriginalFilename),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log (best-effort)", zap.Any("error", auditErr))
	}

	return nil
}

func (s *attachmentService) ResolveTaskID(taskIDOrKey string) (uuid.UUID, *response.Error) {
	taskCtx, err := s.taskRepo.GetTaskAccessContextByIDOrKey(taskIDOrKey)
	if err != nil {
		return uuid.Nil, err
	}
	return taskCtx.TaskID, nil
}

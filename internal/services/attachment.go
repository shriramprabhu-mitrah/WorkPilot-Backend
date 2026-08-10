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
	UploadAttachments(taskID, userID, projectID, orgID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error)
	GetAttachments(taskID, userID, projectID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error)
	DownloadAttachment(taskID, attachmentID, userID, projectID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteAttachment(taskID, attachmentID, userID, projectID, orgID uuid.UUID) *response.Error

	// Comment attachments
	UploadCommentAttachments(commentID, taskID, userID, orgID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error)
	GetCommentAttachments(commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error)
	DownloadCommentAttachment(commentID, attachmentID, userID, taskID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	DeleteCommentAttachment(commentID, attachmentID, userID, orgID, taskID uuid.UUID) *response.Error
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

func (s *attachmentService) checkAuthorization(projectID, userID uuid.UUID) (bool, *response.Error) {
	project, err := s.projectRepo.GetProjectByID(projectID)
	if err != nil {
		return false, err
	}
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return false, err
	}
	if user.Role == string(dto.RoleSuperAdmin) {
		return false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Super admins are not allowed to perform organization-level activities",
		}
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == project.OrganizationID {
		return true, nil
	}
	isMember, err := s.projectRepo.IsUserProjectMember(projectID, userID)
	if err != nil {
		return false, err
	}
	return isMember, nil
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

func (s *attachmentService) UploadAttachments(taskID, userID, projectID, orgID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
	// Validate project membership
	authorized, err := s.checkAuthorization(projectID, userID)
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

	// Verify task exists in the project
	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, err
	}

	maxSizeMB, valErr := s.validateFiles(files)
	if valErr != nil {
		return nil, valErr
	}

	// 2. Upload and save files
	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.TaskAttachment

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

		_, key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadAttachment(file, header, task.ID, maxSizeMB)
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

		// Log audit activity for each file
		auditLog := models.AuditLog{
			UserID:         &userID,
			OrganizationID: &orgID,
			ProjectID:      &projectID,
			Action:         "attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to task %s", attachment.OriginalFilename, task.Key),
			CreatedAt:      time.Now(),
		}
		if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
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

func (s *attachmentService) GetAttachments(taskID, userID, projectID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error) {
	authorized, err := s.checkAuthorization(projectID, userID)
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

	// Verify task exists in the project
	_, err = s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, err
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

func (s *attachmentService) DownloadAttachment(taskID, attachmentID, userID, projectID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	authorized, err := s.checkAuthorization(projectID, userID)
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

	// Verify task exists in the project
	_, err = s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return nil, "", "", 0, err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	if attachment.TaskID != taskID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Attachment does not belong to the specified task",
		}
	}

	ctx := context.Background()
	stream, size, getErr := s.storageClient.GetObject(ctx, attachment.StoragePath)
	if getErr != nil {
		return nil, "", "", 0, getErr
	}

	return stream, attachment.OriginalFilename, attachment.MIMEType, size, nil
}

func (s *attachmentService) DeleteAttachment(taskID, attachmentID, userID, projectID, orgID uuid.UUID) *response.Error {
	authorized, err := s.checkAuthorization(projectID, userID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	// Verify task exists in the project
	task, err := s.taskRepo.GetTaskByID(taskID, projectID)
	if err != nil {
		return err
	}

	attachment, dbErr := s.attachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	if attachment.TaskID != taskID {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Attachment does not belong to the specified task",
		}
	}

	// Check permissions: Only the uploader, Project Managers, or Organization Administrators can delete attachments
	allowed := false
	if attachment.UploadedBy == userID {
		allowed = true
	} else {
		// Get requester details
		requester, reqErr := s.authRepo.GetUserByID(userID)
		if reqErr == nil && requester.Role == string(dto.RoleOrgAdmin) {
			allowed = true
		} else {
			// Check project roles
			member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(userID, projectID)
			if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
				allowed = true
			}
		}
	}

	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	// Delete from Supabase Storage
	ctx := context.Background()
	_ = s.storageClient.DeleteObject(ctx, attachment.StoragePath)

	// Delete metadata from DB
	dbErr = s.attachmentRepo.DeleteAttachment(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	// Log audit activity
	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      &projectID,
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

func (s *attachmentService) checkCommentAuthorization(userID, taskID uuid.UUID) (*uuid.UUID, bool, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, false, err
	}
	task, err := s.taskRepo.GetTaskDetailsByID(taskID)
	if err != nil {
		return nil, false, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Task does not belong to the specified project",
		}
	}
	if user.Role == string(dto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == task.Project.OrganizationID {
		return &task.ProjectID, true, nil
	}
	if user.OrganizationID == nil || *user.OrganizationID != task.Project.OrganizationID {
		return &task.ProjectID, false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You are not authorized to access this organization",
		}
	}
	isMember, err := s.projectRepo.IsUserProjectMember(task.ProjectID, userID)
	if err != nil {
		return nil, false, err
	}
	return &task.ProjectID, isMember, nil
}

func (s *attachmentService) UploadCommentAttachments(commentID, taskID, userID, orgID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	projectID, authorized, err := s.checkCommentAuthorization(userID, taskID)
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

	if comment.UserID != userID {
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You can only upload attachments to your own comments",
		}
	}

	maxSizeMB, valErr := s.validateFiles(files)
	if valErr != nil {
		return nil, valErr
	}

	var uploadedKeys []string
	var createdIDs []uuid.UUID
	var createdAttachments []models.CommentAttachment

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

		_, key, sanitizedName, detectedMIME, uploadErr := s.storageClient.UploadCommentAttachment(file, header, comment.ID, maxSizeMB)
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
			ProjectID:      projectID,
			Action:         "comment_attachment_uploaded",
			ResourceType:   "attachment",
			ResourceID:     attachment.ID.String(),
			Details:        fmt.Sprintf("Attachment %s uploaded to comment %s", attachment.OriginalFilename, comment.ID),
			CreatedAt:      time.Now(),
		}
		if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
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

func (s *attachmentService) GetCommentAttachments(commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	_, authorized, err := s.checkCommentAuthorization(userID, taskID)
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

func (s *attachmentService) DownloadCommentAttachment(commentID, attachmentID, userID, taskID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	_, authorized, err := s.checkCommentAuthorization(userID, taskID)
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

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return nil, "", "", 0, err
	}
	if comment.TaskID != taskID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}

	attachment, dbErr := s.commentAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return nil, "", "", 0, dbErr
	}

	if attachment.CommentID != commentID {
		return nil, "", "", 0, &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Attachment does not belong to the specified comment",
		}
	}

	ctx := context.Background()
	stream, size, getErr := s.storageClient.GetObject(ctx, attachment.StoragePath)
	if getErr != nil {
		return nil, "", "", 0, getErr
	}

	return stream, attachment.OriginalFilename, attachment.MIMEType, size, nil
}

func (s *attachmentService) DeleteCommentAttachment(commentID, attachmentID, userID, orgID, taskID uuid.UUID) *response.Error {
	projectID, authorized, err := s.checkCommentAuthorization(userID, taskID)
	if err != nil {
		return err
	}
	if !authorized {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to access this project",
		}
	}

	comment, err := s.commentsRepo.GetCommentByID(commentID)
	if err != nil {
		return err
	}
	if comment.TaskID != taskID {
		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Comment does not belong to the specified task",
		}
	}

	attachment, dbErr := s.commentAttachmentRepo.GetAttachmentByID(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	if attachment.CommentID != commentID {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Attachment does not belong to the specified comment",
		}
	}

	allowed := false
	if attachment.UploadedBy == userID || comment.UserID == userID {
		allowed = true
	} else {
		requester, reqErr := s.authRepo.GetUserByID(userID)
		if reqErr == nil && requester.Role == string(dto.RoleOrgAdmin) {
			allowed = true
		} else {
			member, memErr := s.projectRepo.GetProjectMemberByUserAndProjectID(userID, *projectID)
			if memErr == nil && (member.ProjectRole == string(dto.ProjectRoleOrgAdmin) || member.ProjectRole == string(dto.ProjectRoleProjectManager)) {
				allowed = true
			}
		}
	}

	if !allowed {
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only the uploader, comment author, Project Managers, or Organization Administrators can delete this attachment",
		}
	}

	ctx := context.Background()
	_ = s.storageClient.DeleteObject(ctx, attachment.StoragePath)

	dbErr = s.commentAttachmentRepo.DeleteAttachment(attachmentID)
	if dbErr != nil {
		return dbErr
	}

	auditLog := models.AuditLog{
		UserID:         &userID,
		OrganizationID: &orgID,
		ProjectID:      projectID,
		Action:         "comment_attachment_deleted",
		ResourceType:   "attachment",
		ResourceID:     attachmentID.String(),
		Details:        fmt.Sprintf("Attachment %s deleted from comment %s", attachment.OriginalFilename, commentID),
		CreatedAt:      time.Now(),
	}
	if auditErr := s.auditRepo.CreateAuditLog(auditLog); auditErr != nil {
		s.logger.Warn("Failed to create audit log", zap.Any("error", auditErr))
	}

	return nil
}

package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

type CommentsService interface {
	CreateComments(req requestdto.CreateCommentsRequest) *response.Error
	GetCommentByID(req requestdto.GetComments) (*responsedto.CommentsResponse, *response.Error)
	UpdateComments(req requestdto.UpdateCommentsRequest) *response.Error
	DeleteComments(req requestdto.DeleteComments) *response.Error
	GetCommentsByTaskID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error)
	GetCommentsByParentID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error)
}

func InitCommentsService(commentsRepo commentsrepo.CommentsRepository, taskRepo taskrepo.TaskRepository, projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, auditRepo auditrepo.AuditLogRepository, logger *zap.Logger) CommentsService {
	return &commentsService{
		commentsRepo: commentsRepo,
		taskRepo:     taskRepo,
		projectRepo:  projectRepo,
		authRepo:     authRepo,
		auditRepo:    auditRepo,
		logger:       logger,
	}
}

type commentsService struct {
	commentsRepo commentsrepo.CommentsRepository
	taskRepo     taskrepo.TaskRepository
	projectRepo  projectrepo.ProjectRepository
	authRepo     authrepo.AuthRepository
	auditRepo    auditrepo.AuditLogRepository
	logger       *zap.Logger
}

func (s *commentsService) checkAuthorization(userID, taskID uuid.UUID) (*uuid.UUID, bool, *response.Error) {

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

	if user.Role == string(requestdto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == task.Project.OrganizationID {
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

func (s *commentsService) validateParentComment(parentCommentID, taskID, projectID, organizationID uuid.UUID) *response.Error {

	parentComment, err := s.commentsRepo.GetCommentByID(parentCommentID)
	if err != nil {
		return err
	}

	if parentComment.TaskID != taskID {
		s.logger.Error("Parent comment belongs to a different task",
			zap.String("parent_comment_id", parentCommentID.String()))

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Parent comment belongs to a different task",
		}
	}

	if parentComment.ProjectID != projectID {
		s.logger.Error("Parent comment belongs to a different project",
			zap.String("parent_comment_id", parentCommentID.String()))

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Parent comment belongs to a different project",
		}
	}

	if parentComment.OrganizationID != organizationID {
		s.logger.Error("Parent comment belongs to a different organization",
			zap.String("parent_comment_id", parentCommentID.String()))

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Parent comment belongs to a different organization",
		}
	}

	if parentComment.IsDeleted {
		hasReplies, err := s.commentsRepo.HasReplies(parentCommentID)
		if err != nil {
			return err
		}

		if !hasReplies {
			return &response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Cannot reply to a deleted comment.",
			}
		}
	}

	return nil
}

func (s *commentsService) CreateComments(req requestdto.CreateCommentsRequest) *response.Error {

	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to add comments to this project",
			zap.String("user_id", req.UserID.String()))
		return err
	}

	if !authorized {
		s.logger.Error("You do not have permission to add comments to this project",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to add comments to this project",
		}
	}

	if req.ParentCommentID != nil {
		err := s.validateParentComment(*req.ParentCommentID, req.TaskID, *projectID, req.OrganizationID)
		if err != nil {
			return err
		}
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Content cannot be empty",
		}
	}

	comment := models.Comments{
		TaskID:          req.TaskID,
		UserID:          req.UserID,
		ProjectID:       *projectID,
		OrganizationID:  req.OrganizationID,
		Content:         req.Content,
		ParentCommentID: req.ParentCommentID,
		IsDeleted:       false,
	}

	if err := s.commentsRepo.CreateComment(comment); err != nil {
		return err
	}

	action := fmt.Sprintf("Comment created on Task : %v ", req.TaskID)
	if req.ParentCommentID != nil {
		action = fmt.Sprintf("Reply added to comment : %v ", req.ParentCommentID)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         action,
		ResourceType:   "Task",
		ResourceID:     req.TaskID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), action, req.TaskID.String()),
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	return nil
}

func (s *commentsService) GetCommentByID(req requestdto.GetComments) (*responsedto.CommentsResponse, *response.Error) {

	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to get comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, err
	}

	if !authorized {
		s.logger.Error("You do not have permission to get comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}

	comment, respErr := s.commentsRepo.GetCommentByID(req.CommentID)
	if respErr != nil {
		return nil, respErr
	}

	commentResponse := responsedto.CommentsFromModel(*comment)

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "Get Comments",
		ResourceType:   "Task",
		ResourceID:     req.CommentID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Getting Comments", req.TaskID.String()),
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	return &commentResponse, nil
}

func (s *commentsService) UpdateComments(req requestdto.UpdateCommentsRequest) *response.Error {

	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to update comments to this project",
			zap.String("user_id", req.UserID.String()))
		return err
	}

	if !authorized {
		s.logger.Error("You do not have permission to update comments to this project",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Content cannot be empty",
		}
	}

	comment, err := s.commentsRepo.GetCommentByID(req.CommentID)
	if err != nil {
		return err
	}

	if comment.UserID != req.UserID {
		s.logger.Error("You can only update your own comments",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You can only update your own comments",
		}
	}

	updateComment := models.Comments{
		Content: req.Content,
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "Updated Comment",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Updated Comment", req.TaskID.String()),
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return s.commentsRepo.UpdateComment(req.CommentID, updateComment)
}

func (s *commentsService) DeleteComments(req requestdto.DeleteComments) *response.Error {

	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to delete comments to this project",
			zap.String("user_id", req.UserID.String()))
		return err
	}

	if !authorized {
		s.logger.Error("You do not have permission to delete comments to this project",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}

	comment, err := s.commentsRepo.GetCommentByID(req.CommentID)
	if err != nil {
		return err
	}

	if comment.OrganizationID != req.OrganizationID {
		s.logger.Error("You do not have permission to delete comments to this project",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to delete this comment",
		}
	}

	if comment.UserID != req.UserID {
		s.logger.Error("You can only update your own comments",
			zap.String("user_id", req.UserID.String()))
		return &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You can only delete your own comments",
		}
	}

	hasReplies, err := s.commentsRepo.HasReplies(req.CommentID)
	if err != nil {
		return err
	}

	if hasReplies {
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      projectID,
			Action:         "Deleted Comment",
			ResourceType:   "Comment",
			ResourceID:     req.CommentID.String(),
			Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Deleted Comment", req.TaskID.String()),
			CreatedAt:      time.Now(),
		}
		s.auditRepo.CreateAuditLog(auditLog)

		return s.commentsRepo.MarkCommentAsDeleted(req.CommentID)
	}

	err = s.commentsRepo.DeleteComment(req.CommentID)
	if err != nil {
		return err
	}
	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "Deleted Comment",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Deleted Comment", req.TaskID.String()),
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return nil
}

func (s *commentsService) GetCommentsByTaskID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error) {
	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to list comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, response.Pagination{}, err
	}

	if !authorized {
		s.logger.Error("You do not have permission to list comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}
	comments, pagination, err := s.commentsRepo.GetCommentsByTaskID(req)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	commentResponse := make([]responsedto.CommentsResponse, 0, len(comments))
	for _, comment := range comments {
		commentResponse = append(commentResponse, responsedto.CommentsFromModel(comment))
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "Get Comments task ID",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Get Comments task ID", req.TaskID.String()),
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

func (s *commentsService) GetCommentsByParentID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error) {
	projectID, authorized, err := s.checkAuthorization(req.UserID, req.TaskID)
	if err != nil {
		s.logger.Error("You do not have permission to list comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, response.Pagination{}, err
	}

	if !authorized {
		s.logger.Error("You do not have permission to list comments to this project",
			zap.String("user_id", req.UserID.String()))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}
	comments, pagination, err := s.commentsRepo.GetCommentsByParentID(req)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	commentResponse := make([]responsedto.CommentsResponse, 0, len(comments))
	for _, comment := range comments {
		commentResponse = append(commentResponse, responsedto.CommentsFromModel(comment))
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "Get Comments by Parent Comment ID",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        fmt.Sprintf("User %s %s on task %s", req.UserID.String(), "Get Comments by Parent Comment ID", req.TaskID.String()),
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

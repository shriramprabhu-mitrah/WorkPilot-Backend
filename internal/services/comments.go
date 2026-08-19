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
	"github.com/ms-kanban-server/internal/pkg/utils"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	commentsrepo "github.com/ms-kanban-server/internal/repository/comments-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	userstoryrepo "github.com/ms-kanban-server/internal/repository/user-story-repo"
	"go.uber.org/zap"
)

type CommentsService interface {
	CreateComments(req requestdto.CreateCommentsRequest) (responsedto.CommentedUserResponse, *response.Error)
	GetCommentByID(req requestdto.GetComments) (*responsedto.CommentsResponse, *response.Error)
	UpdateComments(req requestdto.UpdateCommentsRequest) (responsedto.CommentedUserResponse, *response.Error)
	DeleteComments(req requestdto.DeleteComments) *response.Error
	GetCommentsByTaskID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error)
	GetCommentsByUserStoryID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error)
	GetCommentsByParentID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error)
}

func InitCommentsService(commentsRepo commentsrepo.CommentsRepository, taskRepo taskrepo.TaskRepository, userStoryRepo userstoryrepo.UserStoryRepository, projectRepo projectrepo.ProjectRepository, authRepo authrepo.AuthRepository, auditRepo auditrepo.AuditLogRepository, logger *zap.Logger) CommentsService {
	return &commentsService{
		commentsRepo:  commentsRepo,
		taskRepo:      taskRepo,
		userStoryRepo: userStoryRepo,
		projectRepo:   projectRepo,
		authRepo:      authRepo,
		auditRepo:     auditRepo,
		logger:        logger,
	}
}

type commentsService struct {
	commentsRepo  commentsrepo.CommentsRepository
	taskRepo      taskrepo.TaskRepository
	userStoryRepo userstoryrepo.UserStoryRepository
	projectRepo   projectrepo.ProjectRepository
	authRepo      authrepo.AuthRepository
	auditRepo     auditrepo.AuditLogRepository
	logger        *zap.Logger
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

func (s *commentsService) checkUserStoryAuthorization(userID, userStoryID uuid.UUID) (*uuid.UUID, bool, *response.Error) {
	user, err := s.authRepo.GetUserByID(userID)
	if err != nil {
		return nil, false, err
	}

	storyCtx, err := s.userStoryRepo.GetUserStoryAccessContext(userStoryID)
	if err != nil {
		return nil, false, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "User story does not belong to the specified project",
		}
	}

	if user.Role == string(requestdto.RoleOrgAdmin) && user.OrganizationID != nil && *user.OrganizationID == storyCtx.OrganizationID {
		return &storyCtx.ProjectID, true, nil
	}

	if user.OrganizationID == nil || *user.OrganizationID != storyCtx.OrganizationID {
		return &storyCtx.ProjectID, false, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You are not authorized to access this organization",
		}
	}

	isMember, err := s.projectRepo.IsUserProjectMember(storyCtx.ProjectID, userID)
	if err != nil {
		return nil, false, err
	}

	return &storyCtx.ProjectID, isMember, nil
}

func (s *commentsService) validateParentComment(parentCommentID uuid.UUID, taskID *uuid.UUID, userStoryID *uuid.UUID, projectID, organizationID uuid.UUID) *response.Error {

	parentComment, err := s.commentsRepo.GetCommentByID(parentCommentID)
	if err != nil {
		return err
	}

	if taskID != nil && parentComment.TaskID != nil && *parentComment.TaskID != *taskID {
		s.logger.Error("Parent comment belongs to a different task",
			zap.String("parent_comment_id", parentCommentID.String()))

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Parent comment belongs to a different task",
		}
	}

	if userStoryID != nil && parentComment.UserStoryID != nil && *parentComment.UserStoryID != *userStoryID {
		s.logger.Error("Parent comment belongs to a different user story",
			zap.String("parent_comment_id", parentCommentID.String()))

		return &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Parent comment belongs to a different user story",
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

func (s *commentsService) CreateComments(req requestdto.CreateCommentsRequest) (responsedto.CommentedUserResponse, *response.Error) {

	var projectID *uuid.UUID
	var authorized bool
	var err *response.Error

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
	} else {
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Either Task ID or User Story ID must be provided",
		}
	}

	if err != nil {
		s.logger.Error("You do not have permission to add comments to this project",
			zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, err
	}

	if !authorized {
		s.logger.Error("You do not have permission to add comments to this project",
			zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to add comments to this project",
		}
	}

	if req.ParentCommentID != nil {
		err := s.validateParentComment(*req.ParentCommentID, req.TaskID, req.UserStoryID, *projectID, req.OrganizationID)
		if err != nil {
			return responsedto.CommentedUserResponse{}, err
		}
	}

	req.Content = utils.SanitizeHTML(req.Content)
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Content cannot be empty",
		}
	}

	comment := &models.Comments{
		TaskID:          req.TaskID,
		UserStoryID:     req.UserStoryID,
		UserID:          req.UserID,
		ProjectID:       *projectID,
		OrganizationID:  req.OrganizationID,
		Content:         req.Content,
		ParentCommentID: req.ParentCommentID,
		IsDeleted:       false,
	}

	if err := s.commentsRepo.CreateComment(comment); err != nil {
		return responsedto.CommentedUserResponse{}, err
	}

	var action string
	var resourceType string
	var resourceID string

	if req.UserStoryID != nil {
		resourceType = "UserStory"
		resourceID = req.UserStoryID.String()
		action = fmt.Sprintf("Comment created on User Story : %v ", req.UserStoryID)
		if req.ParentCommentID != nil {
			action = fmt.Sprintf("Reply added to comment : %v ", req.ParentCommentID)
		}
	} else {
		resourceType = "Task"
		resourceID = req.TaskID.String()
		action = fmt.Sprintf("Comment created on Task : %v ", req.TaskID)
		if req.ParentCommentID != nil {
			action = fmt.Sprintf("Reply added to comment : %v ", req.ParentCommentID)
		}
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "created",
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Details:        action,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	user, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		s.logger.Error("Failed to fetch user details after creating comment", zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, err
	}

	var avatarURL *string
	if user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}

	response := responsedto.CommentedUserResponse{
		ID:        comment.ID,
		UserID:    user.ID,
		UserName:  user.UserName,
		FullName:  user.FullName,
		AvatarURL: avatarURL,
	}

	return response, nil
}

func (s *commentsService) GetCommentByID(req requestdto.GetComments) (*responsedto.CommentsResponse, *response.Error) {

	var projectID *uuid.UUID
	var authorized bool
	var err *response.Error

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
	} else {
		comment, respErr := s.commentsRepo.GetCommentByID(req.CommentID)
		if respErr != nil {
			return nil, respErr
		}
		if comment.UserStoryID != nil {
			projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *comment.UserStoryID)
		} else if comment.TaskID != nil {
			projectID, authorized, err = s.checkAuthorization(req.UserID, *comment.TaskID)
		}
	}

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

	var resourceID, action string
	if comment.UserStoryID != nil {
		resourceID = comment.UserStoryID.String()
		action = "Comment created on User Story : " + resourceID
	} else if comment.TaskID != nil {
		resourceID = comment.TaskID.String()
		action = "Comment created on Task : " + resourceID
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "viewed",
		ResourceType:   "Comment",
		ResourceID:     resourceID,
		Details:        action,
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	return &commentResponse, nil
}

func (s *commentsService) UpdateComments(req requestdto.UpdateCommentsRequest) (responsedto.CommentedUserResponse, *response.Error) {

	var projectID *uuid.UUID
	var authorized bool
	var err *response.Error

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
	} else {
		comment, respErr := s.commentsRepo.GetCommentByID(req.CommentID)
		if respErr != nil {
			return responsedto.CommentedUserResponse{}, respErr
		}
		if comment.UserStoryID != nil {
			projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *comment.UserStoryID)
		} else if comment.TaskID != nil {
			projectID, authorized, err = s.checkAuthorization(req.UserID, *comment.TaskID)
		}
	}

	if err != nil {
		s.logger.Error("You do not have permission to update comments to this project",
			zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, err
	}

	if !authorized {
		s.logger.Error("You do not have permission to update comments to this project",
			zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You do not have permission to view this comment",
		}
	}

	req.Content = utils.SanitizeHTML(req.Content)
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Content cannot be empty",
		}
	}

	comment, err := s.commentsRepo.GetCommentByID(req.CommentID)
	if err != nil {
		return responsedto.CommentedUserResponse{}, err
	}

	if comment.UserID != req.UserID {
		s.logger.Error("You can only update your own comments",
			zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, &response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "You can only update your own comments",
		}
	}

	updateComment := models.Comments{
		Content: req.Content,
	}

	var resourceID, action string
	if comment.UserStoryID != nil {
		resourceID = comment.UserStoryID.String()
		action = "Comment Updated on User Story : " + resourceID
	} else if comment.TaskID != nil {
		resourceID = comment.TaskID.String()
		action = "Comment Updated on Task : " + resourceID
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "updated",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        action,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	user, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		s.logger.Error("Failed to fetch user details after creating comment", zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, err
	}

	var avatarURL *string
	if user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}

	response := responsedto.CommentedUserResponse{
		ID:        req.CommentID,
		UserID:    user.ID,
		UserName:  user.UserName,
		FullName:  user.FullName,
		AvatarURL: avatarURL,
	}

	return response, s.commentsRepo.UpdateComment(req.CommentID, &updateComment)
}

func (s *commentsService) DeleteComments(req requestdto.DeleteComments) *response.Error {

	var projectID *uuid.UUID
	var authorized bool
	var err *response.Error

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
	} else {
		comment, respErr := s.commentsRepo.GetCommentByID(req.CommentID)
		if respErr != nil {
			return respErr
		}
		if comment.UserStoryID != nil {
			projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *comment.UserStoryID)
		} else if comment.TaskID != nil {
			projectID, authorized, err = s.checkAuthorization(req.UserID, *comment.TaskID)
		}
	}

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

	var resourceID, action string
	if comment.UserStoryID != nil {
		resourceID = comment.UserStoryID.String()
		action = "Comment : \" " + comment.Content + " \" deleted on User Story : " + resourceID
	} else if comment.TaskID != nil {
		resourceID = comment.TaskID.String()
		action = "Comment : \" " + comment.Content + " \" deleted on Task : " + resourceID
	}

	if hasReplies {
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      projectID,
			Action:         "deleted",
			ResourceType:   "Comment",
			ResourceID:     req.CommentID.String(),
			Details:        action,
			Type:           models.AuditLogTypeActivity,
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
		Action:         "deleted",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        action,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return nil
}

func (s *commentsService) GetCommentsByTaskID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error) {
	taskID := uuid.Nil
	if req.TaskID != nil {
		taskID = *req.TaskID
	}
	projectID, authorized, err := s.checkAuthorization(req.UserID, taskID)
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
		Action:         "viewed",
		ResourceType:   "Comment",
		ResourceID:     taskID.String(),
		Details:        fmt.Sprintf("User %s viewed comments on task %s", req.UserID.String(), taskID.String()),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

func (s *commentsService) GetCommentsByParentID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error) {
	var projectID *uuid.UUID
	var authorized bool
	var err *response.Error

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
	}

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

	var taskOrStoryIDStr, action string
	if req.TaskID != nil {
		taskOrStoryIDStr = req.TaskID.String()
		action = "viewed comments on task " + taskOrStoryIDStr
	} else if req.UserStoryID != nil {
		taskOrStoryIDStr = req.UserStoryID.String()
		action = "viewed comments on user story " + taskOrStoryIDStr
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		Action:         "viewed",
		ResourceType:   "Comment",
		ResourceID:     req.CommentID.String(),
		Details:        action,
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

func (s *commentsService) GetCommentsByUserStoryID(req requestdto.GetComments) ([]responsedto.CommentsResponse, response.Pagination, *response.Error) {
	userStoryID := uuid.Nil
	if req.UserStoryID != nil {
		userStoryID = *req.UserStoryID
	}
	projectID, authorized, err := s.checkUserStoryAuthorization(req.UserID, userStoryID)
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
	comments, pagination, err := s.commentsRepo.GetCommentsByUserStoryID(req)
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
		Action:         "viewed",
		ResourceType:   "Comment",
		ResourceID:     userStoryID.String(),
		Details:        fmt.Sprintf("User %s viewed comments on user story %s", req.UserID.String(), userStoryID.String()),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

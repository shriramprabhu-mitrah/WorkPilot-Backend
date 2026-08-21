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

	var taskID *uuid.UUID
	var userStoryID *uuid.UUID

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
		userStoryID = req.UserStoryID
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
		taskID = req.TaskID
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
		err := s.validateParentComment(*req.ParentCommentID, taskID, userStoryID, *projectID, req.OrganizationID)
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
		TaskID:          taskID,
		UserStoryID:     userStoryID,
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

	user, err := s.authRepo.GetUserByID(req.UserID)
	if err != nil {
		s.logger.Error("Failed to fetch user details after creating comment", zap.String("user_id", req.UserID.String()))
		return responsedto.CommentedUserResponse{}, err
	}

	userName := user.UserName
	if userName == "" {
		userName = user.FullName
	}
	if userName == "" {
		userName = user.Email
	}
	if userName == "" {
		userName = req.UserID.String()
	}

	var resourceTitle string
	var detail string

	if userStoryID != nil {
		story, storyErr := s.userStoryRepo.GetUserStoryByID(*userStoryID, *projectID)
		if storyErr == nil && story != nil {
			resourceTitle = story.Title
			detail = fmt.Sprintf("%s commented on the userstory: %s as %s", userName, story.Title, comment.Content)
		} else {
			detail = fmt.Sprintf("%s commented on the userstory: %s as %s", userName, userStoryID.String(), comment.Content)
		}
	} else if taskID != nil {
		task, taskErr := s.taskRepo.GetTaskByID(*taskID, *projectID)
		if taskErr == nil && task != nil {
			resourceTitle = task.Title
			detail = fmt.Sprintf("%s commented on the task: %s as %s", userName, task.Title, comment.Content)
		} else {
			detail = fmt.Sprintf("%s commented on the task: %s as %s", userName, taskID.String(), comment.Content)
		}
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		TaskID:         taskID,
		UserStoryID:    userStoryID,
		Action:         "created",
		ResourceType:   "comment",
		ResourceID:     comment.ID.String(),
		Title:          resourceTitle,
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	var avatarURL *string
	if user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}

	response := responsedto.CommentedUserResponse{
		ID:          comment.ID,
		TaskID:      taskID,
		UserStoryID: userStoryID,
		UserID:      user.ID,
		UserName:    user.UserName,
		FullName:    user.FullName,
		AvatarURL:   avatarURL,
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
	var taskID *uuid.UUID
	var userStoryID *uuid.UUID

	if comment.UserStoryID != nil {
		userStoryID = comment.UserStoryID
		resourceID = comment.UserStoryID.String()
		action = "Comment viewed on User Story : " + resourceID
	} else if comment.TaskID != nil {
		taskID = comment.TaskID
		resourceID = comment.TaskID.String()
		action = "Comment viewed on Task : " + resourceID
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		TaskID:         taskID,
		UserStoryID:    userStoryID,
		Action:         "viewed",
		ResourceType:   "comment",
		ResourceID:     resourceID,
		Details:        action,
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}

	s.auditRepo.CreateAuditLog(auditLog)

	return &commentResponse, nil
}

func resolveUserName(user models.User, fallbackID uuid.UUID) string {
	if user.UserName != "" {
		return user.UserName
	}
	if user.FullName != "" {
		return user.FullName
	}
	if user.Email != "" {
		return user.Email
	}
	return fallbackID.String()
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

	user, userErr := s.authRepo.GetUserByID(req.UserID)
	userName := req.UserID.String()
	if userErr == nil {
		userName = resolveUserName(user, req.UserID)
	}

	var resourceTitle string
	var targetStr string
	var taskID *uuid.UUID
	var userStoryID *uuid.UUID

	if comment.UserStoryID != nil && projectID != nil {
		userStoryID = comment.UserStoryID
		story, storyErr := s.userStoryRepo.GetUserStoryByID(*comment.UserStoryID, *projectID)
		if storyErr == nil && story != nil {
			resourceTitle = story.Title
			targetStr = fmt.Sprintf("userstory: %s", story.Title)
		} else {
			targetStr = fmt.Sprintf("userstory: %s", comment.UserStoryID.String())
		}
	} else if comment.TaskID != nil && projectID != nil {
		taskID = comment.TaskID
		task, taskErr := s.taskRepo.GetTaskByID(*comment.TaskID, *projectID)
		if taskErr == nil && task != nil {
			resourceTitle = task.Title
			targetStr = fmt.Sprintf("task: %s", task.Title)
		} else {
			targetStr = fmt.Sprintf("task: %s", comment.TaskID.String())
		}
	}

	var detail string
	if comment.Content != req.Content {
		detail = fmt.Sprintf("%s updated comment on the %s: content changed from '%s' to '%s'", userName, targetStr, comment.Content, req.Content)
	} else {
		detail = fmt.Sprintf("%s updated comment on the %s", userName, targetStr)
	}

	updateComment := models.Comments{
		Content: req.Content,
	}

	err = s.commentsRepo.UpdateComment(req.CommentID, &updateComment)
	if err != nil {
		return responsedto.CommentedUserResponse{}, err
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		TaskID:         taskID,
		UserStoryID:    userStoryID,
		Action:         "updated",
		ResourceType:   "comment",
		ResourceID:     req.CommentID.String(),
		Title:          resourceTitle,
		Details:        detail,
		Type:           models.AuditLogTypeActivity,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	var avatarURL *string
	if userErr == nil && user.AvatarURL != "" {
		avatarURL = &user.AvatarURL
	}

	response := responsedto.CommentedUserResponse{
		ID:          req.CommentID,
		TaskID:      taskID,
		UserStoryID: userStoryID,
		UserID:      req.UserID,
		UserName:    user.UserName,
		FullName:    user.FullName,
		AvatarURL:   avatarURL,
	}

	return response, nil
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

	user, _ := s.authRepo.GetUserByID(req.UserID)
	userName := resolveUserName(user, req.UserID)

	var resourceTitle string
	var targetStr string
	var taskID *uuid.UUID
	var userStoryID *uuid.UUID

	if comment.UserStoryID != nil && projectID != nil {
		userStoryID = comment.UserStoryID
		story, storyErr := s.userStoryRepo.GetUserStoryByID(*comment.UserStoryID, *projectID)
		if storyErr == nil && story != nil {
			resourceTitle = story.Title
			targetStr = fmt.Sprintf("userstory: %s", story.Title)
		} else {
			targetStr = fmt.Sprintf("userstory: %s", comment.UserStoryID.String())
		}
	} else if comment.TaskID != nil && projectID != nil {
		taskID = comment.TaskID
		task, taskErr := s.taskRepo.GetTaskByID(*comment.TaskID, *projectID)
		if taskErr == nil && task != nil {
			resourceTitle = task.Title
			targetStr = fmt.Sprintf("task: %s", task.Title)
		} else {
			targetStr = fmt.Sprintf("task: %s", comment.TaskID.String())
		}
	}

	detail := fmt.Sprintf("Comment on the %s was deleted by %s", targetStr, userName)

	hasReplies, err := s.commentsRepo.HasReplies(req.CommentID)
	if err != nil {
		return err
	}

	if hasReplies {
		auditLog := models.AuditLog{
			UserID:         &req.UserID,
			OrganizationID: &req.OrganizationID,
			ProjectID:      projectID,
			TaskID:         taskID,
			UserStoryID:    userStoryID,
			Action:         "deleted",
			ResourceType:   "comment",
			ResourceID:     req.CommentID.String(),
			Title:          resourceTitle,
			Details:        detail,
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
		TaskID:         taskID,
		UserStoryID:    userStoryID,
		Action:         "deleted",
		ResourceType:   "comment",
		ResourceID:     req.CommentID.String(),
		Title:          resourceTitle,
		Details:        detail,
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

	user, _ := s.authRepo.GetUserByID(req.UserID)
	userName := resolveUserName(user, req.UserID)

	taskTitle := taskID.String()
	if projectID != nil {
		task, taskErr := s.taskRepo.GetTaskByID(taskID, *projectID)
		if taskErr == nil && task != nil && task.Title != "" {
			taskTitle = task.Title
		}
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		TaskID:         &taskID,
		Action:         "viewed",
		ResourceType:   "comment",
		ResourceID:     taskID.String(),
		Title:          taskTitle,
		Details:        fmt.Sprintf("Comments on task '%s' viewed by %s", taskTitle, userName),
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

	var taskID *uuid.UUID
	var userStoryID *uuid.UUID

	if req.UserStoryID != nil {
		projectID, authorized, err = s.checkUserStoryAuthorization(req.UserID, *req.UserStoryID)
		userStoryID = req.UserStoryID
	} else if req.TaskID != nil {
		projectID, authorized, err = s.checkAuthorization(req.UserID, *req.TaskID)
		taskID = req.TaskID
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

	user, _ := s.authRepo.GetUserByID(req.UserID)
	userName := resolveUserName(user, req.UserID)

	var resourceTitle, action string
	if taskID != nil && projectID != nil {
		task, taskErr := s.taskRepo.GetTaskByID(*taskID, *projectID)
		taskTitle := taskID.String()
		if taskErr == nil && task != nil && task.Title != "" {
			taskTitle = task.Title
		}
		resourceTitle = taskTitle
		action = fmt.Sprintf("Comment replies on task '%s' viewed by %s", taskTitle, userName)
	} else if userStoryID != nil && projectID != nil {
		story, storyErr := s.userStoryRepo.GetUserStoryByID(*userStoryID, *projectID)
		storyTitle := userStoryID.String()
		if storyErr == nil && story != nil && story.Title != "" {
			storyTitle = story.Title
		}
		resourceTitle = storyTitle
		action = fmt.Sprintf("Comment replies on user story '%s' viewed by %s", storyTitle, userName)
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		TaskID:         taskID,
		UserStoryID:    userStoryID,
		Action:         "viewed",
		ResourceType:   "comment",
		ResourceID:     req.CommentID.String(),
		Title:          resourceTitle,
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

	user, _ := s.authRepo.GetUserByID(req.UserID)
	userName := resolveUserName(user, req.UserID)

	storyTitle := userStoryID.String()
	if projectID != nil {
		story, storyErr := s.userStoryRepo.GetUserStoryByID(userStoryID, *projectID)
		if storyErr == nil && story != nil && story.Title != "" {
			storyTitle = story.Title
		}
	}

	auditLog := models.AuditLog{
		UserID:         &req.UserID,
		OrganizationID: &req.OrganizationID,
		ProjectID:      projectID,
		UserStoryID:    &userStoryID,
		Action:         "viewed",
		ResourceType:   "comment",
		ResourceID:     userStoryID.String(),
		Title:          storyTitle,
		Details:        fmt.Sprintf("Comments on user story '%s' viewed by %s", storyTitle, userName),
		Type:           models.AuditLogTypeAudit,
		CreatedAt:      time.Now(),
	}
	s.auditRepo.CreateAuditLog(auditLog)

	return commentResponse, pagination, nil
}

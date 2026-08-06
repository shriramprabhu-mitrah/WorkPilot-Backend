package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitCommentsHandler(commentService services.CommentsService, logger *zap.Logger) *commentsHandler {
	return &commentsHandler{
		commentService: commentService,
		logger:         logger,
	}
}

type commentsHandler struct {
	commentService services.CommentsService
	logger         *zap.Logger
}

// CreateComments godoc
//
//	@Summary		Create a comment
//	@Description	Creates a new comment for the specified task. To create a reply, provide the parent_comment_id of an existing comment. The comment content is trimmed and cannot be empty after trimming.
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string								true	"Task ID (UUID)"
//	@Param			request	body		requestdto.CreateCommentsRequest	true	"Comment details"
//
// @Success 201 {object} response.SuccessResponse
//
//	@Failure		400			{object}	response.ErrorResponse				"Invalid request, task ID, parent comment, or comment content"
//	@Failure		401			{object}	response.ErrorResponse				"Unauthorized"
//	@Failure		403			{object}	response.ErrorResponse				"Forbidden"
//	@Failure		500			{object}	response.ErrorResponse				"Internal server error"
//	@Router			/task/{task_id}/comment [post]
func (h *commentsHandler) CreateComments(g *gin.Context) {

	var payload requestdto.CreateCommentsRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}

		g.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	payload.TaskID = taskUUID
	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	if err := h.commentService.CreateComments(payload); err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Comment created successfully",
	})
}

// GetCommentByID godoc
//
//	@Summary		Get Comment By ID
//	@Description	Get a comment by ID along with its replies
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id		path		string	true	"Task ID"
//	@Param			comment_id	path		string	true	"Comment ID"
// @Success 200 {object} response.SuccessResponse
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/task/{task_id}/comment/{comment_id} [get]
func (h *commentsHandler) GetCommentByID(g *gin.Context) {

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert task_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	commentParam := g.Param("comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	req := requestdto.GetComments{
		TaskID:         taskUUID,
		CommentID:      commentUUID,
		UserID:         userUUID,
		OrganizationID: organizationUUID,
	}

	comment, err := h.commentService.GetCommentByID(req)
	if err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success: true,
		Message: "Comment fetched successfully",
		Data:    comment,
	})
}

// UpdateComments godoc
//
//	@Summary		Update Comment
//	@Description	Update an existing comment
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id		path		string								true	"Task ID"
//	@Param			comment_id	path		string								true	"Comment ID"
//	@Param			request		body		requestdto.UpdateCommentsRequest	true	"Update Comment"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/task/{task_id}/comment/{comment_id} [patch]
func (h *commentsHandler) UpdateComments(g *gin.Context) {

	var req requestdto.UpdateCommentsRequest

	if err := g.ShouldBindJSON(&req); err != nil {
		message := utils.ValidationErrorMessage(err, req)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}

		g.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert task_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	commentParam := g.Param("comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	req.CommentID = commentUUID
	req.TaskID = taskUUID
	req.UserID = userUUID
	req.OrganizationID = organizationUUID

	if err := h.commentService.UpdateComments(req); err != nil {
		g.JSON(err.StatusCode, err)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		Message:    "Comment updated successfully",
		StatusCode: http.StatusOK,
		Data: map[string]uuid.UUID{
			"comment_id": commentUUID,
		},
	})
}

// DeleteComments godoc
//
//	@Summary		Delete Comment
//	@Description	Delete a comment
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id		path		string	true	"Task ID"
//	@Param			comment_id	path		string	true	"Comment ID"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/tasks/{task_id}/comments/{comment_id} [delete]
func (h *commentsHandler) DeleteComments(g *gin.Context) {

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert task_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	commentParam := g.Param("comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	req := requestdto.DeleteComments{
		TaskID:         taskUUID,
		CommentID:      commentUUID,
		UserID:         userUUID,
		OrganizationID: organizationUUID,
	}

	if err := h.commentService.DeleteComments(req); err != nil {
		g.JSON(err.StatusCode, err)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Comment deleted successfully",
	})
}

// GetCommentsByTaskID godoc
//
//	@Summary		Get Task Comments
//	@Description	Get paginated top-level comments for a task along with their replies
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id		path		string	true	"Task ID"
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			page_size	query		int		false	"Number of comments per page"	default(10)
//  @Success 200    {object} response.SuccessResponse
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		401			{object}	response.ErrorResponse
//	@Failure		403			{object}	response.ErrorResponse
//	@Failure		404			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/task/{task_id}/comment [get]
func (h *commentsHandler) GetCommentsByTaskID(g *gin.Context) {

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert task_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	page, pageErr := strconv.Atoi(g.DefaultQuery("page", "1"))
	pageSize, pageSizeErr := strconv.Atoi(g.DefaultQuery("page_size", "10"))
	if pageErr != nil || pageSizeErr != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid pagination parameters.",
			},
		}

		h.logger.Error("Invalid pagination parameters")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	paginationQuery := response.PaginationQuery{
		Page:     page,
		PageSize: pageSize,
	}

	result, meta, err := h.commentService.GetCommentsByTaskID(requestdto.GetComments{
		PaginationQuery: paginationQuery,
		TaskID:          taskUUID,
		OrganizationID:  organizationUUID,
		UserID:          userUUID,
	})

	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Comments received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       result,
		Meta:       &meta,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetCommentsByParentID godoc
//
//	@Summary		Get Reply Comments
//	@Description	Get all replies for a parent comment in a task
//	@Tags			Comments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id				path		string	true	"Task ID"
//	@Param			parent_comment_id	path		string	true	"Parent Comment ID"
//	@Param			page				query		int		false	"Page Number"		default(1)
//	@Param			page_size			query		int		false	"Page Size"			default(10)
//  @Success 200    {object} response.SuccessResponse
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/task/{task_id}/comment/replies/{parent_comment_id} [get]
func (h *commentsHandler) GetCommentsByParentID(g *gin.Context) {

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert task_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	commentParam := g.Param("parent_comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment_id into UUID")
		g.JSON(errorResponse.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	page, pageErr := strconv.Atoi(g.DefaultQuery("page", "1"))
	pageSize, pageSizeErr := strconv.Atoi(g.DefaultQuery("page_size", "10"))
	if pageErr != nil || pageSizeErr != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid pagination parameters.",
			},
		}

		h.logger.Error("Invalid pagination parameters")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	paginationQuery := response.PaginationQuery{
		Page:     page,
		PageSize: pageSize,
	}

	result, meta, err := h.commentService.GetCommentsByParentID(requestdto.GetComments{
		PaginationQuery: paginationQuery,
		TaskID:          taskUUID,
		OrganizationID:  organizationUUID,
		UserID:          userUUID,
		CommentID: commentUUID,
	})

	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Comments received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       result,
		Meta:       &meta,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}
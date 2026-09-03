package handlers

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var _ = responsedto.AttachmentResponse{}

type attachmentHandler struct {
	service services.AttachmentService
	logger  *zap.Logger
}

func InitAttachmentHandler(service services.AttachmentService, logger *zap.Logger) *attachmentHandler {
	return &attachmentHandler{
		service: service,
		logger:  logger,
	}
}

func (h *attachmentHandler) parseFiles(g *gin.Context) ([]*multipart.FileHeader, *response.Error, string) {
	cfg := h.service.GetConfig()
	maxSizeMB := cfg.MaxFileSizeMB
	maxFiles := cfg.MaxFiles

	maxRequestSize := (maxSizeMB * 1024 * 1024 * int64(maxFiles)) + (10 * 1024 * 1024)
	g.Request.Body = http.MaxBytesReader(g.Writer, g.Request.Body, maxRequestSize)

	form, formErr := g.MultipartForm()
	if formErr != nil {
		h.logger.Error("Failed to parse multipart form", zap.Error(formErr))
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Failed to parse multipart form",
		}, "Failed to parse multipart form"
	}

	var allHeaders []*multipart.FileHeader
	for key, headers := range form.File {
		if key == "file" || key == "files" {
			allHeaders = append(allHeaders, headers...)
		}
	}

	if len(allHeaders) == 0 {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Missing file(s) in request payload (use form-data keys 'file' or 'files')",
		}, "Missing file parameter"
	}

	if len(allHeaders) > maxFiles {
		return nil, &response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("Maximum of %d files can be uploaded per request.", maxFiles),
		}, "Too many files in request"
	}

	for _, header := range allHeaders {
		if header.Size > maxSizeMB*1024*1024 {
			return nil, &response.Error{
				Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
				StatusCode: http.StatusRequestEntityTooLarge,
				Message:    fmt.Sprintf("File %s exceeds the maximum allowed size of %d MB.", header.Filename, maxSizeMB),
			}, "File size limit exceeded"
		}
	}

	return allHeaders, nil, ""
}

func writeAttachmentDownload(
	g *gin.Context,
	stream io.ReadCloser,
	filename string,
	mimeType string,
	size int64,
) {
	defer stream.Close()

	// Sanitize original filename to prevent CR/LF header injection
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.ReplaceAll(filename, "\r", "")

	// Use RFC 6266 Content-Disposition formatting to handle special characters securely
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	g.Header("Content-Disposition", disposition)
	g.Header("Content-Type", mimeType)
	g.Header("Content-Length", strconv.FormatInt(size, 10))

	g.DataFromReader(http.StatusOK, size, mimeType, stream, nil)
}

// UploadAttachment godoc
// @Summary Upload Attachment to Existing Task
// @Description Upload one or more files and link them directly to an already existing Task. Use multipart/form-data with field name 'file' or 'files'.
// @Tags Task Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param task_id path string true "Task ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.AttachmentResponse} "Attachments uploaded and linked successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/attachments [post]
func (h *attachmentHandler) UploadAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadAttachments(g.Request.Context(), &taskUUID, projectUUID, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// GetAttachments godoc
// @Summary List Attachments for Task
// @Description Retrieve metadata (IDs, filenames, MIME types, sizes, URLs) for all files attached to a specific Task.
// @Tags Task Attachments
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param task_id path string true "Task ID (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.AttachmentResponse} "Attachments retrieved successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/attachments [get]
func (h *attachmentHandler) GetAttachments(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachments, err := h.service.GetAttachments(g.Request.Context(), taskUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachments retrieved successfully",
		Data:       attachments,
	})
}

// DownloadAttachment godoc
// @Summary Download Task Attachment File
// @Description Stream and download the binary file content of a specific task attachment.
// @Tags Task Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param task_id path string true "Task ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {file} file "Attachment File Stream"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/attachments/{attachment_id}/download [get]
func (h *attachmentHandler) DownloadAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadAttachment(g.Request.Context(), attachmentUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	writeAttachmentDownload(g, stream, filename, mimeType, size)
}

// DeleteAttachment godoc
// @Summary Delete Task Attachment
// @Description Permanently delete an attachment associated with a Task from both storage and database.
// @Tags Task Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param task_id path string true "Task ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {object} response.SuccessResponse "Attachment deleted successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/{task_id}/attachments/{attachment_id} [delete]
func (h *attachmentHandler) DeleteAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteAttachment(g.Request.Context(), attachmentUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachment deleted successfully",
	})
}

// UploadCommentAttachment godoc
// @Summary Upload Attachment to Existing Task Comment
// @Description Upload one or more files and link them directly to an already existing Comment on a Task.
// @Tags Comment Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Task ID (UUID or Key)"
// @Param comment_id path string true "Comment ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.CommentAttachmentResponse} "Comment attachments uploaded successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments [post]
func (h *attachmentHandler) UploadCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errResp := h.service.ResolveTaskID(g.Param("task_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	commentUUID, errorResponse := utils.StringToUUID(g.Param("comment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadCommentAttachments(g.Request.Context(), &commentUUID, &taskUUID, nil, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// GetCommentAttachments godoc
// @Summary List Attachments for Task Comment
// @Description Retrieve metadata (IDs, filenames, MIME types, sizes, URLs) for all files attached to a specific Comment on a Task.
// @Tags Comment Attachments
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Task ID (UUID or Key)"
// @Param comment_id path string true "Comment ID (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.CommentAttachmentResponse} "Comment attachments retrieved successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments [get]
func (h *attachmentHandler) GetCommentAttachments(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errResp := h.service.ResolveTaskID(g.Param("task_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	commentUUID, errorResponse := utils.StringToUUID(g.Param("comment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert comment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachments, err := h.service.GetCommentAttachments(g.Request.Context(), commentUUID, taskUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachments retrieved successfully",
		Data:       attachments,
	})
}

// DownloadCommentAttachment godoc
// @Summary Download Task Comment Attachment File
// @Description Stream and download the binary file content of a specific attachment on a Task Comment.
// @Tags Comment Attachments
// @Security BearerAuth
// @Param task_id path string true "Task ID (UUID or Key)"
// @Param comment_id path string true "Comment ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {file} file "Attachment File Stream"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments/{attachment_id}/download [get]
func (h *attachmentHandler) DownloadCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errResp := h.service.ResolveTaskID(g.Param("task_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadCommentAttachment(g.Request.Context(), attachmentUUID, taskUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	writeAttachmentDownload(g, stream, filename, mimeType, size)
}

// DeleteCommentAttachment godoc
// @Summary Delete Task Comment Attachment
// @Description Permanently delete an attachment associated with a Task Comment from both storage and database.
// @Tags Comment Attachments
// @Security BearerAuth
// @Param task_id path string true "Task ID (UUID or Key)"
// @Param comment_id path string true "Comment ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {object} response.SuccessResponse "Attachment deleted successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments/{attachment_id} [delete]
func (h *attachmentHandler) DeleteCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errResp := h.service.ResolveTaskID(g.Param("task_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteCommentAttachment(g.Request.Context(), attachmentUUID, taskUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachment deleted successfully",
	})
}

// UploadUserStoryAttachment godoc
// @Summary Upload Attachment to Existing User Story
// @Description Upload one or more files and link them directly to an already existing User Story. Use multipart/form-data with field name 'file' or 'files'.
// @Tags User Story Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.UserStoryAttachmentResponse} "Attachments uploaded and linked successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/attachments [post]
func (h *attachmentHandler) UploadUserStoryAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	storyUUID, errResp := h.service.ResolveUserStoryID(g.Param("user_story_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadUserStoryAttachments(g.Request.Context(), &storyUUID, projectUUID, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// GetUserStoryAttachments godoc
// @Summary List Attachments for User Story
// @Description Retrieve metadata (IDs, filenames, MIME types, sizes, URLs) for all files attached to a specific User Story.
// @Tags User Story Attachments
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]responsedto.UserStoryAttachmentResponse} "Attachments retrieved successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/attachments [get]
func (h *attachmentHandler) GetUserStoryAttachments(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	storyUUID, errResp := h.service.ResolveUserStoryID(g.Param("user_story_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	attachments, err := h.service.GetUserStoryAttachments(g.Request.Context(), storyUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachments retrieved successfully",
		Data:       attachments,
	})
}

// DownloadUserStoryAttachment godoc
// @Summary Download User Story Attachment File
// @Description Stream and download the binary file content of a specific User Story attachment.
// @Tags User Story Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {file} file "Attachment File Stream"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/attachments/{attachment_id}/download [get]
func (h *attachmentHandler) DownloadUserStoryAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadUserStoryAttachment(g.Request.Context(), attachmentUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	writeAttachmentDownload(g, stream, filename, mimeType, size)
}

// DeleteUserStoryAttachment godoc
// @Summary Delete User Story Attachment
// @Description Permanently delete an attachment associated with a User Story from both storage and database.
// @Tags User Story Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {object} response.SuccessResponse "Attachment deleted successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/attachments/{attachment_id} [delete]
func (h *attachmentHandler) DeleteUserStoryAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteUserStoryAttachment(g.Request.Context(), attachmentUUID, projectUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachment deleted successfully",
	})
}

// UploadCommentAttachmentWithoutComment godoc
// @Summary Upload Task Comment Attachment (Draft / Before Comment Creation)
// @Description Pre-upload file(s) before creating a comment on a Task. Returns attachment UUIDs and URLs to pass in the 'attachments' array of the CreateComment payload.
// @Tags Comment Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "Task ID (UUID or Key)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.CommentAttachmentResponse} "Draft task comment attachments uploaded successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/attachments [post]
func (h *attachmentHandler) UploadCommentAttachmentWithoutComment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errResp := h.service.ResolveTaskID(g.Param("task_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadCommentAttachments(g.Request.Context(), nil, &taskUUID, nil, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// UploadUserStoryCommentAttachmentWithoutComment godoc
// @Summary Upload User Story Comment Attachment (Draft / Before Comment Creation)
// @Description Pre-upload file(s) before creating a comment on a User Story. Returns attachment UUIDs and URLs to pass in the 'attachments' array of the CreateComment payload.
// @Tags Comment Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.CommentAttachmentResponse} "Draft user story comment attachments uploaded successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/comments/attachments [post]
func (h *attachmentHandler) UploadUserStoryCommentAttachmentWithoutComment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	userStoryUUID, errResp := h.service.ResolveUserStoryID(g.Param("user_story_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadCommentAttachments(g.Request.Context(), nil, nil, &userStoryUUID, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// DownloadUserStoryCommentAttachment godoc
// @Summary Download User Story Comment Attachment File
// @Description Stream and download the binary file content of a specific attachment on a User Story Comment.
// @Tags Comment Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {file} file "Attachment File Stream"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/comments/attachments/{attachment_id}/download [get]
func (h *attachmentHandler) DownloadUserStoryCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	userStoryUUID, errResp := h.service.ResolveUserStoryID(g.Param("user_story_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadCommentAttachment(g.Request.Context(), attachmentUUID, userStoryUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	writeAttachmentDownload(g, stream, filename, mimeType, size)
}

// DeleteUserStoryCommentAttachment godoc
// @Summary Delete User Story Comment Attachment
// @Description Permanently delete an attachment associated with a User Story Comment from both storage and database.
// @Tags Comment Attachments
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param user_story_id path string true "User Story ID (UUID)"
// @Param attachment_id path string true "Attachment ID (UUID)"
// @Success 200 {object} response.SuccessResponse "Attachment deleted successfully"
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/{user_story_id}/comments/attachments/{attachment_id} [delete]
func (h *attachmentHandler) DeleteUserStoryCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	userStoryUUID, errResp := h.service.ResolveUserStoryID(g.Param("user_story_id"))
	if errResp != nil {
		g.JSON(errResp.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		})
		return
	}

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteCommentAttachment(g.Request.Context(), attachmentUUID, userStoryUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Attachment deleted successfully",
	})
}

// UploadAttachmentWithoutTask godoc
// @Summary Upload Task Attachment (Draft / Before Task Creation)
// @Description Pre-upload file(s) before creating a Task. Returns attachment UUIDs and URLs that must be passed in the 'attachments' array of the CreateTask payload.
// @Tags Task Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.AttachmentResponse} "Draft task attachments uploaded successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/tasks/attachments [post]
func (h *attachmentHandler) UploadAttachmentWithoutTask(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadAttachments(g.Request.Context(), nil, projectUUID, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

// UploadUserStoryAttachmentWithoutUserStory godoc
// @Summary Upload User Story Attachment (Draft / Before User Story Creation)
// @Description Pre-upload file(s) before creating a User Story. Returns attachment UUIDs and URLs that must be passed in the 'attachments' array of the CreateUserStory payload.
// @Tags User Story Attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param project_id path string true "Project ID (UUID)"
// @Param file formData file true "File to upload (field name 'file' or 'files')"
// @Success 201 {object} response.SuccessResponse{data=[]responsedto.UserStoryAttachmentResponse} "Draft user story attachments uploaded successfully"
// @Failure 400 {object} response.ErrorResponse
// @Failure 413 {object} response.ErrorResponse "Payload Too Large"
// @Failure 415 {object} response.ErrorResponse "Unsupported Media Type"
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{project_id}/user-stories/attachments [post]
func (h *attachmentHandler) UploadUserStoryAttachmentWithoutUserStory(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	projectUUID, errorResponse := utils.StringToUUID(g.Param("project_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert project ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadUserStoryAttachments(g.Request.Context(), nil, projectUUID, userUUID, allHeaders)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Attachments uploaded successfully",
		Data:       res,
	})
}

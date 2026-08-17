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
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

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
// @Summary Upload Task Attachment
// @Description Upload a file associated with a task
// @Tags Attachment
// @Accept multipart/form-data
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param file formData file true "File to upload"
// @Success 201 {object} response.SuccessResponse
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

	res, err := h.service.UploadAttachments(g.Request.Context(), taskUUID, projectUUID, userUUID, allHeaders)
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
// @Summary Retrieve Task Attachments
// @Description Get all attachments associated with a task
// @Tags Attachment
// @Produce json
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Success 200 {object} response.SuccessResponse
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
// @Summary Download Attachment
// @Description Validate membership and download a task attachment
// @Tags Attachment
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param attachment_id path string true "Attachment ID"
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
// @Summary Delete Attachment
// @Description Delete task attachment if authorized
// @Tags Attachment
// @Param project_id path string true "Project ID"
// @Param task_id path string true "Task ID"
// @Param attachment_id path string true "Attachment ID"
// @Success 200 {object} response.SuccessResponse
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
// @Summary Upload Comment Attachment
// @Description Upload a file associated with a comment
// @Tags Comment Attachment
// @Accept multipart/form-data
// @Produce json
// @Param task_id path string true "Task ID"
// @Param comment_id path string true "Comment ID"
// @Param file formData file true "File to upload"
// @Success 201 {object} response.SuccessResponse
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

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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

	res, err := h.service.UploadCommentAttachments(g.Request.Context(), commentUUID, taskUUID, userUUID, allHeaders)
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
// @Summary Retrieve Comment Attachments
// @Description Get all attachments associated with a comment
// @Tags Comment Attachment
// @Produce json
// @Param task_id path string true "Task ID"
// @Param comment_id path string true "Comment ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments [get]
func (h *attachmentHandler) GetCommentAttachments(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
// @Summary Download Comment Attachment
// @Description Validate membership and download a comment attachment
// @Tags Comment Attachment
// @Param task_id path string true "Task ID"
// @Param comment_id path string true "Comment ID"
// @Param attachment_id path string true "Attachment ID"
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

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
// @Summary Delete Comment Attachment
// @Description Delete comment attachment if authorized
// @Tags Comment Attachment
// @Param task_id path string true "Task ID"
// @Param comment_id path string true "Comment ID"
// @Param attachment_id path string true "Attachment ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /task/{task_id}/comments/{comment_id}/attachments/{attachment_id} [delete]
func (h *attachmentHandler) DeleteCommentAttachment(g *gin.Context) {
	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskUUID, errorResponse := utils.StringToUUID(g.Param("task_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
// @Summary Upload User Story Attachment
// @Description Upload a file associated with a user story
// @Tags User Story Attachment
// @Accept multipart/form-data
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Param file formData file true "File to upload"
// @Success 201 {object} response.SuccessResponse
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

	storyUUID, errorResponse := utils.StringToUUID(g.Param("user_story_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert user story ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	allHeaders, apiErr, logMsg := h.parseFiles(g)
	if apiErr != nil {
		writeErrorResponse(g, h.logger, *apiErr, logMsg)
		return
	}

	res, err := h.service.UploadUserStoryAttachments(g.Request.Context(), storyUUID, projectUUID, userUUID, allHeaders)
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
// @Summary Retrieve User Story Attachments
// @Description Get all attachments associated with a user story
// @Tags User Story Attachment
// @Produce json
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Success 200 {object} response.SuccessResponse
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

	storyUUID, errorResponse := utils.StringToUUID(g.Param("user_story_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert user story ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
// @Summary Download User Story Attachment
// @Description Validate membership and download a user story attachment
// @Tags User Story Attachment
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Param attachment_id path string true "Attachment ID"
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
// @Description Delete user story attachment if authorized
// @Tags User Story Attachment
// @Param project_id path string true "Project ID"
// @Param user_story_id path string true "User Story ID"
// @Param attachment_id path string true "Attachment ID"
// @Success 200 {object} response.SuccessResponse
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

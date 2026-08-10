package handlers

import (
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/config"
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
	// Configure limits from environment variables to prevent coupling with service internals.
	maxSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}
	maxFiles := 5
	if v := config.GetEnv("ATTACHMENT_MAX_FILES_COUNT", ""); v != "" {
		var parsed int
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxFiles = parsed
		}
	}

	maxRequestSize := (maxSizeMB * 1024 * 1024 * int64(maxFiles)) + (10 * 1024 * 1024)
	g.Request.Body = http.MaxBytesReader(g.Writer, g.Request.Body, maxRequestSize)

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

	form, formErr := g.MultipartForm()
	if formErr != nil {
		h.logger.Error("Failed to parse multipart form", zap.Error(formErr))
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Failed to parse multipart form",
		}, "Failed to parse multipart form")
		return
	}

	var allHeaders []*multipart.FileHeader
	for key, headers := range form.File {
		if key == "file" || key == "files" {
			allHeaders = append(allHeaders, headers...)
		}
	}

	if len(allHeaders) == 0 {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Missing file(s) in request payload (use form-data keys 'file' or 'files')",
		}, "Missing file parameter")
		return
	}

	if len(allHeaders) > maxFiles {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("Maximum of %d files can be uploaded per request.", maxFiles),
		}, "Too many files in request")
		return
	}

	for _, header := range allHeaders {
		if header.Size > maxSizeMB*1024*1024 {
			writeErrorResponse(g, h.logger, response.Error{
				Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
				StatusCode: http.StatusRequestEntityTooLarge,
				Message:    fmt.Sprintf("File %s exceeds the maximum allowed size of %d MB.", header.Filename, maxSizeMB),
			}, "File size limit exceeded")
			return
		}
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
	maxSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}
	maxFiles := 5
	if v := config.GetEnv("ATTACHMENT_MAX_FILES_COUNT", ""); v != "" {
		var parsed int
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxFiles = parsed
		}
	}

	maxRequestSize := (maxSizeMB * 1024 * 1024 * int64(maxFiles)) + (10 * 1024 * 1024)
	g.Request.Body = http.MaxBytesReader(g.Writer, g.Request.Body, maxRequestSize)

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

	form, formErr := g.MultipartForm()
	if formErr != nil {
		h.logger.Error("Failed to parse multipart form", zap.Error(formErr))
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Failed to parse multipart form",
		}, "Failed to parse multipart form")
		return
	}

	var allHeaders []*multipart.FileHeader
	for key, headers := range form.File {
		if key == "file" || key == "files" {
			allHeaders = append(allHeaders, headers...)
		}
	}

	if len(allHeaders) == 0 {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    "Missing file(s) in request payload (use form-data keys 'file' or 'files')",
		}, "Missing file parameter")
		return
	}

	if len(allHeaders) > maxFiles {
		writeErrorResponse(g, h.logger, response.Error{
			Code:       response.ErrBadRequest,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("Maximum of %d files can be uploaded per request.", maxFiles),
		}, "Too many files in request")
		return
	}

	for _, header := range allHeaders {
		if header.Size > maxSizeMB*1024*1024 {
			writeErrorResponse(g, h.logger, response.Error{
				Code:       response.ErrorCode("PAYLOAD_TOO_LARGE"),
				StatusCode: http.StatusRequestEntityTooLarge,
				Message:    fmt.Sprintf("File %s exceeds the maximum allowed size of %d MB.", header.Filename, maxSizeMB),
			}, "File size limit exceeded")
			return
		}
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

package handlers

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

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
	// Enforce request-level limits at the HTTP layer.
	maxSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}
	maxRequestSize := (maxSizeMB * 1024 * 1024 * 5) + (10 * 1024 * 1024)
	g.Request.Body = http.MaxBytesReader(g.Writer, g.Request.Body, maxRequestSize)

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	taskIDParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the task ID string into UUID")
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

	res, err := h.service.UploadAttachments(g.Request.Context(), taskUUID, userUUID, allHeaders)
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

	taskIDParam := g.Param("task_id")
	taskUUID, errorResponse := utils.StringToUUID(taskIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the task ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachments, err := h.service.GetAttachments(g.Request.Context(), taskUUID, userUUID)
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

	attachmentIDParam := g.Param("attachment_id")
	attachmentUUID, errorResponse := utils.StringToUUID(attachmentIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadAttachment(g.Request.Context(), attachmentUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}
	defer stream.Close()

	g.Header("Content-Disposition", "attachment; filename="+filename)
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

	attachmentIDParam := g.Param("attachment_id")
	attachmentUUID, errorResponse := utils.StringToUUID(attachmentIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteAttachment(g.Request.Context(), attachmentUUID, userUUID)
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
	// Enforce request-level limits at the HTTP layer.
	maxSizeMB := int64(10)
	if v := config.GetEnv("ATTACHMENT_MAX_FILE_SIZE_MB", ""); v != "" {
		var parsed int64
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed > 0 {
			maxSizeMB = parsed
		}
	}
	maxRequestSize := (maxSizeMB * 1024 * 1024 * 5) + (10 * 1024 * 1024)
	g.Request.Body = http.MaxBytesReader(g.Writer, g.Request.Body, maxRequestSize)

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	commentIDParam := g.Param("comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the comment ID string into UUID")
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

	res, err := h.service.UploadCommentAttachments(g.Request.Context(), commentUUID, userUUID, allHeaders)
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

	commentIDParam := g.Param("comment_id")
	commentUUID, errorResponse := utils.StringToUUID(commentIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the comment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	attachments, err := h.service.GetCommentAttachments(g.Request.Context(), commentUUID, userUUID)
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

	attachmentIDParam := g.Param("attachment_id")
	attachmentUUID, errorResponse := utils.StringToUUID(attachmentIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	stream, filename, mimeType, size, err := h.service.DownloadCommentAttachment(g.Request.Context(), attachmentUUID, userUUID)
	if err != nil {
		writeErrorResponse(g, h.logger, *err, err.Message)
		return
	}
	defer stream.Close()

	g.Header("Content-Disposition", "attachment; filename="+filename)
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

	attachmentUUID, errorResponse := utils.StringToUUID(g.Param("attachment_id"))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the attachment ID string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.DeleteCommentAttachment(g.Request.Context(), attachmentUUID, userUUID)
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

package handlers_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type mockAttachmentService struct {
	services.AttachmentService
	calledMethod string
	passedParams map[string]interface{}
	errResponse  *response.Error
}

func (m *mockAttachmentService) GetConfig() models.AttachmentConfig {
	return models.AttachmentConfig{MaxFileSizeMB: 10, MaxFiles: 5}
}

func (m *mockAttachmentService) UploadAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
	m.calledMethod = "UploadAttachments"
	m.passedParams = map[string]interface{}{
		"taskID":    taskID,
		"projectID": projectID,
		"userID":    userID,
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) GetAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error) {
	m.calledMethod = "GetAttachments"
	m.passedParams = map[string]interface{}{
		"taskID":    taskID,
		"projectID": projectID,
		"userID":    userID,
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) DownloadAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	m.calledMethod = "DownloadAttachment"
	m.passedParams = map[string]interface{}{
		"attachmentID": attachmentID,
		"projectID":    projectID,
		"userID":       userID,
	}
	if m.errResponse != nil {
		return nil, "", "", 0, m.errResponse
	}
	return io.NopCloser(strings.NewReader("")), "test.png", "image/png", 0, nil
}

func (m *mockAttachmentService) DeleteAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error {
	m.calledMethod = "DeleteAttachment"
	m.passedParams = map[string]interface{}{
		"attachmentID": attachmentID,
		"projectID":    projectID,
		"userID":       userID,
	}
	return m.errResponse
}

func (m *mockAttachmentService) UploadCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	m.calledMethod = "UploadCommentAttachments"
	m.passedParams = map[string]interface{}{
		"commentID": commentID,
		"taskID":    taskID,
		"userID":    userID,
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) GetCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	m.calledMethod = "GetCommentAttachments"
	m.passedParams = map[string]interface{}{
		"commentID": commentID,
		"taskID":    taskID,
		"userID":    userID,
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) DownloadCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	m.calledMethod = "DownloadCommentAttachment"
	m.passedParams = map[string]interface{}{
		"attachmentID": attachmentID,
		"taskID":       taskID,
		"userID":       userID,
	}
	if m.errResponse != nil {
		return nil, "", "", 0, m.errResponse
	}
	return io.NopCloser(strings.NewReader("")), "test.png", "image/png", 0, nil
}

func (m *mockAttachmentService) DeleteCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) *response.Error {
	m.calledMethod = "DeleteCommentAttachment"
	m.passedParams = map[string]interface{}{
		"attachmentID": attachmentID,
		"taskID":       taskID,
		"userID":       userID,
	}
	return m.errResponse
}

func setupTestContext(service services.AttachmentService) (*gin.Engine, *mockAttachmentService) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	
	mockSvc := service.(*mockAttachmentService)
	h := handlers.InitAttachmentHandler(mockSvc, zap.NewNop())
	
	// Inject fake user_id middleware mock
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "00000000-0000-0000-0000-000000000001")
		c.Next()
	})

	r.POST("/projects/:project_id/tasks/:task_id/attachments", h.UploadAttachment)
	r.GET("/projects/:project_id/tasks/:task_id/attachments", h.GetAttachments)
	r.GET("/projects/:project_id/tasks/:task_id/attachments/:attachment_id/download", h.DownloadAttachment)
	r.DELETE("/projects/:project_id/tasks/:task_id/attachments/:attachment_id", h.DeleteAttachment)

	r.POST("/task/:task_id/comments/:comment_id/attachments", h.UploadCommentAttachment)
	r.GET("/task/:task_id/comments/:comment_id/attachments", h.GetCommentAttachments)
	r.GET("/task/:task_id/comments/:comment_id/attachments/:attachment_id/download", h.DownloadCommentAttachment)
	r.DELETE("/task/:task_id/comments/:comment_id/attachments/:attachment_id", h.DeleteCommentAttachment)

	return r, mockSvc
}

func TestAttachmentHandler_ParameterPropagation(t *testing.T) {
	projectID := uuid.Must(uuid.NewV7())
	taskID := uuid.Must(uuid.NewV7())
	attachmentID := uuid.Must(uuid.NewV7())
	commentID := uuid.Must(uuid.NewV7())
	expectedUserID := uuid.Must(uuid.FromString("00000000-0000-0000-0000-000000000001"))

	t.Run("Upload Task Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.png")
		_, _ = part.Write([]byte("fake png"))
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/projects/"+projectID.String()+"/tasks/"+taskID.String()+"/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Fatalf("expected StatusCreated, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "UploadAttachments" {
			t.Fatalf("expected UploadAttachments to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["projectID"].(uuid.UUID) != projectID || mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to UploadAttachments: %v", mockSvc.passedParams)
		}
	})

	t.Run("Get Task Attachments Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/tasks/"+taskID.String()+"/attachments", nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "GetAttachments" {
			t.Fatalf("expected GetAttachments to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["projectID"].(uuid.UUID) != projectID || mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to GetAttachments: %v", mockSvc.passedParams)
		}
	})

	t.Run("Download Task Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/tasks/"+taskID.String()+"/attachments/"+attachmentID.String()+"/download", nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "DownloadAttachment" {
			t.Fatalf("expected DownloadAttachment to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["projectID"].(uuid.UUID) != projectID || mockSvc.passedParams["attachmentID"].(uuid.UUID) != attachmentID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to DownloadAttachment: %v", mockSvc.passedParams)
		}
	})

	t.Run("Delete Task Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("DELETE", "/projects/"+projectID.String()+"/tasks/"+taskID.String()+"/attachments/"+attachmentID.String(), nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "DeleteAttachment" {
			t.Fatalf("expected DeleteAttachment to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["projectID"].(uuid.UUID) != projectID || mockSvc.passedParams["attachmentID"].(uuid.UUID) != attachmentID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to DeleteAttachment: %v", mockSvc.passedParams)
		}
	})

	t.Run("Upload Comment Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.png")
		_, _ = part.Write([]byte("fake png"))
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/task/"+taskID.String()+"/comments/"+commentID.String()+"/attachments", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Fatalf("expected StatusCreated, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "UploadCommentAttachments" {
			t.Fatalf("expected UploadCommentAttachments to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["commentID"].(uuid.UUID) != commentID || mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to UploadCommentAttachments: %v", mockSvc.passedParams)
		}
	})

	t.Run("Get Comment Attachments Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("GET", "/task/"+taskID.String()+"/comments/"+commentID.String()+"/attachments", nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "GetCommentAttachments" {
			t.Fatalf("expected GetCommentAttachments to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["commentID"].(uuid.UUID) != commentID || mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to GetCommentAttachments: %v", mockSvc.passedParams)
		}
	})

	t.Run("Download Comment Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("GET", "/task/"+taskID.String()+"/comments/"+commentID.String()+"/attachments/"+attachmentID.String()+"/download", nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "DownloadCommentAttachment" {
			t.Fatalf("expected DownloadCommentAttachment to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["attachmentID"].(uuid.UUID) != attachmentID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to DownloadCommentAttachment: %v", mockSvc.passedParams)
		}
	})

	t.Run("Delete Comment Attachment Passes Correct Parameters", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r, _ := setupTestContext(mockSvc)

		req := httptest.NewRequest("DELETE", "/task/"+taskID.String()+"/comments/"+commentID.String()+"/attachments/"+attachmentID.String(), nil)
		resp := httptest.NewRecorder()

		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}
		if mockSvc.calledMethod != "DeleteCommentAttachment" {
			t.Fatalf("expected DeleteCommentAttachment to be called, got %q", mockSvc.calledMethod)
		}
		if mockSvc.passedParams["taskID"].(uuid.UUID) != taskID || mockSvc.passedParams["attachmentID"].(uuid.UUID) != attachmentID || mockSvc.passedParams["userID"].(uuid.UUID) != expectedUserID {
			t.Errorf("unexpected parameters passed to DeleteCommentAttachment: %v", mockSvc.passedParams)
		}
	})
}

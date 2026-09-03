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
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	handlers "github.com/ms-kanban-server/internal/handlers/http"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type mockAttachmentService struct {
	services.AttachmentService
	errResponse          *response.Error
	uploadAttachmentsFn  func(ctx context.Context, taskID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error)
	getAttachmentsFn     func(ctx context.Context, taskID, projectID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error)
	downloadAttachmentFn func(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	deleteAttachmentFn   func(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error

	uploadCommentFn   func(ctx context.Context, commentID, taskID, userStoryID *uuid.UUID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error)
	getCommentFn      func(ctx context.Context, commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error)
	downloadCommentFn func(ctx context.Context, attachmentID, taskID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error)
	deleteCommentFn   func(ctx context.Context, attachmentID, taskID, userID uuid.UUID) *response.Error
}

func (m *mockAttachmentService) GetConfig() models.AttachmentConfig {
	return models.AttachmentConfig{MaxFileSizeMB: 10, MaxFiles: 5}
}

func (m *mockAttachmentService) UploadAttachments(ctx context.Context, taskID *uuid.UUID, projectID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.AttachmentResponse, *response.Error) {
	if m.uploadAttachmentsFn != nil {
		var resolvedTaskID uuid.UUID
		if taskID != nil {
			resolvedTaskID = *taskID
		}
		return m.uploadAttachmentsFn(ctx, resolvedTaskID, projectID, userID, files)
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return []responsedto.AttachmentResponse{{ID: uuid.Must(uuid.NewV7()), OriginalFilename: "test.png"}}, nil
}

func (m *mockAttachmentService) GetAttachments(ctx context.Context, taskID, projectID, userID uuid.UUID) ([]responsedto.AttachmentResponse, *response.Error) {
	if m.getAttachmentsFn != nil {
		return m.getAttachmentsFn(ctx, taskID, projectID, userID)
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) DownloadAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	if m.downloadAttachmentFn != nil {
		return m.downloadAttachmentFn(ctx, attachmentID, projectID, userID)
	}
	if m.errResponse != nil {
		return nil, "", "", 0, m.errResponse
	}
	return io.NopCloser(strings.NewReader("fake content")), "test.png", "image/png", 12, nil
}

func (m *mockAttachmentService) DeleteAttachment(ctx context.Context, attachmentID, projectID, userID uuid.UUID) *response.Error {
	if m.deleteAttachmentFn != nil {
		return m.deleteAttachmentFn(ctx, attachmentID, projectID, userID)
	}
	return m.errResponse
}

func (m *mockAttachmentService) UploadCommentAttachments(ctx context.Context, commentID, taskID, userStoryID *uuid.UUID, userID uuid.UUID, files []*multipart.FileHeader) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	if m.uploadCommentFn != nil {
		return m.uploadCommentFn(ctx, commentID, taskID, userStoryID, userID, files)
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return []responsedto.CommentAttachmentResponse{{ID: uuid.Must(uuid.NewV7()), OriginalFilename: "test.png"}}, nil
}

func (m *mockAttachmentService) GetCommentAttachments(ctx context.Context, commentID, taskID, userID uuid.UUID) ([]responsedto.CommentAttachmentResponse, *response.Error) {
	if m.getCommentFn != nil {
		return m.getCommentFn(ctx, commentID, taskID, userID)
	}
	if m.errResponse != nil {
		return nil, m.errResponse
	}
	return nil, nil
}

func (m *mockAttachmentService) DownloadCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
	if m.downloadCommentFn != nil {
		return m.downloadCommentFn(ctx, attachmentID, taskID, userID)
	}
	if m.errResponse != nil {
		return nil, "", "", 0, m.errResponse
	}
	return io.NopCloser(strings.NewReader("fake content")), "test.png", "image/png", 12, nil
}

func (m *mockAttachmentService) DeleteCommentAttachment(ctx context.Context, attachmentID, taskID, userID uuid.UUID) *response.Error {
	if m.deleteCommentFn != nil {
		return m.deleteCommentFn(ctx, attachmentID, taskID, userID)
	}
	return m.errResponse
}

func (m *mockAttachmentService) ResolveTaskID(taskIDOrKey string) (uuid.UUID, *response.Error) {
	if m.errResponse != nil {
		return uuid.Nil, m.errResponse
	}
	id, err := uuid.FromString(taskIDOrKey)
	if err == nil {
		return id, nil
	}
	return uuid.Must(uuid.NewV4()), nil
}

func (m *mockAttachmentService) ResolveUserStoryID(userStoryIDOrKey string) (uuid.UUID, *response.Error) {
	if m.errResponse != nil {
		return uuid.Nil, m.errResponse
	}
	id, err := uuid.FromString(userStoryIDOrKey)
	if err == nil {
		return id, nil
	}
	return uuid.Must(uuid.NewV4()), nil
}

func setupTestContext(mockSvc *mockAttachmentService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := handlers.InitAttachmentHandler(mockSvc, zap.NewNop())

	// Inject fake user_id middleware mock
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.Must(uuid.NewV7()).String())
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

	return r
}

func createMultipartRequest(method, url string, files []struct{ name, filename, content string }) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, f := range files {
		part, err := writer.CreateFormFile(f.name, f.filename)
		if err != nil {
			return nil, err
		}
		if _, err = part.Write([]byte(f.content)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestAttachmentHandler_Upload(t *testing.T) {
	projectID := uuid.Must(uuid.NewV7()).String()
	taskID := uuid.Must(uuid.NewV7()).String()
	commentID := uuid.Must(uuid.NewV7()).String()

	t.Run("Valid upload", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/projects/"+projectID+"/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"file", "test.png", "fake-png-content"},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Errorf("expected StatusCreated, got %d", resp.Code)
		}
	})

	t.Run("Invalid UUID", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/projects/invalid-uuid/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"file", "test.png", "fake"},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected StatusBadRequest for invalid UUID, got %d", resp.Code)
		}
	})

	t.Run("Missing file", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/projects/"+projectID+"/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"wrong-key", "test.png", "fake"},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected StatusBadRequest for missing file, got %d", resp.Code)
		}
	})

	t.Run("File too large", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		// 11MB file (exceeds 10MB config)
		largeContent := make([]byte, 11*1024*1024)
		req, _ := createMultipartRequest("POST", "/projects/"+projectID+"/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"file", "test.png", string(largeContent)},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected StatusRequestEntityTooLarge, got %d", resp.Code)
		}
	})

	t.Run("Too many files", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/projects/"+projectID+"/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"files", "1.png", "a"},
			{"files", "2.png", "b"},
			{"files", "3.png", "c"},
			{"files", "4.png", "d"},
			{"files", "5.png", "e"},
			{"files", "6.png", "f"}, // Exceeds MaxFiles = 5
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected StatusBadRequest for too many files, got %d", resp.Code)
		}
	})

	t.Run("Service error mapping", func(t *testing.T) {
		mockSvc := &mockAttachmentService{
			errResponse: &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Access Denied",
			},
		}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/projects/"+projectID+"/tasks/"+taskID+"/attachments", []struct{ name, filename, content string }{
			{"file", "test.png", "fake"},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Errorf("expected StatusForbidden, got %d", resp.Code)
		}
	})

	t.Run("Valid comment attachment upload", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := createMultipartRequest("POST", "/task/"+taskID+"/comments/"+commentID+"/attachments", []struct{ name, filename, content string }{
			{"file", "test.png", "fake"},
		})
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusCreated {
			t.Errorf("expected StatusCreated, got %d", resp.Code)
		}
	})
}

func TestAttachmentHandler_Download(t *testing.T) {
	projectID := uuid.Must(uuid.NewV7()).String()
	taskID := uuid.Must(uuid.NewV7()).String()
	attachmentID := uuid.Must(uuid.NewV7()).String()

	t.Run("Successful download and header handling", func(t *testing.T) {
		mockSvc := &mockAttachmentService{}
		r := setupTestContext(mockSvc)

		req, _ := http.NewRequest("GET", "/projects/"+projectID+"/tasks/"+taskID+"/attachments/"+attachmentID+"/download", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected StatusOK, got %d", resp.Code)
		}

		if ctype := resp.Header().Get("Content-Type"); ctype != "image/png" {
			t.Errorf("expected Content-Type image/png, got %q", ctype)
		}

		if length := resp.Header().Get("Content-Length"); length != "12" {
			t.Errorf("expected Content-Length 12, got %q", length)
		}

		expectedDisp := `attachment; filename=test.png`
		if disp := resp.Header().Get("Content-Disposition"); disp != expectedDisp {
			t.Errorf("expected Content-Disposition %q, got %q", expectedDisp, disp)
		}

		if body := resp.Body.String(); body != "fake content" {
			t.Errorf("expected body 'fake content', got %q", body)
		}
	})

	t.Run("Download CR/LF injection protection", func(t *testing.T) {
		mockSvc := &mockAttachmentService{
			downloadAttachmentFn: func(ctx context.Context, attachmentID, projectID, userID uuid.UUID) (io.ReadCloser, string, string, int64, *response.Error) {
				return io.NopCloser(strings.NewReader("fake")), "injected\r\nname.png", "image/png", 4, nil
			},
		}
		r := setupTestContext(mockSvc)

		req, _ := http.NewRequest("GET", "/projects/"+projectID+"/tasks/"+taskID+"/attachments/"+attachmentID+"/download", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		expectedDisp := `attachment; filename=injectedname.png`
		if disp := resp.Header().Get("Content-Disposition"); disp != expectedDisp {
			t.Errorf("expected Content-Disposition %q, got %q", expectedDisp, disp)
		}
	})

	t.Run("Download service rejection", func(t *testing.T) {
		mockSvc := &mockAttachmentService{
			errResponse: &response.Error{
				Code:       response.ErrForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "Access Denied",
			},
		}
		r := setupTestContext(mockSvc)

		req, _ := http.NewRequest("GET", "/projects/"+projectID+"/tasks/"+taskID+"/attachments/"+attachmentID+"/download", nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Errorf("expected StatusForbidden, got %d", resp.Code)
		}
	})
}

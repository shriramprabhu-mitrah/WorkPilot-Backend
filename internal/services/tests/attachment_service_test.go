package services_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

// createTestMultipartFileHeader builds a fully-functional *multipart.FileHeader for unit tests.
func createTestMultipartFileHeader(filename string, content []byte) (*multipart.FileHeader, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	_, err = part.Write(content)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "/", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	err = req.ParseMultipartForm(10 << 20)
	if err != nil {
		return nil, err
	}

	return req.MultipartForm.File["file"][0], nil
}

// Define mock storage client
type mockStorageClient struct {
	uploadErr     *response.Error
	getObjectErr  *response.Error
	uploadFunc    func(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, string, *response.Error)
	getObjectFunc func(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error)
	deleteFunc    func(ctx context.Context, key string) error
	deletedKeys   []string
}

func (m *mockStorageClient) UploadLogo(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return "", "", nil
}

func (m *mockStorageClient) UploadAvatar(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return "", "", nil
}

func (m *mockStorageClient) DeleteObject(ctx context.Context, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

func (m *mockStorageClient) UploadAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, string, *response.Error) {
	if m.uploadErr != nil {
		return "", "", "", "", m.uploadErr
	}
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, file, header, taskID, cfg)
	}
	return "http://localhost/tasks/taskid/file.png", "tasks/taskid/file.png", "file.png", "image/png", nil
}

func (m *mockStorageClient) UploadCommentAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, string, *response.Error) {
	if m.uploadErr != nil {
		return "", "", "", "", m.uploadErr
	}
	return "http://localhost/comments/commentid/file.png", "comments/commentid/file.png", "file.png", "image/png", nil
}

func (m *mockStorageClient) UploadUserStoryAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, userStoryID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, string, *response.Error) {
	if m.uploadErr != nil {
		return "", "", "", "", m.uploadErr
	}
	return "http://localhost/user_stories/storyid/file.png", "user_stories/storyid/file.png", "file.png", "image/png", nil
}

func (m *mockStorageClient) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error) {
	if m.getObjectErr != nil {
		return nil, 0, m.getObjectErr
	}
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, key)
	}
	return io.NopCloser(bytes.NewReader([]byte("file content"))), 12, nil
}

// Define stub repositories
type stubAttachmentRepo struct {
	attachments  map[uuid.UUID]*models.TaskAttachment
	orphanedLogs []string
	createErr    *response.Error
	getErr       *response.Error
	deleteErr    *response.Error
}

func (s *stubAttachmentRepo) CreateAttachment(a *models.TaskAttachment) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.attachments == nil {
		s.attachments = make(map[uuid.UUID]*models.TaskAttachment)
	}
	if a.ID == uuid.Nil {
		a.ID, _ = uuid.NewV7()
	}
	s.attachments[a.ID] = a
	return nil
}

func (s *stubAttachmentRepo) GetAttachmentByID(id uuid.UUID) (*models.TaskAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	a, ok := s.attachments[id]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Attachment not found"}
	}
	return a, nil
}

func (s *stubAttachmentRepo) GetAttachmentsByTaskID(taskID uuid.UUID) ([]models.TaskAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	var list []models.TaskAttachment
	for _, a := range s.attachments {
		if a.TaskID == taskID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (s *stubAttachmentRepo) DeleteAttachment(id uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, id)
	return nil
}

func (s *stubAttachmentRepo) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, attachmentID)
	s.orphanedLogs = append(s.orphanedLogs, storagePath)
	return nil
}

type stubCommentAttachmentRepo struct {
	attachments  map[uuid.UUID]*models.CommentAttachment
	orphanedLogs []string
	createErr    *response.Error
	getErr       *response.Error
	deleteErr    *response.Error
}

func (s *stubCommentAttachmentRepo) CreateAttachment(a *models.CommentAttachment) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.attachments == nil {
		s.attachments = make(map[uuid.UUID]*models.CommentAttachment)
	}
	if a.ID == uuid.Nil {
		a.ID, _ = uuid.NewV7()
	}
	s.attachments[a.ID] = a
	return nil
}

func (s *stubCommentAttachmentRepo) GetAttachmentByID(id uuid.UUID) (*models.CommentAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	a, ok := s.attachments[id]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Attachment not found"}
	}
	return a, nil
}

func (s *stubCommentAttachmentRepo) GetAttachmentsByCommentID(commentID uuid.UUID) ([]models.CommentAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	var list []models.CommentAttachment
	for _, a := range s.attachments {
		if a.CommentID == commentID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (s *stubCommentAttachmentRepo) DeleteAttachment(id uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, id)
	return nil
}

func (s *stubCommentAttachmentRepo) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, attachmentID)
	s.orphanedLogs = append(s.orphanedLogs, storagePath)
	return nil
}

type stubFileCleanupRepo struct {
	orphanedLogs  []string
	createErr     *response.Error
	orphanedFiles map[uuid.UUID]*models.OrphanedFile
}

func (s *stubFileCleanupRepo) CreateOrphanedFile(ctx context.Context, file *models.OrphanedFile) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.orphanedFiles == nil {
		s.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
	}
	if file.ID == uuid.Nil {
		file.ID, _ = uuid.NewV7()
	}
	s.orphanedFiles[file.ID] = file
	s.orphanedLogs = append(s.orphanedLogs, file.StoragePath)
	return nil
}

func (s *stubFileCleanupRepo) ClaimOrphanedFiles(ctx context.Context, now time.Time, claimTTL time.Duration, limit int) ([]models.OrphanedFile, *response.Error) {
	return nil, nil
}

func (s *stubFileCleanupRepo) ReleaseOrphanedFile(ctx context.Context, id uuid.UUID, lastErr string, lastAttempt time.Time, nextAttempt time.Time) *response.Error {
	return nil
}

func (s *stubFileCleanupRepo) DeleteOrphanedFile(ctx context.Context, id uuid.UUID) *response.Error {
	return nil
}

type stubAttachmentTaskRepo struct {
	task *models.Task
	err  *response.Error
}

func (s *stubAttachmentTaskRepo) CreateTask(task *models.Task) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) UpdateTask(taskID uuid.UUID, updates map[string]interface{}) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubAttachmentTaskRepo) GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error) { return 0, nil }
func (s *stubAttachmentTaskRepo) IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error) { return true, nil }
func (s *stubAttachmentTaskRepo) IsUserStoryInProject(userStoryID, projectID uuid.UUID) (bool, *response.Error) { return true, nil }
func (s *stubAttachmentTaskRepo) VerifyLabelIDs(projectID uuid.UUID, labelIDs []uuid.UUID) ([]models.Label, *response.Error) { return nil, nil }
func (s *stubAttachmentTaskRepo) UpdateTaskLabels(taskID uuid.UUID, labels []models.Label) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) UpdateTaskWithLabels(taskID uuid.UUID, updates map[string]interface{}, labels []models.Label) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) AttachLabel(taskID uuid.UUID, label *models.Label) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) RemoveLabel(taskID uuid.UUID, label *models.Label) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) GetSprintStatus(sprintID uuid.UUID) (string, *response.Error) { return "", nil }
func (s *stubAttachmentTaskRepo) CountTasksByStatus(projectID uuid.UUID, status string) (int64, *response.Error) { return 0, nil }
func (s *stubAttachmentTaskRepo) UpdateTaskStatusName(projectID uuid.UUID, oldStatus, newStatus string) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) GetTaskCountsByProjectIDs(projectIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error) {
	counts := make(map[uuid.UUID]int64)
	if s.task != nil {
		counts[s.task.ProjectID] = 1
	}
	return counts, nil
}
func (s *stubAttachmentTaskRepo) GetTaskDetailsByID(id uuid.UUID) (*models.Task, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.task, nil
}
func (s *stubAttachmentTaskRepo) GetTaskByIDUnscoped(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	return s.task, s.err
}
func (s *stubAttachmentTaskRepo) GetTaskByID(id uuid.UUID, projectID uuid.UUID) (*models.Task, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.task != nil && s.task.ID == id && s.task.ProjectID == projectID {
		return s.task, nil
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
}
func (s *stubAttachmentTaskRepo) GetTaskAccessContext(id uuid.UUID) (*models.TaskAccessContext, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.task != nil {
		return &models.TaskAccessContext{
			TaskID:         s.task.ID,
			ProjectID:      s.task.ProjectID,
			OrganizationID: s.task.Project.OrganizationID,
			TaskKey:        s.task.Key,
		}, nil
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
}

func (s *stubAttachmentTaskRepo) GetTasksByUserStoryID(userStoryID uuid.UUID) ([]models.Task, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	return nil, nil
}

type stubAttachmentProjectRepo struct {
	project     models.Project
	isMember    bool
	member      *models.ProjectMember
	getProjErr  *response.Error
	getMemErr   *response.Error
	activityErr *response.Error
}

func (s *stubAttachmentProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error { return nil }
func (s *stubAttachmentProjectRepo) UpdateProject(projectID uuid.UUID, updates map[string]interface{}) *response.Error { return nil }
func (s *stubAttachmentProjectRepo) GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubAttachmentProjectRepo) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	if s.getProjErr != nil {
		return models.Project{}, s.getProjErr
	}
	s.project.ID = id
	return s.project, nil
}
func (s *stubAttachmentProjectRepo) CreateProjectMember(row models.ProjectMember) *response.Error { return nil }
func (s *stubAttachmentProjectRepo) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubAttachmentProjectRepo) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error { return nil }
func (s *stubAttachmentProjectRepo) GetProjectActivity(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubAttachmentProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return s.isMember, nil
}
func (s *stubAttachmentProjectRepo) DeleteProject(projectID, organizationID uuid.UUID) *response.Error { return nil }
func (s *stubAttachmentProjectRepo) GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error) { return nil, nil }
func (s *stubAttachmentProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	if s.getMemErr != nil {
		return nil, s.getMemErr
	}
	if s.member != nil {
		return s.member, nil
	}
	return &models.ProjectMember{UserID: userID, ProjectID: projectID, ProjectRole: "developer"}, nil
}
func (s *stubAttachmentProjectRepo) UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error { return nil }

type stubAttachmentAuditLogRepo struct {
	logs []models.AuditLog
}

func (s *stubAttachmentAuditLogRepo) CreateAuditLog(log models.AuditLog) *response.Error {
	s.logs = append(s.logs, log)
	return nil
}
func (s *stubAttachmentAuditLogRepo) GetAuditLogs(req dto.GetAudit) ([]models.AuditLog, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }

type stubCommentRepo struct {
	comments map[uuid.UUID]*models.Comments
	getErr   *response.Error
}

func (s *stubCommentRepo) CreateComment(comment *models.Comments) *response.Error { return nil }
func (s *stubCommentRepo) GetCommentByID(commentID uuid.UUID) (*models.Comments, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	c, ok := s.comments[commentID]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Comment not found"}
	}
	return c, nil
}
func (s *stubCommentRepo) GetCommentsByTaskID(req dto.GetComments) ([]models.Comments, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubCommentRepo) GetCommentsByUserStoryID(req dto.GetComments) ([]models.Comments, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubCommentRepo) UpdateComment(commentID uuid.UUID, req *models.Comments) *response.Error { return nil }
func (s *stubCommentRepo) DeleteComment(commentID uuid.UUID) *response.Error { return nil }
func (s *stubCommentRepo) GetCommentsByParentID(req dto.GetComments) ([]models.Comments, response.Pagination, *response.Error) { return nil, response.Pagination{}, nil }
func (s *stubCommentRepo) HasReplies(commentID uuid.UUID) (bool, *response.Error)   { return false, nil }
func (s *stubCommentRepo) MarkCommentAsDeleted(commentID uuid.UUID) *response.Error { return nil }

type stubCommentTaskRepo struct {
	stubAttachmentTaskRepo
}

func (s *stubCommentTaskRepo) GetTaskDetailsByID(id uuid.UUID) (*models.Task, *response.Error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.task, nil
}

type testFixture struct {
	service               services.AttachmentService
	attachmentRepo        *stubAttachmentRepo
	commentAttachmentRepo *stubCommentAttachmentRepo
	cleanupRepo           *stubFileCleanupRepo
	commentsRepo          *stubCommentRepo
	taskRepo              *stubAttachmentTaskRepo
	projectRepo           *stubAttachmentProjectRepo
	authRepo              *stubAuthRepository
	auditRepo             *stubAttachmentAuditLogRepo
	storageClient         *mockStorageClient
	ctx                   context.Context
}

func newTestFixture(orgID, projectID, taskID, userID uuid.UUID) *testFixture {
	attachmentRepo := &stubAttachmentRepo{attachments: make(map[uuid.UUID]*models.TaskAttachment)}
	commentAttachmentRepo := &stubCommentAttachmentRepo{attachments: make(map[uuid.UUID]*models.CommentAttachment)}
	cleanupRepo := &stubFileCleanupRepo{orphanedFiles: make(map[uuid.UUID]*models.OrphanedFile)}
	commentsRepo := &stubCommentRepo{comments: make(map[uuid.UUID]*models.Comments)}
	
	taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
		ID:        taskID,
		ProjectID: projectID,
		Key:       "TASK-1",
		Project:   models.Project{ID: projectID, OrganizationID: orgID},
	}}
	projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
	authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
	auditRepo := &stubAttachmentAuditLogRepo{}
	storageClient := &mockStorageClient{}

	service := services.InitAttachmentService(
		attachmentRepo,
		commentAttachmentRepo,
		nil, // userStoryAttachmentRepo
		cleanupRepo,
		commentsRepo, 
		taskRepo,
		nil, // userStoryRepo
		projectRepo,
		authRepo,
		auditRepo,
		storageClient,
		zap.NewNop(),
		nil,
	)

	return &testFixture{
		service:               service,
		attachmentRepo:        attachmentRepo,
		commentAttachmentRepo: commentAttachmentRepo,
		cleanupRepo:           cleanupRepo,
		commentsRepo:          commentsRepo,
		taskRepo:              taskRepo,
		projectRepo:           projectRepo,
		authRepo:              authRepo,
		auditRepo:             auditRepo,
		storageClient:         storageClient,
		ctx:                   context.Background(),
	}
}

func TestAttachmentService_UploadAttachment(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	t.Run("Success Upload", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)

		fileData := []byte("image content")
		fileHeader, err := createTestMultipartFileHeader("test.png", fileData)
		if err != nil {
			t.Fatalf("failed to create mock file header: %v", err)
		}

		res, apiErr := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr != nil {
			t.Fatalf("expected nil error, got %v", apiErr)
		}

		if len(res) != 1 || res[0].OriginalFilename != "test.png" {
			t.Errorf("expected 1 attachment with original filename test.png, got %v", res)
		}
		if len(f.attachmentRepo.attachments) != 1 {
			t.Errorf("expected 1 attachment in DB, got %d", len(f.attachmentRepo.attachments))
		}
		if len(f.auditRepo.logs) != 1 || f.auditRepo.logs[0].Action != "attachment_uploaded" {
			t.Errorf("expected attachment_uploaded audit log, got logs: %v", f.auditRepo.logs)
		}
	})

	t.Run("Storage Upload S3 Failure", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.storageClient.uploadErr = &response.Error{StatusCode: 500, Message: "S3 Error"}

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, apiErr := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr == nil || apiErr.StatusCode != 500 {
			t.Fatalf("expected S3 500 failure, got %v", apiErr)
		}
		if len(f.attachmentRepo.attachments) != 0 {
			t.Errorf("expected 0 DB attachments, got %d", len(f.attachmentRepo.attachments))
		}
	})

	t.Run("DB Failure After S3 Upload Rollback", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.createErr = &response.Error{StatusCode: 500, Message: "DB error"}

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, apiErr := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr == nil {
			t.Fatal("expected error, got nil")
		}
		if len(f.storageClient.deletedKeys) != 0 {
			t.Errorf("expected no immediate S3 rollback deletion call, got: %v", f.storageClient.deletedKeys)
		}
		if len(f.cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 orphaned file record in outbox, got %d", len(f.cleanupRepo.orphanedLogs))
		}
	})
}

func TestAttachmentService_DownloadAttachment(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	attachmentID := uuid.Must(uuid.NewV4())

	t.Run("Success Download", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{
			ID:               attachmentID,
			TaskID:           taskID,
			OriginalFilename: "doc.pdf",
			MIMEType:         "application/pdf",
			StoragePath:      "tasks/task/doc.pdf",
		}

		stream, filename, mime, _, err := f.service.DownloadAttachment(f.ctx, attachmentID, projectID, userID)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		defer stream.Close()

		if filename != "doc.pdf" || mime != "application/pdf" {
			t.Errorf("got unexpected details: filename=%s, mime=%s", filename, mime)
		}
	})

	t.Run("Download S3 Failure", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{
			ID:               attachmentID,
			TaskID:           taskID,
			OriginalFilename: "doc.pdf",
			StoragePath:      "path",
		}
		f.storageClient.getObjectErr = &response.Error{StatusCode: 500, Message: "S3 Error"}

		_, _, _, _, err := f.service.DownloadAttachment(f.ctx, attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 500 {
			t.Fatalf("expected 500 error, got %v", err)
		}
	})
}

func TestAttachmentService_DeleteAttachment(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	attachmentID := uuid.Must(uuid.NewV4())

	t.Run("Uploader Delete Success", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{
			ID:          attachmentID,
			TaskID:      taskID,
			StoragePath: "path",
			UploadedBy:  userID,
		}

		err := f.service.DeleteAttachment(f.ctx, attachmentID, projectID, userID)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(f.attachmentRepo.attachments) != 0 {
			t.Errorf("expected DB deletion, got %d attachments remaining", len(f.attachmentRepo.attachments))
		}
		if len(f.attachmentRepo.orphanedLogs) != 1 || f.attachmentRepo.orphanedLogs[0] != "path" {
			t.Errorf("expected DB transaction to write to orphaned file cleanup log, got: %v", f.attachmentRepo.orphanedLogs)
		}
	})

	t.Run("DB Failure Prevents S3 Delete Record", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{
			ID:          attachmentID,
			TaskID:      taskID,
			StoragePath: "path",
			UploadedBy:  userID,
		}
		f.attachmentRepo.deleteErr = &response.Error{StatusCode: 500, Message: "DB error"}

		err := f.service.DeleteAttachment(f.ctx, attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 500 {
			t.Fatalf("expected 500 error, got %v", err)
		}
		if len(f.attachmentRepo.orphanedLogs) != 0 {
			t.Errorf("expected 0 orphaned logs, got %v", f.attachmentRepo.orphanedLogs)
		}
	})
}

func TestCommentAttachmentService_UploadAttachments(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	orgID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())
	taskID := uuid.Must(uuid.NewV7())
	commentID := uuid.Must(uuid.NewV7())

	t.Run("Successfully upload comment attachments", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.commentsRepo.comments[commentID] = &models.Comments{
			ID:             commentID,
			TaskID:         &taskID,
			UserID:         userID,
			ProjectID:      projectID,
			OrganizationID: orgID,
		}

		fileHeader, err := createTestMultipartFileHeader("test.png", []byte("fake content"))
		if err != nil {
			t.Fatalf("failed to create test file header: %v", err)
		}

		res, uploadErr := f.service.UploadCommentAttachments(f.ctx, commentID, taskID, userID, []*multipart.FileHeader{fileHeader})
		if uploadErr != nil {
			t.Fatalf("expected no error, got %v", uploadErr)
		}
		if len(res) != 1 {
			t.Errorf("expected 1 uploaded attachment response, got %d", len(res))
		}
	})
}

func TestAttachmentService_AuthorizationMatrix(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	anotherOrgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	attachmentID := uuid.Must(uuid.NewV4())
	commentID := uuid.Must(uuid.NewV4())

	fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))

	t.Run("Non-member Access Denied", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{ID: attachmentID, TaskID: taskID}
		f.projectRepo.isMember = false

		_, err := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for upload, got %v", err)
		}

		_, _, _, _, err = f.service.DownloadAttachment(f.ctx, attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for download, got %v", err)
		}

		err = f.service.DeleteAttachment(f.ctx, attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for delete, got %v", err)
		}
	})

	t.Run("Super Admin Denied Org-level Activity", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.authRepo.user.Role = string(dto.RoleSuperAdmin)

		_, err := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for super admin, got %v", err)
		}
	})

	t.Run("Org Admin From Another Org Denied", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.projectRepo.isMember = false
		f.authRepo.user.Role = string(dto.RoleOrgAdmin)
		f.authRepo.user.OrganizationID = &anotherOrgID

		_, err := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden, got %v", err)
		}
	})

	t.Run("Comment Author Can Delete Comment Attachment", func(t *testing.T) {
		commentAuthorID := uuid.Must(uuid.NewV4())
		f := newTestFixture(orgID, projectID, taskID, commentAuthorID)
		f.commentAttachmentRepo.attachments[attachmentID] = &models.CommentAttachment{
			ID:          attachmentID,
			CommentID:   commentID,
			StoragePath: "path",
			UploadedBy:  userID,
		}
		f.commentsRepo.comments[commentID] = &models.Comments{
			ID:     commentID,
			TaskID: &taskID,
			UserID: commentAuthorID,
		}

		err := f.service.DeleteCommentAttachment(f.ctx, attachmentID, taskID, commentAuthorID)
		if err != nil {
			t.Fatalf("expected comment author to successfully delete another user's comment attachment, got %v", err)
		}
	})

	t.Run("Mismatched Task Project ID", func(t *testing.T) {
		wrongProjectID := uuid.Must(uuid.NewV4())
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{ID: attachmentID, TaskID: taskID, StoragePath: "path"}

		// 1. Upload
		header, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, err := f.service.UploadAttachments(f.ctx, taskID, wrongProjectID, userID, []*multipart.FileHeader{header})
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 2. Get
		_, err = f.service.GetAttachments(f.ctx, taskID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 3. Download
		_, _, _, _, err = f.service.DownloadAttachment(f.ctx, attachmentID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 4. Delete
		err = f.service.DeleteAttachment(f.ctx, attachmentID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}
	})

	t.Run("Mismatched Comment Task ID", func(t *testing.T) {
		wrongTaskID := uuid.Must(uuid.NewV4())
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.commentAttachmentRepo.attachments[attachmentID] = &models.CommentAttachment{ID: attachmentID, CommentID: commentID, StoragePath: "path"}
		f.commentsRepo.comments[commentID] = &models.Comments{ID: commentID, TaskID: &taskID}

		// 1. Upload
		header, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, err := f.service.UploadCommentAttachments(f.ctx, commentID, wrongTaskID, userID, []*multipart.FileHeader{header})
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment upload, got %v", err)
		}

		// 2. Get
		_, err = f.service.GetCommentAttachments(f.ctx, commentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment get, got %v", err)
		}

		// 3. Download
		_, _, _, _, err = f.service.DownloadCommentAttachment(f.ctx, attachmentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment download, got %v", err)
		}

		// 4. Delete
		err = f.service.DeleteCommentAttachment(f.ctx, attachmentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment delete, got %v", err)
		}
	})

	t.Run("Upload Rollback Trigger and Repository Correctness", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.attachmentRepo.createErr = &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: "DB error"}

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, err := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil {
			t.Fatal("expected upload error, got nil")
		}
		if len(f.cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 orphan in cleanupRepo, got %d", len(f.cleanupRepo.orphanedLogs))
		}
	})

	t.Run("Comment Upload Rollback Repository Correctness", func(t *testing.T) {
		f := newTestFixture(orgID, projectID, taskID, userID)
		f.commentAttachmentRepo.createErr = &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: "DB error"}
		f.commentsRepo.comments[commentID] = &models.Comments{ID: commentID, TaskID: &taskID}

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, err := f.service.UploadCommentAttachments(f.ctx, commentID, taskID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil {
			t.Fatal("expected comment upload error, got nil")
		}
		if len(f.cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 comment orphan in cleanupRepo, got %d", len(f.cleanupRepo.orphanedLogs))
		}
	})

	t.Run("Worker Shutdown context leak test", func(t *testing.T) {
		cleanupRepo := &stubFileCleanupRepo{}
		ctx, cancel := context.WithCancel(context.Background())
		_ = services.InitAttachmentService(nil, nil, nil, cleanupRepo, nil, &stubAttachmentTaskRepo{}, nil, &stubAttachmentProjectRepo{}, &stubAuthRepository{}, nil, &mockStorageClient{}, zap.NewNop(), ctx)
		cancel()
	})
}

func TestAttachmentService_TaskProjectMismatch(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectA := uuid.Must(uuid.NewV4())
	projectB := uuid.Must(uuid.NewV4()) // task lives here
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	attachmentID := uuid.Must(uuid.NewV4())

	f := newTestFixture(orgID, projectB, taskID, userID)
	f.attachmentRepo.attachments[attachmentID] = &models.TaskAttachment{ID: attachmentID, TaskID: taskID, StoragePath: "path"}

	header, _ := createTestMultipartFileHeader("test.png", []byte("content"))

	// Upload — project mismatch must yield ErrBadRequest
	_, err := f.service.UploadAttachments(f.ctx, taskID, projectA, userID, []*multipart.FileHeader{header})
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Upload: expected ErrBadRequest on project mismatch, got %v", err)
	}

	// Get
	_, err = f.service.GetAttachments(f.ctx, taskID, projectA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Get: expected ErrBadRequest on project mismatch, got %v", err)
	}

	// Download
	_, _, _, _, err = f.service.DownloadAttachment(f.ctx, attachmentID, projectA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Download: expected ErrBadRequest on project mismatch, got %v", err)
	}

	// Delete
	err = f.service.DeleteAttachment(f.ctx, attachmentID, projectA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Delete: expected ErrBadRequest on project mismatch, got %v", err)
	}
}

func TestAttachmentService_CommentTaskMismatch(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskA := uuid.Must(uuid.NewV4())
	taskB := uuid.Must(uuid.NewV4()) // comment lives here
	commentID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	attachmentID := uuid.Must(uuid.NewV4())

	f := newTestFixture(orgID, projectID, taskA, userID)
	f.commentAttachmentRepo.attachments[attachmentID] = &models.CommentAttachment{ID: attachmentID, CommentID: commentID, StoragePath: "path"}
	f.commentsRepo.comments[commentID] = &models.Comments{ID: commentID, TaskID: &taskB}

	header, _ := createTestMultipartFileHeader("test.png", []byte("content"))

	// Upload — task mismatch must yield ErrBadRequest
	_, err := f.service.UploadCommentAttachments(f.ctx, commentID, taskA, userID, []*multipart.FileHeader{header})
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Upload: expected ErrBadRequest on comment/task mismatch, got %v", err)
	}

	// Get
	_, err = f.service.GetCommentAttachments(f.ctx, commentID, taskA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Get: expected ErrBadRequest on comment/task mismatch, got %v", err)
	}

	// Download
	_, _, _, _, err = f.service.DownloadCommentAttachment(f.ctx, attachmentID, taskA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Download: expected ErrBadRequest on comment/task mismatch, got %v", err)
	}

	// Delete
	err = f.service.DeleteCommentAttachment(f.ctx, attachmentID, taskA, userID)
	if err == nil || err.Code != response.ErrBadRequest {
		t.Errorf("Delete: expected ErrBadRequest on comment/task mismatch, got %v", err)
	}
}

func TestAttachmentService_NonMemberRejected(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newTestFixture(orgID, projectID, taskID, userID)
	f.projectRepo.isMember = false
	f.projectRepo.getMemErr = &response.Error{Code: response.ErrNotFound, StatusCode: http.StatusNotFound, Message: "not a member"}

	header, _ := createTestMultipartFileHeader("test.png", []byte("content"))

	_, err := f.service.UploadAttachments(f.ctx, taskID, projectID, userID, []*multipart.FileHeader{header})
	if err == nil {
		t.Fatal("expected error for non-member upload, got nil")
	}
	if err.Code != response.ErrForbidden {
		t.Errorf("expected ErrForbidden for non-member, got code=%v statusCode=%d", err.Code, err.StatusCode)
	}
}

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

// mockMultipartFile wraps bytes.Reader to satisfy multipart.File interface
type mockMultipartFile struct {
	*bytes.Reader
}

func (m *mockMultipartFile) Close() error {
	return nil
}

func newMockMultipartFile(data []byte) multipart.File {
	return &mockMultipartFile{
		Reader: bytes.NewReader(data),
	}
}

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
	uploadFunc    func(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error)
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

func (m *mockStorageClient) UploadAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	if m.uploadErr != nil {
		return "", "", "", m.uploadErr
	}
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, file, header, taskID, cfg)
	}
	return "tasks/taskid/file.png", "file.png", "image/png", nil
}

func (m *mockStorageClient) UploadCommentAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	if m.uploadErr != nil {
		return "", "", "", m.uploadErr
	}
	return "comments/commentid/file.png", "file.png", "image/png", nil
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
func (s *stubAttachmentTaskRepo) UpdateTask(task *models.Task) *response.Error { return nil }
func (s *stubAttachmentTaskRepo) DeleteTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) RestoreTask(id uuid.UUID, projectID uuid.UUID) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) GetTasks(projectID uuid.UUID, filter dto.TaskFilter) ([]models.Task, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubAttachmentTaskRepo) GetNextSequenceNumber(projectID uuid.UUID) (int, *response.Error) {
	return 0, nil
}
func (s *stubAttachmentTaskRepo) IsSprintInProject(sprintID, projectID uuid.UUID) (bool, *response.Error) {
	return true, nil
}
func (s *stubAttachmentTaskRepo) VerifyLabelIDs(projectID uuid.UUID, labelIDs []uuid.UUID) ([]models.Label, *response.Error) {
	return nil, nil
}
func (s *stubAttachmentTaskRepo) UpdateTaskLabels(taskID uuid.UUID, labels []models.Label) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) UpdateTaskWithLabels(task *models.Task, labels []models.Label) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) AttachLabel(taskID uuid.UUID, label *models.Label) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) RemoveLabel(taskID uuid.UUID, label *models.Label) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) MoveIncompleteTasksToBacklog(sprintID uuid.UUID) *response.Error {
	return nil
}
func (s *stubAttachmentTaskRepo) GetSprintStatus(sprintID uuid.UUID) (string, *response.Error) {
	return "", nil
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

type stubAttachmentProjectRepo struct {
	project     models.Project
	isMember    bool
	member      *models.ProjectMember
	getProjErr  *response.Error
	getMemErr   *response.Error
	activityErr *response.Error
}

func (s *stubAttachmentProjectRepo) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {
	return nil
}
func (s *stubAttachmentProjectRepo) UpdateProject(projectID uuid.UUID, req models.Project) *response.Error {
	return nil
}
func (s *stubAttachmentProjectRepo) GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubAttachmentProjectRepo) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {
	if s.getProjErr != nil {
		return models.Project{}, s.getProjErr
	}
	s.project.ID = id
	return s.project, nil
}
func (s *stubAttachmentProjectRepo) CreateProjectMember(row models.ProjectMember) *response.Error {
	return nil
}
func (s *stubAttachmentProjectRepo) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubAttachmentProjectRepo) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {
	return nil
}
func (s *stubAttachmentProjectRepo) GetProjectActivity(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubAttachmentProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return s.isMember, nil
}
func (s *stubAttachmentProjectRepo) DeleteProject(projectID, organizationID uuid.UUID) *response.Error {
	return nil
}
func (s *stubAttachmentProjectRepo) GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error) {
	return nil, nil
}
func (s *stubAttachmentProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	if s.getMemErr != nil {
		return nil, s.getMemErr
	}
	if s.member != nil {
		return s.member, nil
	}
	return &models.ProjectMember{UserID: userID, ProjectID: projectID, ProjectRole: "developer"}, nil
}
func (s *stubAttachmentProjectRepo) UpdateProjectMember(projectID, userID uuid.UUID, projectRole string) *response.Error {
	return nil
}

type stubAttachmentAuditLogRepo struct {
	logs []models.AuditLog
}

func (s *stubAttachmentAuditLogRepo) CreateAuditLog(log models.AuditLog) *response.Error {
	s.logs = append(s.logs, log)
	return nil
}
func (s *stubAttachmentAuditLogRepo) GetAuditLogByUserID(userID uuid.UUID, pagination response.PaginationQuery) ([]models.AuditLog, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func TestAttachmentService_UploadAttachment(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	t.Run("Success Upload", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{attachments: make(map[uuid.UUID]*models.TaskAttachment)}
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

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, auditRepo, storageClient, zap.NewNop(), nil)

		fileData := []byte("image content")
		fileHeader, headerErr := createTestMultipartFileHeader("test.png", fileData)
		if headerErr != nil {
			t.Fatalf("failed to create mock file header: %v", headerErr)
		}

		res, apiErr := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr != nil {
			t.Fatalf("expected nil error, got %v", apiErr)
		}

		if len(res) != 1 || res[0].OriginalFilename != "test.png" {
			t.Errorf("expected 1 attachment with original filename test.png, got %v", res)
		}

		if len(attachmentRepo.attachments) != 1 {
			t.Errorf("expected 1 attachment in DB, got %d", len(attachmentRepo.attachments))
		}

		if len(auditRepo.logs) != 1 || auditRepo.logs[0].Action != "attachment_uploaded" {
			t.Errorf("expected attachment_uploaded audit log, got logs: %v", auditRepo.logs)
		}
	})

	t.Run("Storage Upload S3 Failure", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{attachments: make(map[uuid.UUID]*models.TaskAttachment)}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		storageClient := &mockStorageClient{uploadErr: &response.Error{StatusCode: 500, Message: "S3 Error"}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, storageClient, zap.NewNop(), nil)

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, apiErr := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr == nil || apiErr.StatusCode != 500 {
			t.Fatalf("expected S3 500 failure, got %v", apiErr)
		}

		if len(attachmentRepo.attachments) != 0 {
			t.Errorf("expected 0 DB attachments, got %d", len(attachmentRepo.attachments))
		}
	})

	t.Run("DB Failure After S3 Upload Rollback", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{createErr: &response.Error{StatusCode: 500, Message: "DB error"}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		storageClient := &mockStorageClient{}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, storageClient, zap.NewNop(), nil)

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, apiErr := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if apiErr == nil {
			t.Fatal("expected error, got nil")
		}

		if len(storageClient.deletedKeys) != 0 {
			t.Errorf("expected no immediate S3 rollback deletion call, got: %v", storageClient.deletedKeys)
		}

		if len(cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 orphaned file record in outbox, got %d", len(cleanupRepo.orphanedLogs))
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
		attachmentRepo := &stubAttachmentRepo{attachments: map[uuid.UUID]*models.TaskAttachment{
			attachmentID: {
				ID:               attachmentID,
				TaskID:           taskID,
				OriginalFilename: "doc.pdf",
				MIMEType:         "application/pdf",
				StoragePath:      "tasks/task/doc.pdf",
			},
		}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		storageClient := &mockStorageClient{}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, storageClient, zap.NewNop(), nil)

		stream, filename, mime, _, err := service.DownloadAttachment(context.Background(), attachmentID, projectID, userID)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		defer stream.Close()

		if filename != "doc.pdf" || mime != "application/pdf" {
			t.Errorf("got unexpected details: filename=%s, mime=%s", filename, mime)
		}
	})

	t.Run("Download S3 Failure", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{attachments: map[uuid.UUID]*models.TaskAttachment{
			attachmentID: {
				ID:               attachmentID,
				TaskID:           taskID,
				OriginalFilename: "doc.pdf",
				StoragePath:      "path",
			},
		}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		storageClient := &mockStorageClient{getObjectErr: &response.Error{StatusCode: 500, Message: "S3 Error"}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, storageClient, zap.NewNop(), nil)

		_, _, _, _, err := service.DownloadAttachment(context.Background(), attachmentID, projectID, userID)
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
		attachmentRepo := &stubAttachmentRepo{
			attachments: map[uuid.UUID]*models.TaskAttachment{
				attachmentID: {
					ID:          attachmentID,
					TaskID:      taskID,
					StoragePath: "path",
					UploadedBy:  userID,
				},
			},
		}
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

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, auditRepo, storageClient, zap.NewNop(), nil)

		err := service.DeleteAttachment(context.Background(), attachmentID, projectID, userID)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if len(attachmentRepo.attachments) != 0 {
			t.Errorf("expected DB deletion, got %d attachments remaining", len(attachmentRepo.attachments))
		}

		if len(attachmentRepo.orphanedLogs) != 1 || attachmentRepo.orphanedLogs[0] != "path" {
			t.Errorf("expected DB transaction to write to orphaned file cleanup log, got: %v", attachmentRepo.orphanedLogs)
		}
	})

	t.Run("DB Failure Prevents S3 Delete Record", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{
			deleteErr: &response.Error{StatusCode: 500, Message: "DB error"},
			attachments: map[uuid.UUID]*models.TaskAttachment{
				attachmentID: {
					ID:          attachmentID,
					TaskID:      taskID,
					StoragePath: "path",
					UploadedBy:  userID,
				},
			},
		}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		storageClient := &mockStorageClient{}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, storageClient, zap.NewNop(), nil)

		err := service.DeleteAttachment(context.Background(), attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 500 {
			t.Fatalf("expected 500 error, got %v", err)
		}

		// Storage outbox should NOT be written if DB transaction fails
		if len(attachmentRepo.orphanedLogs) != 0 {
			t.Errorf("expected 0 orphaned logs, got %v", attachmentRepo.orphanedLogs)
		}
	})
}

// Stub structures for Comment Attachment tests
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

type stubCommentRepo struct {
	comments map[uuid.UUID]*models.Comments
	getErr   *response.Error
}

func (s *stubCommentRepo) CreateComment(comment models.Comments) *response.Error { return nil }
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
func (s *stubCommentRepo) GetCommentsByTaskID(req dto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
func (s *stubCommentRepo) UpdateComment(commentID uuid.UUID, req models.Comments) *response.Error {
	return nil
}
func (s *stubCommentRepo) DeleteComment(commentID uuid.UUID) *response.Error { return nil }
func (s *stubCommentRepo) GetCommentsByParentID(req dto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}
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

func TestCommentAttachmentService_UploadAttachments(t *testing.T) {
	userID := uuid.Must(uuid.NewV7())
	orgID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())
	taskID := uuid.Must(uuid.NewV7())
	commentID := uuid.Must(uuid.NewV7())

	t.Run("Successfully upload comment attachments", func(t *testing.T) {
		commentAttachmentRepo := &stubCommentAttachmentRepo{}
		commentsRepo := &stubCommentRepo{
			comments: map[uuid.UUID]*models.Comments{
				commentID: {
					ID:             commentID,
					TaskID:         taskID,
					UserID:         userID,
					ProjectID:      projectID,
					OrganizationID: orgID,
				},
			},
		}
		taskRepo := &stubCommentTaskRepo{
			stubAttachmentTaskRepo: stubAttachmentTaskRepo{
				task: &models.Task{
					ID:        taskID,
					ProjectID: projectID,
					Project:   models.Project{ID: projectID, OrganizationID: orgID},
				},
			},
		}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{
			user: models.User{
				ID:             userID,
				Role:           string(dto.RoleMember),
				OrganizationID: &orgID,
			},
		}
		auditRepo := &stubAttachmentAuditLogRepo{}
		storageClient := &mockStorageClient{}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(nil, commentAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, projectRepo, authRepo, auditRepo, storageClient, zap.NewNop(), nil)

		fileHeader, err := createTestMultipartFileHeader("test.png", []byte("fake content"))
		if err != nil {
			t.Fatalf("failed to create test file header: %v", err)
		}

		res, uploadErr := service.UploadCommentAttachments(context.Background(), commentID, taskID, userID, []*multipart.FileHeader{fileHeader})
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
		attachmentRepo := &stubAttachmentRepo{attachments: map[uuid.UUID]*models.TaskAttachment{
			attachmentID: {ID: attachmentID, TaskID: taskID},
		}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: false, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		_, err := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for upload, got %v", err)
		}

		_, _, _, _, err = service.DownloadAttachment(context.Background(), attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for download, got %v", err)
		}

		err = service.DeleteAttachment(context.Background(), attachmentID, projectID, userID)
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for delete, got %v", err)
		}
	})

	t.Run("Super Admin Denied Org-level Activity", func(t *testing.T) {
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: string(dto.RoleSuperAdmin)}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(nil, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		_, err := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden for super admin, got %v", err)
		}
	})

	t.Run("Org Admin From Another Org Denied", func(t *testing.T) {
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: false, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: string(dto.RoleOrgAdmin), OrganizationID: &anotherOrgID}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(nil, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		_, err := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil || err.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden, got %v", err)
		}
	})

	t.Run("Comment Author Can Delete Comment Attachment", func(t *testing.T) {
		commentAuthorID := uuid.Must(uuid.NewV4())
		commentAttachmentRepo := &stubCommentAttachmentRepo{
			attachments: map[uuid.UUID]*models.CommentAttachment{
				attachmentID: {
					ID:          attachmentID,
					CommentID:   commentID,
					StoragePath: "path",
					UploadedBy:  userID,
				},
			},
		}
		commentsRepo := &stubCommentRepo{
			comments: map[uuid.UUID]*models.Comments{
				commentID: {
					ID:     commentID,
					TaskID: taskID,
					UserID: commentAuthorID,
				},
			},
		}
		taskRepo := &stubCommentTaskRepo{
			stubAttachmentTaskRepo: stubAttachmentTaskRepo{
				task: &models.Task{
					ID:        taskID,
					ProjectID: projectID,
					Project:   models.Project{ID: projectID, OrganizationID: orgID},
				},
			},
		}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: commentAuthorID, Role: "member", OrganizationID: &orgID}}

		cleanupRepo := &stubFileCleanupRepo{}
		service := services.InitAttachmentService(nil, commentAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, projectRepo, authRepo, &stubAttachmentAuditLogRepo{}, &mockStorageClient{}, zap.NewNop(), nil)

		err := service.DeleteCommentAttachment(context.Background(), attachmentID, taskID, commentAuthorID)
		if err != nil {
			t.Fatalf("expected comment author to successfully delete another user's comment attachment, got %v", err)
		}
	})

	t.Run("Mismatched Task Project ID", func(t *testing.T) {
		wrongProjectID := uuid.Must(uuid.NewV4())
		attachmentRepo := &stubAttachmentRepo{attachments: map[uuid.UUID]*models.TaskAttachment{
			attachmentID: {ID: attachmentID, TaskID: taskID, StoragePath: "path"},
		}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{ID: projectID, OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		cleanupRepo := &stubFileCleanupRepo{}

		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		// 1. Upload
		header, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, err := service.UploadAttachments(context.Background(), taskID, wrongProjectID, userID, []*multipart.FileHeader{header})
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 2. Get
		_, err = service.GetAttachments(context.Background(), taskID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 3. Download
		_, _, _, _, err = service.DownloadAttachment(context.Background(), attachmentID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}

		// 4. Delete
		err = service.DeleteAttachment(context.Background(), attachmentID, wrongProjectID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest, got %v", err)
		}
	})

	t.Run("Mismatched Comment Task ID", func(t *testing.T) {
		wrongTaskID := uuid.Must(uuid.NewV4())
		commentAttachmentRepo := &stubCommentAttachmentRepo{
			attachments: map[uuid.UUID]*models.CommentAttachment{
				attachmentID: {ID: attachmentID, CommentID: commentID, StoragePath: "path"},
			},
		}
		commentsRepo := &stubCommentRepo{
			comments: map[uuid.UUID]*models.Comments{
				commentID: {ID: commentID, TaskID: taskID},
			},
		}
		taskRepo := &stubCommentTaskRepo{
			stubAttachmentTaskRepo: stubAttachmentTaskRepo{
				task: &models.Task{
					ID:        taskID,
					ProjectID: projectID,
					Project:   models.Project{ID: projectID, OrganizationID: orgID},
				},
			},
		}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		cleanupRepo := &stubFileCleanupRepo{}

		service := services.InitAttachmentService(nil, commentAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		// 1. Upload
		header, _ := createTestMultipartFileHeader("test.png", []byte("content"))
		_, err := service.UploadCommentAttachments(context.Background(), commentID, wrongTaskID, userID, []*multipart.FileHeader{header})
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment upload, got %v", err)
		}

		// 2. Get
		_, err = service.GetCommentAttachments(context.Background(), commentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment get, got %v", err)
		}

		// 3. Download
		_, _, _, _, err = service.DownloadCommentAttachment(context.Background(), attachmentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment download, got %v", err)
		}

		// 4. Delete
		err = service.DeleteCommentAttachment(context.Background(), attachmentID, wrongTaskID, userID)
		if err == nil || err.Code != response.ErrBadRequest {
			t.Errorf("expected ErrBadRequest for comment delete, got %v", err)
		}
	})

	t.Run("Upload Rollback Trigger and Repository Correctness", func(t *testing.T) {
		attachmentRepo := &stubAttachmentRepo{createErr: &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: "DB error"}}
		taskRepo := &stubAttachmentTaskRepo{task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{ID: projectID, OrganizationID: orgID},
		}}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		cleanupRepo := &stubFileCleanupRepo{}

		service := services.InitAttachmentService(attachmentRepo, nil, cleanupRepo, nil, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, err := service.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil {
			t.Fatal("expected upload error, got nil")
		}

		// Verify task rollback records to cleanupRepo
		if len(cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 orphan in cleanupRepo, got %d", len(cleanupRepo.orphanedLogs))
		}
	})

	t.Run("Comment Upload Rollback Repository Correctness", func(t *testing.T) {
		commentAttachmentRepo := &stubCommentAttachmentRepo{createErr: &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: "DB error"}}
		commentsRepo := &stubCommentRepo{
			comments: map[uuid.UUID]*models.Comments{
				commentID: {ID: commentID, TaskID: taskID},
			},
		}
		taskRepo := &stubCommentTaskRepo{
			stubAttachmentTaskRepo: stubAttachmentTaskRepo{
				task: &models.Task{
					ID:        taskID,
					ProjectID: projectID,
					Project:   models.Project{ID: projectID, OrganizationID: orgID},
				},
			},
		}
		projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
		authRepo := &stubAuthRepository{user: models.User{ID: userID, Role: "member", OrganizationID: &orgID}}
		cleanupRepo := &stubFileCleanupRepo{}

		service := services.InitAttachmentService(nil, commentAttachmentRepo, cleanupRepo, commentsRepo, taskRepo, projectRepo, authRepo, nil, &mockStorageClient{}, zap.NewNop(), nil)

		fileHeader, _ := createTestMultipartFileHeader("test.png", []byte("png content"))
		_, err := service.UploadCommentAttachments(context.Background(), commentID, taskID, userID, []*multipart.FileHeader{fileHeader})
		if err == nil {
			t.Fatal("expected comment upload error, got nil")
		}

		// Verify comment rollback explicitly records to cleanupRepo
		if len(cleanupRepo.orphanedLogs) != 1 {
			t.Errorf("expected 1 comment orphan in cleanupRepo, got %d", len(cleanupRepo.orphanedLogs))
		}
	})

	t.Run("Worker Shutdown context leak test", func(t *testing.T) {
		cleanupRepo := &stubFileCleanupRepo{}
		ctx, cancel := context.WithCancel(context.Background())
		
		// Init service, starting cleanup worker goroutine
		_ = services.InitAttachmentService(nil, nil, cleanupRepo, nil, &stubAttachmentTaskRepo{}, &stubAttachmentProjectRepo{}, &stubAuthRepository{}, nil, &mockStorageClient{}, zap.NewNop(), ctx)
		
		cancel() // worker should stop cleanly on cancel
	})
}

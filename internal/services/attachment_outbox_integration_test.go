package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	projectrepo "github.com/ms-kanban-server/internal/repository/project-repo"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

// spyStorageClient integration test helper
type spyStorageClient struct {
	deleteFunc    func(ctx context.Context, key string) error
	deletedKeys   []string
	uploadErr     *response.Error
}

func (s *spyStorageClient) UploadLogo(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return "", "", nil
}

func (s *spyStorageClient) UploadAvatar(file multipart.File, header *multipart.FileHeader) (string, string, *response.Error) {
	return "", "", nil
}

func (s *spyStorageClient) DeleteObject(ctx context.Context, key string) error {
	s.deletedKeys = append(s.deletedKeys, key)
	if s.deleteFunc != nil {
		return s.deleteFunc(ctx, key)
	}
	return nil
}

func (s *spyStorageClient) UploadAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, taskID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	if s.uploadErr != nil {
		return "", "", "", s.uploadErr
	}
	return "tasks/taskid/attachments/" + header.Filename, header.Filename, "image/png", nil
}

func (s *spyStorageClient) UploadCommentAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader, commentID uuid.UUID, cfg models.AttachmentConfig) (string, string, string, *response.Error) {
	if s.uploadErr != nil {
		return "", "", "", s.uploadErr
	}
	return "comments/commentid/attachments/" + header.Filename, header.Filename, "image/png", nil
}

func (s *spyStorageClient) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, *response.Error) {
	return nil, 0, nil
}

// statefulAttachmentRepo implements stateful repository operations in memory for testing
type statefulAttachmentRepo struct {
	attachments   map[uuid.UUID]*models.TaskAttachment
	orphanedFiles map[uuid.UUID]*models.OrphanedFile
	createErr     error
}

func newStatefulAttachmentRepo() *statefulAttachmentRepo {
	return &statefulAttachmentRepo{
		attachments:   make(map[uuid.UUID]*models.TaskAttachment),
		orphanedFiles: make(map[uuid.UUID]*models.OrphanedFile),
	}
}

func (r *statefulAttachmentRepo) CreateAttachment(a *models.TaskAttachment) *response.Error {
	if r.createErr != nil {
		return &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: r.createErr.Error()}
	}
	if a.ID == uuid.Nil {
		a.ID, _ = uuid.NewV7()
	}
	r.attachments[a.ID] = a
	return nil
}

func (r *statefulAttachmentRepo) GetAttachmentByID(id uuid.UUID) (*models.TaskAttachment, *response.Error) {
	a, ok := r.attachments[id]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Attachment not found"}
	}
	return a, nil
}

func (r *statefulAttachmentRepo) GetAttachmentsByTaskID(taskID uuid.UUID) ([]models.TaskAttachment, *response.Error) {
	var list []models.TaskAttachment
	for _, a := range r.attachments {
		if a.TaskID == taskID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (r *statefulAttachmentRepo) DeleteAttachment(id uuid.UUID) *response.Error {
	delete(r.attachments, id)
	return nil
}

func (r *statefulAttachmentRepo) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	delete(r.attachments, attachmentID)
	orphanID, _ := uuid.NewV7()
	r.orphanedFiles[orphanID] = &models.OrphanedFile{
		ID:          orphanID,
		StoragePath: storagePath,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (r *statefulAttachmentRepo) CreateOrphanedFile(file *models.OrphanedFile) *response.Error {
	if file.ID == uuid.Nil {
		file.ID, _ = uuid.NewV7()
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now()
	}
	r.orphanedFiles[file.ID] = file
	return nil
}

func (r *statefulAttachmentRepo) GetOrphanedFiles() ([]models.OrphanedFile, *response.Error) {
	var list []models.OrphanedFile
	for _, f := range r.orphanedFiles {
		list = append(list, *f)
	}
	return list, nil
}

func (r *statefulAttachmentRepo) DeleteOrphanedFile(id uuid.UUID) *response.Error {
	delete(r.orphanedFiles, id)
	return nil
}

func (r *statefulAttachmentRepo) ClaimOrphanedFiles(now time.Time, claimedUntil time.Time, limit int) ([]models.OrphanedFile, *response.Error) {
	var claimed []models.OrphanedFile
	for _, f := range r.orphanedFiles {
		if len(claimed) >= limit {
			break
		}
		isUnclaimed := f.ClaimedUntil == nil || f.ClaimedUntil.Before(now)
		isDue := f.NextAttemptAt == nil || f.NextAttemptAt.Before(now) || f.NextAttemptAt.Equal(now)
		if isUnclaimed && isDue {
			f.ClaimedUntil = &claimedUntil
			f.Attempts++
			f.LastAttemptAt = &now
			claimed = append(claimed, *f)
		}
	}
	return claimed, nil
}

func (r *statefulAttachmentRepo) ReleaseOrphanedFile(id uuid.UUID, lastErr string, lastAttempt time.Time, nextAttempt time.Time) *response.Error {
	if f, ok := r.orphanedFiles[id]; ok {
		f.ClaimedUntil = nil
		f.LastError = lastErr
		f.LastAttemptAt = &lastAttempt
		f.NextAttemptAt = &nextAttempt
	}
	return nil
}

// Interface-embedded mock implementations to satisfy GORM interfaces cleanly
type dummyTaskRepo struct {
	taskrepo.TaskRepository
	task *models.Task
}

func (d *dummyTaskRepo) GetTaskAccessContext(id uuid.UUID) (*models.TaskAccessContext, *response.Error) {
	if d.task != nil {
		return &models.TaskAccessContext{
			TaskID:         d.task.ID,
			ProjectID:      d.task.ProjectID,
			OrganizationID: d.task.Project.OrganizationID,
			TaskKey:        d.task.Key,
		}, nil
	}
	return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Task not found"}
}

type dummyProjectRepo struct {
	projectrepo.ProjectRepository
}

func (d *dummyProjectRepo) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	return true, nil
}

func (d *dummyProjectRepo) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {
	return &models.ProjectMember{ProjectRole: "member"}, nil
}

type dummyAuthRepo struct {
	authrepo.AuthRepository
	orgID uuid.UUID
}

func (d *dummyAuthRepo) GetUserByID(id uuid.UUID) (models.User, *response.Error) {
	return models.User{ID: id, Role: "member", OrganizationID: &d.orgID}, nil
}

type dummyAuditRepo struct {
	auditrepo.AuditLogRepository
}

func (d *dummyAuditRepo) CreateAuditLog(log models.AuditLog) *response.Error {
	return nil
}

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

func TestAttachmentOutbox_StatefulIntegration(t *testing.T) {
	attachmentRepo := newStatefulAttachmentRepo()
	storageClient := &spyStorageClient{}
	
	taskID := uuid.Must(uuid.NewV7())
	projectID := uuid.Must(uuid.NewV7())
	userID := uuid.Must(uuid.NewV7())
	orgID := uuid.Must(uuid.NewV7())

	taskRepo := &dummyTaskRepo{
		task: &models.Task{
			ID:        taskID,
			ProjectID: projectID,
			Project:   models.Project{ID: projectID, OrganizationID: orgID},
			Key:       "TASK-999",
		},
	}
	projectRepo := &dummyProjectRepo{}
	authRepo := &dummyAuthRepo{orgID: orgID}
	auditRepo := &dummyAuditRepo{}

	// Create service
	s := InitAttachmentService(
		attachmentRepo,
		nil,
		nil,
		taskRepo,
		projectRepo,
		authRepo,
		auditRepo,
		storageClient,
		zap.NewNop(),
		nil,
	).(*attachmentService)

	t.Run("Delete outbox flow - success and failure retry backoff", func(t *testing.T) {
		// Clean maps
		attachmentRepo.attachments = make(map[uuid.UUID]*models.TaskAttachment)
		attachmentRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
		storageClient.deletedKeys = nil

		// Setup attachment in memory DB
		attachmentID := uuid.Must(uuid.NewV7())
		attachment := &models.TaskAttachment{
			ID:          attachmentID,
			TaskID:      taskID,
			StoragePath: "tasks/taskid/attachments/file.png",
			UploadedBy:  userID,
		}
		attachmentRepo.attachments[attachmentID] = attachment

		// 1. Delete attachment metadata. It should delete metadata and record orphan.
		err := s.DeleteAttachment(context.Background(), attachmentID, projectID, userID)
		if err != nil {
			t.Fatalf("DeleteAttachment failed: %v", err)
		}

		// Verify metadata gone
		if _, getErr := attachmentRepo.GetAttachmentByID(attachmentID); getErr == nil {
			t.Fatal("expected metadata to be deleted")
		}

		// Verify orphan exists in DB
		orphans, _ := attachmentRepo.GetOrphanedFiles()
		if len(orphans) != 1 {
			t.Fatalf("expected 1 orphan, got %d", len(orphans))
		}
		orphan := orphans[0]
		if orphan.StoragePath != "tasks/taskid/attachments/file.png" {
			t.Errorf("unexpected storage path: %s", orphan.StoragePath)
		}

		// 2. Worker runs when S3 delete fails
		storageClient.deleteFunc = func(ctx context.Context, key string) error {
			return errors.New("S3 network failure")
		}
		
		s.processOrphanedFiles(context.Background())

		// Verify orphan still exists, attempts = 1, next_attempt_at is populated
		orphans, _ = attachmentRepo.GetOrphanedFiles()
		if len(orphans) != 1 {
			t.Fatalf("expected orphan to persist, got %d", len(orphans))
		}
		orphan = orphans[0]
		if orphan.Attempts != 1 {
			t.Errorf("expected attempts=1, got %d", orphan.Attempts)
		}
		if orphan.NextAttemptAt == nil {
			t.Fatal("expected NextAttemptAt to be set")
		}

		// 3. Worker runs again immediately (time hasn't advanced). It should NOT claim the file.
		storageClient.deletedKeys = nil
		s.processOrphanedFiles(context.Background())
		if len(storageClient.deletedKeys) > 0 {
			t.Fatalf("expected worker to skip file due to backoff, but it attempted to delete: %v", storageClient.deletedKeys)
		}

		// 4. Advance time past next_attempt_at, and let worker run and succeed
		futureTime := time.Now().Add(1 * time.Minute)
		storageClient.deleteFunc = nil // S3 delete will succeed

		// We claim files using futureTime
		claimedFiles, _ := attachmentRepo.ClaimOrphanedFiles(futureTime, futureTime.Add(2*time.Minute), 50)
		if len(claimedFiles) != 1 {
			t.Fatalf("expected 1 file to be claimed when advancing time, got %d", len(claimedFiles))
		}
		
		// Simulate successful delete in worker processing loop
		for _, file := range claimedFiles {
			delErr := storageClient.DeleteObject(context.Background(), file.StoragePath)
			if delErr == nil {
				attachmentRepo.DeleteOrphanedFile(file.ID)
			}
		}

		// Verify orphan is completely gone
		orphans, _ = attachmentRepo.GetOrphanedFiles()
		if len(orphans) != 0 {
			t.Fatalf("expected 0 orphans remaining after success, got %d", len(orphans))
		}
	})

	t.Run("Upload Rollback Outbox durability", func(t *testing.T) {
		attachmentRepo.attachments = make(map[uuid.UUID]*models.TaskAttachment)
		attachmentRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
		storageClient.deletedKeys = nil
		storageClient.uploadErr = nil
		
		// Force DB insert of attachment metadata to fail during bulk upload
		attachmentRepo.createErr = errors.New("DB constraint check failed")

		// Prepare files
		header, err := createTestMultipartFileHeader("rollback_file.png", []byte("filedata"))
		if err != nil {
			t.Fatalf("failed to create test file header: %v", err)
		}

		// Upload attachments (S3 upload succeeds, DB insert fails, rollback is triggered)
		_, uploadErr := s.UploadAttachments(context.Background(), taskID, projectID, userID, []*multipart.FileHeader{header})
		if uploadErr == nil {
			t.Fatal("expected upload to fail due to DB error, but it succeeded")
		}

		// Verify no attachment was created in DB
		if len(attachmentRepo.attachments) > 0 {
			t.Fatalf("expected 0 attachments in DB, got %d", len(attachmentRepo.attachments))
		}

		// Verify the outbox has a record for the S3 file because rollback was triggered
		orphans, _ := attachmentRepo.GetOrphanedFiles()
		if len(orphans) != 1 {
			t.Fatalf("expected exactly 1 orphan record created during rollback, got %d", len(orphans))
		}
		expectedPath := "tasks/taskid/attachments/rollback_file.png"
		if orphans[0].StoragePath != expectedPath {
			t.Errorf("expected storage path %s, got %s", expectedPath, orphans[0].StoragePath)
		}
	})
}

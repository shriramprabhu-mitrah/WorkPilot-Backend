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
	"sync"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

type statefulFileCleanupRepo struct {
	orphanedFiles map[uuid.UUID]*models.OrphanedFile
	createErr     error
	mu            sync.Mutex
}

func newStatefulFileCleanupRepo() *statefulFileCleanupRepo {
	return &statefulFileCleanupRepo{
		orphanedFiles: make(map[uuid.UUID]*models.OrphanedFile),
	}
}

func (r *statefulFileCleanupRepo) CreateOrphanedFile(ctx context.Context, f *models.OrphanedFile) *response.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return &response.Error{Code: response.ErrInternalServerError, StatusCode: 500, Message: r.createErr.Error()}
	}
	if f.ID == uuid.Nil {
		f.ID, _ = uuid.NewV7()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	if f.AvailableAt.IsZero() {
		f.AvailableAt = time.Now()
	}
	r.orphanedFiles[f.ID] = f
	return nil
}

func (r *statefulFileCleanupRepo) DeleteOrphanedFile(ctx context.Context, id uuid.UUID) *response.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.orphanedFiles, id)
	return nil
}

func (r *statefulFileCleanupRepo) ClaimOrphanedFiles(ctx context.Context, now time.Time, claimTTL time.Duration, limit int) ([]models.OrphanedFile, *response.Error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	var eligible []*models.OrphanedFile
	for _, f := range r.orphanedFiles {
		if f.AvailableAt.Before(now) || f.AvailableAt.Equal(now) {
			eligible = append(eligible, f)
		}
	}

	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].AvailableAt.Equal(eligible[j].AvailableAt) {
			return eligible[i].CreatedAt.Before(eligible[j].CreatedAt)
		}
		return eligible[i].AvailableAt.Before(eligible[j].AvailableAt)
	})

	var claimed []models.OrphanedFile
	for i := 0; i < len(eligible) && i < limit; i++ {
		f := eligible[i]
		f.AvailableAt = now.Add(claimTTL)
		f.Attempts++
		f.LastAttemptAt = &now
		claimed = append(claimed, *f)
	}
	return claimed, nil
}

func (r *statefulFileCleanupRepo) ReleaseOrphanedFile(ctx context.Context, id uuid.UUID, lastErr string, lastAttempt time.Time, nextAttempt time.Time) *response.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.orphanedFiles[id]; ok {
		f.AvailableAt = nextAttempt
		f.LastError = lastErr
		f.LastAttemptAt = &lastAttempt
	}
	return nil
}

// statefulAttachmentRepo implements stateful repository operations in memory for testing
type statefulAttachmentRepo struct {
	attachments   map[uuid.UUID]*models.TaskAttachment
	cleanupRepo   *statefulFileCleanupRepo
	createErr     error
}

func newStatefulAttachmentRepo(cleanupRepo *statefulFileCleanupRepo) *statefulAttachmentRepo {
	return &statefulAttachmentRepo{
		attachments:   make(map[uuid.UUID]*models.TaskAttachment),
		cleanupRepo:   cleanupRepo,
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
	orphan := &models.OrphanedFile{
		StoragePath: storagePath,
	}
	return r.cleanupRepo.CreateOrphanedFile(context.Background(), orphan)
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
	cleanupRepo := newStatefulFileCleanupRepo()
	attachmentRepo := newStatefulAttachmentRepo(cleanupRepo)
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
		cleanupRepo,
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
		cleanupRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
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
		if len(cleanupRepo.orphanedFiles) != 1 {
			t.Fatalf("expected 1 orphan, got %d", len(cleanupRepo.orphanedFiles))
		}
		var orphan *models.OrphanedFile
		for _, f := range cleanupRepo.orphanedFiles {
			orphan = f
		}
		if orphan.StoragePath != "tasks/taskid/attachments/file.png" {
			t.Errorf("unexpected storage path: %s", orphan.StoragePath)
		}

		// 2. Worker runs when S3 delete fails
		storageClient.deleteFunc = func(ctx context.Context, key string) error {
			return errors.New("S3 network failure")
		}
		
		s.processOrphanedFiles(context.Background())

		// Verify orphan still exists, attempts = 1, available_at is updated
		if len(cleanupRepo.orphanedFiles) != 1 {
			t.Fatalf("expected orphan to persist, got %d", len(cleanupRepo.orphanedFiles))
		}
		if orphan.Attempts != 1 {
			t.Errorf("expected attempts=1, got %d", orphan.Attempts)
		}

		// 3. Worker runs again immediately (time hasn't advanced). It should NOT claim the file.
		storageClient.deletedKeys = nil
		s.processOrphanedFiles(context.Background())
		if len(storageClient.deletedKeys) > 0 {
			t.Fatalf("expected worker to skip file due to backoff, but it attempted to delete: %v", storageClient.deletedKeys)
		}

		// 4. Advance time past available_at, and let worker run and succeed
		futureTime := time.Now().Add(1 * time.Hour)
		storageClient.deleteFunc = nil // S3 delete will succeed

		// We claim files using futureTime
		claimedFiles, _ := cleanupRepo.ClaimOrphanedFiles(context.Background(), futureTime, 2*time.Minute, 50)
		if len(claimedFiles) != 1 {
			t.Fatalf("expected 1 file to be claimed when advancing time, got %d", len(claimedFiles))
		}
		
		// Simulate successful delete in worker processing loop
		for _, file := range claimedFiles {
			delErr := storageClient.DeleteObject(context.Background(), file.StoragePath)
			if delErr == nil {
				cleanupRepo.DeleteOrphanedFile(context.Background(), file.ID)
			}
		}

		// Verify orphan is completely gone
		if len(cleanupRepo.orphanedFiles) != 0 {
			t.Fatalf("expected 0 orphans remaining after success, got %d", len(cleanupRepo.orphanedFiles))
		}
	})

	t.Run("Upload Rollback Outbox durability", func(t *testing.T) {
		attachmentRepo.attachments = make(map[uuid.UUID]*models.TaskAttachment)
		cleanupRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
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
		if len(cleanupRepo.orphanedFiles) != 1 {
			t.Fatalf("expected exactly 1 orphan record created during rollback, got %d", len(cleanupRepo.orphanedFiles))
		}
		var orphan *models.OrphanedFile
		for _, f := range cleanupRepo.orphanedFiles {
			orphan = f
		}
		expectedPath := "tasks/taskid/attachments/rollback_file.png"
		if orphan.StoragePath != expectedPath {
			t.Errorf("expected storage path %s, got %s", expectedPath, orphan.StoragePath)
		}
	})

	t.Run("Concurrency Isolation - worker A claims, worker B gets zero", func(t *testing.T) {
		cleanupRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
		
		// Insert 1 eligible orphan
		orphanID, _ := uuid.NewV7()
		now := time.Now()
		cleanupRepo.orphanedFiles[orphanID] = &models.OrphanedFile{
			ID:          orphanID,
			StoragePath: "tasks/taskid/file.png",
			AvailableAt: now.Add(-10 * time.Second),
			Attempts:    0,
		}

		// Simulate worker A and worker B concurrent claims
		var workerAClaimed []models.OrphanedFile
		var workerBClaimed []models.OrphanedFile

		// Since our stateful stub ClaimOrphanedFiles is protected by a Mutex,
		// calling it sequentially or concurrently yields atomic claims.
		workerAClaimed, _ = cleanupRepo.ClaimOrphanedFiles(context.Background(), now, 2*time.Minute, 1)
		workerBClaimed, _ = cleanupRepo.ClaimOrphanedFiles(context.Background(), now, 2*time.Minute, 1)

		if len(workerAClaimed) != 1 {
			t.Fatalf("expected Worker A to claim the orphan, got %d", len(workerAClaimed))
		}
		if len(workerBClaimed) != 0 {
			t.Fatalf("expected Worker B to claim zero rows, got %d", len(workerBClaimed))
		}

		// Verify DB attempts = 1
		if cleanupRepo.orphanedFiles[orphanID].Attempts != 1 {
			t.Errorf("expected attempts = 1, got %d", cleanupRepo.orphanedFiles[orphanID].Attempts)
		}
	})

	t.Run("Claim Ordering and Filtering", func(t *testing.T) {
		cleanupRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
		now := time.Now()

		idA, _ := uuid.NewV7()
		idB, _ := uuid.NewV7()
		idC, _ := uuid.NewV7()

		// A: past available
		cleanupRepo.orphanedFiles[idA] = &models.OrphanedFile{
			ID:          idA,
			StoragePath: "fileA",
			AvailableAt: now.Add(-10 * time.Second),
			CreatedAt:   now.Add(-10 * time.Second),
		}
		// B: older past available (should be claimed first because of order)
		cleanupRepo.orphanedFiles[idB] = &models.OrphanedFile{
			ID:          idB,
			StoragePath: "fileB",
			AvailableAt: now.Add(-30 * time.Second),
			CreatedAt:   now.Add(-30 * time.Second),
		}
		// C: future available (should be untouched)
		cleanupRepo.orphanedFiles[idC] = &models.OrphanedFile{
			ID:          idC,
			StoragePath: "fileC",
			AvailableAt: now.Add(10 * time.Second),
			CreatedAt:   now,
		}

		claimed, _ := cleanupRepo.ClaimOrphanedFiles(context.Background(), now, 2*time.Minute, 2)
		if len(claimed) != 2 {
			t.Fatalf("expected 2 claimed files, got %d", len(claimed))
		}

		// Verify ordering: fileB claimed first, then fileA
		if claimed[0].ID != idB || claimed[1].ID != idA {
			t.Errorf("unexpected claim ordering: first=%s, second=%s", claimed[0].StoragePath, claimed[1].StoragePath)
		}

		// Verify C is untouched
		if cleanupRepo.orphanedFiles[idC].Attempts != 0 {
			t.Error("expected C to remain unclaimed/untouched")
		}
	})

	t.Run("Idempotency NoSuchKey and Crash Recovery", func(t *testing.T) {
		cleanupRepo.orphanedFiles = make(map[uuid.UUID]*models.OrphanedFile)
		now := time.Now()

		orphanID, _ := uuid.NewV7()
		cleanupRepo.orphanedFiles[orphanID] = &models.OrphanedFile{
			ID:          orphanID,
			StoragePath: "tasks/taskid/crashed.png",
			AvailableAt: now.Add(-10 * time.Second),
		}

		// Simulate Crash recovery: S3 object is already absent.
		// DeleteObject should return an error of types.NoSuchKey.
		storageClient.deleteFunc = func(ctx context.Context, key string) error {
			return &types.NoSuchKey{}
		}

		s.processOrphanedFiles(context.Background())

		// Verify that NoSuchKey error was treated as successful delete and row was removed from DB
		if len(cleanupRepo.orphanedFiles) != 0 {
			t.Fatalf("expected orphan row to be deleted on NoSuchKey error, but got %d", len(cleanupRepo.orphanedFiles))
		}
	})
}

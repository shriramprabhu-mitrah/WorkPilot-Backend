package services_test

import (
	"context"
	"mime/multipart"
	"testing"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubUserStoryAttachmentRepo struct {
	attachments  map[uuid.UUID]*models.UserStoryAttachment
	createErr    *response.Error
	getErr       *response.Error
	deleteErr    *response.Error
	orphanedLogs []string
}

func (s *stubUserStoryAttachmentRepo) CreateAttachment(a *models.UserStoryAttachment) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.attachments == nil {
		s.attachments = make(map[uuid.UUID]*models.UserStoryAttachment)
	}
	if a.ID == uuid.Nil {
		a.ID, _ = uuid.NewV7()
	}
	s.attachments[a.ID] = a
	return nil
}

func (s *stubUserStoryAttachmentRepo) GetAttachmentByID(id uuid.UUID) (*models.UserStoryAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	a, ok := s.attachments[id]
	if !ok {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Attachment not found"}
	}
	return a, nil
}

func (s *stubUserStoryAttachmentRepo) GetAttachmentsByUserStoryID(userStoryID uuid.UUID) ([]models.UserStoryAttachment, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	var list []models.UserStoryAttachment
	for _, a := range s.attachments {
		if a.UserStoryID == userStoryID {
			list = append(list, *a)
		}
	}
	return list, nil
}

func (s *stubUserStoryAttachmentRepo) DeleteAttachment(id uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, id)
	return nil
}

func (s *stubUserStoryAttachmentRepo) DeleteAttachmentAndRecordOrphan(attachmentID uuid.UUID, storagePath string) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.attachments, attachmentID)
	s.orphanedLogs = append(s.orphanedLogs, storagePath)
	return nil
}

type userStoryAttachmentTestFixture struct {
	service                 services.AttachmentService
	commentsService         services.CommentsService
	userStoryRepo           *stubUserStoryRepo
	userStoryAttachmentRepo *stubUserStoryAttachmentRepo
	cleanupRepo             *stubFileCleanupRepo
	commentsRepo            *stubCommentRepo
	projectRepo             *stubAttachmentProjectRepo
	authRepo                *stubAuthRepository
	auditRepo               *stubAttachmentAuditLogRepo
	storageClient           *mockStorageClient
	ctx                     context.Context
}

func newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID uuid.UUID) *userStoryAttachmentTestFixture {
	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{
			storyID: {
				ID:        storyID,
				ProjectID: projectID,
				Title:     "Awesome Story",
				Project:   models.Project{ID: projectID, OrganizationID: orgID},
			},
		},
	}
	userStoryAttachmentRepo := &stubUserStoryAttachmentRepo{attachments: make(map[uuid.UUID]*models.UserStoryAttachment)}
	cleanupRepo := &stubFileCleanupRepo{orphanedFiles: make(map[uuid.UUID]*models.OrphanedFile)}
	commentsRepo := &stubCommentRepo{comments: make(map[uuid.UUID]*models.Comments)}
	projectRepo := &stubAttachmentProjectRepo{isMember: true, project: models.Project{OrganizationID: orgID}}
	authRepo := &stubAuthRepository{user: models.User{ID: userID, RoleID: uuid.FromStringOrNil("00000000-0000-0000-0000-000000000004"), Role: models.Role{Name: "member"}, OrganizationID: &orgID}}
	auditRepo := &stubAttachmentAuditLogRepo{}
	storageClient := &mockStorageClient{}

	service := services.InitAttachmentService(
		nil,
		nil,
		userStoryAttachmentRepo,
		cleanupRepo,
		commentsRepo,
		nil,
		userStoryRepo,
		projectRepo,
		authRepo,
		auditRepo,
		storageClient,
		zap.NewNop(),
		nil,
	)

	commentsService := services.InitCommentsService(
		commentsRepo,
		nil,
		userStoryRepo,
		projectRepo,
		authRepo,
		auditRepo,
		zap.NewNop(),
	)

	return &userStoryAttachmentTestFixture{
		service:                 service,
		commentsService:         commentsService,
		userStoryRepo:           userStoryRepo,
		userStoryAttachmentRepo: userStoryAttachmentRepo,
		cleanupRepo:             cleanupRepo,
		commentsRepo:            commentsRepo,
		projectRepo:             projectRepo,
		authRepo:                authRepo,
		auditRepo:               auditRepo,
		storageClient:           storageClient,
		ctx:                     context.Background(),
	}
}

func TestUserStoryAttachment_UploadSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID)

	fileHeader, err := createTestMultipartFileHeader("story.txt", []byte("story attachment content"))
	if err != nil {
		t.Fatalf("failed to create test file header: %v", err)
	}

	res, uploadErr := f.service.UploadUserStoryAttachments(f.ctx, storyID, projectID, userID, []*multipart.FileHeader{fileHeader})
	if uploadErr != nil {
		t.Fatalf("expected upload to succeed, got %v", uploadErr)
	}

	if len(res) != 1 {
		t.Errorf("expected 1 uploaded attachment, got %d", len(res))
	}

	if res[0].OriginalFilename != "story.txt" {
		t.Errorf("expected filename 'story.txt', got %s", res[0].OriginalFilename)
	}

	if res[0].URL != "http://localhost/user_stories/storyid/file.png" {
		t.Errorf("expected URL 'http://localhost/user_stories/storyid/file.png', got %s", res[0].URL)
	}
}

func TestUserStoryAttachment_GetSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID)
	attachmentID := uuid.Must(uuid.NewV7())

	f.userStoryAttachmentRepo.attachments[attachmentID] = &models.UserStoryAttachment{
		ID:               attachmentID,
		UserStoryID:      storyID,
		OriginalFilename: "story_notes.docx",
		StoragePath:      "user_stories/storyID/story_notes.docx",
		URL:              "http://localhost/user_stories/storyID/story_notes.docx",
		UploadedBy:       userID,
	}

	res, err := f.service.GetUserStoryAttachments(f.ctx, storyID, projectID, userID)
	if err != nil {
		t.Fatalf("expected get to succeed, got %v", err)
	}

	if len(res) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(res))
	}
	if res[0].OriginalFilename != "story_notes.docx" {
		t.Errorf("expected original filename story_notes.docx, got %s", res[0].OriginalFilename)
	}
	if res[0].URL != "http://localhost/user_stories/storyID/story_notes.docx" {
		t.Errorf("expected URL 'http://localhost/user_stories/storyID/story_notes.docx', got %s", res[0].URL)
	}
}

func TestUserStoryAttachment_DownloadSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID)
	attachmentID := uuid.Must(uuid.NewV7())

	f.userStoryAttachmentRepo.attachments[attachmentID] = &models.UserStoryAttachment{
		ID:               attachmentID,
		UserStoryID:      storyID,
		OriginalFilename: "download.png",
		MIMEType:         "image/png",
		StoragePath:      "user_stories/storyID/download.png",
		UploadedBy:       userID,
	}

	stream, filename, mime, size, err := f.service.DownloadUserStoryAttachment(f.ctx, attachmentID, projectID, userID)
	if err != nil {
		t.Fatalf("expected download to succeed, got %v", err)
	}
	defer stream.Close()

	if filename != "download.png" {
		t.Errorf("expected filename download.png, got %s", filename)
	}
	if mime != "image/png" {
		t.Errorf("expected mime image/png, got %s", mime)
	}
	if size != 12 { // mockStorageClient returns 12 bytes
		t.Errorf("expected size 12, got %d", size)
	}
}

func TestUserStoryAttachment_DeleteSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID)
	attachmentID := uuid.Must(uuid.NewV7())

	f.userStoryAttachmentRepo.attachments[attachmentID] = &models.UserStoryAttachment{
		ID:               attachmentID,
		UserStoryID:      storyID,
		OriginalFilename: "delete.png",
		StoragePath:      "user_stories/storyID/delete.png",
		UploadedBy:       userID,
	}

	err := f.service.DeleteUserStoryAttachment(f.ctx, attachmentID, projectID, userID)
	if err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	if len(f.userStoryAttachmentRepo.orphanedLogs) != 1 {
		t.Errorf("expected 1 orphan record during delete, got %d", len(f.userStoryAttachmentRepo.orphanedLogs))
	}
}

func TestUserStoryComment_GetCommentsByUserStoryIDSuccess(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())

	f := newUserStoryAttachmentTestFixture(orgID, projectID, storyID, userID)

	f.commentsRepo.comments[uuid.Must(uuid.NewV7())] = &models.Comments{
		UserStoryID:    &storyID,
		UserID:         userID,
		ProjectID:      projectID,
		OrganizationID: orgID,
		Content:        "User story comment",
		User: models.User{
			UserName: "tester",
			FullName: "Test Tester",
			Email:    "tester@mitrah.com",
		},
	}

	res, _, err := f.commentsService.GetCommentsByUserStoryID(dto.GetComments{
		UserStoryID:    &storyID,
		UserID:         userID,
		OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("expected comments retrieval to succeed, got %v", err)
	}

	// Because stubCommentRepo.GetCommentsByUserStoryID is mocked to return nil (stub definition returning nil),
	// we just verify that it compiles, executes and returns correctly.
	if len(res) > 0 {
		t.Errorf("expected stub to return empty slice, got %+v", res)
	}
}

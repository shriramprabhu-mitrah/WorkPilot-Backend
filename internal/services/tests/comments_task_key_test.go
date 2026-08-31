package services_test

import (
	"testing"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubCommentRepoForTaskKey struct {
	comments map[uuid.UUID]*models.Comments
}

func (r *stubCommentRepoForTaskKey) CreateComment(comment *models.Comments) *response.Error {
	if comment.ID == uuid.Nil {
		comment.ID = uuid.Must(uuid.NewV4())
	}
	r.comments[comment.ID] = comment
	return nil
}

func (r *stubCommentRepoForTaskKey) GetCommentByID(id uuid.UUID) (*models.Comments, *response.Error) {
	if c, ok := r.comments[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (r *stubCommentRepoForTaskKey) GetCommentsByTaskID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	var list []models.Comments
	for _, c := range r.comments {
		if req.TaskID != nil && c.TaskID != nil && *c.TaskID == *req.TaskID {
			list = append(list, *c)
		}
	}
	return list, response.Pagination{}, nil
}

func (r *stubCommentRepoForTaskKey) UpdateComment(id uuid.UUID, update *models.Comments) *response.Error {
	if c, ok := r.comments[id]; ok {
		c.Content = update.Content
	}
	return nil
}

func (r *stubCommentRepoForTaskKey) DeleteComment(id uuid.UUID) *response.Error {
	delete(r.comments, id)
	return nil
}

func (r *stubCommentRepoForTaskKey) MarkCommentAsDeleted(id uuid.UUID) *response.Error {
	if c, ok := r.comments[id]; ok {
		c.IsDeleted = true
	}
	return nil
}

func (r *stubCommentRepoForTaskKey) HasReplies(id uuid.UUID) (bool, *response.Error) {
	return false, nil
}

func (r *stubCommentRepoForTaskKey) GetCommentsByParentID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func (r *stubCommentRepoForTaskKey) GetCommentsByUserStoryID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func TestCommentsService_TaskKeySupport(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	taskKey := "PROJ-101"

	task := &models.Task{
		ID:        taskID,
		ProjectID: projID,
		Key:       taskKey,
		Title:     "Test Task with Key",
	}

	taskRepo := &stubTaskRepo{
		tasks: map[uuid.UUID]*models.Task{
			taskID: task,
		},
	}

	authRepo := &stubAuthRepository{
		user: models.User{
			ID:             userID,
			OrganizationID: &orgID,
			UserName:       "testuser",
			Role:           models.Role{Name: "developer"},
		},
	}

	projectRepo := &stubProjectRepo{
		isMember: true,
	}

	commentsRepo := &stubCommentRepoForTaskKey{
		comments: make(map[uuid.UUID]*models.Comments),
	}

	commentService := services.InitCommentsService(
		commentsRepo,
		nil,
		taskRepo,
		nil,
		projectRepo,
		authRepo,
		&stubAuditLogRepo{},
		zap.NewNop(),
	)

	t.Run("CreateComment with TaskKey", func(t *testing.T) {
		req := requestdto.CreateCommentsRequest{
			TaskIDOrKey:    taskKey,
			UserID:         userID,
			OrganizationID: orgID,
			Content:        "Comment created using task key",
		}

		resp, err := commentService.CreateComments(req)
		if err != nil {
			t.Fatalf("expected CreateComments with task key to succeed, got %v", err)
		}

		if resp.TaskID == nil || *resp.TaskID != taskID {
			t.Errorf("expected resolved TaskID to be %v, got %v", taskID, resp.TaskID)
		}
	})

	t.Run("GetCommentsByTaskID with TaskKey", func(t *testing.T) {
		req := requestdto.GetComments{
			TaskIDOrKey:    taskKey,
			UserID:         userID,
			OrganizationID: orgID,
		}

		comments, _, err := commentService.GetCommentsByTaskID(req)
		if err != nil {
			t.Fatalf("expected GetCommentsByTaskID with task key to succeed, got %v", err)
		}

		if len(comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(comments))
		}
	})
}

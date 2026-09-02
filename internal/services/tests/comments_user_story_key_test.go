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

type stubCommentRepoForUserStoryKey struct {
	comments map[uuid.UUID]*models.Comments
}

func (r *stubCommentRepoForUserStoryKey) CreateComment(comment *models.Comments) *response.Error {
	if comment.ID == uuid.Nil {
		comment.ID = uuid.Must(uuid.NewV4())
	}
	r.comments[comment.ID] = comment
	return nil
}

func (r *stubCommentRepoForUserStoryKey) GetCommentByID(id uuid.UUID) (*models.Comments, *response.Error) {
	if c, ok := r.comments[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (r *stubCommentRepoForUserStoryKey) GetCommentsByTaskID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func (r *stubCommentRepoForUserStoryKey) UpdateComment(id uuid.UUID, update *models.Comments) *response.Error {
	if c, ok := r.comments[id]; ok {
		c.Content = update.Content
	}
	return nil
}

func (r *stubCommentRepoForUserStoryKey) DeleteComment(id uuid.UUID) *response.Error {
	delete(r.comments, id)
	return nil
}

func (r *stubCommentRepoForUserStoryKey) MarkCommentAsDeleted(id uuid.UUID) *response.Error {
	if c, ok := r.comments[id]; ok {
		c.IsDeleted = true
	}
	return nil
}

func (r *stubCommentRepoForUserStoryKey) HasReplies(id uuid.UUID) (bool, *response.Error) {
	return false, nil
}

func (r *stubCommentRepoForUserStoryKey) GetCommentsByParentID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	return nil, response.Pagination{}, nil
}

func (r *stubCommentRepoForUserStoryKey) GetCommentsByUserStoryID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {
	var list []models.Comments
	for _, c := range r.comments {
		if req.UserStoryID != nil && c.UserStoryID != nil && *c.UserStoryID == *req.UserStoryID {
			list = append(list, *c)
		}
	}
	return list, response.Pagination{}, nil
}

func TestCommentsService_UserStoryKeySupport(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	storyID := uuid.Must(uuid.NewV4())
	storyKey := "US-1"

	story := &models.UserStory{
		ID:        storyID,
		ProjectID: projID,
		Key:       storyKey,
		Title:     "Test User Story with Key US-1",
		Project:   models.Project{OrganizationID: orgID},
	}

	userStoryRepo := &stubUserStoryRepo{
		stories: map[uuid.UUID]*models.UserStory{
			storyID: story,
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

	commentsRepo := &stubCommentRepoForUserStoryKey{
		comments: make(map[uuid.UUID]*models.Comments),
	}

	commentService := services.InitCommentsService(
		commentsRepo,
		nil,
		nil,
		userStoryRepo,
		projectRepo,
		authRepo,
		&stubAuditLogRepo{},
		zap.NewNop(),
	)

	t.Run("CreateComment with UserStoryKey US-1", func(t *testing.T) {
		req := requestdto.CreateCommentsRequest{
			UserStoryIDOrKey: storyKey,
			UserID:           userID,
			OrganizationID:   orgID,
			Content:          "Comment created using user story key US-1",
		}

		resp, err := commentService.CreateComments(req)
		if err != nil {
			t.Fatalf("expected CreateComments with user story key to succeed, got %v", err)
		}

		if resp.UserStoryID == nil || *resp.UserStoryID != storyID {
			t.Errorf("expected resolved UserStoryID to be %v, got %v", storyID, resp.UserStoryID)
		}
	})

	t.Run("GetCommentsByUserStoryID with UserStoryKey US-1", func(t *testing.T) {
		req := requestdto.GetComments{
			UserStoryIDOrKey: storyKey,
			UserID:           userID,
			OrganizationID:   orgID,
		}

		comments, _, err := commentService.GetCommentsByUserStoryID(req)
		if err != nil {
			t.Fatalf("expected GetCommentsByUserStoryID with user story key US-1 to succeed, got %v", err)
		}

		if len(comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(comments))
		}
	})
}

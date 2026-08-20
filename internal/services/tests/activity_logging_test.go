package services_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func TestActivityLogging_UserStoryAndTaskSections(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	taskID := uuid.Must(uuid.NewV4())
	userStoryID := uuid.Must(uuid.NewV4())
	avatarURL := "https://example.com/avatar.png"

	user := models.User{
		ID:             userID,
		OrganizationID: &orgID,
		FullName:       "Davy Jones",
		UserName:       "davyjones",
		Email:          "davy@example.com",
		Role:           "member",
		AvatarURL:      avatarURL,
	}

	auditLogs := []models.AuditLog{
		{
			ID:             uuid.Must(uuid.NewV4()),
			UserID:         &userID,
			User:           user,
			OrganizationID: &orgID,
			ProjectID:      &projectID,
			TaskID:         &taskID,
			Action:         "changed status from \"To Do\" to \"In Progress\"",
			ResourceType:   "task",
			ResourceID:     taskID.String(),
			Details:        "Status changed",
			Title:          "Task 1",
			TaskKey:        "PROJ-1",
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now().Add(-10 * time.Minute),
		},
		{
			ID:             uuid.Must(uuid.NewV4()),
			UserID:         &userID,
			User:           user,
			OrganizationID: &orgID,
			ProjectID:      &projectID,
			UserStoryID:    &userStoryID,
			Action:         "created",
			ResourceType:   "user_story",
			ResourceID:     userStoryID.String(),
			Details:        "User Story created: Checkout flow",
			Title:          "Checkout flow",
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now().Add(-5 * time.Minute),
		},
		{
			ID:             uuid.Must(uuid.NewV4()),
			UserID:         &userID,
			User:           user,
			OrganizationID: &orgID,
			ProjectID:      &projectID,
			TaskID:         &taskID,
			Action:         "created",
			ResourceType:   "comment",
			ResourceID:     uuid.Must(uuid.NewV4()).String(),
			Details:        "Davy Jones mentioned Rishi",
			Title:          "Task 1",
			TaskKey:        "PROJ-1",
			Type:           models.AuditLogTypeActivity,
			CreatedAt:      time.Now().Add(-1 * time.Minute),
		},
	}

	t.Run("AuditLogFromModel includes User profile with AvatarURL", func(t *testing.T) {
		resp := responsedto.AuditLogFromModel(auditLogs[0])
		if resp.User == nil {
			t.Fatalf("expected non-nil User in AuditLogResponse")
		}
		if resp.User.FullName != "Davy Jones" {
			t.Errorf("expected FullName 'Davy Jones', got %s", resp.User.FullName)
		}
		if resp.User.AvatarURL == nil || *resp.User.AvatarURL != avatarURL {
			t.Errorf("expected AvatarURL %s, got %v", avatarURL, resp.User.AvatarURL)
		}
		if resp.TaskKey != "PROJ-1" {
			t.Errorf("expected TaskKey 'PROJ-1', got %s", resp.TaskKey)
		}
	})

	t.Run("ProjectActivityFilter resource_type supports All, Comments, Activity", func(t *testing.T) {
		repo := &stubProjectRepo{
			project:  models.Project{ID: projectID, OrganizationID: orgID, Name: "Test Project"},
			isMember: true,
			getProjectActivity: func(pID uuid.UUID, filter requestdto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
				var filtered []models.AuditLog
				resType := filter.ResourceType
				if resType == "" {
					resType = filter.Type
				}
				for _, log := range auditLogs {
					switch resType {
					case "all", "":
						filtered = append(filtered, log)
					case "project":
						if log.ResourceType == "project" || log.ResourceType == "project_member" {
							filtered = append(filtered, log)
						}
					case "comments":
						if log.ResourceType == "comment" {
							filtered = append(filtered, log)
						}
					case "activity":
						if log.ResourceType != "comment" {
							filtered = append(filtered, log)
						}
					}
				}
				return filtered, response.Pagination{TotalItems: len(filtered)}, nil
			},
		}

		service := services.InitProjectService(repo, &dummyAuthRepo{orgID: &orgID}, &dummySprintRepo{}, nil, &stubAuditLogRepo{}, zap.NewNop())

		// Test ResourceType Empty (Default All)
		emptyRes, _, err := service.GetProjectActivity(userID, "member", orgID, projectID, requestdto.ProjectActivityFilterRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(emptyRes) != 3 {
			t.Errorf("expected 3 items for empty resource_type, got %d", len(emptyRes))
		}

		// Test ResourceType All
		allRes, _, err := service.GetProjectActivity(userID, "member", orgID, projectID, requestdto.ProjectActivityFilterRequest{ResourceType: "all"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(allRes) != 3 {
			t.Errorf("expected 3 items for resource_type 'all', got %d", len(allRes))
		}

		// Test ResourceType Project
		projectRes, _, err := service.GetProjectActivity(userID, "member", orgID, projectID, requestdto.ProjectActivityFilterRequest{ResourceType: "project"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(projectRes) != 0 {
			t.Errorf("expected 0 project-level items in test logs, got %d", len(projectRes))
		}

		// Test ResourceType Comments
		commentsRes, _, err := service.GetProjectActivity(userID, "member", orgID, projectID, requestdto.ProjectActivityFilterRequest{ResourceType: "comments"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commentsRes) != 1 {
			t.Errorf("expected 1 item for resource_type 'comments', got %d", len(commentsRes))
		}
		if len(commentsRes) > 0 && commentsRes[0].ResourceType != "comment" {
			t.Errorf("expected comment resource_type, got %s", commentsRes[0].ResourceType)
		}

		// Test ResourceType Activity
		activityRes, _, err := service.GetProjectActivity(userID, "member", orgID, projectID, requestdto.ProjectActivityFilterRequest{ResourceType: "activity"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(activityRes) != 2 {
			t.Errorf("expected 2 items for resource_type 'activity', got %d", len(activityRes))
		}
	})
}

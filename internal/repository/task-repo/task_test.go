package taskrepo_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	configs "github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/drivers/postgres"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	taskrepo "github.com/ms-kanban-server/internal/repository/task-repo"
	"go.uber.org/zap"
)

func loadEnvSH() {
	// Traverse up to find env.sh at project root
	paths := []string{"../../../env.sh", "../../env.sh", "../../../../env.sh"}
	var data []byte
	var err error
	for _, p := range paths {
		data, err = ioutil.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "export ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "export "), "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				os.Setenv(key, val)
			}
		}
	}
}

func TestTaskRepository_GetTasks_Filters(t *testing.T) {
	loadEnvSH()

	cfg := configs.LoadEnv()
	if cfg.Database.Host == "" {
		t.Skip("Skipping integration test: DB_HOST environment variable not set")
	}

	db, err := postgres.InitDB(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Run within a transaction to prevent test data from polluting the database
	tx := db.Begin()
	defer tx.Rollback()

	// Seed Organization
	org := models.Organization{
		Name: "Integration Test Org",
	}
	if err := tx.Create(&org).Error; err != nil {
		t.Fatalf("failed to create organization: %v", err)
	}

	// Seed Users (Assignees) - must be created before project since project has a creator foreign key
	user1 := models.User{
		OrganizationID: &org.ID,
		Email:          "user1@example.com",
		UserName:       "user1",
		PasswordHash:   "xxx",
		Role:           "member",
		IsActive:       true,
	}
	user2 := models.User{
		OrganizationID: &org.ID,
		Email:          "user2@example.com",
		UserName:       "user2",
		PasswordHash:   "xxx",
		Role:           "member",
		IsActive:       true,
	}
	if err := tx.Create(&user1).Error; err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}
	if err := tx.Create(&user2).Error; err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	// Seed Project with CreatedBy user
	proj := models.Project{
		OrganizationID: org.ID,
		Name:           "Integration Test Project",
		CreatedBy:      user1.ID,
		Status:         "active",
	}
	if err := tx.Create(&proj).Error; err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Seed Custom Statuses
	statusTodo := models.CustomStatus{
		ProjectID:    proj.ID,
		Name:         "Todo",
		Color:        "#808080",
		DisplayOrder: 0,
		IsDefault:    true,
	}
	statusInProgress := models.CustomStatus{
		ProjectID:    proj.ID,
		Name:         "In Progress",
		Color:        "#1E90FF",
		DisplayOrder: 1,
	}
	if err := tx.Create(&statusTodo).Error; err != nil {
		t.Fatalf("failed to create statusTodo: %v", err)
	}
	if err := tx.Create(&statusInProgress).Error; err != nil {
		t.Fatalf("failed to create statusInProgress: %v", err)
	}

	// Seed Sprint
	sprint1 := models.Sprint{
		ProjectID:   proj.ID,
		Name:        "Sprint 1",
		Status:      "active",
		CreatedByID: user1.ID,
	}
	if err := tx.Create(&sprint1).Error; err != nil {
		t.Fatalf("failed to create sprint: %v", err)
	}

	// Seed Tasks
	// Task 1: Todo, Assignee 1, Sprint 1, High priority, task type
	t1 := models.Task{
		ProjectID:      proj.ID,
		StatusID:       statusTodo.ID,
		Status:         statusTodo.Name,
		AssigneeID:     &user1.ID,
		SprintID:       &sprint1.ID,
		Priority:       "high",
		Type:           "task",
		Title:          "Task 1",
		Key:            "TEST-1",
		SequenceNumber: 1,
	}
	// Task 2: In Progress, Assignee 2, Sprint nil, Medium priority, bug type
	t2 := models.Task{
		ProjectID:      proj.ID,
		StatusID:       statusInProgress.ID,
		Status:         statusInProgress.Name,
		AssigneeID:     &user2.ID,
		SprintID:       nil,
		Priority:       "medium",
		Type:           "bug",
		Title:          "Task 2",
		Key:            "TEST-2",
		SequenceNumber: 2,
	}
	// Task 3: Todo, Assignee nil, Sprint nil, High priority, task type
	t3 := models.Task{
		ProjectID:      proj.ID,
		StatusID:       statusTodo.ID,
		Status:         statusTodo.Name,
		AssigneeID:     nil,
		SprintID:       nil,
		Priority:       "high",
		Type:           "task",
		Title:          "Task 3",
		Key:            "TEST-3",
		SequenceNumber: 3,
	}

	if err := tx.Create(&t1).Error; err != nil {
		t.Fatalf("failed to create task 1: %v", err)
	}
	if err := tx.Create(&t2).Error; err != nil {
		t.Fatalf("failed to create task 2: %v", err)
	}
	if err := tx.Create(&t3).Error; err != nil {
		t.Fatalf("failed to create task 3: %v", err)
	}

	repo := taskrepo.InitTaskRepository(models.Config{
		Database: tx,
		Logger:   zap.NewNop(),
	})

	t.Run("status=todo,in_progress -> matches both statuses", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			StatusIDs: []uuid.UUID{statusTodo.ID, statusInProgress.ID},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		if len(res) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(res))
		}
	})

	t.Run("priority=high,critical -> matches high priority tasks", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			Priority: []string{"high", "critical"},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res))
		}
		for _, task := range res {
			if task.Priority != "high" {
				t.Errorf("expected priority high, got %s", task.Priority)
			}
		}
	})

	t.Run("status=todo,in_progress AND priority=high,critical -> matches tasks with (Todo OR In Progress) AND High priority", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			StatusIDs: []uuid.UUID{statusTodo.ID, statusInProgress.ID},
			Priority:  []string{"high", "critical"},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		// Matches Task 1 and Task 3
		if len(res) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res))
		}
	})

	t.Run("assignee_id=user1,user2 -> matches both assignees", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			Assignee: []string{user1.ID.String(), user2.ID.String()},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		// Matches Task 1 and Task 2
		if len(res) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res))
		}
	})

	t.Run("assignee_id=none -> matches unassigned tasks", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			Assignee: []string{"none"},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		// Matches Task 3
		if len(res) != 1 {
			t.Errorf("expected 1 task, got %d", len(res))
		}
		if res[0].ID != t3.ID {
			t.Errorf("expected task %s, got %s", t3.Title, res[0].Title)
		}
	})

	t.Run("assignee_id=user1,none -> matches user1 OR unassigned", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			Assignee: []string{user1.ID.String(), "none"},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		// Matches Task 1 and Task 3
		if len(res) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res))
		}
	})

	t.Run("duplicate statuses -> deduplicated correctly", func(t *testing.T) {
		res, _, err := repo.GetTasks(proj.ID, dto.TaskFilter{
			StatusIDs: []uuid.UUID{statusTodo.ID, statusTodo.ID},
		})
		if err != nil {
			t.Fatalf("GetTasks failed: %v", err)
		}
		// Matches Task 1 and Task 3
		if len(res) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res))
		}
	})
}

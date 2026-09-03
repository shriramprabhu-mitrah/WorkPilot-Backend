package authrepo_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	configs "github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/drivers/postgres"
	"github.com/ms-kanban-server/internal/pkg/models"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"go.uber.org/zap"
)

func loadEnvSH() {
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

func TestCreateUser_NullRoleWhenNoDefaultRole(t *testing.T) {
	loadEnvSH()
	cfg := configs.LoadEnv()
	if cfg.Database.Host == "" {
		t.Skip("Skipping integration test: DB_HOST environment variable not set")
	}

	db, err := postgres.InitDB(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	repo := authrepo.InitAuthRepository(models.Config{
		Database: db,
		Logger:   zap.NewNop(),
	})

	testEmail := "test_null_role_" + uuid.Must(uuid.NewV4()).String()[:8] + "@example.com"
	testUsername := "user_null_" + uuid.Must(uuid.NewV4()).String()[:8]

	user := models.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        testEmail,
		UserName:     testUsername,
		FullName:     "Test Null Role User",
		PasswordHash: "hashedpass",
		RoleID:       uuid.Nil,
	}

	createErr := repo.CreateUser(user)
	if createErr != nil {
		t.Fatalf("expected CreateUser to succeed with NULL role_id, got error: %v", createErr)
	}

	// Clean up created user
	db.Exec("DELETE FROM users WHERE id = ?", user.ID)
}

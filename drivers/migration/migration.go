package migration

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"gorm.io/gorm"
)

func RunOutboxMigration(dbConn *gorm.DB) error {
	// Check if orphaned_files table exists
	var tableExists bool
	if err := dbConn.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'orphaned_files')").Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		// Table doesn't exist yet, AutoMigrate will create it with the new schema. Nothing to migrate.
		return nil
	}

	// Check if next_attempt_at column exists
	var columnExists bool
	if err := dbConn.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'orphaned_files' AND column_name = 'next_attempt_at')").Scan(&columnExists).Error; err != nil {
		return err
	}
	if !columnExists {
		// Column next_attempt_at doesn't exist, migration has already run or table is new.
		return nil
	}

	return dbConn.Transaction(func(tx *gorm.DB) error {
		// 1. Add column as nullable first to allow smooth transition
		if err := tx.Exec("ALTER TABLE orphaned_files ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ").Error; err != nil {
			return err
		}

		// 2. Backfill existing columns
		if err := tx.Exec("UPDATE orphaned_files SET available_at = COALESCE(next_attempt_at, created_at, NOW()) WHERE available_at IS NULL").Error; err != nil {
			return err
		}

		// 3. Apply NOT NULL constraint
		if err := tx.Exec("ALTER TABLE orphaned_files ALTER COLUMN available_at SET NOT NULL").Error; err != nil {
			return err
		}

		// 4. Drop legacy columns
		if err := tx.Exec("ALTER TABLE orphaned_files DROP COLUMN IF EXISTS claimed_until").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE orphaned_files DROP COLUMN IF EXISTS next_attempt_at").Error; err != nil {
			return err
		}

		// 5. Clean up old indexes and apply new index
		if err := tx.Exec("DROP INDEX IF EXISTS idx_orphaned_files_cleanup").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_orphaned_files_available ON orphaned_files (available_at, created_at)").Error; err != nil {
			return err
		}

		return nil
	})
}

func AutoMigrate(dbConn *gorm.DB) error {
	// Run custom outbox migration first
	if err := RunOutboxMigration(dbConn); err != nil {
		return err
	}

	if err := dbConn.Exec("DROP INDEX IF EXISTS idx_project_task_key").Error; err != nil {
		return err
	}

	err := dbConn.AutoMigrate(
		&models.Organization{},
		&models.User{},
		&models.RefreshToken{},
		&models.OrganizationInvitation{},
		&models.AuditLog{},
		&models.Project{},
		&models.ProjectMember{},
		&models.Sprint{},
		&models.Task{},
		&models.SprintSnapshot{},
		&models.Label{},
		&models.Comments{},
		&models.TaskAttachment{},
		&models.CommentAttachment{},
		&models.OrphanedFile{},
	)
	if err != nil {
		return err
	}

	return nil
}

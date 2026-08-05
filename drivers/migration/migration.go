package migration

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"gorm.io/gorm"
)

func AutoMigrate(dbConn *gorm.DB) error {
	// Perform auto-migration for your models here

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
	)
	if err != nil {
		return err
	}

	return nil
}

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
		&models.UserStory{},
		&models.SprintSnapshot{},
		&models.Label{},
		&models.Comments{},
		&models.TaskAttachment{},
		&models.CommentAttachment{},
		&models.UserStoryAttachment{},
		&models.OrphanedFile{},
		&models.CustomStatus{},
		&models.UserStoryStatus{},
	)
	if err != nil {
		return err
	}

	return MigrateGlobalSerialNumbers(dbConn)
}

func MigrateGlobalSerialNumbers(dbConn *gorm.DB) error {
	isPostgres := dbConn.Dialector.Name() == "postgres"
	if isPostgres {
		if err := dbConn.Exec("CREATE SEQUENCE IF NOT EXISTS global_work_item_serial_seq START WITH 1 INCREMENT BY 1;").Error; err != nil {
			return err
		}
	}

	var currentMax int64
	var maxTask, maxStory int64
	_ = dbConn.Model(&models.Task{}).Unscoped().Select("COALESCE(MAX(serial_number), 0)").Scan(&maxTask).Error
	_ = dbConn.Model(&models.UserStory{}).Unscoped().Select("COALESCE(MAX(serial_number), 0)").Scan(&maxStory).Error
	if maxStory > maxTask {
		currentMax = maxStory
	} else {
		currentMax = maxTask
	}

	var unassignedTasks []models.Task
	var unassignedStories []models.UserStory
	_ = dbConn.Model(&models.Task{}).Unscoped().Where("serial_number IS NULL OR serial_number = 0").Order("created_at ASC, id ASC").Find(&unassignedTasks).Error
	_ = dbConn.Model(&models.UserStory{}).Unscoped().Where("serial_number IS NULL OR serial_number = 0").Order("created_at ASC, id ASC").Find(&unassignedStories).Error

	type itemRef struct {
		isTask    bool
		id        interface{}
		createdAt interface{}
	}

	var allUnassigned []itemRef
	for _, t := range unassignedTasks {
		allUnassigned = append(allUnassigned, itemRef{isTask: true, id: t.ID, createdAt: t.CreatedAt})
	}
	for _, s := range unassignedStories {
		allUnassigned = append(allUnassigned, itemRef{isTask: false, id: s.ID, createdAt: s.CreatedAt})
	}

	if len(allUnassigned) > 0 {
		for _, item := range allUnassigned {
			currentMax++
			if item.isTask {
				_ = dbConn.Model(&models.Task{}).Unscoped().Where("id = ?", item.id).Update("serial_number", currentMax).Error
			} else {
				_ = dbConn.Model(&models.UserStory{}).Unscoped().Where("id = ?", item.id).Update("serial_number", currentMax).Error
			}
		}
	}

	if isPostgres {
		if currentMax == 0 {
			_ = dbConn.Exec("SELECT setval('global_work_item_serial_seq', 1, false);").Error
		} else {
			_ = dbConn.Exec("SELECT setval('global_work_item_serial_seq', ?, true);", currentMax).Error
		}
	}

	return nil
}

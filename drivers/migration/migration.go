package migration

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"gorm.io/gorm"
)

func AutoMigrate(dbConn *gorm.DB) error {
	// Perform auto-migration for your models here
	err := dbConn.AutoMigrate(
		&models.Role{},
		&models.Permission{},
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

	if err := SeedDefaultRoles(dbConn); err != nil {
		return err
	}

	return MigrateGlobalSerialNumbers(dbConn)
}

func SeedDefaultRoles(dbConn *gorm.DB) error {
	defaultRoles := []models.Role{
		{
			Name:        "super_admin",
			Description: "Super Administrator with platform-wide access",
			IsSystem:    true,
		},
		{
			Name:        "org_admin",
			Description: "Organization Administrator with full control over the organization",
			IsSystem:    true,
		},
		{
			Name:        "project_manager",
			Description: "Project Manager with full control over assigned projects",
			IsSystem:    true,
		},
		{
			Name:        "developer",
			Description: "Developer who can view projects and manage tasks/user stories",
			IsSystem:    true,
		},
		{
			Name:        "qa",
			Description: "QA/Tester who can test tasks and report issues",
			IsSystem:    true,
		},
		{
			Name:        "stakeholder",
			Description: "Stakeholder with read-only access to projects",
			IsSystem:    true,
		},
	}

	for _, r := range defaultRoles {
		var count int64
		if err := dbConn.Model(&models.Role{}).Where("name = ? AND organization_id IS NULL", r.Name).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := dbConn.Create(&r).Error; err != nil {
				return err
			}
		}
	}
	return nil
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

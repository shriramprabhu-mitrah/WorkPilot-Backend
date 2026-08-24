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
		&models.Favorite{},
	)
	if err != nil {
		return err
	}

	err = MigrateProjectSlugs(dbConn)
	if err != nil {
		return err
	}

	err = MigrateUserStoryKeys(dbConn)
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

func MigrateProjectSlugs(dbConn *gorm.DB) error {
	var projects []models.Project
	// Fetch all projects (including soft-deleted) that do not have a slug set
	err := dbConn.Unscoped().Where("slug IS NULL OR slug = ''").Find(&projects).Error
	if err != nil {
		return fmt.Errorf("failed to fetch projects without slug: %w", err)
	}

	for _, p := range projects {
		slug := utils.Slugify(p.Name)
		if slug == "" {
			slug = "project"
		}

		uniqueSlug := slug
		suffix := 1
		for {
			var count int64
			// Check if this slug is taken by any other project (excluding current project)
			err := dbConn.Unscoped().Model(&models.Project{}).Where("slug = ? AND id != ?", uniqueSlug, p.ID).Count(&count).Error
			if err != nil {
				return fmt.Errorf("failed to check slug uniqueness: %w", err)
			}
			if count == 0 {
				break
			}
			uniqueSlug = fmt.Sprintf("%s-%d", slug, suffix)
			suffix++
		}

		err = dbConn.Unscoped().Model(&models.Project{}).Where("id = ?", p.ID).Update("slug", uniqueSlug).Error
		if err != nil {
			return fmt.Errorf("failed to update project slug for project %s: %w", p.ID, err)
		}
	}

	// Create unique index for active projects manually
	isPostgres := dbConn.Dialector.Name() == "postgres"
	if isPostgres {
		err = dbConn.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug ON projects (slug) WHERE deleted_at IS NULL;").Error
		if err != nil {
			return fmt.Errorf("failed to create unique index on project slugs: %w", err)
		}
	} else {
		// Fallback for tests or other dialects
		_ = dbConn.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug ON projects (slug);")
	}

	return nil
}

func MigrateUserStoryKeys(dbConn *gorm.DB) error {
	var stories []models.UserStory
	// Fetch all user stories (including soft-deleted) that do not have a key set
	err := dbConn.Unscoped().Where("key IS NULL OR key = ''").Order("created_at ASC, id ASC").Find(&stories).Error
	if err != nil {
		return fmt.Errorf("failed to fetch user stories without key: %w", err)
	}

	for _, s := range stories {
		var maxSeq int64
		// Get max sequence number for this project so far (including soft-deleted)
		err := dbConn.Unscoped().Model(&models.UserStory{}).
			Where("project_id = ? AND key IS NOT NULL AND key != ''", s.ProjectID).
			Select("COALESCE(MAX(sequence_number), 0)").
			Scan(&maxSeq).Error
		if err != nil {
			return fmt.Errorf("failed to calculate next user story sequence number: %w", err)
		}

		nextSeq := int(maxSeq) + 1
		key := fmt.Sprintf("US-%d", nextSeq)

		err = dbConn.Unscoped().Model(&models.UserStory{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
			"sequence_number": nextSeq,
			"key":             key,
		}).Error
		if err != nil {
			return fmt.Errorf("failed to update user story key for ID %s: %w", s.ID, err)
		}
	}

	// Create composite unique index for active user stories manually
	isPostgres := dbConn.Dialector.Name() == "postgres"
	if isPostgres {
		err = dbConn.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_project_user_story_key ON user_stories (project_id, key) WHERE deleted_at IS NULL;").Error
		if err != nil {
			return fmt.Errorf("failed to create unique index on user story keys: %w", err)
		}
	} else {
		// Fallback for tests/sqlite
		_ = dbConn.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_project_user_story_key ON user_stories (project_id, key);")
	}

	return nil
}

package searchrepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SearchRepository interface {
	SearchTasks(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Task, error)
	SearchUserStories(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.UserStory, error)
	SearchProjects(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Project, error)
	SearchMembers(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.User, error)
	SearchSprints(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Sprint, error)
}

type searchRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitSearchRepository(deps models.Config) SearchRepository {
	return &searchRepository{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

func (r *searchRepository) SearchTasks(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Task, error) {
	var tasks []models.Task

	query := r.db.Table("tasks").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Where("projects.organization_id = ? AND tasks.deleted_at IS NULL AND projects.deleted_at IS NULL", orgID)

	if tsQuery != "" {
		query = query.Where(
			"(to_tsvector('english', coalesce(tasks.title, '') || ' ' || coalesce(tasks.description, '') || ' ' || coalesce(tasks.key, '')) @@ to_tsquery('english', ?)) OR tasks.key ILIKE ? OR CAST(tasks.serial_number AS TEXT) = ?",
			tsQuery, "%"+rawQuery+"%", rawQuery,
		)
	} else {
		query = query.Where(
			"tasks.key ILIKE ? OR tasks.title ILIKE ? OR CAST(tasks.serial_number AS TEXT) = ?",
			"%"+rawQuery+"%", "%"+rawQuery+"%", rawQuery,
		)
	}

	err := query.Preload("Project").Limit(20).Find(&tasks).Error
	if err != nil {
		r.logger.Error("failed to search tasks", zap.Error(err), zap.String("orgID", orgID.String()))
		return nil, err
	}

	return tasks, nil
}

func (r *searchRepository) SearchUserStories(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.UserStory, error) {
	var stories []models.UserStory

	query := r.db.Table("user_stories").
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("projects.organization_id = ? AND user_stories.deleted_at IS NULL AND projects.deleted_at IS NULL", orgID)

	if tsQuery != "" {
		query = query.Where(
			"(to_tsvector('english', coalesce(user_stories.title, '') || ' ' || coalesce(user_stories.description, '') || ' ' || coalesce(user_stories.key, '')) @@ to_tsquery('english', ?)) OR user_stories.key ILIKE ? OR CAST(user_stories.serial_number AS TEXT) = ?",
			tsQuery, "%"+rawQuery+"%", rawQuery,
		)
	} else {
		query = query.Where(
			"user_stories.key ILIKE ? OR user_stories.title ILIKE ? OR CAST(user_stories.serial_number AS TEXT) = ?",
			"%"+rawQuery+"%", "%"+rawQuery+"%", rawQuery,
		)
	}

	err := query.Preload("Project").Limit(20).Find(&stories).Error
	if err != nil {
		r.logger.Error("failed to search user stories", zap.Error(err), zap.String("orgID", orgID.String()))
		return nil, err
	}

	return stories, nil
}

func (r *searchRepository) SearchProjects(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Project, error) {
	var projects []models.Project

	query := r.db.Table("projects").
		Where("organization_id = ? AND deleted_at IS NULL", orgID)

	if tsQuery != "" {
		query = query.Where(
			"(to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, '') || ' ' || coalesce(slug, '')) @@ to_tsquery('english', ?)) OR name ILIKE ? OR slug ILIKE ?",
			tsQuery, "%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	} else {
		query = query.Where(
			"name ILIKE ? OR slug ILIKE ?",
			"%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	}

	err := query.Limit(20).Find(&projects).Error
	if err != nil {
		r.logger.Error("failed to search projects", zap.Error(err), zap.String("orgID", orgID.String()))
		return nil, err
	}

	return projects, nil
}

func (r *searchRepository) SearchMembers(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.User, error) {
	var users []models.User

	query := r.db.Table("users").
		Where("organization_id = ? AND deleted_at IS NULL", orgID)

	if tsQuery != "" {
		query = query.Where(
			"(to_tsvector('english', coalesce(full_name, '') || ' ' || coalesce(email, '') || ' ' || coalesce(username, '')) @@ to_tsquery('english', ?)) OR full_name ILIKE ? OR username ILIKE ? OR email ILIKE ?",
			tsQuery, "%"+rawQuery+"%", "%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	} else {
		query = query.Where(
			"full_name ILIKE ? OR username ILIKE ? OR email ILIKE ?",
			"%"+rawQuery+"%", "%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	}

	err := query.Limit(20).Find(&users).Error
	if err != nil {
		r.logger.Error("failed to search members", zap.Error(err), zap.String("orgID", orgID.String()))
		return nil, err
	}

	return users, nil
}

func (r *searchRepository) SearchSprints(orgID uuid.UUID, tsQuery string, rawQuery string) ([]models.Sprint, error) {
	var sprints []models.Sprint

	query := r.db.Table("sprints").Joins("JOIN projects ON projects.id = sprints.project_id").
		Where("projects.organization_id = ? AND sprints.deleted_at IS NULL AND projects.deleted_at IS NULL", orgID)

	if tsQuery != "" {
		query = query.Where(
			"(to_tsvector('english', coalesce(sprints.name, '') || ' ' || coalesce(sprints.goal, '')) @@ to_tsquery('english', ?)) OR sprints.name ILIKE ? OR sprints.goal ILIKE ?",
			tsQuery, "%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	} else {
		query = query.Where(
			"sprints.name ILIKE ? OR sprints.goal ILIKE ?",
			"%"+rawQuery+"%", "%"+rawQuery+"%",
		)
	}

	err := query.Limit(20).Find(&sprints).Error
	if err != nil {
		r.logger.Error("failed to search sprints", zap.Error(err), zap.String("orgID", orgID.String()))
		return nil, err
	}

	return sprints, nil
}

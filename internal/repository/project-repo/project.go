package projectrepo

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *projectDatabase) CreateProjectWithMember(project *models.Project, projectMember *models.ProjectMember) *response.Error {

	tx := d.db.Begin()
	if tx.Error != nil {
		d.logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Create Project
	if err := tx.Create(&project).Error; err != nil {
		tx.Rollback()

		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Project duplicate key conflict", zap.Error(err))
			return utils.ParseProjectDuplicateError(err)
		}

		d.logger.Error("Failed to create project", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Assign generated project ID to project member
	projectMember.ProjectID = project.ID

	// Create Project Member
	if err := tx.Create(&projectMember).Error; err != nil {
		tx.Rollback()

		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Project member duplicate key conflict", zap.Error(err))
			return utils.ParseProjectMemberDuplicateError(err)
		}

		d.logger.Error("Failed to create project member", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Create default statuses for the project
	defaultStatuses := []models.CustomStatus{
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Todo",
			Color:        "#808080",
			DisplayOrder: 0,
			IsDefault:    true,
			IsFinal:      false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "In Progress",
			Color:        "#1E90FF",
			DisplayOrder: 1,
			IsDefault:    true,
			IsFinal:      false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "In Review",
			Color:        "#FF8C00",
			DisplayOrder: 2,
			IsDefault:    true,
			IsFinal:      false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Testing",
			Color:        "#8A2BE2",
			DisplayOrder: 3,
			IsDefault:    true,
			IsFinal:      false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Completed",
			Color:        "#228B22",
			DisplayOrder: 4,
			IsDefault:    true,
			IsFinal:      true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Blocked",
			Color:        "#DC143C",
			DisplayOrder: 5,
			IsDefault:    true,
			IsFinal:      false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, ds := range defaultStatuses {
		if err := tx.Create(&ds).Error; err != nil {
			tx.Rollback()
			d.logger.Error("Failed to create default status", zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}
		}
	}

	// Create default statuses for user stories
	defaultUserStoryStatuses := []models.UserStoryStatus{
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Todo",
			Color:        "#808080",
			DisplayOrder: 0,
			IsDefault:    true,
			IsClosed:     false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "In Progress",
			Color:        "#1E90FF",
			DisplayOrder: 1,
			IsDefault:    true,
			IsClosed:     false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "In Review",
			Color:        "#FF8C00",
			DisplayOrder: 2,
			IsDefault:    true,
			IsClosed:     false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Testing",
			Color:        "#8A2BE2",
			DisplayOrder: 3,
			IsDefault:    true,
			IsClosed:     false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Completed",
			Color:        "#228B22",
			DisplayOrder: 4,
			IsDefault:    true,
			IsClosed:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.Must(uuid.NewV7()),
			ProjectID:    project.ID,
			Name:         "Blocked",
			Color:        "#DC143C",
			DisplayOrder: 5,
			IsDefault:    true,
			IsClosed:     false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, ds := range defaultUserStoryStatuses {
		if err := tx.Create(&ds).Error; err != nil {
			tx.Rollback()
			d.logger.Error("Failed to create default user story status", zap.Error(err))
			return &response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Something went wrong. Please try again later.",
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		d.logger.Error("Failed to commit transaction", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return nil
}

func (d *projectDatabase) UpdateProject(projectID uuid.UUID, updates map[string]interface{}) *response.Error {

	result := d.db.
		Model(&models.Project{}).
		Where("id = ?", projectID).
		Updates(updates)
	if result.Error != nil {

		d.logger.Error("Database error occurred",
			zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("ProjectID not found",
			zap.String("user_id", projectID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Project not found",
		}
	}

	return nil
}

func (d *projectDatabase) GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {

	var projects []models.Project
	var totalItems int64

	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.db.Model(&models.Project{}).
		Where("organization_id = ?", organizationID)

	if filter.Name != "" {
		name := "%" + strings.ToLower(strings.TrimSpace(filter.Name)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ?", name)
	}

	if filter.Status != "" {
		baseQuery = baseQuery.Where(
			"LOWER(status) = ?",
			strings.ToLower(strings.TrimSpace(filter.Status)),
		)
	}

	// Determine order clause based on sorting parameters
	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"name":       "name",
			"created_at": "created_at",
			"updated_at": "updated_at",
			"status":     "status",
		}
		if col, ok := allowed[filter.SortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", col, direction)
		}
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred",
			zap.String("Organization Id", organizationID.String()),
			zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Order(orderClause).
		Limit(filter.PageSize).
		Offset(offset).
		Find(&projects).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("Organization Id", organizationID.String()),
			zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return projects, pagination, nil
}

func (d *projectDatabase) GetAllProjects(filter dto.GlobalProjectFilterRequest) ([]models.Project, response.Pagination, *response.Error) {
	var projects []models.Project
	var totalItems int64

	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.db.Model(&models.Project{})

	if filter.OrganizationID != nil && *filter.OrganizationID != uuid.Nil {
		baseQuery = baseQuery.Where("organization_id = ?", *filter.OrganizationID)
	}

	if filter.CreatedBy != nil && *filter.CreatedBy != uuid.Nil {
		baseQuery = baseQuery.Where("created_by = ?", *filter.CreatedBy)
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", searchTerm, searchTerm)
	}

	if filter.Name != "" {
		name := "%" + strings.ToLower(strings.TrimSpace(filter.Name)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ?", name)
	}

	if filter.Status != "" {
		baseQuery = baseQuery.Where(
			"LOWER(status) = ?",
			strings.ToLower(strings.TrimSpace(string(filter.Status))),
		)
	}

	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"name":       "name",
			"created_at": "created_at",
			"updated_at": "updated_at",
			"status":     "status",
		}
		if col, ok := allowed[filter.SortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", col, direction)
		}
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred while counting projects", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("Organization").
		Preload("Creator").
		Order(orderClause).
		Limit(filter.PageSize).
		Offset(offset).
		Find(&projects).Error; err != nil {
		d.logger.Error("Database error occurred while fetching projects", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return projects, pagination, nil
}

func (d *projectDatabase) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {

	var row models.Project

	err := d.db.
		Where("id = ?", id).
		Preload("Organization").
		Preload("Creator").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Project not found",
			}
			d.logger.Error("Project not found in database",
				zap.String("Id", id.String()),
				zap.Error(err))
			return models.Project{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", id.String()),
			zap.Error(err))
		return models.Project{}, &errorResponse
	}
	return row, nil
}

func (d *projectDatabase) GetProjectActivity(projectID uuid.UUID, filter dto.ProjectActivityFilter) ([]models.AuditLog, response.Pagination, *response.Error) {
	var logs []models.AuditLog
	var totalItems int64

	// Normalize pagination
	filter.PaginationQuery.Normalize(10)

	// Maximum page size
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	offset := (filter.Page - 1) * filter.PageSize

	// Always enforce project-level scoping.
	// This prevents activity from another project from being returned.
	baseQuery := d.db.
		Model(&models.AuditLog{}).
		Where("project_id = ?", projectID)

	// Filter by Action
	if filter.Action != "" {
		action := strings.ToLower(strings.TrimSpace(filter.Action))

		baseQuery = baseQuery.Where(
			"LOWER(action) = ?",
			action,
		)
	}

	// Filter by User / Actor
	if filter.UserID != nil && *filter.UserID != uuid.Nil {
		baseQuery = baseQuery.Where(
			"user_id = ?",
			*filter.UserID,
		)
	}

	// Filter by Task
	if filter.TaskID != nil && *filter.TaskID != uuid.Nil {
		taskID := *filter.TaskID
		taskIDStr := taskID.String()

		baseQuery = baseQuery.Where(
			"task_id = ? OR "+
				"(LOWER(resource_type) IN ('task', 'task_attachment', 'comment') "+
				"AND resource_id = ?)",
			taskID,
			taskIDStr,
		)
	}

	// Filter by User Story
	if filter.UserStoryID != nil && *filter.UserStoryID != uuid.Nil {
		userStoryID := *filter.UserStoryID
		userStoryIDStr := userStoryID.String()

		baseQuery = baseQuery.Where(
			"user_story_id = ? OR "+
				"(LOWER(resource_type) IN "+
				"('user_story', 'userstory', 'user_story_attachment', 'comment') "+
				"AND resource_id = ?)",
			userStoryID,
			userStoryIDStr,
		)
	}

	// Filter by Sprint
	if filter.SprintID != nil && *filter.SprintID != uuid.Nil {
		sprintID := *filter.SprintID
		sprintIDStr := sprintID.String()

		baseQuery = baseQuery.Where(
			"sprint_id = ? OR "+
				"(LOWER(resource_type) IN ('sprint', 'sprints') "+
				"AND resource_id = ?)",
			sprintID,
			sprintIDStr,
		)
	}

	// Filter by Resource Type
	resType := strings.ToLower(strings.TrimSpace(filter.ResourceType))

	switch resType {
	case "task":
		baseQuery = baseQuery.Where(
			"task_id IS NOT NULL OR " +
				"LOWER(resource_type) IN ('task', 'task_attachment')",
		)

	case "userstory", "user_story":
		baseQuery = baseQuery.Where(
			"user_story_id IS NOT NULL OR " +
				"LOWER(resource_type) IN " +
				"('user_story', 'userstory', 'user_story_attachment')",
		)

	case "sprint":
		baseQuery = baseQuery.Where(
			"sprint_id IS NOT NULL OR " +
				"LOWER(resource_type) IN ('sprint', 'sprints')",
		)

	case "project":
		baseQuery = baseQuery.Where(
			"LOWER(resource_type) IN ('project', 'project_member')",
		)

	case "all", "":
		// Project is already strictly scoped using project_id.
		// No additional resource_type condition is required.

	case "comments", "comment":
		baseQuery = baseQuery.Where(
			"LOWER(resource_type) IN " +
				"('comment', 'comments', 'comment_attachment') " +
				"OR LOWER(action) LIKE '%comment%' " +
				"OR LOWER(action) LIKE '%reply%'",
		)

	default:
		if resType != "" {
			baseQuery = baseQuery.Where(
				"LOWER(resource_type) = ?",
				resType,
			)
		}
	}

	// Filter by Resource ID
	if filter.ResourceID != "" {
		resourceID := strings.TrimSpace(filter.ResourceID)

		baseQuery = baseQuery.Where(
			"resource_id = ? OR task_id = ? OR user_story_id = ? OR sprint_id = ?",
			resourceID,
			resourceID,
			resourceID,
			resourceID,
		)
	}

	// Filter by Start Date
	if filter.StartDate != "" {
		baseQuery = baseQuery.Where(
			"created_at >= ?",
			filter.StartDate,
		)
	}

	// Filter by End Date
	if filter.EndDate != "" {
		baseQuery = baseQuery.Where(
			"created_at <= ?",
			filter.EndDate,
		)
	}

	// Filter by Audit Log Type
	if filter.Type != "" {
		logType := strings.ToLower(strings.TrimSpace(filter.Type))

		switch logType {

		case string(models.AuditLogTypeView):
			// View logs can either explicitly have type=view
			// or have an action containing "view".
			baseQuery = baseQuery.Where(
				"type = ? OR LOWER(action) LIKE '%view%'",
				models.AuditLogTypeView,
			)

		case string(models.AuditLogTypeActivity):
			// Activity includes normal actions and legacy records
			// where type is NULL/empty, but excludes view actions.
			baseQuery = baseQuery.Where(
				"(type = ? OR type IS NULL OR type = '') "+
					"AND LOWER(action) NOT LIKE '%view%'",
				models.AuditLogTypeActivity,
			)

		case string(models.AuditLogTypeAudit):
			baseQuery = baseQuery.Where(
				"type = ?",
				models.AuditLogTypeAudit,
			)

		default:
			// Fallback to activity for unknown type values.
			baseQuery = baseQuery.Where(
				"(type = ? OR type IS NULL OR type = '') "+
					"AND LOWER(action) NOT LIKE '%view%'",
				models.AuditLogTypeActivity,
			)
		}
	}

	// Count total records
	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error(
			"Database error counting project activity logs",
			zap.String("project_id", projectID.String()),
			zap.Error(err),
		)

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Fetch paginated activity logs
	if err := baseQuery.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			// Include soft-deleted users so historical activity
			// still contains the actor information.
			return db.Unscoped()
		}).
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&logs).Error; err != nil {

		d.logger.Error(
			"Database error finding project activity logs",
			zap.String("project_id", projectID.String()),
			zap.Error(err),
		)

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Populate additional audit-log details
	populateAuditLogDetails(d.db, logs)

	// Calculate pagination
	totalPages := int(
		math.Ceil(
			float64(totalItems) / float64(filter.PageSize),
		),
	)

	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return logs, pagination, nil
}

func (d *projectDatabase) DeleteProject(projectID, organizationID uuid.UUID) *response.Error {

	tx := d.db.Begin()
	if tx.Error != nil {
		d.logger.Error("Failed to begin project deletion transaction",
			zap.Error(tx.Error),
			zap.String("project_id", projectID.String()),
			zap.String("organization_id", organizationID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// Rollback automatically if anything panics.
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Verify project exists and belongs to organization.
	var project models.Project

	result := tx.
		Where("id = ? AND organization_id = ?", projectID, organizationID).
		First(&project)

	if result.Error != nil {
		tx.Rollback()

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			d.logger.Warn("Project not found for deletion",
				zap.String("project_id", projectID.String()),
				zap.String("organization_id", organizationID.String()),
			)

			return &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Project not found",
			}
		}

		d.logger.Error("Failed to find project for deletion",
			zap.Error(result.Error),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	// 2. Delete comments related to project.
	if err := tx.
		Where("project_id = ?", projectID).
		Delete(&models.Comments{}).Error; err != nil {

		tx.Rollback()

		d.logger.Error("Failed to delete project comments",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete project data",
		}
	}

	// 3. Delete tasks related to project.
	if err := tx.
		Where("project_id = ?", projectID).
		Delete(&models.Task{}).Error; err != nil {

		tx.Rollback()

		d.logger.Error("Failed to delete project tasks",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete project data",
		}
	}

	// 4. Delete sprints related to project.
	if err := tx.
		Where("project_id = ?", projectID).
		Delete(&models.Sprint{}).Error; err != nil {

		tx.Rollback()

		d.logger.Error("Failed to delete project sprints",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete project data",
		}
	}

	// 5. Delete project members.
	if err := tx.
		Where("project_id = ?", projectID).
		Delete(&models.ProjectMember{}).Error; err != nil {

		tx.Rollback()

		d.logger.Error("Failed to delete project members",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete project data",
		}
	}

	// 6. Delete labels.
	if err := tx.
		Where("project_id = ?", projectID).
		Delete(&models.Label{}).Error; err != nil {

		tx.Rollback()

		d.logger.Error("Failed to delete project labels",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to delete project data",
		}
	}

	// 7. Delete the project itself.
	result = tx.
		Where("id = ? AND organization_id = ?", projectID, organizationID).
		Delete(&models.Project{})

	if result.Error != nil {
		tx.Rollback()

		d.logger.Error("Failed to delete project",
			zap.Error(result.Error),
			zap.String("project_id", projectID.String()),
			zap.String("organization_id", organizationID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		tx.Rollback()

		d.logger.Warn("Project could not be found for deletion",
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Project not found",
		}
	}

	// 8. Commit transaction.
	if err := tx.Commit().Error; err != nil {
		d.logger.Error("Failed to commit project deletion",
			zap.Error(err),
			zap.String("project_id", projectID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	d.logger.Info("Project deleted successfully",
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	return nil
}

func (d *projectDatabase) GetProjectsByUserID(userID uuid.UUID) ([]models.ProjectMember, *response.Error) {

	var projects []models.ProjectMember

	if err := d.db.
		Preload("Project").
		Where("user_id = ?", userID).
		Find(&projects).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("User ID", userID.String()),
			zap.Error(err))

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return projects, nil
}

func (d *projectDatabase) GetProjectMemberByUserAndProjectID(userID, projectID uuid.UUID) (*models.ProjectMember, *response.Error) {

	var member models.ProjectMember

	if err := d.db.
		Preload("Role.Permissions").
		Where("user_id = ? AND project_id = ?", userID, projectID).
		First(&member).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.logger.Warn("Project member not found",
				zap.String("User ID", userID.String()),
				zap.String("Project ID", projectID.String()))

			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Project member not found.",
			}
		}

		d.logger.Error("Database error occurred",
			zap.String("User ID", userID.String()),
			zap.String("Project ID", projectID.String()),
			zap.Error(err))

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return &member, nil
}

func populateAuditLogDetails(db *gorm.DB, logs []models.AuditLog) {
	var taskIDs []uuid.UUID
	var userStoryIDs []uuid.UUID
	var projectIDs []uuid.UUID
	var sprintIDs []uuid.UUID

	for _, log := range logs {
		if log.TaskID != nil && *log.TaskID != uuid.Nil {
			taskIDs = append(taskIDs, *log.TaskID)
		}
		if log.UserStoryID != nil && *log.UserStoryID != uuid.Nil {
			userStoryIDs = append(userStoryIDs, *log.UserStoryID)
		}
		if log.ProjectID != nil && *log.ProjectID != uuid.Nil {
			projectIDs = append(projectIDs, *log.ProjectID)
		}
		if log.SprintID != nil && *log.SprintID != uuid.Nil {
			sprintIDs = append(sprintIDs, *log.SprintID)
		}

		rID, err := uuid.FromString(log.ResourceID)
		if err == nil && rID != uuid.Nil {
			switch strings.ToLower(log.ResourceType) {
			case "task", "task_attachment":
				taskIDs = append(taskIDs, rID)
			case "user_story", "userstory", "user_story_attachment":
				userStoryIDs = append(userStoryIDs, rID)
			case "project", "project_member":
				projectIDs = append(projectIDs, rID)
			case "sprint":
				sprintIDs = append(sprintIDs, rID)
			}
		}
	}

	// Fetch task details (title, key)
	taskMap := make(map[uuid.UUID]struct{ Title, Key string })
	if len(taskIDs) > 0 {
		type TaskInfo struct {
			ID    uuid.UUID
			Title string
			Key   string
		}
		var tasks []TaskInfo
		if err := db.Unscoped().Model(&models.Task{}).Where("id IN ?", taskIDs).Select("id, title, key").Find(&tasks).Error; err == nil {
			for _, t := range tasks {
				taskMap[t.ID] = struct{ Title, Key string }{Title: t.Title, Key: t.Key}
			}
		}
	}

	// Fetch user story details (title, serial_number)
	userStoryMap := make(map[uuid.UUID]struct {
		Title        string
		SerialNumber int64
	})
	if len(userStoryIDs) > 0 {
		type StoryInfo struct {
			ID           uuid.UUID
			Title        string
			SerialNumber int64
		}
		var stories []StoryInfo
		if err := db.Unscoped().Model(&models.UserStory{}).Where("id IN ?", userStoryIDs).Select("id, title, serial_number").Find(&stories).Error; err == nil {
			for _, s := range stories {
				userStoryMap[s.ID] = struct {
					Title        string
					SerialNumber int64
				}{Title: s.Title, SerialNumber: s.SerialNumber}
			}
		}
	}

	// Fetch project details (name)
	projectMap := make(map[uuid.UUID]string)
	if len(projectIDs) > 0 {
		type ProjectInfo struct {
			ID   uuid.UUID
			Name string
		}
		var projects []ProjectInfo
		if err := db.Unscoped().Model(&models.Project{}).Where("id IN ?", projectIDs).Select("id, name").Find(&projects).Error; err == nil {
			for _, p := range projects {
				projectMap[p.ID] = p.Name
			}
		}
	}

	// Fetch sprint details (name)
	sprintMap := make(map[uuid.UUID]string)
	if len(sprintIDs) > 0 || len(projectIDs) > 0 {
		type SprintInfo struct {
			ID   uuid.UUID
			Name string
		}
		var sprints []SprintInfo
		query := db.Unscoped().Model(&models.Sprint{})
		if len(sprintIDs) > 0 && len(projectIDs) > 0 {
			query = query.Where("id IN ? OR project_id IN ?", sprintIDs, projectIDs)
		} else if len(sprintIDs) > 0 {
			query = query.Where("id IN ?", sprintIDs)
		} else if len(projectIDs) > 0 {
			query = query.Where("project_id IN ?", projectIDs)
		}
		if err := query.Select("id, name").Find(&sprints).Error; err == nil {
			for _, s := range sprints {
				sprintMap[s.ID] = s.Name
			}
		}
	}

	// Populate transient fields
	for i, log := range logs {
		if logs[i].Type == "" {
			if strings.Contains(strings.ToLower(logs[i].Action), "view") {
				logs[i].Type = models.AuditLogTypeView
			} else {
				logs[i].Type = models.AuditLogTypeActivity
			}
		}

		rID, err := uuid.FromString(log.ResourceID)
		hasRID := (err == nil && rID != uuid.Nil)

		// Always set ProjectName if project is known
		if log.ProjectID != nil {
			if name, ok := projectMap[*log.ProjectID]; ok {
				logs[i].ProjectName = name
			}
		}
		if logs[i].ProjectName == "" && hasRID {
			if name, ok := projectMap[rID]; ok {
				logs[i].ProjectName = name
			}
		}

		// 1. Try matching task (by ResourceID or TaskID)
		if hasRID {
			if t, ok := taskMap[rID]; ok {
				logs[i].Title = t.Title
				logs[i].TaskName = t.Title
				logs[i].TaskKey = t.Key
				continue
			}
		}
		if log.TaskID != nil {
			if t, ok := taskMap[*log.TaskID]; ok {
				logs[i].Title = t.Title
				logs[i].TaskName = t.Title
				logs[i].TaskKey = t.Key
				continue
			}
		}

		// 2. Try matching user story (by ResourceID or UserStoryID)
		if hasRID {
			if s, ok := userStoryMap[rID]; ok {
				logs[i].Title = s.Title
				logs[i].UserStoryName = s.Title
				continue
			}
		}
		if log.UserStoryID != nil {
			if s, ok := userStoryMap[*log.UserStoryID]; ok {
				logs[i].Title = s.Title
				logs[i].UserStoryName = s.Title
				continue
			}
		}

		// 3. Try matching sprint (by ResourceID or SprintID)
		if hasRID {
			if name, ok := sprintMap[rID]; ok {
				logs[i].Title = name
				logs[i].SprintName = name
				continue
			}
		}
		if log.SprintID != nil {
			if name, ok := sprintMap[*log.SprintID]; ok {
				logs[i].Title = name
				logs[i].SprintName = name
				continue
			}
		}
		if strings.EqualFold(log.ResourceType, "sprint") {
			if len(sprintMap) == 1 {
				for _, name := range sprintMap {
					logs[i].Title = name
					logs[i].SprintName = name
					break
				}
				continue
			}
		}

		// 4. Try matching project (by ResourceID or ProjectID fallback)
		if hasRID {
			if name, ok := projectMap[rID]; ok {
				logs[i].Title = name
				continue
			}
		}
		if log.ProjectID != nil {
			if name, ok := projectMap[*log.ProjectID]; ok {
				if logs[i].Title == "" {
					logs[i].Title = name
				}
				continue
			}
		}

		if strings.EqualFold(log.ResourceType, "comment") && strings.Contains(strings.ToLower(log.Action), "deleted") {
			logs[i].Details = "Comment deleted"
		}
	}
}

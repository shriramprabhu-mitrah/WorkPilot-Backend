package auditrepo

import (
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *auditDatabase) CreateAuditLog(log models.AuditLog) *response.Error {
	go func() {
		if err := d.db.Create(&log).Error; err != nil {
			d.logger.Error("Database error occurred while creating audit log asynchronously", zap.Error(err))
		}
	}()
	return nil
}

func (d *auditDatabase) GetAuditLogs(req requestdto.GetAudit) ([]models.AuditLog, response.Pagination, *response.Error) {

	var (
		audits     []models.AuditLog
		totalItems int64
	)

	req.PaginationQuery.Normalize(10)
	offset := (req.Page - 1) * req.PageSize

	baseQuery := d.db.Model(&models.AuditLog{})

	if req.OrganizationID != nil && *req.OrganizationID != uuid.Nil {
		baseQuery = baseQuery.Where("organization_id = ?", req.OrganizationID)
	}

	if req.ProjectID != nil && *req.ProjectID != uuid.Nil {
		baseQuery = baseQuery.Where("project_id = ?", req.ProjectID)
	}

	if req.TaskID != nil && *req.TaskID != uuid.Nil {
		baseQuery = baseQuery.Where("task_id = ? OR (LOWER(resource_type) IN ('task', 'task_attachment', 'comment') AND resource_id = ?)", *req.TaskID, req.TaskID.String())
	}

	if req.UserStoryID != nil && *req.UserStoryID != uuid.Nil {
		baseQuery = baseQuery.Where("user_story_id = ? OR (LOWER(resource_type) IN ('user_story', 'userstory', 'user_story_attachment', 'comment') AND resource_id = ?)", *req.UserStoryID, req.UserStoryID.String())
	}

	if req.ResourceType != "" {
		baseQuery = baseQuery.Where("LOWER(resource_type) = ?", strings.ToLower(strings.TrimSpace(req.ResourceType)))
	}

	if req.ResourceID != "" {
		baseQuery = baseQuery.Where("resource_id = ? OR task_id = ? OR user_story_id = ?", req.ResourceID, req.ResourceID, req.ResourceID)
	}

	if req.Type != "" && strings.ToLower(strings.TrimSpace(req.Type)) != "all" {
		auditType := strings.ToLower(strings.TrimSpace(req.Type))
		switch auditType {
		case "view":
			baseQuery = baseQuery.Where("LOWER(type) = ? OR (type IS NULL AND LOWER(action) LIKE '%view%')", auditType)
		case "activity":
			baseQuery = baseQuery.Where("LOWER(type) = ? OR (type IS NULL AND LOWER(action) NOT LIKE '%view%')", auditType)
		default:
			baseQuery = baseQuery.Where("LOWER(type) = ?", auditType)
		}
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Failed to count audit logs", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Order("created_at DESC, id DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&audits).Error; err != nil {

		d.logger.Error("Failed to fetch audit logs", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	populateAuditLogDetails(d.db, audits)

	totalPages := int(math.Ceil(
		float64(totalItems) / float64(req.PageSize),
	))

	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        req.Page,
		PageSize:    req.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     req.Page < totalPages,
		HasPrevious: req.Page > 1,
	}

	return audits, pagination, nil
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
	if len(sprintIDs) > 0 {
		type SprintInfo struct {
			ID   uuid.UUID
			Name string
		}
		var sprints []SprintInfo
		if err := db.Unscoped().Model(&models.Sprint{}).Where("id IN ?", sprintIDs).Select("id, name").Find(&sprints).Error; err == nil {
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
				logs[i].TaskKey = t.Key
				continue
			}
		}
		if log.TaskID != nil {
			if t, ok := taskMap[*log.TaskID]; ok {
				logs[i].Title = t.Title
				logs[i].TaskKey = t.Key
				continue
			}
		}

		// 2. Try matching user story (by ResourceID or UserStoryID)
		if hasRID {
			if s, ok := userStoryMap[rID]; ok {
				logs[i].Title = s.Title
				continue
			}
		}
		if log.UserStoryID != nil {
			if s, ok := userStoryMap[*log.UserStoryID]; ok {
				logs[i].Title = s.Title
				continue
			}
		}

		// 3. Try matching project (by ResourceID or ProjectID)
		if hasRID {
			if name, ok := projectMap[rID]; ok {
				logs[i].Title = name
				continue
			}
		}
		if log.ProjectID != nil {
			if name, ok := projectMap[*log.ProjectID]; ok {
				logs[i].Title = name
				continue
			}
		}

		// 4. Try matching sprint (by ResourceID or SprintID)
		if hasRID {
			if name, ok := sprintMap[rID]; ok {
				logs[i].Title = name
				continue
			}
		}
		if log.SprintID != nil {
			if name, ok := sprintMap[*log.SprintID]; ok {
				logs[i].Title = name
				continue
			}
		}

		if strings.EqualFold(log.ResourceType, "comment") && strings.Contains(strings.ToLower(log.Action), "deleted") {
			logs[i].Details = "Comment deleted"
		}
	}
}

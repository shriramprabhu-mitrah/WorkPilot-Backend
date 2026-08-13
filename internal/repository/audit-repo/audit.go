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
	if err := d.db.Create(&log).Error; err != nil {
		d.logger.Error("Database error occurred while creating audit log", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return nil
}

func (d *auditDatabase) GetAuditLogs(req requestdto.GetAudit) ([]models.AuditLog, response.Pagination, *response.Error) {

	var (
		audits     []models.AuditLog
		totalItems int64
	)

	req.PaginationQuery.Normalize(10)

	offset := (req.Page - 1) * req.PageSize

	baseQuery := d.db.
		Model(&models.AuditLog{}).
		Where(
			"organization_id = ? AND user_id = ?",
			req.OrganizationID,
			req.UserID,
		)

	pattern := "%view%"
	if strings.EqualFold(req.ActivityType, string(models.AuditLogTypeActivity)) {
		baseQuery = baseQuery.Where("type = ? OR ((type IS NULL OR type = '') AND LOWER(action) NOT LIKE ?)", models.AuditLogTypeActivity, pattern)
	} else {
		// Default to AuditLogTypeView ("view", "viewed", or empty)
		baseQuery = baseQuery.Where("type = ? OR ((type IS NULL OR type = '') AND LOWER(action) LIKE ?)", models.AuditLogTypeView, pattern)
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error(
			"Failed to count audit logs",
			zap.String("User ID", req.UserID.String()),
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.Error(err),
		)

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Order("created_at DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&audits).Error; err != nil {

		d.logger.Error(
			"Failed to fetch audit logs",
			zap.String("User ID", req.UserID.String()),
			zap.String("Organization ID", req.OrganizationID.String()),
			zap.Error(err),
		)

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
	var projectIDs []uuid.UUID
	var sprintIDs []uuid.UUID

	for _, log := range logs {
		rID, err := uuid.FromString(log.ResourceID)
		if err != nil || rID == uuid.Nil {
			continue
		}
		switch strings.ToLower(log.ResourceType) {
		case "task":
			taskIDs = append(taskIDs, rID)
		case "project":
			projectIDs = append(projectIDs, rID)
		case "sprint":
			sprintIDs = append(sprintIDs, rID)
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
		if err != nil {
			continue
		}
		switch strings.ToLower(log.ResourceType) {
		case "task":
			if t, ok := taskMap[rID]; ok {
				logs[i].Title = t.Title
				logs[i].TaskKey = t.Key
			}
		case "project":
			if name, ok := projectMap[rID]; ok {
				logs[i].Title = name
			}
		case "sprint":
			if name, ok := sprintMap[rID]; ok {
				logs[i].Title = name
			}
		}
	}
}

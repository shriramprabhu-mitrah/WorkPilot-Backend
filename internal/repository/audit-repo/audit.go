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
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
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

	// Fetch task titles in bulk for task-related activities
	var taskIDs []uuid.UUID
	for _, audit := range audits {
		if strings.ToLower(audit.ResourceType) == "task" {
			if taskID, err := uuid.FromString(audit.ResourceID); err == nil && taskID != uuid.Nil {
				taskIDs = append(taskIDs, taskID)
			}
		}
	}

	if len(taskIDs) > 0 {
		type TaskInfo struct {
			ID    uuid.UUID
			Title string
		}
		var tasks []TaskInfo
		if err := d.db.Unscoped().Model(&models.Task{}).Where("id IN ?", taskIDs).Select("id, title").Find(&tasks).Error; err == nil {
			taskTitleMap := make(map[uuid.UUID]string)
			for _, t := range tasks {
				taskTitleMap[t.ID] = t.Title
			}
			for i, audit := range audits {
				if strings.ToLower(audit.ResourceType) == "task" {
					if taskID, err := uuid.FromString(audit.ResourceID); err == nil {
						if title, ok := taskTitleMap[taskID]; ok {
							audits[i].TaskTitle = title
						}
					}
				}
			}
		}
	}

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

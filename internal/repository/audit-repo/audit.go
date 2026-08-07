package auditrepo

import (
	"net/http"

	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
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

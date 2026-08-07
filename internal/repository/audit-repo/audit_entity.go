package auditrepo

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AuditLogRepository interface {
	CreateAuditLog(log models.AuditLog) *response.Error
}

func InitAuditLogRepository(deps models.Config) AuditLogRepository {
	return &auditDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

type auditDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

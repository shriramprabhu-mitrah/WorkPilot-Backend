package auditrepo

import (
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AuditLogRepository interface {
	CreateAuditLog(log models.AuditLog) *response.Error
	GetAuditLogs(req requestdto.GetAudit) ([]models.AuditLog, response.Pagination, *response.Error)
}

func InitAuditRepository(deps models.Config) AuditLogRepository {
	return &auditDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

type auditDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

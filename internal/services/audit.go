package services

import (
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	"go.uber.org/zap"
)

type AuditService interface {
	GetAuditLogs(req requestdto.GetAudit) ([]responsedto.AuditLogResponse, response.Pagination, *response.Error)
}

func InitAuditService(auditRepo auditrepo.AuditLogRepository, logger *zap.Logger) AuditService {
	return &auditService{
		auditRepo: auditRepo,
		logger:    logger,
	}
}

type auditService struct {
	auditRepo auditrepo.AuditLogRepository
	logger    *zap.Logger
}

func (s *auditService) GetAuditLogs(req requestdto.GetAudit) ([]responsedto.AuditLogResponse, response.Pagination, *response.Error) {
	audits, pagination, err := s.auditRepo.GetAuditLogs(req)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	responses := make([]responsedto.AuditLogResponse, 0, len(audits))

	for _, audit := range audits {
		responses = append(responses, responsedto.AuditLogFromModel(audit))
	}

	return responses, pagination, nil
}

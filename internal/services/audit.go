package services

import (
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	auditrepo "github.com/ms-kanban-server/internal/repository/audit-repo"
	authrepo "github.com/ms-kanban-server/internal/repository/auth-repo"
	"go.uber.org/zap"
)

type AuditService interface {
	GetAuditLogs(req requestdto.GetAudit) (*responsedto.AuditLogResponseWrapper, response.Pagination, *response.Error)
}

func InitAuditService(auditRepo auditrepo.AuditLogRepository, authRepo authrepo.AuthRepository, logger *zap.Logger) AuditService {
	return &auditService{
		auditRepo: auditRepo,
		authRepo:  authRepo,
		logger:    logger,
	}
}

type auditService struct {
	auditRepo auditrepo.AuditLogRepository
	authRepo  authrepo.AuthRepository
	logger    *zap.Logger
}

func (s *auditService) GetAuditLogs(req requestdto.GetAudit) (*responsedto.AuditLogResponseWrapper, response.Pagination, *response.Error) {
	audits, pagination, err := s.auditRepo.GetAuditLogs(req)
	if err != nil {
		return nil, response.Pagination{}, err
	}

	activities := make([]responsedto.AuditLogResponse, 0, len(audits))
	for _, audit := range audits {
		activities = append(activities, responsedto.AuditLogFromModel(audit))
	}

	var userSummary *responsedto.UserSummary
	if req.UserID != nil {
		user, userErr := s.authRepo.GetUserByID(*req.UserID)
		if userErr == nil {
			var avatarURL *string
			if user.AvatarURL != "" {
				avatarURL = &user.AvatarURL
			}
			userSummary = &responsedto.UserSummary{
				ID:        user.ID,
				FullName:  user.FullName,
				Email:     user.Email,
				AvatarURL: avatarURL,
				Role:      user.Role,
			}
		}
	}

	wrapper := &responsedto.AuditLogResponseWrapper{
		User:       userSummary,
		Activities: activities,
	}

	return wrapper, pagination, nil
}

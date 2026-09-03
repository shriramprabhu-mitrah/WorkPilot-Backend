package filecleanuprepo

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FileCleanupRepository interface {
	CreateOrphanedFile(ctx context.Context, file *models.OrphanedFile) *response.Error
	ClaimOrphanedFiles(ctx context.Context, now time.Time, claimTTL time.Duration, limit int) ([]models.OrphanedFile, *response.Error)
	ReleaseOrphanedFile(ctx context.Context, id uuid.UUID, lastErr string, lastAttempt time.Time, nextAttempt time.Time) *response.Error
	DeleteOrphanedFile(ctx context.Context, id uuid.UUID) *response.Error
}

type fileCleanupDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

func InitFileCleanupRepository(deps models.Config) FileCleanupRepository {
	return &fileCleanupDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

package filecleanuprepo

import (
	"context"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

func (d *fileCleanupDatabase) CreateOrphanedFile(ctx context.Context, file *models.OrphanedFile) *response.Error {
	if err := d.db.WithContext(ctx).Create(file).Error; err != nil {
		d.logger.Error("Failed to record orphaned file in database", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to record cleanup task",
		}
	}
	return nil
}

func (d *fileCleanupDatabase) DeleteOrphanedFile(ctx context.Context, id uuid.UUID) *response.Error {
	if err := d.db.WithContext(ctx).Where("id = ?", id).Delete(&models.OrphanedFile{}).Error; err != nil {
		d.logger.Error("Failed to delete orphaned file task from database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to complete cleanup task",
		}
	}
	return nil
}

func (d *fileCleanupDatabase) ClaimOrphanedFiles(ctx context.Context, now time.Time, claimTTL time.Duration, limit int) ([]models.OrphanedFile, *response.Error) {
	var claimed []models.OrphanedFile
	claimedUntil := now.Add(claimTTL)

	// Execute explicit raw SQL query with CTE for atomic UPDATE RETURNING matching reservation state
	err := d.db.WithContext(ctx).Raw(`
		WITH claimed AS (
			SELECT id
			FROM orphaned_files
			WHERE available_at <= ?
			ORDER BY available_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT ?
		)
		UPDATE orphaned_files o
		SET
			available_at = ?,
			attempts = o.attempts + 1,
			last_attempt_at = ?
		FROM claimed
		WHERE o.id = claimed.id
		RETURNING o.id, o.storage_path, o.attempts, o.last_attempt_at, o.last_error, o.available_at, o.created_at
	`, now, limit, claimedUntil, now).Scan(&claimed).Error

	if err != nil {
		d.logger.Error("Failed to claim orphaned files from database using raw CTE query", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to claim cleanup tasks",
		}
	}

	return claimed, nil
}

func (d *fileCleanupDatabase) ReleaseOrphanedFile(ctx context.Context, id uuid.UUID, lastErr string, lastAttempt time.Time, nextAttempt time.Time) *response.Error {
	err := d.db.WithContext(ctx).Model(&models.OrphanedFile{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"available_at":    nextAttempt,
			"last_error":      lastErr,
			"last_attempt_at": lastAttempt,
		}).Error

	if err != nil {
		d.logger.Error("Failed to release orphaned file in database", zap.Error(err), zap.String("id", id.String()))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to update cleanup task status",
		}
	}
	return nil
}

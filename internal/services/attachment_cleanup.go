package services

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.uber.org/zap"
)

func (s *attachmentService) startCleanupWorker(ctx context.Context) {
	s.logger.Info("Starting orphaned files cleanup outbox worker...")
	
	// Process once immediately before entering the ticker loop
	s.processOrphanedFiles(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Orphaned files cleanup outbox worker stopping...")
			return
		case <-ticker.C:
			s.processOrphanedFiles(ctx)
		}
	}
}

func calculateBackoff(attempts int, initial, max time.Duration) time.Duration {
	if attempts <= 0 {
		return initial
	}
	shift := attempts - 1
	if shift > 7 {
		shift = 7
	}
	delay := initial * time.Duration(1<<uint(shift))
	if delay > max {
		delay = max
	}
	return delay
}

func (s *attachmentService) calculateNextAttempt(attempts int, now time.Time) time.Time {
	base := calculateBackoff(attempts, 30*time.Second, 1*time.Hour)
	var jitter int64
	if base > 0 {
		jitter = s.jitterSource.Int63n(int64(base / 3))
	}
	delay := base + time.Duration(jitter)
	if delay > 1*time.Hour {
		delay = 1 * time.Hour
	}
	return now.Add(delay)
}

func (s *attachmentService) processOrphanedFiles(ctx context.Context) {
	now := time.Now()

	files, err := s.cleanupRepo.ClaimOrphanedFiles(ctx, now, s.claimTTL, 50)
	if err != nil {
		return
	}

	for _, file := range files {
		delErr := s.storageClient.DeleteObject(ctx, file.StoragePath)
		if delErr == nil || isS3NoSuchKey(delErr) {
			_ = s.cleanupRepo.DeleteOrphanedFile(ctx, file.ID)
		} else {
			lastErrStr := delErr.Error()
			if len(lastErrStr) > 500 {
				lastErrStr = lastErrStr[:500]
			}
			newAttempts := file.Attempts + 1
			nextAttempt := s.calculateNextAttempt(newAttempts, now)
			_ = s.cleanupRepo.ReleaseOrphanedFile(ctx, file.ID, lastErrStr, now, nextAttempt)
			s.logger.Error("Orphaned files cleanup worker failed to delete storage object, marked for retry",
				zap.String("path", file.StoragePath),
				zap.Error(delErr),
			)
		}
	}
}

func isS3NoSuchKey(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	// S3-compatible providers (e.g. Supabase, MinIO) may return a generic smithy APIError
	// with code "NoSuchKey" rather than the typed *types.NoSuchKey.
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NoSuchKey"
	}
	return false
}

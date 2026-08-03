package scheduler

import (
	"time"

	"github.com/ms-kanban-server/internal/pkg/models"
	sprintrepo "github.com/ms-kanban-server/internal/repository/sprint-repo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Start(db *gorm.DB, logger *zap.Logger) {
	sprintRepo := sprintrepo.InitSprintRepository(models.Config{Database: db, Logger: logger})

	// Wait until next midnight to run the first job
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	duration := nextMidnight.Sub(now)

	logger.Info("Starting sprint snapshot scheduler", zap.Duration("delay_until_first_run", duration))

	time.AfterFunc(duration, func() {
		runSnapshotJob(sprintRepo, logger)

		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			runSnapshotJob(sprintRepo, logger)
		}
	})
}

func runSnapshotJob(sprintRepo sprintrepo.SprintRepository, logger *zap.Logger) {
	logger.Info("Running daily sprint snapshot background job")

	sprints, err := sprintRepo.GetActiveSprints()
	if err != nil {
		logger.Error("Failed to fetch active sprints for daily snapshot job", zap.Any("error", err))
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, sprint := range sprints {
		totalPoints, err := sprintRepo.GetTotalStoryPoints(sprint.ID)
		if err != nil {
			logger.Error("Failed to calculate total points for daily sprint snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
			continue
		}

		remainingPoints, err := sprintRepo.GetRemainingStoryPoints(sprint.ID)
		if err != nil {
			logger.Error("Failed to calculate remaining points for daily sprint snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
			continue
		}

		snapshot := models.SprintSnapshot{
			SprintID:             sprint.ID,
			Date:                 today,
			TotalStoryPoints:     totalPoints,
			RemainingStoryPoints: remainingPoints,
			CreatedAt:            now,
		}

		err = sprintRepo.CreateSprintSnapshot(snapshot)
		if err != nil {
			logger.Error("Failed to save daily sprint snapshot in background job", zap.String("sprint_id", sprint.ID.String()), zap.Any("error", err))
		} else {
			logger.Info("Successfully saved daily sprint snapshot", zap.String("sprint_id", sprint.ID.String()), zap.Int("total_points", totalPoints), zap.Int("remaining_points", remainingPoints))
		}
	}
}

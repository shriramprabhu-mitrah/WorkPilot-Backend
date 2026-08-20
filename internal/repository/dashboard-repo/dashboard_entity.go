package dashboardrepo

import (
	"time"

	"github.com/gofrs/uuid"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DashboardRepository interface {
	GetOverview(projectID uuid.UUID) (responsedto.DashboardOverview, *response.Error)
	GetTaskStatus(projectID uuid.UUID) (map[string]int64, *response.Error)
	GetSprintBurndown(projectID uuid.UUID, sprintID uuid.UUID) ([]responsedto.SprintBurndown, *response.Error)
	GetWeeklyProgress(projectID uuid.UUID, startDate time.Time, endDate time.Time) ([]responsedto.WeeklyProgress, *response.Error)
	GetTeamWorkload(projectID uuid.UUID) ([]responsedto.TeamWorkload, *response.Error)
}

func InitDashboardRepository(deps models.Config) DashboardRepository {
	return &dashboardDatabase{
		db:     deps.Database,
		logger: deps.Logger,
	}
}

type dashboardDatabase struct {
	db     *gorm.DB
	logger *zap.Logger
}

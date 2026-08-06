package sprintrepo

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
)

func (d *sprintDatabase) IsSprintExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	var count int64

	if err := d.db.Model(&models.Sprint{}).
		Where("project_id = ? AND LOWER(name) = LOWER(?)", projectID, name).
		Count(&count).Error; err != nil {

		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check sprint",
		}
	}

	return count > 0, nil
}

func (d *sprintDatabase) IsSprintDateRangeExists(projectID uuid.UUID, startDate, endDate time.Time, excludeSprintID uuid.UUID) (bool, *response.Error) {
	var count int64
	query := d.db.Model(&models.Sprint{}).
		Where("project_id = ? AND start_date = ? AND end_date = ?", projectID, startDate, endDate)

	if excludeSprintID != uuid.Nil {
		query = query.Where("id != ?", excludeSprintID)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check sprint date range",
		}
	}

	return count > 0, nil
}

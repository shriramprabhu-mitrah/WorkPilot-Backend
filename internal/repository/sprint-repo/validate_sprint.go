package sprintrepo

import (
	"net/http"

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

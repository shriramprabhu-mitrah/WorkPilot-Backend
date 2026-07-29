package sprintrepo

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *sprintDatabase) CreateSprint(row models.Sprint) *response.Error {

	if err := d.db.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict",
				zap.Error(err))
			return utils.ParseSprintDuplicateError(err)
		}

		d.logger.Error("Database error occurred",
			zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return nil
}

func (d *sprintDatabase) UpdateSprint(projectID, sprintID uuid.UUID, req models.Sprint) *response.Error {

	result := d.db.
		Model(&models.Sprint{}).
		Where("id = ? AND project_id = ?", sprintID, projectID).
		Updates(req)

	if result.Error != nil {
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating sprint",
				zap.Error(result.Error))
			return utils.ParseSprintDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating sprint",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Sprint not found for the given project",
			zap.String("project_id", projectID.String()),
			zap.String("sprint_id", sprintID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found",
		}
	}

	return nil
}

func (d *sprintDatabase) DeleteSprint(id uuid.UUID) *response.Error {

	result := d.db.
		Where("id = ?", id).
		Delete(&models.Sprint{})

	if result.Error != nil {
		d.logger.Error("Failed to delete sprint",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Sprint could not be found for deletion",
			zap.String("sprint_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Sprint not found",
		}
	}

	return nil
}

func (d *sprintDatabase) GetSprintByID(sprintID, projectID uuid.UUID) (*models.Sprint, *response.Error) {

	var sprint models.Sprint

	err := d.db.
		Preload("Project").
		Preload("Project.Organization").
		Preload("Project.Creator").
		Preload("CreatedBy").
		Preload("CreatedBy.Organization").
		Where("id = ? AND project_id = ?", sprintID, projectID).
		First(&sprint).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d.logger.Error("Sprint not found",
				zap.String("sprint_id", sprintID.String()))

			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Sprint not found",
			}
		}

		d.logger.Error("Failed to get sprint",
			zap.Error(err))

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return &sprint, nil
}

func (d *sprintDatabase) GetSprints(projectID uuid.UUID, filter dto.SprintFilter) ([]models.Sprint, response.Pagination, *response.Error) {

	var sprints []models.Sprint
	var totalItems int64

	if filter.Page < 1 {
		filter.Page = 1
	}

	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	offset := (filter.Page - 1) * filter.PageSize

	query := d.db.Model(&models.Sprint{}).
		Where("project_id = ?", projectID)

	if filter.Search != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(filter.Search)+"%")
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if err := query.Count(&totalItems).Error; err != nil {
		d.logger.Error("Failed to count sprints",
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := query.
		Preload("Project").
		Preload("Project.Organization").
		Preload("Project.Creator").
		Preload("CreatedBy").
		Preload("CreatedBy.Organization").
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&sprints).Error; err != nil {

		d.logger.Error("Failed to fetch sprints",
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	pagination := response.Pagination{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalItems: int(totalItems),
		TotalPages: int(math.Ceil(float64(totalItems) / float64(filter.PageSize))),
	}

	return sprints, pagination, nil
}

package projectrepo

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

func (d *projectDatabase) CreateProject(row models.Project) *response.Error {

	if err := d.db.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict", zap.Error(err))
			return utils.ParseProjectDuplicateError(err)
		}

		d.logger.Error("Database error occurred", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return nil
}

func (d *projectDatabase) UpdateProject(projectID uuid.UUID, req models.Project) *response.Error {

	result := d.db.
		Model(&models.Project{}).
		Where("id = ?", projectID).
		Updates(req)
	if result.Error != nil {

		d.logger.Error("Database error occurred",
			zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("ProjectID not found",
			zap.String("user_id", projectID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Project not found",
		}
	}

	return nil
}

func (d *projectDatabase) GetProjectsByOrganizationID(organizationID uuid.UUID, filter dto.ProjectFilter) ([]models.Project, response.Pagination, *response.Error) {

	var projects []models.Project
	var totalItems int64

	if filter.Page < 1 {
		filter.Page = 1
	}

	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.db.Model(&models.Project{}).
		Where("organization_id = ?", organizationID)

	if filter.Key != "" {
		key := "%" + strings.ToLower(strings.TrimSpace(filter.Key)) + "%"
		baseQuery = baseQuery.Where("LOWER(key) LIKE ?", key)
	}

	if filter.Name != "" {
		name := "%" + strings.ToLower(strings.TrimSpace(filter.Name)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ?", name)
	}

	if filter.Status != "" {
		baseQuery = baseQuery.Where(
			"LOWER(status) = ?",
			strings.ToLower(strings.TrimSpace(filter.Status)),
		)
	}

	if filter.StartDate != nil {
		baseQuery = baseQuery.Where("start_date >= ?", filter.StartDate)
	}

	if filter.EndDate != nil {
		baseQuery = baseQuery.Where("end_date >= ?", filter.EndDate)
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred",
			zap.String("Organization Id", organizationID.String()),
			zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("Organization").
		Preload("Creator").
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&projects).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("Organization Id", organizationID.String()),
			zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     filter.Page < totalPages,
		HasPrevious: filter.Page > 1,
	}

	return projects, pagination, nil
}

func (d *projectDatabase) GetProjectByID(id uuid.UUID) (models.Project, *response.Error) {

	var row models.Project

	err := d.db.Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Project not found",
			}
			d.logger.Error("Project not found in database",
				zap.String("Id", id.String()),
				zap.Error(err))
			return models.Project{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", id.String()),
			zap.Error(err))
		return models.Project{}, &errorResponse
	}
	return row, nil
}

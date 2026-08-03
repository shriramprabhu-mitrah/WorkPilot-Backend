package projectrepo

import (
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
)

func (d *projectDatabase) CreateProjectMember(row models.ProjectMember) *response.Error {

	if err := d.db.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict", zap.Error(err))
			return utils.ParseProjectMemberDuplicateError(err)
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

func (d *projectDatabase) GetProjectsMembersByProjectID(projectID uuid.UUID, filter dto.ProjectMemberFilter) ([]models.ProjectMember, response.Pagination, *response.Error) {

	var projectMembers []models.ProjectMember
	var totalItems int64

	filter.PaginationQuery.Normalize(10)

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.db.Model(&models.ProjectMember{}).
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.project_id = ?", projectID)

	if filter.Name != "" {
		name := "%" + strings.ToLower(strings.TrimSpace(filter.Name)) + "%"
		baseQuery = baseQuery.Where("LOWER(users.full_name) LIKE ?", name)
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred",
			zap.String("project_id", projectID.String()),
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("Project.Organization").
		Preload("Project.Creator").
		Preload("User").
		Preload("AddedBy").
		Order("project_members.joined_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&projectMembers).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("project_id", projectID.String()),
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

	return projectMembers, pagination, nil
}

func (d *projectDatabase) RemoveProjectMember(projectID, userID uuid.UUID) *response.Error {

	result := d.db.
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{})

	if result.Error != nil {
		d.logger.Error("Database error occurred",
			zap.Error(result.Error),
			zap.String("project_id", projectID.String()),
			zap.String("user_id", userID.String()),
		)

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Project member not found.",
		}
	}

	return nil
}

func (d *projectDatabase) IsUserProjectMember(projectID, userID uuid.UUID) (bool, *response.Error) {
	var count int64
	err := d.db.Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count).Error
	if err != nil {
		d.logger.Error("Database error checking project membership", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}
	return count > 0, nil
}

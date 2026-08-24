package organizationrepo

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *organizationDatabase) CreateOrganization(row models.Organization) *response.Error {

	if err := d.DB.Create(&row).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicated Key conflict", zap.Error(err))
			return utils.ParseOrgDuplicateError(err)
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

func (d *organizationDatabase) GetByName(name string) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Where("name = ?", name).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Organization not found",
			}
			d.logger.Error("Organization not found in database",
				zap.String("Name", name),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Name", name),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}

	return row, nil
}

func (d *organizationDatabase) GetByID(id uuid.UUID) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Organization not found",
			}
			d.logger.Error("Organization not found in database",
				zap.String("Id", id.String()),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", id.String()),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}
	return row, nil
}

func (d *organizationDatabase) GetByIDUnscoped(id uuid.UUID) (models.Organization, *response.Error) {

	var row models.Organization

	err := d.DB.Unscoped().Where("id = ?", id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorResponse := response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Organization not found",
			}
			d.logger.Error("Organization not found in database",
				zap.String("Id", id.String()),
				zap.Error(err))
			return models.Organization{}, &errorResponse
		}

		errorResponse := response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}

		d.logger.Error("Database error occurred",
			zap.String("Id", id.String()),
			zap.Error(err))
		return models.Organization{}, &errorResponse
	}
	return row, nil
}

func (d *organizationDatabase) GetAllOrganizations(filter dto.OrganizationFilterRequest) ([]models.Organization, response.Pagination, *response.Error) {
	var rows []models.Organization
	var totalItems int64

	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.DB.Unscoped().Model(&models.Organization{})

	if filter.Name != "" {
		name := "%" + strings.ToLower(strings.TrimSpace(filter.Name)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ?", name)
	}

	if filter.Domain != "" {
		domain := "%" + strings.ToLower(strings.TrimSpace(filter.Domain)) + "%"
		baseQuery = baseQuery.Where("LOWER(domain) LIKE ?", domain)
	}

	if filter.Industry != "" {
		baseQuery = baseQuery.Where("LOWER(industry) = ?", strings.ToLower(strings.TrimSpace(filter.Industry)))
	}

	if filter.TeamSize != "" {
		baseQuery = baseQuery.Where("team_size = ?", strings.TrimSpace(filter.TeamSize))
	}

	if filter.Country != "" {
		baseQuery = baseQuery.Where("LOWER(country) = ?", strings.ToLower(strings.TrimSpace(filter.Country)))
	}

	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		baseQuery = baseQuery.Where("LOWER(name) LIKE ? OR LOWER(domain) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(industry) LIKE ?", searchTerm, searchTerm, searchTerm, searchTerm)
	}

	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"name":       "name",
			"created_at": "created_at",
			"updated_at": "updated_at",
			"domain":     "domain",
			"industry":   "industry",
			"team_size":  "team_size",
			"is_active":  "is_active",
		}
		if col, ok := allowed[filter.SortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", col, direction)
		}
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred while counting organizations", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Order(orderClause).
		Limit(filter.PageSize).
		Offset(offset).
		Find(&rows).Error; err != nil {
		d.logger.Error("Database error occurred while fetching all organizations", zap.Error(err))
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

	return rows, pagination, nil
}

func (d *organizationDatabase) UpdateOrganization(OrganizationID uuid.UUID, req models.Organization) *response.Error {

	result := d.DB.
		Model(&models.Organization{}).
		Where("id = ?", OrganizationID).
		Updates(req)

	if result.Error != nil {
		if utils.IsDuplicateKeyError(result.Error) {
			d.logger.Error("Duplicate key error while updating Organization", zap.Error(result.Error))
			return utils.ParseOrgDuplicateError(result.Error)
		}

		d.logger.Error("Database error occurred while updating Organization",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("Organization not found while updating Organization",
			zap.String("Organization_id", OrganizationID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *organizationDatabase) SoftDeleteOrganization(orgID uuid.UUID) *response.Error {
	result := d.DB.Unscoped().
		Model(&models.Organization{}).
		Where("id = ?", orgID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"deleted_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		d.logger.Error("Database error occurred while soft deleting Organization", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Organization not found while soft deleting Organization", zap.String("organization_id", orgID.String()))
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *organizationDatabase) RestoreOrganization(orgID uuid.UUID) *response.Error {
	result := d.DB.Unscoped().
		Model(&models.Organization{}).
		Where("id = ?", orgID).
		Updates(map[string]interface{}{
			"is_active":  true,
			"deleted_at": nil,
		})

	if result.Error != nil {
		d.logger.Error("Database error occurred while restoring Organization", zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Organization not found while restoring Organization", zap.String("organization_id", orgID.String()))
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}


func (d *organizationDatabase) DeleteOrganization(id uuid.UUID) *response.Error {

	result := d.DB.Where("id = ?", id).Delete(&models.Organization{})

	if result.Error != nil {
		d.logger.Error("Failed to delete Organization",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("The Organization could not be found for deletion",
			zap.String("organization_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Organization not found",
		}
	}

	return nil
}

func (d *organizationDatabase) DeleteUser(id uuid.UUID) *response.Error {

	result := d.DB.Where("id = ?", id).Delete(&models.User{})

	if result.Error != nil {
		d.logger.Error("Failed to delete User",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("The User could not be found for deletion",
			zap.String("user_id", id.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *organizationDatabase) UpdateStatusAndRole(userID uuid.UUID, req models.User) *response.Error {

	result := d.DB.
		Model(&models.User{}).
		Where("id = ?", userID).
		Save(req)

	if result.Error != nil {

		d.logger.Error("Database error occurred while updating user",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {

		d.logger.Error("User not found while updating user",
			zap.String("user_id", userID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "User not found",
		}
	}

	return nil
}

func (d *organizationDatabase) GetUsersByOrganizationID(organizationID uuid.UUID, filter dto.OrganizationMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	var users []models.User
	var totalItems int64

	filter.PaginationQuery.Normalize(10)

	offset := (filter.Page - 1) * filter.PageSize
		baseQuery := d.DB.Model(&models.User{}).Where("organization_id = ? and is_active = ?", organizationID, true)

	if filter.FullName != "" {
		baseQuery = baseQuery.Where("full_name ILIKE ?", "%"+strings.TrimSpace(filter.FullName)+"%")
	}
	if filter.Email != "" {
		baseQuery = baseQuery.Where("email ILIKE ?", "%"+strings.TrimSpace(filter.Email)+"%")
	}
	if filter.Username != "" {
		baseQuery = baseQuery.Where("username ILIKE ?", "%"+strings.TrimSpace(filter.Username)+"%")
	}
	if filter.Role != "" {
		baseQuery = baseQuery.Where("LOWER(role) = ?", strings.ToLower(strings.TrimSpace(filter.Role)))
	}
	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.IsVerified != nil {
		baseQuery = baseQuery.Where("is_verified = ?", *filter.IsVerified)
	}
	if filter.Timezone != "" {
		baseQuery = baseQuery.Where("timezone ILIKE ?", "%"+strings.TrimSpace(filter.Timezone)+"%")
	}
	if !filter.IncludeOrgAdmins {
		baseQuery = baseQuery.Where("LOWER(role) != ? OR role IS NULL", "org_admin")
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Order("created_at DESC").
		Limit(filter.PageSize).
		Offset(offset).
		Find(&users).Error; err != nil {
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

	return users, pagination, nil
}

func (d *organizationDatabase) GetAllMembers(filter dto.GlobalMemberListFilter) ([]models.User, response.Pagination, *response.Error) {
	var users []models.User
	var totalItems int64

	filter.PaginationQuery.Normalize(10)
	filter.SortQuery.Normalize("created_at", "DESC")

	offset := (filter.Page - 1) * filter.PageSize

	baseQuery := d.DB.Model(&models.User{})

	if filter.OrganizationID != nil && *filter.OrganizationID != uuid.Nil {
		baseQuery = baseQuery.Where("organization_id = ?", *filter.OrganizationID)
	}

	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		baseQuery = baseQuery.Where("LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(username) LIKE ?", searchTerm, searchTerm, searchTerm)
	}

	if filter.FullName != "" {
		baseQuery = baseQuery.Where("LOWER(full_name) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(filter.FullName))+"%")
	}

	if filter.Email != "" {
		baseQuery = baseQuery.Where("LOWER(email) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(filter.Email))+"%")
	}

	if filter.Username != "" {
		baseQuery = baseQuery.Where("LOWER(username) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(filter.Username))+"%")
	}

	if filter.Role != "" {
		baseQuery = baseQuery.Where("LOWER(role) = ?", strings.ToLower(strings.TrimSpace(filter.Role)))
	}

	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}

	if filter.IsVerified != nil {
		baseQuery = baseQuery.Where("is_verified = ?", *filter.IsVerified)
	}

	if filter.Timezone != "" {
		baseQuery = baseQuery.Where("LOWER(timezone) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(filter.Timezone))+"%")
	}

	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		direction := "ASC"
		if strings.ToUpper(filter.SortOrder) == "DESC" {
			direction = "DESC"
		}
		allowed := map[string]string{
			"full_name":  "full_name",
			"name":       "full_name",
			"email":      "email",
			"username":   "username",
			"role":       "role",
			"created_at": "created_at",
			"joined_at":  "joined_at",
			"is_active":  "is_active",
		}
		if col, ok := allowed[filter.SortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", col, direction)
		}
	}

	if err := baseQuery.Count(&totalItems).Error; err != nil {
		d.logger.Error("Database error occurred while counting users", zap.Error(err))
		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("Organization").
		Order(orderClause).
		Limit(filter.PageSize).
		Offset(offset).
		Find(&users).Error; err != nil {
		d.logger.Error("Database error occurred while fetching users", zap.Error(err))
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

	return users, pagination, nil
}

func (d *organizationDatabase) GetProjectCountsByOrganizationIDs(orgIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error) {
	counts := make(map[uuid.UUID]int64)
	if len(orgIDs) == 0 {
		return counts, nil
	}

	type result struct {
		OrganizationID uuid.UUID `gorm:"column:organization_id"`
		Count          int64     `gorm:"column:count"`
	}

	var results []result
	if err := d.DB.Model(&models.Project{}).
		Select("organization_id, count(*) as count").
		Where("organization_id IN ? AND deleted_at IS NULL", orgIDs).
		Group("organization_id").
		Scan(&results).Error; err != nil {
		d.logger.Error("Failed to count projects by organization IDs", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to count projects by organization IDs",
		}
	}

	for _, r := range results {
		counts[r.OrganizationID] = r.Count
	}

	return counts, nil
}

func (d *organizationDatabase) GetMemberCountsByOrganizationIDs(orgIDs []uuid.UUID) (map[uuid.UUID]int64, *response.Error) {
	counts := make(map[uuid.UUID]int64)
	if len(orgIDs) == 0 {
		return counts, nil
	}

	type result struct {
		OrganizationID uuid.UUID `gorm:"column:organization_id"`
		Count          int64     `gorm:"column:count"`
	}

	var results []result
	if err := d.DB.Model(&models.User{}).
		Select("organization_id, count(*) as count").
		Where("organization_id IN ? AND deleted_at IS NULL", orgIDs).
		Group("organization_id").
		Scan(&results).Error; err != nil {
		d.logger.Error("Failed to count members by organization IDs", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to count members by organization IDs",
		}
	}

	for _, r := range results {
		counts[r.OrganizationID] = r.Count
	}

	return counts, nil
}

func (d *organizationDatabase) CreateDefaultRolesForOrg(orgID uuid.UUID) *response.Error {
	rolesToSeed := []struct {
		Name        string
		Description string
		IsSystem    bool
		FilterPerms func(res, act string) bool
	}{
		{
			Name:        "org_admin",
			Description: "Organization administrator with full access to organization resources",
			IsSystem:    true,
			FilterPerms: func(res, act string) bool { return true },
		},
		{
			Name:        "project_manager",
			Description: "Project manager with access to manage projects, sprints, and team activities",
			IsSystem:    true,
			FilterPerms: func(res, act string) bool {
				return !(res == "projects" && (act == "add" || act == "delete"))
			},
		},
		{
			Name:        "developer",
			Description: "Software developer with access to view and modify user stories and tasks",
			IsSystem:    true,
			FilterPerms: func(res, act string) bool {
				return (res == "projects" && act == "view") ||
					(res == "sprints" && act == "view") ||
					(res == "user_stories" && (act == "view" || act == "add" || act == "modify")) ||
					(res == "tasks" && (act == "view" || act == "add" || act == "modify" || act == "delete")) ||
					(res == "comments" && (act == "view" || act == "add" || act == "modify" || act == "delete" || act == "comment"))
			},
		},
		{
			Name:        "qa",
			Description: "Quality assurance engineer with access to test tasks",
			IsSystem:    true,
			FilterPerms: func(res, act string) bool {
				return (res == "projects" && act == "view") ||
					(res == "sprints" && act == "view") ||
					(res == "user_stories" && (act == "view" || act == "modify")) ||
					(res == "tasks" && (act == "view" || act == "add" || act == "modify")) ||
					(res == "comments" && (act == "view" || act == "add" || act == "comment"))
			},
		},
		{
			Name:        "stakeholder",
			Description: "Read-only stakeholder with basic viewing and commenting privileges",
			IsSystem:    true,
			FilterPerms: func(res, act string) bool {
				return (res == "projects" && act == "view") ||
					(res == "sprints" && act == "view") ||
					(res == "user_stories" && act == "view") ||
					(res == "tasks" && act == "view") ||
					(res == "comments" && (act == "view" || act == "comment"))
			},
		},
	}

	var perms []models.Permission
	if err := d.DB.Find(&perms).Error; err != nil {
		d.logger.Error("Failed to fetch permissions during org role seeding", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	for _, seed := range rolesToSeed {
		var role models.Role
		err := d.DB.Where("name = ? AND organization_id = ? AND deleted_at IS NULL", seed.Name, orgID).First(&role).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				role = models.Role{
					ID:             uuid.Must(uuid.NewV7()),
					OrganizationID: &orgID,
					Name:           seed.Name,
					Description:    seed.Description,
					IsSystem:       seed.IsSystem,
				}
				if err := d.DB.Create(&role).Error; err != nil {
					d.logger.Error("Failed to create org-scoped role", zap.Error(err), zap.String("name", seed.Name))
					return &response.Error{
						Code:       response.ErrInternalServerError,
						StatusCode: http.StatusInternalServerError,
						Message:    "Something went wrong. Please try again later.",
					}
				}

				var assoc []models.RolePermission
				for _, p := range perms {
					if seed.FilterPerms(p.Resource, p.Action) {
						assoc = append(assoc, models.RolePermission{
							RoleID:       role.ID,
							PermissionID: p.ID,
						})
					}
				}
				if len(assoc) > 0 {
					if err := d.DB.Create(&assoc).Error; err != nil {
						d.logger.Error("Failed to associate permissions to org-scoped role", zap.Error(err), zap.String("name", seed.Name))
						return &response.Error{
							Code:       response.ErrInternalServerError,
							StatusCode: http.StatusInternalServerError,
							Message:    "Something went wrong. Please try again later.",
						}
					}
				}
			} else {
				d.logger.Error("Failed to check org role existence", zap.Error(err), zap.String("name", seed.Name))
				return &response.Error{
					Code:       response.ErrInternalServerError,
					StatusCode: http.StatusInternalServerError,
					Message:    "Something went wrong. Please try again later.",
				}
			}
		}
	}
	return nil
}

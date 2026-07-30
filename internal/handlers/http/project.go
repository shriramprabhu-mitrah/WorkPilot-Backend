package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/internal/handlers/dto"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitProjectHandler(service services.ProjectService, logger *zap.Logger) *ProjectHandler {
	return &ProjectHandler{
		service: service,
		logger:  logger,
	}
}

type ProjectHandler struct {
	service services.ProjectService
	logger  *zap.Logger
}

// CreateProject godoc
//
//	@Summary		Create a new project
//	@Description	Create a new project in the authenticated user's organization.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CreateProjectRequest	true	"Create Project Request"
//	@Success		201		{object}	response.SuccessResponse	"Project created successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		409		{object}	response.ErrorResponse		"Duplicate project"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects/create [post]
func (h *ProjectHandler) CreateProject(g *gin.Context) {

	var payload dto.CreateProjectRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	payload.OrganizationID = organizationUUID
	payload.UserID = userUUID

	err := h.service.CreateProject(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created Project",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

// UpdateProject godoc
//
//	@Summary		Update project
//	@Description	Update project details such as name, description, and status.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Project ID"
//	@Param			request	body		dto.UpdateProjectRequest	true	"Update Project Request"
//	@Success		200		{object}	response.SuccessResponse	"Project updated successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		404		{object}	response.ErrorResponse		"Project not found"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects/{id} [put]
func (h *ProjectHandler) UpdateProject(g *gin.Context) {

	var payload dto.UpdateProjectRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}

		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	id := g.Param("id")
	projectID, errorResponse := utils.StringToUUID(id)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	payload.OrganizationID = organizationUUID
	payload.UserID = userUUID
	payload.ProjectID = projectID

	err := h.service.UpdateProject(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated Project successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"ProjectID": projectID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetProjects godoc
//
//	@Summary		Get all projects
//	@Description	Get paginated list of projects in the authenticated organization.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"Page Number"		default(1)
//	@Param			page_size	query		int		false	"Page Size"			default(10)
//	@Param			name		query		string	false	"Project Name"
//	@Param			status		query		string	false	"Project Status"	Enums(planning,active,on_hold,completed,cancelled,archived)
//	@Success		200			{object}	response.SuccessResponse	"Projects retrieved successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Invalid query parameters"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects [get]
func (h *ProjectHandler) GetProjects(g *gin.Context) {
	var filter dto.ProjectFilterRequest

	if err := g.ShouldBindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid query parameters",
			}}
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	projects, pagination, err := h.service.GetProjectsByOrganizationID(organizationUUID, filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Projects retrieved successfully.",
		Data:       projects,
		Meta:       &pagination,
	})
}

// CreateProjectMember godoc
//
//	@Summary		Add project members
//	@Description	Add one or more users to a project.
//	@Tags			Project Members
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CreateProjectMemberRequest	true	"Project Member Request"
//	@Success		201		{object}	response.SuccessResponse	"Project members added successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		409		{object}	response.ErrorResponse		"Duplicate member"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects/members [post]
func (h *ProjectHandler) CreateProjectMember(g *gin.Context) {

	var payload dto.CreateProjectMemberRequest

	if err := g.Bind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}
		h.logger.Error("Invalid request payload", zap.Error(err))
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			},
		}

		h.logger.Error("User Id Invalid/Missing",
			zap.String("user id :", fmt.Sprintf("%v", userID)))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	organizationID, exist := g.Get("organization_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			},
		}

		h.logger.Error("Organization Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	payload.OrganizationID = organizationUUID
	payload.AddedByID = userUUID

	err := h.service.CreateProjectMemeber(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Added Project Member",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

// GetProjectMembers godoc
//
//	@Summary		Get project members
//	@Description	Get paginated list of project members.
//	@Tags			Project Members
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID"
//	@Param			page		query		int		false	"Page Number"	default(1)
//	@Param			page_size	query		int		false	"Page Size"		default(10)
//	@Param			name		query		string	false	"Member Name"
//	@Success		200			{object}	response.SuccessResponse	"Project members retrieved successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Invalid request"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		404			{object}	response.ErrorResponse		"Project not found"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects/{project_id}/members [get]
func (h *ProjectHandler) GetProjectMembers(g *gin.Context) {
	var filter dto.ProjectMemberFilter

	if err := g.ShouldBindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid query parameters",
			},
		}
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	projectMembers, pagination, serviceErr := h.service.GetProjectsMembersByProjectID(projectID, filter)
	if serviceErr != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *serviceErr,
		}
		g.JSON(serviceErr.StatusCode, errorResponse)
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Project members retrieved successfully.",
		Data:       projectMembers,
		Meta:       &pagination,
	})
}

// RemoveProjectMember godoc
//
//	@Summary		Remove project member
//	@Description	Remove a user from a project.
//	@Tags			Project Members
//	@Accept			json
//	@Produce		json
//	@Param			project_id	path		string	true	"Project ID"
//	@Param			user_id		path		string	true	"User ID"
//	@Success		200			{object}	response.SuccessResponse	"Project member removed successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Invalid request"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		404			{object}	response.ErrorResponse		"Project member not found"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//	@Router			/projects/{project_id}/members/{user_id} [delete]
func (h *ProjectHandler) RemoveProjectMember(g *gin.Context) {
	projectID := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userID := g.Param("user_id")
	userUUID, errorResponse := utils.StringToUUID(userID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	if serviceErr := h.service.RemoveProjectMember(projectUUID, userUUID); serviceErr != nil {
		g.JSON(serviceErr.StatusCode, response.ErrorResponse{
			Success: false,
			Error:   *serviceErr,
		})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Project member removed successfully.",
	})
}

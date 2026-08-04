package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
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
//	@Param			request	body		requestdto.CreateProjectRequest	true	"Create Project Request"
//	@Success		201		{object}	response.SuccessResponse	"Project created successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		409		{object}	response.ErrorResponse		"Duplicate project"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//
// @Router /project/create [post]
func (h *ProjectHandler) CreateProject(g *gin.Context) {

	var payload requestdto.CreateProjectRequest

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

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
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
//	@Param			request	body		requestdto.UpdateProjectRequest	true	"Update Project Request"
//	@Success		200		{object}	response.SuccessResponse	"Project updated successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		404		{object}	response.ErrorResponse		"Project not found"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//
// @Router /project/update/{id} [patch]
func (h *ProjectHandler) UpdateProject(g *gin.Context) {

	var payload requestdto.UpdateProjectRequest

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
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
//
// @Router /project/get [get]
func (h *ProjectHandler) GetProjects(g *gin.Context) {
	var filter requestdto.ProjectFilterRequest

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

	projectResponses := make([]responsedto.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		projectResponses = append(projectResponses, responsedto.ProjectSummaryFromModel(project))
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Projects retrieved successfully.",
		Data:       projectResponses,
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
//	@Param			request	body		requestdto.CreateProjectMemberRequest	true	"Project Member Request"
//	@Success		201		{object}	response.SuccessResponse	"Project members added successfully"
//	@Failure		400		{object}	response.ErrorResponse		"Validation error"
//	@Failure		401		{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse		"Forbidden"
//	@Failure		409		{object}	response.ErrorResponse		"Duplicate member"
//	@Failure		500		{object}	response.ErrorResponse		"Internal server error"
//
// @Router /project/add-members [post]
func (h *ProjectHandler) CreateProjectMember(g *gin.Context) {

	var payload requestdto.CreateProjectMemberRequest

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

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
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
//
// @Param           project_id path string true "Project ID"
// @Router          /project/members/{project_id} [get]
func (h *ProjectHandler) GetProjectMembers(g *gin.Context) {
	var filter requestdto.ProjectMemberFilter

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

	memberResponses := make([]responsedto.ProjectMember, 0, len(projectMembers))
	for _, member := range projectMembers {
		memberResponses = append(memberResponses, responsedto.ProjectMemberFromModel(member))
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Project members retrieved successfully.",
		Data:       memberResponses,
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
//
// @Param project_id path string true "Project ID"
// @Param user_id path string true "User ID"
// @Router /project/{project_id}/member/{user_id} [delete]
func (h *ProjectHandler) RemoveProjectMember(g *gin.Context) {
	projectID := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userID := g.Param("user_id")
	targetUserUUID, errorResponse := utils.StringToUUID(userID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}

	var performingUserUUID uuid.UUID
	if performingUserID, exist := g.Get("user_id"); exist {
		if pUUID, errResp := utils.StringToUUID(performingUserID.(string)); errResp == nil {
			performingUserUUID = pUUID
		}
	}

	var organizationUUID uuid.UUID
	if orgID, exist := g.Get("organization_id"); exist {
		if oUUID, errResp := utils.StringToUUID(orgID.(string)); errResp == nil {
			organizationUUID = oUUID
		}
	}

	payload := requestdto.RemoveProjectMember{
		ProjectID:        projectUUID,
		OrganizationID:   organizationUUID,
		PerformingUserID: performingUserUUID,
		TargetUserID:     targetUserUUID,
	}
	if serviceErr := h.service.RemoveProjectMember(payload); serviceErr != nil {
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

func (h *ProjectHandler) GetProjectActivity(g *gin.Context) {
	var filter requestdto.ProjectActivityFilterRequest

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
		h.logger.Error("Failed to convert project_id string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	roleVal, _ := g.Get("role")
	userRole, _ := roleVal.(string)

	var userOrgUUID uuid.UUID
	orgIDVal, exist := g.Get("organization_id")
	if exist {
		if orgUUID, errResp := utils.StringToUUID(orgIDVal.(string)); errResp == nil {
			userOrgUUID = orgUUID
		}
	}

	activities, pagination, serviceErr := h.service.GetProjectActivity(userUUID, userRole, userOrgUUID, projectID, filter)
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
		Message:    "Project activity history retrieved successfully.",
		Data:       activities,
		Meta:       &pagination,
	})
}

// GetProjectDetails godoc
//
//	@Summary		Get project details
//	@Description	Retrieve a project's details along with its members and sprints.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			project_id	path		string	true	"Project ID (UUID)"
//	@Success		200	{object}	response.SuccessResponse{data=responsedto.ProjectDetail}	"Project retrieved successfully"
//	@Failure		400	{object}	response.ErrorResponse	"Invalid project ID"
//	@Failure		401	{object}	response.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	response.ErrorResponse	"Forbidden"
//	@Failure		404	{object}	response.ErrorResponse	"Project not found"
//	@Failure		500	{object}	response.ErrorResponse	"Internal server error"
//	@Router			/api/v1/project/{project_id}/detail [get]
func (h *ProjectHandler) GetProjectDetails(g *gin.Context) {

	var payload requestdto.GetProjectDetails

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	payload.OrganizationID = organizationUUID
	payload.ProjectID = projectUUID
	payload.UserID = userUUID

	project, err := h.service.GetProjectDetails(payload)
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
		Message:    "Project retrieved successfully.",
		Data:       project,
	})
}

// DeleteProject godoc
//
//	@Summary		Delete Project
//	@Description	Delete an existing project
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID (UUID)"
//	@Success		200	{object}	response.SuccessResponse	"Project deleted successfully"
//	@Failure		400	{object}	response.ErrorResponse		"Invalid project ID"
//	@Failure		401	{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403	{object}	response.ErrorResponse		"Forbidden"
//	@Failure		404	{object}	response.ErrorResponse		"Project not found"
//	@Failure		500	{object}	response.ErrorResponse		"Internal Server Error"
//	@Router			/project/{project_id} [delete]
func (h *ProjectHandler) Deleteproject(g *gin.Context) {

	var payload requestdto.DeleteProject

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

		h.logger.Error("Internal server error: missing organization context")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	organizationIDStr := organizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(organizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	projectID := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	payload.ProjectID = projectUUID
	payload.OrganizationID = organizationUUID

	err := h.service.DeleteProject(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Project deleted successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"project_id": projectUUID,
		},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetProjectByUser godoc
//
//	@Summary		Get projects by user
//	@Description	Retrieve all projects associated with a specific user within the authenticated organization.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			user_id	query		string	true	"User ID"	format(uuid)
//	@Success		200		{object}	response.SuccessResponse{data=[]models.Project}
//	@Failure		400		{object}	response.ErrorResponse	"Validation Error"
//	@Failure		403		{object}	response.ErrorResponse	"Forbidden"
//	@Failure		500		{object}	response.ErrorResponse	"Internal Server Error"
//	@Security		BearerAuth
//	@Router			/api/v1/project/user [get]
func (h *ProjectHandler) GetProjectByUser(g *gin.Context) {

	var payload requestdto.GetProjectByUserID

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}
	userID := g.Param("user_id")
	userUUID, errorResponse := utils.StringToUUID(userID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	project, err := h.service.GetProjectsByUserID(payload)
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
		Message:    "Project retrieved successfully.",
		Data:       project,
	})
}

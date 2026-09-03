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
//	@Security		BearerAuth
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

	projectid, err := h.service.CreateProject(payload)
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
		Data: map[string]any{
			"project_id": projectid,
		},
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
//	@Security		BearerAuth
//	@Param			project_id	path	string							true	"Project ID"
//	@Param			request		body	requestdto.UpdateProjectRequest	true	"Update Project Request"
//	@Success		200			{object}	response.SuccessResponse	"Project updated successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Validation error"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403			{object}	response.ErrorResponse		"Forbidden"
//	@Failure		404			{object}	response.ErrorResponse		"Project not found"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//
// @Router /project/update/{project_id} [patch]
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

	payload.OrganizationID = organizationUUID
	payload.UserID = userUUID
	payload.ProjectID = projectUUID

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
			"project_id": projectUUID,
		},
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
//	@Security		BearerAuth
//	@Param			page		query		int		false	"Page Number"		default(1)
//	@Param			page_size	query		int		false	"Page Size"			default(10)
//	@Param			name		query		string	false	"Project Name or Slug"
//	@Param			status		query		string	false	"Project Status"	Enums(planning,active,on_hold,completed,cancelled,archived)
//	@Param			sort_by		query		string	false	"Sort by field"	Enums(name,created_at,updated_at,status)
//	@Param			sort_order	query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Param			fields		query		string	false	"Fields to return (comma separated)"
//	@Param			include_sprints	query	bool	false	"Include project sprints in the response"
//	@Success		200			{object}	response.SuccessResponse{data=[]responsedto.ProjectResponse}	"Projects retrieved successfully"
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	roleVal, _ := g.Get("role")
	userRole, _ := roleVal.(string)

	filter.UserID = userUUID
	filter.OrganizationID = organizationUUID
	filter.UserRole = userRole

	projectResponses, pagination, err := h.service.GetProjectsByOrganizationID(filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	var filteredData any = projectResponses
	if filter.Fields != "" {
		var filterErr error
		filteredData, filterErr = utils.FilterFields(projectResponses, filter.Fields)
		if filterErr != nil {
			h.logger.Error("Failed to filter project fields", zap.Error(filterErr))
			filteredData = projectResponses
		}
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Projects retrieved successfully.",
		Data:       filteredData,
		Meta:       &pagination,
	})
}

// GetAllProjects godoc
//
//	@Summary		Get all projects (Super Admin)
//	@Description	Returns a paginated list of all projects across all organizations with search, filters, and sorting.
//	@Tags			Projects
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page			query		int		false	"Page Number"		default(1)
//	@Param			page_size		query		int		false	"Page Size"			default(10)
//	@Param			search			query		string	false	"Search query (name, description, slug)"
//	@Param			name			query		string	false	"Project Name or Slug"
//	@Param			status			query		string	false	"Project Status"	Enums(planning,active,on_hold,completed,cancelled,archived)
//	@Param			organization_id	query		string	false	"Organization ID"
//	@Param			created_by		query		string	false	"Creator User ID"
//	@Param			include_sprints	query		bool	false	"Include project sprints"
//	@Param			sort_by			query		string	false	"Sort by field"	Enums(name,created_at,updated_at,status)
//	@Param			sort_order		query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200				{object}	response.SuccessResponse{data=[]responsedto.ProjectResponse}	"Projects retrieved successfully"
//	@Failure		400				{object}	response.ErrorResponse		"Invalid query parameters"
//	@Failure		401				{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		403				{object}	response.ErrorResponse		"Forbidden"
//	@Failure		500				{object}	response.ErrorResponse		"Internal server error"
//	@Router			/project/all-projects [get]
func (h *ProjectHandler) GetAllProjects(g *gin.Context) {
	var filter requestdto.GlobalProjectFilterRequest

	if err := g.ShouldBindQuery(&filter); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid query parameters",
			},
		}
		g.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	projectResponses, pagination, err := h.service.GetAllProjects(filter)
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
		Message:    "All projects retrieved successfully.",
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
//	@Security		BearerAuth
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
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
//	@Success		200			{object}	response.SuccessResponse{data=[]responsedto.ProjectMember}	"Project members retrieved successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Invalid request"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		404			{object}	response.ErrorResponse		"Project not found"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//
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

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	filter.UserID = userUUID
	filter.OrganizationID = organizationUUID

	projectIDParam := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
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
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID"
//	@Param			user_id		path		string	true	"User ID"
//	@Success		200			{object}	response.SuccessResponse	"Project member removed successfully"
//	@Failure		400			{object}	response.ErrorResponse		"Invalid request"
//	@Failure		401			{object}	response.ErrorResponse		"Unauthorized"
//	@Failure		404			{object}	response.ErrorResponse		"Project member not found"
//	@Failure		500			{object}	response.ErrorResponse		"Internal server error"
//
// @Router /project/{project_id}/member/{user_id} [delete]
func (h *ProjectHandler) RemoveProjectMember(g *gin.Context) {
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

	userID := g.Param("user_id")
	targetUserUUID, errorResponse := utils.StringToUUID(userID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	performingUserUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
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
		Data: map[string]uuid.UUID{
			"ProjectID": projectUUID},
	})
}

// GetProjectActivity godoc
//
//	@Summary		Get project activity history
//	@Description	Retrieve activity logs for a project with optional filters and pagination.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID (UUID)"
//	@Param			type		path		string	true	"Activity Type" Enums(view, activity)
//	@Param			page		query		int		false	"Page number" default(1)
//	@Param			page_size	query		int		false	"Page size" default(10)
//	@Param			action		query		string	false	"Filter by action"
//	@Param			resource_type	query		string	false	"Filter by resource type" Enums(project, task, userstory, sprint, comment)
//	@Param			resource_id	query		string	false	"Filter by resource ID"
//	@Param			task_id		query		string	false	"Filter by task ID (UUID)"
//	@Param			user_story_id	query		string	false	"Filter by user story ID (UUID)"
//	@Param			sprint_id	query		string	false	"Filter by sprint ID (UUID)"
//	@Param			user_id		query		string	false	"Filter by user ID"
//	@Param			start_date	query		string	false	"Start date (ISO-8601)"
//	@Param			end_date		query		string	false	"End date (ISO-8601)"
//	@Success		200			{object}	response.SuccessResponse{data=[]responsedto.ProjectActivityResponse}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		401			{object}	response.ErrorResponse
//	@Failure		403			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/project/{project_id}/activity/{type} [get]
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

	if typeParam := g.Param("type"); typeParam != "" {
		filter.Type = typeParam
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

	userOrgUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
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
//	@Description	Retrieve a project's details along with its members, sprints, and current active sprint.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID (UUID) or Project Slug"
//	@Success		200	{object}	response.SuccessResponse{data=responsedto.ProjectDetail}	"Project retrieved successfully"
//	@Failure		400	{object}	response.ErrorResponse	"Invalid project ID or Slug"
//	@Failure		401	{object}	response.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	response.ErrorResponse	"Forbidden"
//	@Failure		404	{object}	response.ErrorResponse	"Project not found"
//	@Failure		500	{object}	response.ErrorResponse	"Internal server error"
//	@Router			/project/{project_id}/detail [get]
func (h *ProjectHandler) GetProjectDetails(g *gin.Context) {

	var payload requestdto.GetProjectDetails

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	projectIDParam := g.Param("project_id")
	projectUUID, err := uuid.FromString(projectIDParam)
	if err == nil {
		payload.ProjectID = projectUUID
	} else {
		payload.ProjectSlug = projectIDParam
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	payload.OrganizationID = organizationUUID
	payload.UserID = userUUID

	project, errResp := h.service.GetProjectDetails(payload)
	if errResp != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *errResp,
		}
		g.JSON(errResp.StatusCode, errorResponse)
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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
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
	payload.UserID = userUUID

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
//	@Param			user_id	path		string	true	"User ID (UUID)"
//	@Success		200		{object}	response.SuccessResponse{data=responsedto.GetProjectByUserIDResponse}
//	@Failure		400		{object}	response.ErrorResponse	"Validation Error"
//	@Failure		403		{object}	response.ErrorResponse	"Forbidden"
//	@Failure		500		{object}	response.ErrorResponse	"Internal Server Error"
//	@Security		BearerAuth
//	@Router			/project/user/{user_id} [get]
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

	callerUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}
	roleVal, _ := g.Get("role")
	callerRole, _ := roleVal.(string)

	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID
	payload.CallerID = callerUUID
	payload.CallerRole = callerRole

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

// GetProjectRole godoc
//
//	@Summary		Get user role in project
//	@Description	Retrieve the role of the authenticated user in the specified project.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Param			project_id	path		string	true	"Project ID (UUID)"
//	@Success		200		{object}	response.SuccessResponse{data=responsedto.GetUserProjectRoleResponse}
//	@Failure		400		{object}	response.ErrorResponse	"Validation Error"
//	@Failure		403		{object}	response.ErrorResponse	"Forbidden"
//	@Failure		404		{object}	response.ErrorResponse	"Not Found"
//	@Failure		500		{object}	response.ErrorResponse	"Internal Server Error"
//	@Security		BearerAuth
//	@Router			/project/{project_id}/user-role [get]
func (h *ProjectHandler) GetProjectRole(g *gin.Context) {

	projectIDParam := g.Param("project_id")
	projectUUID, errorResponse := utils.StringToUUID(projectIDParam)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
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

	payload := requestdto.GetUserProjectRoleRequest{
		ProjectID:      projectUUID,
		UserID:         userUUID,
		OrganizationID: organizationUUID,
	}

	roleResp, err := h.service.GetUserProjectRole(payload)
	if err != nil {
		g.JSON(err.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "User project role retrieved successfully.",
		Data:       roleResp,
	})
}

// GetRecentProjects godoc
//
//	@Summary		Get recent projects
//	@Description	Retrieve the recent projects assigned to the user that have at least one valid task.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	response.SuccessResponse{data=responsedto.GetProjectByUserIDResponse}
//	@Failure		400		{object}	response.ErrorResponse	"Validation Error"
//	@Failure		403		{object}	response.ErrorResponse	"Forbidden"
//	@Failure		500		{object}	response.ErrorResponse	"Internal Server Error"
//	@Security		BearerAuth
//	@Router			/project/recent [get]
func (h *ProjectHandler) GetRecentProjects(g *gin.Context) {

	var payload requestdto.GetProjectByUserID

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	payload.UserID = userUUID
	payload.OrganizationID = organizationUUID

	project, err := h.service.GetRecentProjects(payload)
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
		Message:    "Recent projects retrieved successfully.",
		Data:       project,
	})
}

// UpdateProjectMember godoc
//
//	@Summary		Update project member role
//	@Description	Update the role of an existing project member. Only authorized users can update member roles based on project role permissions.
//	@Tags			Project Members
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string									true	"Project ID"	Format(uuid)
//	@Param			user_id		path		string									true	"User ID"		Format(uuid)
//	@Param			request		body		requestdto.UpdateProjectMemberRequest	true	"Update Project Member Request"
//	@Success		200		{object}	response.SuccessResponse	"Project member updated successfully"
//	@Failure		400			{object}	response.ErrorResponse	"Invalid request payload or validation error"
//	@Failure		401			{object}	response.ErrorResponse	"Unauthorized"
//	@Failure		403			{object}	response.ErrorResponse	"Forbidden"
//	@Failure		404			{object}	response.ErrorResponse	"Project member not found"
//	@Failure		500			{object}	response.ErrorResponse	"Internal server error"
//	@Router			/project/{project_id}/member/{user_id} [patch]
func (h *ProjectHandler) UpdateProjectMember(g *gin.Context) {

	var payload requestdto.UpdateProjectMemberRequest

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

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	UpdatedBy, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	id := g.Param("project_id")
	projectID, errorResponse := utils.StringToUUID(id)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *errorResponse,
		})
		return
	}

	payload.OrganizationID = organizationUUID
	payload.ProjectID = projectID
	payload.UpdatedBy = UpdatedBy
	payload.MemberID = userUUID

	err := h.service.UpdateProjectMember(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Project member updated successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"ProjectID": projectID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

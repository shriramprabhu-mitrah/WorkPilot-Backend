package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/config"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	cookies "github.com/ms-kanban-server/internal/pkg/cookie"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/storage"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitOrganizationHandler(service services.OrganizationService, publicService services.PublicService, storage storage.StorageClient, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		service:       service,
		publicService: publicService,
		storage:       storage,
		logger:        logger,
	}
}

type OrganizationHandler struct {
	service       services.OrganizationService
	publicService services.PublicService
	storage       storage.StorageClient
	logger        *zap.Logger
}

// deleteOrganization godoc
//
// @Summary      delete current Organization
// @Description  Returns the profile of the authenticated Organization.
// @Tags         Organizations
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=responsedto.OrganizationSummary}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/delete [delete]
func (h *OrganizationHandler) DeleteOrganization(g *gin.Context) {
	id, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	err := h.service.DeleteOrganization(id)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Organization deleted successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"organizationID": id},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateOrganization godoc
//
// @Summary      Update Organization
// @Description  Updates Organization profile. Send as multipart/form-data. Include a 'logo' file field to replace the logo.
// @Tags         Organizations
// @Accept       multipart/form-data
// @Produce      json
// @Param        name      formData string false "Organization name"
// @Param        domain    formData string false "Organization domain"
// @Param        team_size formData string false "Team size" Enums(1-10,11-50,51-200,201-500,501-1000,1000+)
// @Param        country_id formData string false "Country ID (UUID)"
// @Param        logo      formData file   false "Organization logo (PNG, JPG/JPEG, WEBP — max configurable MB)"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/update [patch]
func (h *OrganizationHandler) UpdateOrganization(g *gin.Context) {

	var payload requestdto.UpdateOrganizationRequest

	if err := g.ShouldBind(&payload); err != nil {
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

	// Handle optional logo upload.
	var logoURL string
	var uploadedKey string
	logoFile, logoHeader, fileErr := g.Request.FormFile("logo")
	if fileErr == nil {
		defer logoFile.Close()
		var uploadErr *response.Error
		logoURL, uploadedKey, uploadErr = h.storage.UploadLogo(logoFile, logoHeader)
		if uploadErr != nil {
			h.logger.Error("Logo upload failed during organization update", zap.String("error", uploadErr.Message))
			g.JSON(uploadErr.StatusCode, &response.ErrorResponse{Success: false, Error: *uploadErr})
			return
		}
	}

	credentials := models.Organization{
		Name:     payload.Name,
		Domain:   payload.Domain,
		LogoURL:  logoURL,
		TeamSize: string(payload.TeamSize),
	}

	if payload.CountryID != "" {
		countryUUID, errorResponse := utils.StringToUUID(payload.CountryID)
		if errorResponse != nil {
			h.logger.Error("Invalid country id")
			// Clean up orphaned upload.
			if uploadedKey != "" {
				_ = h.storage.DeleteObject(context.Background(), uploadedKey)
			}
			g.JSON(errorResponse.StatusCode, errorResponse)
			return
		}

		country, err := h.publicService.GetCountryByID(countryUUID)
		if err != nil {
			h.logger.Error("Failed to resolve country id",
				zap.String("message", err.Message),
				zap.Int("status", err.StatusCode))
			// Clean up orphaned upload.
			if uploadedKey != "" {
				_ = h.storage.DeleteObject(context.Background(), uploadedKey)
			}
			g.JSON(err.StatusCode, &response.ErrorResponse{Success: false, Error: *err})
			return
		}

		credentials.Country = country.Name
	}

	updateErr := h.service.UpdateOrganization(organizationUUID, credentials)
	if updateErr != nil {
		// Clean up orphaned upload since the DB update failed.
		if uploadedKey != "" {
			_ = h.storage.DeleteObject(context.Background(), uploadedKey)
		}
		g.JSON(updateErr.StatusCode, &response.ErrorResponse{Success: false, Error: *updateErr})
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated Organization successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"organizationID": organizationUUID},
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

// GetOrganization godoc
//
// @Summary      Get current Organization
// @Description  Returns the profile of the authenticated Organization.
// @Tags         Organizations
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=responsedto.OrganizationSummary}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/get [get]
func (h *OrganizationHandler) GetOrganizationByID(g *gin.Context) {

	id, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	userUUID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	result, err := h.service.GetOrganizationByID(id,userUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	organizationResponse := responsedto.OrganizationFromModel(result)

	successResponse := &response.SuccessResponse{
		Message:    "Organization detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       organizationResponse,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetAllOrganizations godoc
//
//	@Summary		Get all Organizations
//	@Description	Returns a list of all registered Organizations with filtering, sorting, and pagination.
//	@Tags			Organizations
//	@Produce		json
//	@Param			page		query		int		false	"Page Number"		default(1)
//	@Param			page_size	query		int		false	"Page Size"			default(10)
//	@Param			name		query		string	false	"Organization Name"
//	@Param			domain		query		string	false	"Domain"
//	@Param			industry	query		string	false	"Industry"
//	@Param			team_size	query		string	false	"Team Size"
//	@Param			country		query		string	false	"Country"
//	@Param			is_active	query		bool	false	"Is Active"
//	@Param			search		query		string	false	"Search term across name, domain, industry, slug"
//	@Param			sort_by		query		string	false	"Sort by field"	Enums(name,created_at,updated_at,domain,industry,team_size,is_active)
//	@Param			sort_order	query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200			{object}	response.SuccessResponse{data=[]responsedto.OrganizationSummary}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		401			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/organization [get]
func (h *OrganizationHandler) GetAllOrganizations(g *gin.Context) {
	var filter requestdto.OrganizationFilterRequest

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

	results, pagination, err := h.service.GetAllOrganizations(filter)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	orgResponses := make([]responsedto.OrganizationSummary, 0, len(results))
	for _, org := range results {
		orgResponses = append(orgResponses, responsedto.OrganizationFromModel(org))
	}

	successResponse := &response.SuccessResponse{
		Message:    "All organizations retrieved successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       orgResponses,
		Meta:       &pagination,
	}
	g.JSON(http.StatusOK, successResponse)
}

// CreateOrganization godoc
//
// @Summary      Register a new Organization
// @Description  Creates a new Organization account. Send as multipart/form-data. Optionally include a 'logo' file to set the organization logo.
// @Tags         Organizations
// @Accept       multipart/form-data
// @Produce      json
// @Param        name       formData string true  "Organization name"
// @Param        domain     formData string true  "Organization domain"
// @Param        industry   formData string true  "Industry" Enums(Information_Technology,Finance,Healthcare,Education,Manufacturing,Retail,Real Estate,Logistics,Hospitality,Other)
// @Param        team_size  formData string true  "Team size" Enums(1-10,11-50,51-200,201-500,501-1000,1000+)
// @Param        country_id formData string true  "Country ID (UUID)"
// @Param        logo       formData file   false "Organization logo (PNG, JPG/JPEG, WEBP — max configurable MB)"
// @Success      201 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/create [post]
func (h *OrganizationHandler) CreateOrganization(g *gin.Context) {

	var payload requestdto.CreateOrganizationRequest

	if err := g.ShouldBind(&payload); err != nil {
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

	countryUUID, errorResponse := utils.StringToUUID(payload.CountryID)
	if errorResponse != nil {
		h.logger.Error("Invalid country id")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	country, err := h.publicService.GetCountryByID(countryUUID)
	if err != nil {
		h.logger.Error("Failed to resolve country id", zap.String("message", err.Message), zap.Int("status", err.StatusCode))
		g.JSON(err.StatusCode, &response.ErrorResponse{Success: false, Error: *err})
		return
	}

	// Handle optional logo upload.
	var logoURL string
	var uploadedKey string
	logoFile, logoHeader, fileErr := g.Request.FormFile("logo")
	if fileErr == nil {
		defer logoFile.Close()
		var uploadErr *response.Error
		logoURL, uploadedKey, uploadErr = h.storage.UploadLogo(logoFile, logoHeader)
		if uploadErr != nil {
			h.logger.Error("Logo upload failed during organization creation", zap.String("error", uploadErr.Message))
			g.JSON(uploadErr.StatusCode, &response.ErrorResponse{Success: false, Error: *uploadErr})
			return
		}
	}

	credentials := models.Organization{
		Name:      payload.Name,
		Domain:    payload.Domain,
		LogoURL:   logoURL,
		CreatedBy: userUUID,
		Industry:  string(payload.Industry),
		TeamSize:  string(payload.TeamSize),
		Country:   country.Name,
	}

	tokens, err := h.service.CreateOrganization(credentials)
	if err != nil {
		// Clean up orphaned upload since the DB creation failed.
		if uploadedKey != "" {
			_ = h.storage.DeleteObject(context.Background(), uploadedKey)
		}
		g.JSON(err.StatusCode, &response.ErrorResponse{Success: false, Error: *err})
		return
	}

	secure, secureErr := utils.StringToBool(config.GetEnv("COOKIE_SECURE", ""))
	if secureErr != nil {
		g.JSON(secureErr.StatusCode, &response.ErrorResponse{Success: false, Error: *secureErr})
		return
	}

	cookies.SetAccessToken(g, tokens.AccessToken, tokens.ExpiresIn, secure)
	cookies.SetRefreshToken(g, tokens.RefreshToken, tokens.RefreshExpiresIn, secure)

	g.JSON(http.StatusCreated, &response.SuccessResponse{
		Message:    "Successfully Created",
		StatusCode: http.StatusCreated,
		Success:    true,
	})
}

// UpdateUserStatus godoc
//
// @Summary      Update User Status
// @Description  Updates User profile.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body requestdto.UserStatusRequest true "Update User Status Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/user-status [patch]
func (h *OrganizationHandler) UpdateUserStatus(g *gin.Context) {

	var payload requestdto.UserStatusRequest

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

	userID, errorResponse := utils.StringToUUID(payload.UserID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	credentials := requestdto.UpdateUserStatus{
		OrganizationID: &organizationUUID,
		UserID:         userID,
		IsActive:       payload.IsActive,
	}
	err := h.service.UpdateUserStatus(credentials)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated User Status successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"OrganizationID": organizationUUID,
			"user_id":        payload.UserID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateUserRole godoc
//
// @Summary      Update User Role
// @Description  Updates User profile.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body requestdto.UserRoleRequest true "Update User Role Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/user-role [patch]
func (h *OrganizationHandler) UpdateUserRole(g *gin.Context) {

	var payload requestdto.UserRoleRequest

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

	userID, errorResponse := utils.StringToUUID(payload.UserID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	if err := payload.Role.Validate(); err != nil {
		h.logger.Error("Invalid role", zap.Error(err))

		resp := response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Role must be one of org_admin or member.",
		}

		g.JSON(resp.StatusCode, response.ErrorResponse{Success: false, Error: resp})
		return
	}

	credentials := requestdto.UpdateUserRole{
		OrganizationID: &organizationUUID,
		UserID:         userID,
		Role:           string(payload.Role),
	}
	err := h.service.UpdateUserRole(credentials)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated User Role successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"OrganizationID": organizationUUID,
			"user_id":        payload.UserID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetUserInOrganization godoc
//
// @Summary      Get current Organization
// @Description  Returns the profile of the authenticated Organization.
// @Tags         Organizations
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=[]responsedto.UserProfile}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/get-users [get]
func (h *OrganizationHandler) GetUserInOrganization(g *gin.Context) {

	organizationUUID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	page, pageErr := strconv.Atoi(g.DefaultQuery("page", "1"))
	pageSize, pageSizeErr := strconv.Atoi(g.DefaultQuery("page_size", "10"))
	if pageErr != nil || pageSizeErr != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid pagination parameters.",
			},
		}

		h.logger.Error("Invalid pagination parameters")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	fullName := strings.TrimSpace(g.Query("full_name"))
	email := strings.TrimSpace(g.Query("email"))
	username := strings.TrimSpace(g.Query("username"))
	role := strings.TrimSpace(g.Query("role"))
	timezone := strings.TrimSpace(g.Query("timezone"))

	isActiveQuery := g.Query("is_active")
	var isActive *bool
	if isActiveQuery != "" {
		v := strings.EqualFold(isActiveQuery, "true")
		isActive = &v
	}

	isVerifiedQuery := g.Query("is_verified")
	var isVerified *bool
	if isVerifiedQuery != "" {
		v := strings.EqualFold(isVerifiedQuery, "true")
		isVerified = &v
	}

	includeOrgAdminsQuery := g.Query("include_org_admins")
	includeOrgAdmins := strings.EqualFold(includeOrgAdminsQuery, "true")

	filter := requestdto.OrganizationMemberListFilter{
		PaginationQuery: response.PaginationQuery{
			Page:     page,
			PageSize: pageSize,
		},
		FullName:         fullName,
		Email:            email,
		Username:         username,
		Role:             role,
		IsActive:         isActive,
		IsVerified:       isVerified,
		Timezone:         timezone,
		IncludeOrgAdmins: includeOrgAdmins,
	}

	users, pagination, respErr := h.service.GetUserInOrganization(organizationUUID, filter)
	if respErr != nil {
		g.JSON(respErr.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *respErr,
		})
		return
	}

	usersResponse := make([]responsedto.UserProfile, 0, len(users))
	for _, user := range users {
		usersResponse = append(usersResponse, responsedto.UserProfileFromModel(user))
	}

	successResponse := &response.SuccessResponse{
		Message:    "Organization detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       usersResponse,
		Meta:       &pagination,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// RemoveUser godoc
//
//	@Summary		Remove user from organization
//	@Description	Removes a user from the current organization.
//	@Tags			Organizations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			user_id	path		string	true	"User ID (UUID)"
//	@Success		200	{object}	response.SuccessResponse
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		401	{object}	response.ErrorResponse
//	@Failure		403	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/organization/remove-user/{user_id} [delete]
func (h *OrganizationHandler) RemoveUser(g *gin.Context) {

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

	err := h.service.RemoveUser(requestdto.RemoveUser{
		OrganizationID: &organizationUUID,
		UserID:         userUUID,
	})

	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Removed User Successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]uuid.UUID{
			"OrganizationID": organizationUUID,
			"user_id":        userUUID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// InviteOrganizationMember godoc
//
// @Summary      Invite member to organization
// @Description  Sends an email invitation to a user to join the organization.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body requestdto.InviteOrganizationMemberRequest true "Invite Organization Member Request"
// @Success      201 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/invite [post]
func (h *OrganizationHandler) InviteOrganizationMember(g *gin.Context) {
	var payload requestdto.InviteOrganizationMemberRequest
	if err := g.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("Invalid invite payload", zap.Error(err))
		message := utils.ValidationErrorMessage(err, payload)
		g.JSON(http.StatusBadRequest, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			}},
		)
		return
	}

	organizationID, ok := getRequiredContextUUID(g, h.logger, "organization_id", "organization")
	if !ok {
		return
	}

	inviterID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		return
	}

	if err := h.service.InviteOrganizationMember(inviterID, organizationID, payload); err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Invitation sent successfully"})
}

// AcceptInvitationPage godoc
//
// @Summary      Show accept invitation page or redirect to login
// @Description  Validates the token, checks login status, and renders acceptance page or redirects to login.
// @Tags         Organizations
// @Produce      html
// @Param        token query string true "Invitation token"
// @Success      200 {string} string "HTML Confirmation Page"
// @Router       /organization/invitations/accept [get]
func (h *OrganizationHandler) AcceptInvitationPage(g *gin.Context) {
	token := g.Query("token")
	if token == "" {
		g.JSON(http.StatusBadRequest, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invitation token is required",
			}},
		)
		return
	}

	baseURL := strings.TrimSuffix(config.GetEnv("FRONTEND_DASHBOARD_URL", "http://localhost:3000"), "/")
	dashboardURL := baseURL + "/dashboard"
	loginURL := baseURL + "/signin"

	dataMap := map[string]any{
		"DashboardURL": dashboardURL,
		"LoginURL":     loginURL,
	}

	invitation, err := h.service.GetInvitationByToken(token)
	if err != nil {
		htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_error.html", map[string]any{
			"Message":      err.Message,
			"DashboardURL": dashboardURL,
		})
		if renderErr != nil {
			g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: err.Message}})
			return
		}
		g.Header("Content-Type", "text/html")
		g.String(http.StatusOK, htmlString)
		return
	}

	if invitation.ID == uuid.Nil {
		htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_not_found.html", dataMap)
		if renderErr != nil {
			g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Invitation not found"}})
			return
		}
		g.Header("Content-Type", "text/html")
		g.String(http.StatusOK, htmlString)
		return
	}

	if invitation.Status == models.InvitationStatusAccepted {
		htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_already_accepted.html", dataMap)
		if renderErr != nil {
			g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Invitation has already been accepted"}})
			return
		}
		g.Header("Content-Type", "text/html")
		g.String(http.StatusOK, htmlString)
		return
	}

	if invitation.Status == models.InvitationStatusExpired || invitation.ExpiresAt.Before(time.Now()) {
		htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_expired.html", dataMap)
		if renderErr != nil {
			g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Invitation has expired"}})
			return
		}
		g.Header("Content-Type", "text/html")
		g.String(http.StatusOK, htmlString)
		return
	}

	_, cookieErr := g.Cookie("access_token")
	if cookieErr != nil {
		htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_login_required.html", dataMap)
		if renderErr != nil {
			g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Login required"}})
			return
		}
		g.Header("Content-Type", "text/html")
		g.String(http.StatusOK, htmlString)
		return
	}

	htmlString, renderErr := utils.RenderEmbeddedTemplate("accept_invitation.html", map[string]any{
		"Token": token,
	})
	if renderErr != nil {
		h.logger.Error("Failed to render AcceptInvitationPage template", zap.Error(renderErr))
		g.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrInternalServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to render invitation acceptance page",
			}},
		)
		return
	}

	g.Header("Content-Type", "text/html")
	g.String(http.StatusOK, htmlString)
}

// AcceptInvitation godoc
//
// @Summary      Accept organization invitation
// @Description  Accepts a pending organization invitation using the provided token.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body requestdto.AcceptInvitationRequest true "Accept invitation"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/invitations/accept [post]
func (h *OrganizationHandler) AcceptInvitation(g *gin.Context) {
	var payload requestdto.AcceptInvitationRequest
	if err := g.ShouldBind(&payload); err != nil {
		message := utils.ValidationErrorMessage(err, payload)
		if g.ContentType() == "application/x-www-form-urlencoded" {
			htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_error.html", map[string]any{
				"Message": message,
			})
			if renderErr != nil {
				g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: message}})
				return
			}
			g.Header("Content-Type", "text/html")
			g.String(http.StatusOK, htmlString)
			return
		}
		g.JSON(http.StatusBadRequest, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			}},
		)
		return
	}

	userID, ok := getRequiredContextUUID(g, h.logger, "user_id", "user")
	if !ok {
		if g.ContentType() == "application/x-www-form-urlencoded" {
			htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_login_required.html", nil)
			if renderErr != nil {
				g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: "Unauthorized"}})
				return
			}
			g.Header("Content-Type", "text/html")
			g.String(http.StatusOK, htmlString)
			return
		}
		return
	}

	if err := h.service.AcceptInvitation(userID, payload.Token); err != nil {
		if g.ContentType() == "application/x-www-form-urlencoded" {
			htmlString, renderErr := utils.RenderEmbeddedTemplate("invitation_error.html", map[string]any{
				"Message": err.Message,
			})
			if renderErr != nil {
				g.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Error: response.Error{Code: response.ErrInternalServerError, StatusCode: http.StatusInternalServerError, Message: err.Message}})
				return
			}
			g.Header("Content-Type", "text/html")
			g.String(http.StatusOK, htmlString)
			return
		}
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	if g.ContentType() == "application/x-www-form-urlencoded" {
		baseURL := strings.TrimSuffix(config.GetEnv("FRONTEND_DASHBOARD_URL", "http://localhost:3000"), "/")
		g.Redirect(http.StatusFound, baseURL+"/teams")
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Invitation accepted successfully"})
}

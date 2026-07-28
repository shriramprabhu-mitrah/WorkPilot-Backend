package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	cookies "github.com/ms-kanban-server/internal/pkg/cookie"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitOrganizationHandler(service services.OrganizationService, publicService services.PublicService, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		service:       service,
		publicService: publicService,
		logger:        logger,
	}
}

type OrganizationHandler struct {
	service       services.OrganizationService
	publicService services.PublicService
	logger        *zap.Logger
}

// deleteOrganization godoc
//
// @Summary      delete current Organization
// @Description  Returns the profile of the authenticated Organization.
// @Tags         Organizations
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=models.Organization}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/delete [delete]
func (h *OrganizationHandler) DeleteOrganization(g *gin.Context) {

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateOrganization godoc
//
// @Summary      Update Organization
// @Description  Updates Organization profile.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body dto.UpdateOrganizationRequest true "Update Organization Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/update [patch]
func (h *OrganizationHandler) UpdateOrganization(g *gin.Context) {

	var payload dto.UpdateOrganizationRequest

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

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	credentials := models.Organization{
		Name:    payload.Name,
		Domain:  payload.Domain,
		LogoURL: payload.LogoURL,
	}

	if payload.CountryID != "" {
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

		credentials.Country = country.Name
	}
	err := h.service.UpdateOrganization(id, credentials)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated Organization successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"OrganizationID": id},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetOrganization godoc
//
// @Summary      Get current Organization
// @Description  Returns the profile of the authenticated Organization.
// @Tags         Organizations
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=models.Organization}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/get [get]
func (h *OrganizationHandler) GetOrganizationByID(g *gin.Context) {

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	result, err := h.service.GetOrganizationByID(id)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Organization detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       result,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// CreateOrganization godoc
//
// @Summary      Register a new Organization
// @Description  Creates a new Organization account.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateOrganizationRequest true "Creates new Organization"
// @Success      201 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/create [post]
func (h *OrganizationHandler) CreateOrganization(g *gin.Context) {

	var payload dto.CreateOrganizationRequest

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

	UserUUID, errorResponse := utils.StringToUUID(userID.(string))
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
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

	credentials := models.Organization{
		Name:      payload.Name,
		Domain:    payload.Domain,
		LogoURL:   payload.LogoURL,
		CreatedBy: UserUUID,
		Industry:  string(payload.Industry),
		TeamSize:  string(payload.TeamSize),
		Country:   country.Name,
	}

	tokens, err := h.service.CreateOrganization(credentials)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	secure, secureErr := utils.StringToBool(config.GetEnv("COOKIE_SECURE", ""))
	if secureErr != nil {
		g.JSON(secureErr.StatusCode, &response.ErrorResponse{Success: false, Error: *secureErr})
		return
	}

	cookies.SetAccessToken(g, tokens.AccessToken, tokens.ExpiresIn, secure)
	cookies.SetRefreshToken(g, tokens.RefreshToken, tokens.RefreshExpiresIn, secure)

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateUserStatus godoc
//
// @Summary      Update User Status
// @Description  Updates User profile.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body dto.UserStatusRequest true "Update User Status Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router      /organization/user-status/ [patch]
func (h *OrganizationHandler) UpdateUserStatus(g *gin.Context) {

	var payload dto.UserStatusRequest

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

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	organizationUUID, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userID, errorResponse := utils.StringToUUID(payload.UserID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	credentials := dto.UpdateUserStatus{
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
// @Param        request body dto.UserRoleRequest true "Update User Role Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /user-role/ [patch]
func (h *OrganizationHandler) UpdateUserRole(g *gin.Context) {

	var payload dto.UserRoleRequest

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

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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
			Message:    "Role must be one of org_admin, project_manager, developer, viewer.",
		}

		g.JSON(resp.StatusCode, response.ErrorResponse{Success: false, Error: resp})
		return
	}

	credentials := dto.UpdateUserRole{
		OrganizationID: &id,
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
			"OrganizationID": id,
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
// @Success      200 {object} response.SuccessResponse{data=models.Organization}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/get-user [get]
func (h *OrganizationHandler) GetUserInOrganization(g *gin.Context) {

	OrganizationID, exist := g.Get("organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
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

	filter := dto.OrganizationMemberListFilter{
		Page:       page,
		PageSize:   pageSize,
		FullName:   fullName,
		Email:      email,
		Username:   username,
		Role:       role,
		IsActive:   isActive,
		IsVerified: isVerified,
		Timezone:   timezone,
	}

	users, pagination, respErr := h.service.GetUserInOrganization(id, filter)
	if respErr != nil {
		g.JSON(respErr.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *respErr,
		})
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Organization detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       users,
		Meta:       &pagination,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateUserRole godoc
//
// @Summary      RemoveUser
// @Description  RemoveUser.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body dto.RemoveUserRequest true "RemoveUser Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /remove-user/ [delete]
func (h *OrganizationHandler) RemoveUser(g *gin.Context) {

	var payload dto.RemoveUserRequest

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

	OrganizationID, exist := g.Get("Organization_id")
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
	OrganizationIDStr := OrganizationID.(string)

	id, errorResponse := utils.StringToUUID(OrganizationIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	userID, errorResponse := utils.StringToUUID(payload.UserID)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	credentials := dto.RemoveUser{
		OrganizationID: &id,
		UserID:         userID,
	}
	err := h.service.RemoveUser(credentials)
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
		Data: map[string]any{
			"OrganizationID": id,
			"user_id":        payload.UserID},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

func (h *OrganizationHandler) InviteOrganizationMember(g *gin.Context) {
	var payload dto.InviteOrganizationMemberRequest
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

	organizationIDVal, exist := g.Get("organization_id")
	if !exist {
		g.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing organization context",
			}},
		)
		return
	}
	organizationID, convErr := utils.StringToUUID(organizationIDVal.(string))
	if convErr != nil {
		g.JSON(convErr.StatusCode, response.ErrorResponse{Success: false, Error: *convErr})
		return
	}

	inviterIDVal, exist := g.Get("user_id")
	if !exist {
		g.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			}},
		)
		return
	}
	inviterID, convErr := utils.StringToUUID(inviterIDVal.(string))
	if convErr != nil {
		g.JSON(convErr.StatusCode, response.ErrorResponse{Success: false, Error: *convErr})
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

// AcceptInvitation godoc
//
// @Summary      Accept organization invitation
// @Description  Accepts a pending organization invitation using the provided token.
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        request body dto.AcceptInvitationRequest true "Accept invitation"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /organization/invitations/accept [post]
func (h *OrganizationHandler) AcceptInvitation(g *gin.Context) {
	var payload dto.AcceptInvitationRequest
	if err := g.ShouldBindJSON(&payload); err != nil {
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

	userIDVal, exist := g.Get("user_id")
	if !exist {
		g.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
			}},
		)
		return
	}
	userID, convErr := utils.StringToUUID(userIDVal.(string))
	if convErr != nil {
		g.JSON(convErr.StatusCode, response.ErrorResponse{Success: false, Error: *convErr})
		return
	}

	if err := h.service.AcceptInvitation(userID, payload.Token); err != nil {
		g.JSON(err.StatusCode, response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Invitation accepted successfully"})
}

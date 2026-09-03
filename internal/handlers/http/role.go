package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

var _ = responsedto.RoleResponse{}

type RoleHandler struct {
	roleService services.RoleService
	authService services.AuthService
	logger      *zap.Logger
}

func InitRoleHandler(roleService services.RoleService, authService services.AuthService, logger *zap.Logger) *RoleHandler {
	return &RoleHandler{
		roleService: roleService,
		authService: authService,
		logger:      logger,
	}
}

func (h *RoleHandler) getOrgIDAndVerifyAdmin(c *gin.Context) (uuid.UUID, bool) {
	orgIDVal, exists := c.Get("organization_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	orgIDStr, ok := orgIDVal.(string)
	if !ok || orgIDStr == "" {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	orgID, err := uuid.FromString(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid organization ID in token",
		})
		return uuid.Nil, false
	}

	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	userIDStr, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	userID := uuid.FromStringOrNil(userIDStr)
	user, errResp := h.authService.GetUserByID(userID, orgID)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return uuid.Nil, false
	}

	if user.Role.Name != "org_admin" {
		c.JSON(http.StatusForbidden, response.Error{
			Code:       response.ErrForbidden,
			StatusCode: http.StatusForbidden,
			Message:    "Only Organization Admins can manage roles",
		})
		return uuid.Nil, false
	}

	return orgID, true
}

func (h *RoleHandler) getOrgIDAndVerifyMember(c *gin.Context) (uuid.UUID, bool) {
	orgIDVal, exists := c.Get("organization_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	orgIDStr, ok := orgIDVal.(string)
	if !ok || orgIDStr == "" {
		c.JSON(http.StatusUnauthorized, response.Error{
			Code:       response.ErrUnauthorized,
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
		return uuid.Nil, false
	}

	orgID, err := uuid.FromString(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid organization ID in token",
		})
		return uuid.Nil, false
	}

	return orgID, true
}

// CreateRole godoc
//
//	@Summary      Create Custom Role
//	@Description  Creates a new custom role with custom permissions for the organization.
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        request body requestdto.CreateRoleRequest true "Create Role Request"
//	@Success      201 {object} response.SuccessResponse{data=responsedto.RoleResponse} "Role created successfully"
//	@Failure      400 {object} response.ErrorResponse
//	@Failure      401 {object} response.ErrorResponse
//	@Failure      403 {object} response.ErrorResponse
//	@Failure      500 {object} response.ErrorResponse
//	@Router       /organization/roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	orgID, authorized := h.getOrgIDAndVerifyAdmin(c)
	if !authorized {
		return
	}

	var req requestdto.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body: " + err.Error(),
		})
		return
	}

	resp, errResp := h.roleService.CreateRole(orgID, req)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusCreated,
		Message:    "Role created successfully",
		Data:       resp,
	})
}

// GetRoles godoc
//
//	@Summary      Get Organization Roles
//	@Description  Lists all custom and system roles available in the organization.
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {object} response.SuccessResponse{data=[]responsedto.RoleResponse} "Roles fetched successfully"
//	@Failure      401 {object} response.ErrorResponse
//	@Failure      500 {object} response.ErrorResponse
//	@Router       /organization/roles [get]
func (h *RoleHandler) GetRoles(c *gin.Context) {
	orgID, authorized := h.getOrgIDAndVerifyMember(c)
	if !authorized {
		return
	}

	resp, errResp := h.roleService.GetRolesByOrganizationID(orgID)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Roles fetched successfully",
		Data:       resp,
	})
}

// GetRole godoc
//
//	@Summary      Get Role Details
//	@Description  Retrieves detail specifications and permission toggles for a given role.
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        role_id path string true "Role ID" Format(uuid)
//	@Success      200 {object} response.SuccessResponse{data=responsedto.RoleResponse} "Role fetched successfully"
//	@Failure      400 {object} response.ErrorResponse
//	@Failure      401 {object} response.ErrorResponse
//	@Failure      404 {object} response.ErrorResponse
//	@Failure      500 {object} response.ErrorResponse
//	@Router       /organization/roles/{role_id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	orgID, authorized := h.getOrgIDAndVerifyMember(c)
	if !authorized {
		return
	}

	roleIDStr := c.Param("role_id")
	roleID, err := uuid.FromString(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid role ID",
		})
		return
	}

	resp, errResp := h.roleService.GetRoleByID(orgID, roleID)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Role fetched successfully",
		Data:       resp,
	})
}

// UpdateRole godoc
//
//	@Summary      Update Custom Role
//	@Description  Updates a custom role's name, description, and permissions.
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        role_id path string true "Role ID" Format(uuid)
//	@Param        request body requestdto.UpdateRoleRequest true "Update Role Request"
//	@Success      200 {object} response.SuccessResponse{data=responsedto.RoleResponse} "Role updated successfully"
//	@Failure      400 {object} response.ErrorResponse
//	@Failure      401 {object} response.ErrorResponse
//	@Failure      403 {object} response.ErrorResponse
//	@Failure      404 {object} response.ErrorResponse
//	@Failure      500 {object} response.ErrorResponse
//	@Router       /organization/roles/{role_id} [patch]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	orgID, authorized := h.getOrgIDAndVerifyAdmin(c)
	if !authorized {
		return
	}

	roleIDStr := c.Param("role_id")
	roleID, err := uuid.FromString(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid role ID",
		})
		return
	}

	var req requestdto.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request body: " + err.Error(),
		})
		return
	}

	resp, errResp := h.roleService.UpdateRole(orgID, roleID, req)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Role updated successfully",
		Data:       resp,
	})
}

// DeleteRole godoc
//
//	@Summary      Delete Custom Role
//	@Description  Deletes a custom role from the organization.
//	@Tags         Roles
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        role_id path string true "Role ID" Format(uuid)
//	@Success      200 {object} response.SuccessResponse "Role deleted successfully"
//	@Failure      400 {object} response.ErrorResponse
//	@Failure      401 {object} response.ErrorResponse
//	@Failure      403 {object} response.ErrorResponse
//	@Failure      404 {object} response.ErrorResponse
//	@Failure      500 {object} response.ErrorResponse
//	@Router       /organization/roles/{role_id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	orgID, authorized := h.getOrgIDAndVerifyAdmin(c)
	if !authorized {
		return
	}

	roleIDStr := c.Param("role_id")
	roleID, err := uuid.FromString(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid role ID",
		})
		return
	}

	errResp := h.roleService.DeleteRole(orgID, roleID)
	if errResp != nil {
		c.JSON(errResp.StatusCode, errResp)
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Role deleted successfully",
		Data:       nil,
	})
}

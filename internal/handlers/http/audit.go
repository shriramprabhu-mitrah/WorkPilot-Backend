package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitAuditHandler(service services.AuditService, logger *zap.Logger) *auditHandler {
	return &auditHandler{
		auditService: service,
		logger:       logger,
	}
}

type auditHandler struct {
	auditService services.AuditService
	logger       *zap.Logger
}

// GetAuditLogs godoc
// @Summary Get audit logs
// @Description Get paginated audit/activity logs for the authenticated user's organization.
// @Tags Audit
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Number of records per page" default(10)
// @Param project_id query string false "Filter audit logs by project ID"
// @Param user_id query string false "Filter audit logs by user ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /audit [get]
func (h *auditHandler) GetAuditLogs(g *gin.Context) {
	var req requestdto.GetAudit

	if err := g.ShouldBindQuery(&req.PaginationQuery); err != nil {
		message := utils.ValidationErrorMessage(err, req)

		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			},
		}

		g.JSON(http.StatusBadRequest, errorResponse)
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

	req.UserID = &userUUID
	req.OrganizationID = &organizationUUID
	audits, pagination, serviceErr := h.auditService.GetAuditLogs(req)
	if serviceErr != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *serviceErr,
		}

		g.JSON(serviceErr.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Activity received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       audits,
		Meta:       &pagination,
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

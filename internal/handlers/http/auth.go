package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/internal/handlers/dto"
	cookies "github.com/ms-kanban-server/internal/pkg/cookie"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitAuthHandler(service services.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logger,
	}
}

type AuthHandler struct {
	service services.AuthService
	logger  *zap.Logger
}

// SignUp godoc
//
// @Summary      Register a new user
// @Description  Creates a new user account.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.SignUpRequest true "Sign Up Request"
// @Success      201 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/signup [post]
func (h *AuthHandler) SignUp(g *gin.Context) {

	var payload dto.SignUpRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request payload",
				Details: []response.Details{
					{
						Field:   "body",
						Message: err.Error()},
				},
			},
		}

		h.logger.Error("Invalid request payload",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	validate := validator.New()
	if err := validate.Struct(payload); err != nil {
		var details []response.Details
		for _, fieldErr := range err.(validator.ValidationErrors) {
			details = append(details, response.Details{
				Field:   fieldErr.Field(),
				Message: fmt.Sprintf("Validation Failed on '%s'", fieldErr.Tag()),
			})
		}

		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Validation failed",
				Details:    details,
			},
		}

		h.logger.Error("Validation failed",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	err := h.service.SignUp(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Created",
		StatusCode: http.StatusCreated,
		Success:    true,
	}

	g.JSON(successResponse.StatusCode, successResponse)

}

// SignIn godoc
//
// @Summary      Sign in user
// @Description  Authenticates a user and returns access and refresh tokens.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.SignInRequest true "Sign In Request"
// @Success      200 {object} response.SuccessResponse{data=dto.AuthTokensResponse}
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/signin [post]
func (h *AuthHandler) SignIn(g *gin.Context) {

	var loginCredentials dto.SignInRequest

	if err := g.ShouldBindJSON(&loginCredentials); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Failed converting json to struct",
				Details: []response.Details{
					{
						Field:   "body",
						Message: err.Error()},
				},
			},
		}

		h.logger.Error("Invalid request payload",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	validate := validator.New()
	if err := validate.Struct(loginCredentials); err != nil {
		var details []response.Details
		for _, fieldErr := range err.(validator.ValidationErrors) {
			details = append(details, response.Details{
				Field:   fieldErr.Field(),
				Message: fmt.Sprintf("Validation Failed on '%s'", fieldErr.Tag()),
			})
		}

		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Validation failed",
				Details:    details,
			},
		}

		h.logger.Error("Validation failed",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	tokens, err := h.service.SignIn(loginCredentials)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	secure, err := utils.StringToBool(config.GetEnv("COOKIE_SECURE", ""))
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	cookies.SetAccessToken(
		g,
		tokens.AccessToken,
		tokens.ExpiresIn,
		secure,
	)

	cookies.SetRefreshToken(
		g,
		tokens.RefreshToken,
		tokens.RefreshExpiresIn,
		secure,
	)

	successResponse := &response.SuccessResponse{
		Message:    "Successfully Logged in",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       tokens,
	}

	g.JSON(successResponse.StatusCode, successResponse)
}

// RequestPasswordReset godoc
//
// @Summary      Request password reset
// @Description  Sends a password reset OTP to the provided email address.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.PasswordResetRequest true "Password reset request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/password-reset/request [post]
func (h *AuthHandler) RequestPasswordReset(g *gin.Context) {

	var payload dto.PasswordResetRequest
	if err := g.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("Invalid request payload",
			zap.Error(err))
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request payload",
				Details: []response.Details{
					{
						Field:   "body",
						Message: err.Error(),
					},
				},
			},
		})
		return
	}

	if err := validator.New().Struct(payload); err != nil {
		h.logger.Error("Validation failed",
			zap.Error(err))
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Validation failed",
				Details: []response.Details{
					{
						Field:   "email",
						Message: "must be a valid email",
					},
				},
			},
		})
		return
	}

	if err := h.service.RequestPasswordReset(payload.Email); err != nil {
		g.JSON(err.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	g.JSON(http.StatusOK, &response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "A password reset OTP has been sent to your email address",
	})
}

// ResetPassword godoc
//
// @Summary      Confirm password reset
// @Description  Validates the reset OTP and updates the user's password.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.ResetPasswordRequest true "Password reset confirmation"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/password-reset/confirm [post]
func (h *AuthHandler) ResetPassword(g *gin.Context) {

	var payload dto.ResetPasswordRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("Invalid request payload",
			zap.Error(err))
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request payload",
				Details: []response.Details{
					{
						Field:   "body",
						Message: err.Error(),
					},
				},
			},
		})
		return
	}

	if err := validator.New().Struct(payload); err != nil {
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Validation failed",
				Details: []response.Details{
					{
						Field:   "otp",
						Message: "OTP is required",
					},
				},
			},
		})
		return
	}

	if err := h.service.ResetPassword(payload); err != nil {
		g.JSON(err.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	g.JSON(http.StatusOK, &response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Password reset successfully",
	})
}

// RefreshToken godoc
//
// @Summary      Refresh access token
// @Description  Generates a new access token using the refresh token.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=dto.AuthTokensResponse}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(g *gin.Context) {

	var payload dto.RefreshTokenRequest

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid User ID",
				Details: []response.Details{
					{
						Field:   "User ID",
						Message: "User Id Invalid/Missing",
					},
				},
			},
		}

		h.logger.Error("User Id Invalid/Missing ")

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	payload.UserID = userID.(string)

	token, cookieError := g.Cookie("refresh_token")
	if cookieError != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Bad Request",
				Details: []response.Details{
					{
						Field:   "refresh_token",
						Message: "Refresh Token Missig",
					},
				},
			},
		}
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	payload.RefreshToken = token

	tokens, err := h.service.RefreshToken(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Token refreshed successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       tokens,
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

// Logout godoc
//
// @Summary      Logout user
// @Description  Revokes the user's refresh token and let user logout.
// @Tags         Authentication
// @Produce      json
// @Success      200 {object} response.SuccessResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(g *gin.Context) {

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid User ID",
				Details: []response.Details{
					{
						Field:   "User ID",
						Message: "User Id Invalid/Missing",
					},
				},
			},
		}

		h.logger.Error("User Id Invalid/Missing ")

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	id := userID.(string)

	err := h.service.Logout(id)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	secure, err := utils.StringToBool(config.GetEnv("COOKIE_SECURE", ""))
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	cookies.Clear(g, secure)

	successResponse := &response.SuccessResponse{
		Message:    "Logedout successfully",
		StatusCode: http.StatusOK,
		Success:    true,
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

// ChangePassword godoc
//
// @Summary      Change password
// @Description  Changes the password of the authenticated user.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.ChangePasswordRequest true "Change Password Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/change-password [post]
func (h *AuthHandler) ChangePassword(g *gin.Context) {

	var payload dto.ChangePasswordRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request payload",
				Details: []response.Details{{
					Field:   "body",
					Message: err.Error(),
				}},
			},
		}

		h.logger.Error("Invalid request payload",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid User ID",
				Details: []response.Details{
					{
						Field:   "User ID",
						Message: "User Id Invalid/Missing",
					},
				},
			},
		}

		h.logger.Error("User Id Invalid/Missing")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	userIDStr := userID.(string)

	id, errorResponse := utils.StringToUUID(userIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		return
	}
	payload.UserID = id

	err := h.service.ChangePassword(payload)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Password Changed successfully",
		StatusCode: http.StatusOK,
		Success:    true,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// UpdateUser godoc
//
// @Summary      Update user
// @Description  Updates user profile.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        request body dto.UpdateUserRequest true "Update User Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/update [patch]
func (h *AuthHandler) UpdateUser(g *gin.Context) {

	var payload dto.UpdateUserRequest

	if err := g.ShouldBindJSON(&payload); err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrBadRequest,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid request payload",
				Details: []response.Details{{
					Field:   "body",
					Message: err.Error(),
				}},
			},
		}

		h.logger.Error("Invalid request payload",
			zap.Error(err))

		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid User ID",
				Details: []response.Details{
					{
						Field:   "User ID",
						Message: "User Id Invalid/Missing",
					},
				},
			},
		}

		h.logger.Error("User Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	userIDStr := userID.(string)

	id, errorResponse := utils.StringToUUID(userIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	err := h.service.UpdateUser(payload, id)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "Updated profile successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"userID": id},
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

// GetUser godoc
//
// @Summary      Get current user
// @Description  Returns the profile of the authenticated user.
// @Tags         Users
// @Produce      json
// @Success      200 {object} response.SuccessResponse{data=models.User}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/me [get]
func (h *AuthHandler) GetUser(g *gin.Context) {

	userID, exist := g.Get("user_id")
	if !exist {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid User ID",
				Details: []response.Details{
					{
						Field:   "User ID",
						Message: "User Id Invalid/Missing",
					},
				},
			},
		}

		h.logger.Error("User Id Invalid/Missing ")
		g.JSON(errorResponse.Error.StatusCode, errorResponse)
		return
	}
	userIDStr := userID.(string)

	id, errorResponse := utils.StringToUUID(userIDStr)
	if errorResponse != nil {
		h.logger.Error("Failed to convert the string into UUID")
		g.JSON(errorResponse.StatusCode, errorResponse)
		return
	}

	result, err := h.service.GetUser(id)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	successResponse := &response.SuccessResponse{
		Message:    "User detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       result,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

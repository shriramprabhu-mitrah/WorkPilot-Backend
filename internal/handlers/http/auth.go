package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
		message := utils.ValidationErrorMessage(err, payload)
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			}}
		h.logger.Error("Invalid request payload", zap.Error(err))
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
		Message:    "Successfully Created. Please verify your email with the OTP sent to your inbox.",
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
		message := utils.ValidationErrorMessage(err, loginCredentials)
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
		h.logger.Error("Invalid request payload", zap.Error(err))
		message := utils.ValidationErrorMessage(err, payload)
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
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
		h.logger.Error("Invalid request payload", zap.Error(err))
		message := utils.ValidationErrorMessage(err, payload)
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
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

// VerifyEmail godoc
//
// @Summary      Verify email address
// @Description  Validates the verification OTP and marks the email as verified.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.VerifyEmailRequest true "Email verification request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Router       /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(g *gin.Context) {
	var payload dto.VerifyEmailRequest
	if err := g.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("Invalid request payload", zap.Error(err))
		message := utils.ValidationErrorMessage(err, payload)
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			}})
		return
	}

	tokens, err := h.service.VerifyEmail(payload)
	if err != nil {
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

	g.JSON(http.StatusOK, &response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Email verified successfully"})
}

// ResendVerificationOTP godoc
//
// @Summary      Resend verification OTP
// @Description  Sends a new email verification OTP to an unverified user.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request body dto.ResendVerificationOTPRequest true "Resend verification OTP request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      429 {object} response.ErrorResponse
// @Router       /auth/resend-verification-otp [post]
func (h *AuthHandler) ResendVerificationOTP(g *gin.Context) {
	var payload dto.ResendVerificationOTPRequest
	if err := g.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("Invalid request payload", zap.Error(err))
		message := utils.ValidationErrorMessage(err, payload)
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    message,
			}})
		return
	}

	if err := h.service.ResendVerificationOTP(payload.Email); err != nil {
		g.JSON(err.StatusCode, &response.ErrorResponse{Success: false, Error: *err})
		return
	}

	g.JSON(http.StatusOK, &response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "A new verification OTP has been sent to your email address"})
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
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
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
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
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
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
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
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal server error: missing user context",
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

// Validate godoc
//
// @Summary      Check whether an email or username is available
// @Description  Returns whether the provided email or username is not already registered.
// @Tags         Authentication
// @Produce      json
// @Param        type   query    string  true  "Validation type: email or username"
// @Param        value  query    string  true  "Value to validate"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/validate [get]
func (h *AuthHandler) Validate(g *gin.Context) {
	validationType := strings.ToLower(g.Query("type"))
	value := g.Query("value")

	if validationType == "" {
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Type is required.",
			},
		})
		return
	}

	if value == "" {
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Value is required.",
			},
		})
		return
	}

	var available bool
	var err *response.Error

	switch validationType {
	case "email":
		available, err = h.service.IsEmailAvailable(value)
	case "username":
		available, err = h.service.IsUsernameAvailable(value)
	default:
		g.JSON(http.StatusBadRequest, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrValidation,
				StatusCode: http.StatusBadRequest,
				Message:    "Type must be 'email' or 'username'.",
			},
		})
		return
	}

	if err != nil {
		g.JSON(err.StatusCode, &response.ErrorResponse{
			Success: false,
			Error:   *err,
		})
		return
	}

	message := fmt.Sprintf("%s is available.", validationType)
	if !available {
		message = fmt.Sprintf("%s is already taken.", validationType)
	}
	successResponse := &response.SuccessResponse{
		Message:    message,
		StatusCode: http.StatusOK,
		Success:    true,
		Data: map[string]any{
			"type":      validationType,
			"value":     value,
			"available": available,
		},
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

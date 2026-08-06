package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/config"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	responsedto "github.com/ms-kanban-server/internal/handlers/dto/response"
	cookies "github.com/ms-kanban-server/internal/pkg/cookie"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/storage"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitAuthHandler(service services.AuthService, storage storage.StorageClient, logger *zap.Logger) *authHandler {
	return &authHandler{
		service: service,
		storage: storage,
		logger:  logger,
	}
}

type authHandler struct {
	service services.AuthService
	storage storage.StorageClient
	logger  *zap.Logger
}

// SignUp godoc
//
// @Summary      Register a new user
// @Description  Creates a new user account. Send as multipart/form-data. Optionally include an 'avatar' file to set the user avatar.
// @Tags         Authentication
// @Accept       multipart/form-data
// @Produce      json
// @Param        email     formData string true  "Email address"
// @Param        password  formData string true  "Password"
// @Param        full_name formData string true  "Full name"
// @Param        username  formData string true  "Username"
// @Param        timezone  formData string false "Timezone"
// @Param        avatar    formData file   false "User avatar image (PNG, JPG/JPEG, WEBP — max configurable MB)"
// @Success      201 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/signup [post]
func (h *authHandler) SignUp(g *gin.Context) {

	var payload requestdto.SignUpRequest

	if err := g.ShouldBind(&payload); err != nil {
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

	// Handle optional avatar upload.
	var avatarURL string
	var uploadedKey string
	avatarFile, avatarHeader, fileErr := g.Request.FormFile("avatar")
	if fileErr == nil {
		defer avatarFile.Close()
		var uploadErr *response.Error
		avatarURL, uploadedKey, uploadErr = h.storage.UploadAvatar(avatarFile, avatarHeader)
		if uploadErr != nil {
			h.logger.Error("Avatar upload failed during sign up", zap.String("error", uploadErr.Message))
			g.JSON(uploadErr.StatusCode, &response.ErrorResponse{Success: false, Error: *uploadErr})
			return
		}
	}
	payload.AvatarURL = avatarURL

	err := h.service.SignUp(payload)
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteObject(context.Background(), uploadedKey)
		}
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
// @Param        request body requestdto.SignInRequest true "Sign In Request"
// @Success      200 {object} response.SuccessResponse{data=requestdto.AuthTokensResponse}
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/signin [post]
func (h *authHandler) SignIn(g *gin.Context) {

	var loginCredentials requestdto.SignInRequest

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
// @Param        request body requestdto.PasswordResetRequest true "Password reset request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/password-reset/request [post]
func (h *authHandler) RequestPasswordReset(g *gin.Context) {

	var payload requestdto.PasswordResetRequest
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
// @Param        request body requestdto.ResetPasswordRequest true "Password reset confirmation"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/password-reset/confirm [post]
func (h *authHandler) ResetPassword(g *gin.Context) {

	var payload requestdto.ResetPasswordRequest

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
// @Param        request body requestdto.VerifyEmailRequest true "Email verification request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Router       /auth/verify-email [post]
func (h *authHandler) VerifyEmail(g *gin.Context) {
	var payload requestdto.VerifyEmailRequest
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
// @Param        request body requestdto.ResendVerificationOTPRequest true "Resend verification OTP request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      429 {object} response.ErrorResponse
// @Router       /auth/resend-verification-otp [post]
func (h *authHandler) ResendVerificationOTP(g *gin.Context) {
	var payload requestdto.ResendVerificationOTPRequest
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
// @Success      200 {object} response.SuccessResponse{data=requestdto.AuthTokensResponse}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/refresh [post]
func (h *authHandler) RefreshToken(g *gin.Context) {

	var payload requestdto.RefreshTokenRequest
	// allow client to send refresh token in body or cookie
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

	if payload.RefreshToken == "" {
		g.JSON(http.StatusUnauthorized, &response.ErrorResponse{
			Success: false,
			Error: response.Error{
				Code:       response.ErrUnauthorized,
				StatusCode: http.StatusUnauthorized,
				Message:    "Authentication required",
			},
		})
		return
	}

	tokens, err := h.service.RefreshToken(payload)
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
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *secureErr,
		}
		g.JSON(secureErr.StatusCode, errorResponse)
		return
	}

	cookies.SetAccessToken(g, tokens.AccessToken, tokens.ExpiresIn, secure)
	cookies.SetRefreshToken(g, tokens.RefreshToken, tokens.RefreshExpiresIn, secure)

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
func (h *authHandler) Logout(g *gin.Context) {

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
		Message:    "Logged out successfully",
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
// @Param        request body requestdto.ChangePasswordRequest true "Change Password Request"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/change-password [post]
func (h *authHandler) ChangePassword(g *gin.Context) {

	var payload requestdto.ChangePasswordRequest

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
// @Description  Updates user profile. Send as multipart/form-data. Include an 'avatar' file field to replace the user avatar.
// @Tags         Users
// @Accept       multipart/form-data
// @Produce      json
// @Param        full_name formData string false "Full name"
// @Param        username  formData string false "Username"
// @Param        timezone  formData string false "Timezone"
// @Param        avatar    formData file   false "User avatar image (PNG, JPG/JPEG, WEBP — max configurable MB)"
// @Success      200 {object} response.SuccessResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/update [patch]
func (h *authHandler) UpdateUser(g *gin.Context) {

	var payload requestdto.UpdateUserRequest

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

	// Handle optional avatar upload.
	var avatarURL string
	var uploadedKey string
	avatarFile, avatarHeader, fileErr := g.Request.FormFile("avatar")
	if fileErr == nil {
		defer avatarFile.Close()
		var uploadErr *response.Error
		avatarURL, uploadedKey, uploadErr = h.storage.UploadAvatar(avatarFile, avatarHeader)
		if uploadErr != nil {
			h.logger.Error("Avatar upload failed during user update", zap.String("error", uploadErr.Message))
			g.JSON(uploadErr.StatusCode, &response.ErrorResponse{Success: false, Error: *uploadErr})
			return
		}
	}
	payload.AvatarURL = avatarURL

	err := h.service.UpdateUser(payload, id)
	if err != nil {
		if uploadedKey != "" {
			_ = h.storage.DeleteObject(context.Background(), uploadedKey)
		}
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
// @Success      200 {object} response.SuccessResponse{data=responsedto.UserProfile}
// @Failure      401 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/me [get]
func (h *authHandler) GetUser(g *gin.Context) {

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

	userResponse := responsedto.UserProfileFromModel(result)

	successResponse := &response.SuccessResponse{
		Message:    "User detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       userResponse,
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
// @Failure      409 {object} response.SuccessResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /auth/validate [get]
func (h *authHandler) Validate(g *gin.Context) {
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

	statusCode := http.StatusOK
	success := true
	message := fmt.Sprintf("%s is available.", validationType)
	if !available {
		statusCode = http.StatusConflict
		success = false
		message = fmt.Sprintf("%s is already taken.", validationType)
	}

	successResponse := &response.SuccessResponse{
		Message:    message,
		StatusCode: statusCode,
		Success:    success,
		Data: map[string]any{
			"type":      validationType,
			"value":     value,
			"available": available,
		},
	}
	g.JSON(successResponse.StatusCode, successResponse)
}

// GetUserByID godoc
//
//	@Summary		Get User by ID
//	@Description	Get user details by user ID within the authenticated user's organization
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			user_id	path		string	true	"User ID (UUID)"
//	@Success		200		{object}	response.SuccessResponse{data=responsedto.UserProfile}	"User retrieved successfully"
//	@Failure		400		{object}	response.ErrorResponse	"Invalid user ID"
//	@Failure		401		{object}	response.ErrorResponse	"Unauthorized"
//	@Failure		403		{object}	response.ErrorResponse	"Forbidden"
//	@Failure		404		{object}	response.ErrorResponse	"User not found"
//	@Failure		500		{object}	response.ErrorResponse	"Internal server error"
//	@Router			/auth/{user_id} [get]
func (h *authHandler) GetUserByID(g *gin.Context) {

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

	result, err := h.service.GetUserByID(userUUID, organizationUUID)
	if err != nil {
		errorResponse := &response.ErrorResponse{
			Success: false,
			Error:   *err,
		}
		g.JSON(err.StatusCode, errorResponse)
		return
	}

	userResponse := responsedto.UserProfileFromModel(*result)

	successResponse := &response.SuccessResponse{
		Message:    "User detail received successfully",
		StatusCode: http.StatusOK,
		Success:    true,
		Data:       userResponse,
	}
	g.JSON(successResponse.StatusCode, successResponse)

}

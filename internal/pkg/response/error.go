package response

type ErrorCode string

const (
	ErrBadRequest          ErrorCode = "BAD_REQUEST"
	ErrMethodNotAllowed    ErrorCode = "METHOD_NOT_ALLOWED"
	ErrValidation          ErrorCode = "VALIDATION_ERROR"
	ErrUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrForbidden           ErrorCode = "FORBIDDEN"
	ErrNotFound            ErrorCode = "RESOURCE_NOT_FOUND"
	ErrConflict            ErrorCode = "CONFLICT"
	ErrGone                ErrorCode = "GONE"
	ErrBusinessRule        ErrorCode = "BUSINESS_RULE_VIOLATION"
	ErrRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"
	ErrGatewayTimeout      ErrorCode = "GATEWAY_TIMEOUT"
)

type Error struct {
	Code       ErrorCode `json:"code"`
	StatusCode int       `json:"status_code"`
	Message    string    `json:"message"`
}

type ErrorResponse struct {
	Success bool  `json:"success"`
	Error   Error `json:"error"`
}

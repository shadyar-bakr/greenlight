package main

// Error codes for different types of errors
const (
	ERRCODE_INTERNAL_SERVER    = "INTERNAL_SERVER"
	ERRCODE_NOT_FOUND          = "NOT_FOUND"
	ERRCODE_METHOD_NOT_ALLOWED = "METHOD_NOT_ALLOWED"
	ERRCODE_BAD_REQUEST        = "BAD_REQUEST"
	ERRCODE_VALIDATION         = "VALIDATION_ERROR"
	ERRCODE_EDIT_CONFLICT      = "EDIT_CONFLICT"
	ERRCODE_RATE_LIMIT         = "RATE_LIMIT_EXCEEDED"
	ERRCODE_INVALID_CREDS      = "INVALID_CREDENTIALS"
	ERRCODE_INVALID_TOKEN      = "INVALID_TOKEN"
	ERRCODE_AUTH_REQUIRED      = "AUTH_REQUIRED"
	ERRCODE_INACTIVE_ACCOUNT   = "INACTIVE_ACCOUNT"
	ERRCODE_NOT_PERMITTED      = "NOT_PERMITTED"
	ERRCODE_TOKEN_EXPIRED      = "TOKEN_EXPIRED"
	ERRCODE_REQUEST_TOO_LARGE  = "REQUEST_TOO_LARGE"
	ERRCODE_UNSUPPORTED_MEDIA  = "UNSUPPORTED_MEDIA_TYPE"
	ERRCODE_NOT_ACCEPTABLE     = "NOT_ACCEPTABLE"
)

// ErrorResponse represents the structure of our error responses
type ErrorResponse struct {
	Code    string      `json:"code"`              // Machine-readable error code
	Message any         `json:"message"`           // Human-readable error message
	Details interface{} `json:"details,omitempty"` // Optional additional error details
}

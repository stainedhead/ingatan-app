// Package domain contains the core business entities and error types for ingatan.
package domain

import "fmt"

// Error code constants for all application errors.
const (
	ErrCodeUnauthorized         = "UNAUTHORIZED"
	ErrCodeForbidden            = "FORBIDDEN"
	ErrCodeNotFound             = "NOT_FOUND"
	ErrCodeConflict             = "CONFLICT"
	ErrCodeContentTooLarge      = "CONTENT_TOO_LARGE"
	ErrCodeInvalidRequest       = "INVALID_REQUEST"
	ErrCodePathNotAllowed       = "PATH_NOT_ALLOWED"
	ErrCodePDFExtractionError   = "PDF_EXTRACTION_ERROR"
	ErrCodeStoreDeleteForbidden = "STORE_DELETE_FORBIDDEN"
	ErrCodeEmbeddingError       = "EMBEDDING_ERROR"
	ErrCodeLLMError             = "LLM_ERROR"
	ErrCodeInternalError        = "INTERNAL_ERROR"
)

// AppError is a structured application error with a code, message, and optional details.
type AppError struct {
	Code    string
	Message string
	Details any
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAppError creates a new AppError with the given code and message.
func NewAppError(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// NewAppErrorWithDetails creates a new AppError with code, message, and details.
func NewAppErrorWithDetails(code, message string, details any) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

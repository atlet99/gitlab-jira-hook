// Package errors provides structured error types and handling utilities
// for the GitLab ↔ Jira Hook service.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents a specific error condition
type ErrorCode string

// Error codes for different types of failures
const (
	// Client errors (4xx)
	ErrCodeInvalidRequest   ErrorCode = "INVALID_REQUEST"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeRateLimited      ErrorCode = "RATE_LIMITED"
	ErrCodeInvalidSignature ErrorCode = "INVALID_SIGNATURE"
	ErrCodeInvalidWebhook   ErrorCode = "INVALID_WEBHOOK"
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// Server errors (5xx)
	ErrCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout            ErrorCode = "TIMEOUT"
	ErrCodeDatabaseError      ErrorCode = "DATABASE_ERROR"
	ErrCodeExternalAPIError   ErrorCode = "EXTERNAL_API_ERROR"

	// GitLab API specific errors
	ErrCodeGitLabAPIError     ErrorCode = "GITLAB_API_ERROR"
	ErrCodeGitLabTimeout      ErrorCode = "GITLAB_TIMEOUT"
	ErrCodeGitLabRateLimit    ErrorCode = "GITLAB_RATE_LIMIT"
	ErrCodeGitLabUnauthorized ErrorCode = "GITLAB_UNAUTHORIZED"
	ErrCodeGitLabNotFound     ErrorCode = "GITLAB_NOT_FOUND"

	// Jira API specific errors
	ErrCodeJiraAPIError     ErrorCode = "JIRA_API_ERROR"
	ErrCodeJiraTimeout      ErrorCode = "JIRA_TIMEOUT"
	ErrCodeJiraRateLimit    ErrorCode = "JIRA_RATE_LIMIT"
	ErrCodeJiraUnauthorized ErrorCode = "JIRA_UNAUTHORIZED"
	ErrCodeJiraNotFound     ErrorCode = "JIRA_NOT_FOUND"

	// Processing errors
	ErrCodeProcessingFailed   ErrorCode = "PROCESSING_FAILED"
	ErrCodeQueueFull          ErrorCode = "QUEUE_FULL"
	ErrCodeWorkerUnavailable  ErrorCode = "WORKER_UNAVAILABLE"
	ErrCodeCircuitBreakerOpen ErrorCode = "CIRCUIT_BREAKER_OPEN"
)

// ErrorCategory represents the type of error for handling strategy
type ErrorCategory string

const (
	// CategoryClientError represents user/client mistakes (4xx HTTP errors)
	CategoryClientError ErrorCategory = "CLIENT_ERROR" // User/client mistake (4xx)
	// CategoryServerError represents our system errors (5xx HTTP errors)
	CategoryServerError ErrorCategory = "SERVER_ERROR" // Our system error (5xx)
	// CategoryExternalError represents external API errors
	CategoryExternalError ErrorCategory = "EXTERNAL_ERROR" // External API error
	// CategoryRetryableError represents errors that can be retried
	CategoryRetryableError ErrorCategory = "RETRYABLE_ERROR" // Can be retried
	// CategoryPermanentError represents errors that should not be retried
	CategoryPermanentError ErrorCategory = "PERMANENT_ERROR" // Should not be retried
	// CategoryRateLimitError represents rate limiting errors
	CategoryRateLimitError ErrorCategory = "RATE_LIMIT_ERROR" // Rate limiting
	// CategoryTimeoutError represents timeout related errors
	CategoryTimeoutError ErrorCategory = "TIMEOUT_ERROR" // Timeout related
)

// Severity levels for error classification
type Severity string

const (
	// SeverityLow represents minor issues with degraded functionality
	SeverityLow Severity = "LOW" // Minor issues, degraded functionality
	// SeverityMedium represents significant issues with some functionality lost
	SeverityMedium Severity = "MEDIUM" // Significant issues, some functionality lost
	// SeverityHigh represents major issues with primary functionality affected
	SeverityHigh Severity = "HIGH" // Major issues, primary functionality affected
	// SeverityCritical represents system-wide issues with service unavailable
	SeverityCritical Severity = "CRITICAL" // System-wide issues, service unavailable
)

// ServiceError represents a structured error with context
type ServiceError struct {
	Code        ErrorCode              `json:"code"`
	Category    ErrorCategory          `json:"category"`
	Severity    Severity               `json:"severity"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Cause       error                  `json:"-"` // Original error, not serialized
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"` // User-friendly message
	RetryAfter  *time.Duration         `json:"retry_after,omitempty"`  // For rate limiting
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap allows errors.Is and errors.As to work with wrapped errors
func (e *ServiceError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error can be retried
func (e *ServiceError) IsRetryable() bool {
	return e.Category == CategoryRetryableError ||
		e.Category == CategoryTimeoutError ||
		e.Category == CategoryRateLimitError ||
		(e.Category == CategoryExternalError && e.Severity != SeverityCritical)
}

// IsClientError returns true if the error is caused by client
func (e *ServiceError) IsClientError() bool {
	return e.Category == CategoryClientError
}

// IsServerError returns true if the error is a server-side error
func (e *ServiceError) IsServerError() bool {
	return e.Category == CategoryServerError
}

// HTTPStatusCode returns the appropriate HTTP status code for the error
func (e *ServiceError) HTTPStatusCode() int {
	switch e.Code {
	case ErrCodeInvalidRequest, ErrCodeInvalidWebhook, ErrCodeValidationFailed:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeGitLabUnauthorized, ErrCodeJiraUnauthorized, ErrCodeInvalidSignature:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeGitLabNotFound, ErrCodeJiraNotFound:
		return http.StatusNotFound
	case ErrCodeRateLimited, ErrCodeGitLabRateLimit, ErrCodeJiraRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeTimeout, ErrCodeGitLabTimeout, ErrCodeJiraTimeout:
		return http.StatusRequestTimeout
	case ErrCodeServiceUnavailable, ErrCodeQueueFull, ErrCodeWorkerUnavailable, ErrCodeCircuitBreakerOpen:
		return http.StatusServiceUnavailable
	case ErrCodeInternalError, ErrCodeDatabaseError, ErrCodeProcessingFailed:
		return http.StatusInternalServerError
	case ErrCodeExternalAPIError, ErrCodeGitLabAPIError, ErrCodeJiraAPIError:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// ErrorBuilder helps construct ServiceError instances
type ErrorBuilder struct {
	error *ServiceError
}

// NewError creates a new ErrorBuilder
func NewError(code ErrorCode) *ErrorBuilder {
	return &ErrorBuilder{
		error: &ServiceError{
			Code:      code,
			Timestamp: time.Now(),
			Context:   make(map[string]interface{}),
		},
	}
}

// WithCategory sets the error category
func (b *ErrorBuilder) WithCategory(category ErrorCategory) *ErrorBuilder {
	b.error.Category = category
	return b
}

// WithSeverity sets the error severity
func (b *ErrorBuilder) WithSeverity(severity Severity) *ErrorBuilder {
	b.error.Severity = severity
	return b
}

// WithMessage sets the error message
func (b *ErrorBuilder) WithMessage(message string) *ErrorBuilder {
	b.error.Message = message
	return b
}

// WithDetails sets additional error details
func (b *ErrorBuilder) WithDetails(details string) *ErrorBuilder {
	b.error.Details = details
	return b
}

// WithCause sets the underlying cause
func (b *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	b.error.Cause = cause
	return b
}

// WithContext adds context information
func (b *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	if b.error.Context == nil {
		b.error.Context = make(map[string]interface{})
	}
	b.error.Context[key] = value
	return b
}

// WithRequestID sets the request ID for tracing
func (b *ErrorBuilder) WithRequestID(requestID string) *ErrorBuilder {
	b.error.RequestID = requestID
	return b
}

// WithUserMessage sets a user-friendly message
func (b *ErrorBuilder) WithUserMessage(message string) *ErrorBuilder {
	b.error.UserMessage = message
	return b
}

// WithRetryAfter sets retry-after duration for rate limiting
func (b *ErrorBuilder) WithRetryAfter(duration time.Duration) *ErrorBuilder {
	b.error.RetryAfter = &duration
	return b
}

// Build returns the constructed ServiceError
func (b *ErrorBuilder) Build() *ServiceError {
	// Set default category based on code if not set
	if b.error.Category == "" {
		b.error.Category = getDefaultCategory(b.error.Code)
	}

	// Set default severity if not set
	if b.error.Severity == "" {
		b.error.Severity = getDefaultSeverity(b.error.Code)
	}

	return b.error
}

// ErrorResponse represents the JSON response format for API errors
type ErrorResponse struct {
	Error       ErrorCode              `json:"error"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
	RetryAfter  *int                   `json:"retry_after_seconds,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"`
}

// ToErrorResponse converts ServiceError to ErrorResponse for API responses
func (e *ServiceError) ToErrorResponse() *ErrorResponse {
	resp := &ErrorResponse{
		Error:     e.Code,
		Message:   e.Message,
		Details:   e.Details,
		Context:   e.Context,
		Timestamp: e.Timestamp,
		RequestID: e.RequestID,
	}

	if e.UserMessage != "" {
		resp.UserMessage = e.UserMessage
	} else {
		resp.UserMessage = getUserFriendlyMessage(e.Code)
	}

	if e.RetryAfter != nil {
		seconds := int(e.RetryAfter.Seconds())
		resp.RetryAfter = &seconds
	}

	return resp
}

// MarshalJSON implements json.Marshaler for structured logging
func (e *ServiceError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToErrorResponse())
}

// getDefaultCategory returns default category for error code
func getDefaultCategory(code ErrorCode) ErrorCategory {
	switch code {
	case ErrCodeInvalidRequest, ErrCodeUnauthorized, ErrCodeForbidden,
		ErrCodeNotFound, ErrCodeInvalidSignature, ErrCodeInvalidWebhook, ErrCodeValidationFailed:
		return CategoryClientError
	case ErrCodeRateLimited, ErrCodeGitLabRateLimit, ErrCodeJiraRateLimit:
		return CategoryRateLimitError
	case ErrCodeTimeout, ErrCodeGitLabTimeout, ErrCodeJiraTimeout:
		return CategoryTimeoutError
	case ErrCodeGitLabAPIError, ErrCodeJiraAPIError:
		return CategoryExternalError
	case ErrCodeServiceUnavailable, ErrCodeQueueFull, ErrCodeWorkerUnavailable:
		return CategoryRetryableError
	case ErrCodeCircuitBreakerOpen:
		return CategoryRetryableError
	default:
		return CategoryServerError
	}
}

// getDefaultSeverity returns default severity for error code
func getDefaultSeverity(code ErrorCode) Severity {
	switch code {
	case ErrCodeInvalidRequest, ErrCodeNotFound, ErrCodeInvalidWebhook:
		return SeverityLow
	case ErrCodeUnauthorized, ErrCodeForbidden, ErrCodeValidationFailed:
		return SeverityMedium
	case ErrCodeRateLimited, ErrCodeTimeout:
		return SeverityMedium
	case ErrCodeServiceUnavailable, ErrCodeCircuitBreakerOpen:
		return SeverityHigh
	case ErrCodeInternalError, ErrCodeDatabaseError:
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

// getUserFriendlyMessage returns user-friendly error messages
func getUserFriendlyMessage(code ErrorCode) string {
	switch code {
	case ErrCodeInvalidRequest:
		return "The request contains invalid data. Please check your input and try again."
	case ErrCodeUnauthorized:
		return "Authentication failed. Please check your credentials."
	case ErrCodeForbidden:
		return "You don't have permission to perform this action."
	case ErrCodeNotFound:
		return "The requested resource was not found."
	case ErrCodeRateLimited:
		return "Too many requests. Please wait and try again later."
	case ErrCodeInvalidSignature:
		return "Webhook signature validation failed. Please check your secret configuration."
	case ErrCodeServiceUnavailable:
		return "Service is temporarily unavailable. Please try again later."
	case ErrCodeTimeout:
		return "Request timed out. Please try again."
	case ErrCodeExternalAPIError:
		return "External service is experiencing issues. Please try again later."
	case ErrCodeGitLabAPIError:
		return "GitLab API is experiencing issues. Please try again later."
	case ErrCodeJiraAPIError:
		return "Jira API is experiencing issues. Please try again later."
	case ErrCodeProcessingFailed:
		return "Failed to process your request. Please try again."
	case ErrCodeQueueFull:
		return "System is busy. Your request has been queued and will be processed shortly."
	default:
		return "An unexpected error occurred. Please try again or contact support."
	}
}

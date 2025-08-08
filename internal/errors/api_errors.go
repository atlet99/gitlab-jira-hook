package errors

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIErrorFactory creates structured errors for API operations
type APIErrorFactory struct{}

// NewAPIErrorFactory creates a new API error factory
func NewAPIErrorFactory() *APIErrorFactory {
	return &APIErrorFactory{}
}

// GitLabAPIError creates a structured error for GitLab API failures
func (f *APIErrorFactory) GitLabAPIError(statusCode int, responseBody, operation string) *ServiceError {
	code, category, severity := f.classifyGitLabError(statusCode)
	const rateLimitMinutes = 5
	return f.createAPIError("GitLab", statusCode, responseBody, operation, code, category, severity, rateLimitMinutes)
}

// JiraAPIError creates a structured error for Jira API failures
func (f *APIErrorFactory) JiraAPIError(statusCode int, responseBody, operation string) *ServiceError {
	code, category, severity := f.classifyJiraError(statusCode)
	const rateLimitMinutes = 3
	return f.createAPIError("Jira", statusCode, responseBody, operation, code, category, severity, rateLimitMinutes)
}

// createAPIError creates a structured error for API failures (common implementation)
func (f *APIErrorFactory) createAPIError(apiName string, statusCode int, responseBody, operation string,
	code ErrorCode, category ErrorCategory, severity Severity, rateLimitMinutes int) *ServiceError {
	builder := NewError(code).
		WithCategory(category).
		WithSeverity(severity).
		WithMessage(fmt.Sprintf("%s API %s failed", apiName, operation)).
		WithContext("status_code", statusCode).
		WithContext("operation", operation)

	// Add response body details if available
	if responseBody != "" {
		builder.WithDetails(responseBody)
	}

	// Add retry-after for rate limiting
	if statusCode == http.StatusTooManyRequests {
		builder.WithRetryAfter(time.Duration(rateLimitMinutes) * time.Minute)
		builder.WithUserMessage(fmt.Sprintf("%s API rate limit exceeded. Please wait before retrying.", apiName))
	}

	return builder.Build()
}

// NetworkError creates a structured error for network failures
func (f *APIErrorFactory) NetworkError(operation string, cause error) *ServiceError {
	return NewError(ErrCodeTimeout).
		WithCategory(CategoryTimeoutError).
		WithSeverity(SeverityMedium).
		WithMessage(fmt.Sprintf("Network error during %s", operation)).
		WithCause(cause).
		WithContext("operation", operation).
		WithUserMessage("Network connection failed. Please check your connection and try again.").
		Build()
}

// ValidationError creates a structured error for validation failures
func (f *APIErrorFactory) ValidationError(field, message string) *ServiceError {
	return NewError(ErrCodeValidationFailed).
		WithCategory(CategoryClientError).
		WithSeverity(SeverityLow).
		WithMessage(fmt.Sprintf("Validation failed for field: %s", field)).
		WithDetails(message).
		WithContext("field", field).
		WithUserMessage(fmt.Sprintf("Invalid %s: %s", field, message)).
		Build()
}

// AuthenticationError creates a structured error for authentication failures
func (f *APIErrorFactory) AuthenticationError(service string) *ServiceError {
	return NewError(ErrCodeUnauthorized).
		WithCategory(CategoryClientError).
		WithSeverity(SeverityMedium).
		WithMessage(fmt.Sprintf("Authentication failed for %s", service)).
		WithContext("service", service).
		WithUserMessage(fmt.Sprintf("Authentication with %s failed. Please check your credentials.", service)).
		Build()
}

// WebhookError creates a structured error for webhook processing failures
func (f *APIErrorFactory) WebhookError(webhookType string, cause error) *ServiceError {
	return NewError(ErrCodeInvalidWebhook).
		WithCategory(CategoryClientError).
		WithSeverity(SeverityLow).
		WithMessage(fmt.Sprintf("Invalid %s webhook", webhookType)).
		WithCause(cause).
		WithContext("webhook_type", webhookType).
		WithUserMessage("The webhook request is invalid. Please check the webhook configuration.").
		Build()
}

// QueueError creates a structured error for queue operations
func (f *APIErrorFactory) QueueError(operation string, cause error) *ServiceError {
	severity := SeverityMedium
	if strings.Contains(operation, "full") {
		severity = SeverityHigh
	}

	const queueRetryTimeout = 30 * time.Second // Queue retry timeout

	return NewError(ErrCodeQueueFull).
		WithCategory(CategoryRetryableError).
		WithSeverity(severity).
		WithMessage(fmt.Sprintf("Queue %s failed", operation)).
		WithCause(cause).
		WithContext("operation", operation).
		WithRetryAfter(queueRetryTimeout).
		WithUserMessage("System is busy. Your request will be processed shortly.").
		Build()
}

// apiErrorMapping holds API-specific error codes for different services
type apiErrorMapping struct {
	unauthorized ErrorCode
	notFound     ErrorCode
	rateLimit    ErrorCode
	apiError     ErrorCode
	timeout      ErrorCode
}

// GitLab error codes
var gitLabErrorMapping = apiErrorMapping{
	unauthorized: ErrCodeGitLabUnauthorized,
	notFound:     ErrCodeGitLabNotFound,
	rateLimit:    ErrCodeGitLabRateLimit,
	apiError:     ErrCodeGitLabAPIError,
	timeout:      ErrCodeGitLabTimeout,
}

// Jira error codes
var jiraErrorMapping = apiErrorMapping{
	unauthorized: ErrCodeJiraUnauthorized,
	notFound:     ErrCodeJiraNotFound,
	rateLimit:    ErrCodeJiraRateLimit,
	apiError:     ErrCodeJiraAPIError,
	timeout:      ErrCodeJiraTimeout,
}

// classifyGitLabError determines error classification based on GitLab API status code
func (f *APIErrorFactory) classifyGitLabError(statusCode int) (ErrorCode, ErrorCategory, Severity) {
	return f.classifyAPIError(statusCode, &gitLabErrorMapping)
}

// classifyJiraError determines error classification based on Jira API status code
func (f *APIErrorFactory) classifyJiraError(statusCode int) (ErrorCode, ErrorCategory, Severity) {
	return f.classifyAPIError(statusCode, &jiraErrorMapping)
}

// classifyAPIError determines error classification based on HTTP status code (common implementation)
func (f *APIErrorFactory) classifyAPIError(statusCode int, mapping *apiErrorMapping) (
	ErrorCode, ErrorCategory, Severity) {
	switch statusCode {
	case http.StatusBadRequest:
		return ErrCodeInvalidRequest, CategoryClientError, SeverityLow
	case http.StatusUnauthorized:
		return mapping.unauthorized, CategoryClientError, SeverityMedium
	case http.StatusForbidden:
		return ErrCodeForbidden, CategoryClientError, SeverityMedium
	case http.StatusNotFound:
		return mapping.notFound, CategoryClientError, SeverityLow
	case http.StatusTooManyRequests:
		return mapping.rateLimit, CategoryRateLimitError, SeverityMedium
	case http.StatusInternalServerError:
		return mapping.apiError, CategoryExternalError, SeverityHigh
	case http.StatusBadGateway:
		return mapping.apiError, CategoryExternalError, SeverityHigh
	case http.StatusServiceUnavailable:
		return mapping.apiError, CategoryRetryableError, SeverityHigh
	case http.StatusGatewayTimeout:
		return mapping.timeout, CategoryTimeoutError, SeverityMedium
	default:
		return mapping.apiError, CategoryExternalError, SeverityMedium
	}
}

// HTTPStatusCodeToServiceError converts HTTP status codes to ServiceError
func HTTPStatusCodeToServiceError(statusCode int, operation, responseBody string) *ServiceError {
	factory := NewAPIErrorFactory()

	const (
		httpSuccessMin     = 200
		httpSuccessMax     = 300
		httpClientErrorMin = 400
		httpClientErrorMax = 500
		httpServerErrorMin = 500
	)

	switch {
	case statusCode >= httpSuccessMin && statusCode < httpSuccessMax:
		return nil // Success, no error
	case statusCode >= httpClientErrorMin && statusCode < httpClientErrorMax:
		return factory.clientErrorFromHTTP(statusCode, operation, responseBody)
	case statusCode >= httpServerErrorMin:
		return factory.serverErrorFromHTTP(statusCode, operation, responseBody)
	default:
		return NewError(ErrCodeInternalError).
			WithCategory(CategoryServerError).
			WithSeverity(SeverityMedium).
			WithMessage(fmt.Sprintf("Unexpected HTTP status code: %d", statusCode)).
			WithContext("status_code", statusCode).
			WithContext("operation", operation).
			Build()
	}
}

// clientErrorFromHTTP creates client errors (4xx) from HTTP status
func (f *APIErrorFactory) clientErrorFromHTTP(statusCode int, operation, responseBody string) *ServiceError {
	var code ErrorCode
	var message string

	switch statusCode {
	case http.StatusBadRequest:
		code = ErrCodeInvalidRequest
		message = "Bad request"
	case http.StatusUnauthorized:
		code = ErrCodeUnauthorized
		message = "Authentication required"
	case http.StatusForbidden:
		code = ErrCodeForbidden
		message = "Access forbidden"
	case http.StatusNotFound:
		code = ErrCodeNotFound
		message = "Resource not found"
	case http.StatusTooManyRequests:
		code = ErrCodeRateLimited
		message = "Rate limit exceeded"
	default:
		code = ErrCodeInvalidRequest
		message = fmt.Sprintf("Client error (HTTP %d)", statusCode)
	}

	builder := NewError(code).
		WithCategory(CategoryClientError).
		WithSeverity(SeverityLow).
		WithMessage(fmt.Sprintf("%s during %s", message, operation)).
		WithContext("status_code", statusCode).
		WithContext("operation", operation)

	if responseBody != "" {
		builder.WithDetails(responseBody)
	}

	return builder.Build()
}

// serverErrorFromHTTP creates server errors (5xx) from HTTP status
func (f *APIErrorFactory) serverErrorFromHTTP(statusCode int, operation, responseBody string) *ServiceError {
	var code ErrorCode
	var category ErrorCategory
	var severity Severity
	var message string

	switch statusCode {
	case http.StatusInternalServerError:
		code = ErrCodeExternalAPIError
		category = CategoryExternalError
		severity = SeverityHigh
		message = "Internal server error"
	case http.StatusBadGateway:
		code = ErrCodeExternalAPIError
		category = CategoryExternalError
		severity = SeverityHigh
		message = "Bad gateway"
	case http.StatusServiceUnavailable:
		code = ErrCodeServiceUnavailable
		category = CategoryRetryableError
		severity = SeverityHigh
		message = "Service unavailable"
	case http.StatusGatewayTimeout:
		code = ErrCodeTimeout
		category = CategoryTimeoutError
		severity = SeverityMedium
		message = "Gateway timeout"
	default:
		code = ErrCodeExternalAPIError
		category = CategoryExternalError
		severity = SeverityMedium
		message = fmt.Sprintf("Server error (HTTP %d)", statusCode)
	}

	builder := NewError(code).
		WithCategory(category).
		WithSeverity(severity).
		WithMessage(fmt.Sprintf("%s during %s", message, operation)).
		WithContext("status_code", statusCode).
		WithContext("operation", operation)

	if responseBody != "" {
		builder.WithDetails(responseBody)
	}

	// Add retry-after for service unavailable
	if statusCode == http.StatusServiceUnavailable {
		const serviceUnavailableRetryTimeout = 2 * time.Minute
		builder.WithRetryAfter(serviceUnavailableRetryTimeout)
	}

	return builder.Build()
}

// ParseRetryAfterHeader parses Retry-After header and returns duration
func ParseRetryAfterHeader(header string) *time.Duration {
	if header == "" {
		return nil
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(header); err == nil {
		duration := time.Duration(seconds) * time.Second
		return &duration
	}

	// Try parsing as HTTP date format
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return &duration
		}
	}

	return nil
}

// ExtractErrorFromResponseBody attempts to extract error information from API response
func ExtractErrorFromResponseBody(body string) (errorType, message string) {
	if body == "" {
		return "", ""
	}

	// Simple extraction - in real implementation you might want to parse JSON
	// and extract specific error fields based on the API response format

	// Truncate body if too long
	const maxBodyLength = 500
	if len(body) > maxBodyLength {
		body = body[:maxBodyLength] + "..."
	}

	return "API Error", body
}

// IsRetryableHTTPStatus determines if an HTTP status code indicates a retryable error
func IsRetryableHTTPStatus(statusCode int) bool {
	retryableStatuses := map[int]bool{
		http.StatusRequestTimeout:      true, // 408
		http.StatusTooManyRequests:     true, // 429
		http.StatusInternalServerError: true, // 500
		http.StatusBadGateway:          true, // 502
		http.StatusServiceUnavailable:  true, // 503
		http.StatusGatewayTimeout:      true, // 504
	}

	return retryableStatuses[statusCode]
}

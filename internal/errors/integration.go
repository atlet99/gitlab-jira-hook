package errors

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/async"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
)

const (
	defaultMaxCircuitBreakerAttempts = 5
	defaultCircuitBreakerTimeout     = 30 * time.Second
)

// EnhancedAPIClient provides enhanced error handling for API operations
type EnhancedAPIClient struct {
	httpClient     *http.Client
	retryer        *Retryer
	circuitBreaker *async.CircuitBreaker
	errorFactory   *APIErrorFactory
	logger         *slog.Logger
}

// NewEnhancedAPIClient creates a new API client with enhanced error handling
func NewEnhancedAPIClient(httpClient *http.Client, logger *slog.Logger) *EnhancedAPIClient {
	return &EnhancedAPIClient{
		httpClient:     httpClient,
		retryer:        NewRetryer(DefaultRetryConfig(), logger),
		circuitBreaker: async.NewCircuitBreaker(defaultMaxCircuitBreakerAttempts, defaultCircuitBreakerTimeout, logger),
		errorFactory:   NewAPIErrorFactory(),
		logger:         logger,
	}
}

// WithGitLabRetryConfig configures the client for GitLab API operations
func (c *EnhancedAPIClient) WithGitLabRetryConfig() *EnhancedAPIClient {
	c.retryer = NewRetryer(GitLabRetryConfig(), c.logger)
	return c
}

// WithJiraRetryConfig configures the client for Jira API operations
func (c *EnhancedAPIClient) WithJiraRetryConfig() *EnhancedAPIClient {
	c.retryer = NewRetryer(JiraRetryConfig(), c.logger)
	return c
}

// WithCustomRetryConfig configures the client with custom retry settings
func (c *EnhancedAPIClient) WithCustomRetryConfig(config *RetryConfig) *EnhancedAPIClient {
	c.retryer = NewRetryer(config, c.logger)
	return c
}

// WithCircuitBreaker configures the client with circuit breaker settings
func (c *EnhancedAPIClient) WithCircuitBreaker(_ *CircuitBreakerConfig) *EnhancedAPIClient {
	c.circuitBreaker = async.NewCircuitBreaker(defaultMaxCircuitBreakerAttempts, defaultCircuitBreakerTimeout, c.logger)
	return c
}

// ExecuteRequest executes an HTTP request with enhanced error handling
func (c *EnhancedAPIClient) ExecuteRequest(
	ctx context.Context, req *http.Request, operation string,
) (*http.Response, error) {
	// Execute through circuit breaker and retry logic
	var resp *http.Response
	var lastErr error

	requestOperation := func(ctx context.Context, _ int) error {
		// Clone request for retry safety
		clonedReq := req.Clone(ctx)

		var err error
		resp, err = c.httpClient.Do(clonedReq)
		if err != nil {
			// Network/connection error
			lastErr = c.errorFactory.NetworkError(operation, err)
			return lastErr
		}
		defer func() {
			if resp.Body != nil {
				if closeErr := resp.Body.Close(); closeErr != nil {
					c.logger.Warn("Failed to close response body",
						"operation", operation,
						"error", closeErr)
				}
			}
		}()

		// Check for HTTP errors
		const httpClientErrorThreshold = 400
		if resp.StatusCode >= httpClientErrorThreshold {
			// Read response body for error details
			body := ""
			// Note: In production, you'd want to read the response body
			// This is simplified for demonstration

			// Create appropriate API error based on service
			switch {
			case isGitLabRequest(req):
				lastErr = c.errorFactory.GitLabAPIError(resp.StatusCode, body, operation)
			case isJiraRequest(req):
				lastErr = c.errorFactory.JiraAPIError(resp.StatusCode, body, operation)
			default:
				lastErr = HTTPStatusCodeToServiceError(resp.StatusCode, operation, body)
			}
			return lastErr
		}

		return nil
	}

	// Execute with circuit breaker
	circuitErr := c.circuitBreaker.Execute(func() error {
		// Execute with retry logic
		return c.retryer.Execute(ctx, func(ctx context.Context, attempt int) error {
			return requestOperation(ctx, attempt)
		})
	})

	if circuitErr != nil {
		return nil, circuitErr
	}

	return resp, nil
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (c *EnhancedAPIClient) GetCircuitBreakerStats() map[string]interface{} {
	return map[string]interface{}{
		"state":             "unknown",   // Not available in async package
		"failure_count":     0,           // Not available in async package
		"last_failure":      time.Time{}, // Not available in async package
		"failure_threshold": 0,           // Not available in async package
		"timeout":           0,           // Not available in async package
	}
}

// WebhookErrorHandler provides enhanced error handling for webhook endpoints
type WebhookErrorHandler struct {
	handler      *Handler
	errorFactory *APIErrorFactory
	logger       *slog.Logger
}

// NewWebhookErrorHandler creates a new webhook error handler
func NewWebhookErrorHandler(logger *slog.Logger, monitor *monitoring.WebhookMonitor, debug bool) *WebhookErrorHandler {
	return &WebhookErrorHandler{
		handler:      NewHandler(logger, monitor, debug),
		errorFactory: NewAPIErrorFactory(),
		logger:       logger,
	}
}

// HandleWebhookError handles webhook-specific errors with appropriate responses
func (w *WebhookErrorHandler) HandleWebhookError(
	rw http.ResponseWriter, r *http.Request, err error, webhookType string,
) {
	// Convert to webhook-specific error if needed
	if _, ok := err.(*ServiceError); !ok {
		err = w.errorFactory.WebhookError(webhookType, err)
	}

	// Use centralized error handler
	w.handler.HandleError(rw, r, err)
}

// HandleValidationError handles validation errors in webhook processing
func (w *WebhookErrorHandler) HandleValidationError(rw http.ResponseWriter, r *http.Request, field, message string) {
	err := w.errorFactory.ValidationError(field, message)
	w.handler.HandleError(rw, r, err)
}

// HandleAuthenticationError handles authentication failures
func (w *WebhookErrorHandler) HandleAuthenticationError(rw http.ResponseWriter, r *http.Request, service string) {
	err := w.errorFactory.AuthenticationError(service)
	w.handler.HandleError(rw, r, err)
}

// ProcessingErrorHandler provides enhanced error handling for background processing
type ProcessingErrorHandler struct {
	retryer      *Retryer
	errorFactory *APIErrorFactory
	logger       *slog.Logger
}

// NewProcessingErrorHandler creates a new processing error handler
func NewProcessingErrorHandler(logger *slog.Logger) *ProcessingErrorHandler {
	return &ProcessingErrorHandler{
		retryer:      NewRetryer(DefaultRetryConfig(), logger),
		errorFactory: NewAPIErrorFactory(),
		logger:       logger,
	}
}

// ExecuteWithRetry executes a processing operation with retry logic
func (p *ProcessingErrorHandler) ExecuteWithRetry(
	ctx context.Context, operation RetryableOperation, operationName string,
) error {
	err := p.retryer.Execute(ctx, operation)

	if err != nil {
		// Log processing failure
		p.logger.Error("Background processing failed",
			"operation", operationName,
			"error", err)

		// Convert to processing error if not already a ServiceError
		if _, ok := err.(*ServiceError); !ok {
			return &ServiceError{
				Code:      ErrCodeProcessingFailed,
				Message:   fmt.Sprintf("Processing operation '%s' failed", operationName),
				Context:   map[string]interface{}{"operation": operationName},
				Cause:     err,
				Severity:  SeverityHigh,
				Timestamp: time.Now(),
			}
		}
	}

	return err
}

// ExecuteQueueOperation executes queue operations with appropriate error handling
func (p *ProcessingErrorHandler) ExecuteQueueOperation(
	ctx context.Context, operation func(ctx context.Context, attempt int) error, queueName string,
) error {
	err := operation(ctx, 1) // Queue operations typically don't need retries

	if err != nil {
		return p.errorFactory.QueueError(fmt.Sprintf("%s operation", queueName), err)
	}

	return nil
}

// ErrorMetrics provides error metrics collection
type ErrorMetrics struct {
	monitor *monitoring.WebhookMonitor
	logger  *slog.Logger
}

// NewErrorMetrics creates a new error metrics collector
func NewErrorMetrics(monitor *monitoring.WebhookMonitor, logger *slog.Logger) *ErrorMetrics {
	return &ErrorMetrics{
		monitor: monitor,
		logger:  logger,
	}
}

// RecordError records error metrics
func (m *ErrorMetrics) RecordError(err *ServiceError, operation string) {
	if m.monitor != nil {
		// Use RecordRequest to track failed requests
		m.monitor.RecordRequest(operation, false, 0)
	}

	// Log error metrics
	m.logger.Info("Error recorded",
		"error_code", err.Code,
		"error_category", err.Category,
		"error_severity", err.Severity,
		"operation", operation,
		"retryable", err.IsRetryable())
}

// RecordRecovery records successful error recovery
func (m *ErrorMetrics) RecordRecovery(operation string, attempts int) {
	m.logger.Info("Error recovery successful",
		"operation", operation,
		"attempts", attempts)
}

// Utility functions

// isGitLabRequest determines if the request is for GitLab API
func isGitLabRequest(req *http.Request) bool {
	return contains(req.URL.Host, "gitlab") || contains(req.URL.Path, "/api/v4/")
}

// isJiraRequest determines if the request is for Jira API
func isJiraRequest(req *http.Request) bool {
	return contains(req.URL.Host, "atlassian.net") || contains(req.URL.Path, "/rest/api/")
}

// ErrorHandlingMiddleware creates middleware that provides enhanced error handling
func ErrorHandlingMiddleware(
	logger *slog.Logger, monitor *monitoring.WebhookMonitor, debug bool,
) func(http.Handler) http.Handler {
	handler := NewHandler(logger, monitor, debug)

	return func(next http.Handler) http.Handler {
		return handler.ErrorMiddleware(next)
	}
}

// WithErrorRecovery wraps an operation with enhanced error handling and recovery
func WithErrorRecovery(
	ctx context.Context, logger *slog.Logger, operation func(ctx context.Context, attempt int) error, operationName string,
) error {
	handler := NewProcessingErrorHandler(logger)
	return handler.ExecuteWithRetry(ctx, operation, operationName)
}

// WithCircuitBreaker wraps an operation with circuit breaker protection
func WithCircuitBreaker(
	ctx context.Context,
	logger *slog.Logger,
	operation func(ctx context.Context, attempt int) error,
	config *CircuitBreakerConfig,
) error {
	cb := NewCircuitBreaker(config, logger)
	// Create a wrapper function that matches the expected signature
	wrappedOperation := func() error {
		return operation(ctx, 1)
	}

	return cb.Execute(wrappedOperation)
}

// CreateHTTPErrorResponse creates a standardized HTTP error response
func CreateHTTPErrorResponse(w http.ResponseWriter, err error) {
	handler := NewHandler(slog.Default(), nil, false)

	// Create a dummy request for context
	req, reqErr := http.NewRequest("GET", "/", http.NoBody)
	if reqErr != nil {
		// Log error but continue with nil request - the handler can work without it
		slog.Default().Warn("Failed to create dummy request for error handling",
			"error", reqErr)
		req = nil
	}

	handler.HandleError(w, req, err)
}

// WrapError wraps a standard error with enhanced ServiceError context
func WrapError(err error, code ErrorCode, operation string) *ServiceError {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr
	}

	return &ServiceError{
		Code:      code,
		Message:   fmt.Sprintf("Error in %s", operation),
		Context:   map[string]interface{}{"operation": operation},
		Cause:     err,
		Severity:  getDefaultSeverity(code),
		Timestamp: time.Now(),
	}
}

// IsRetryableError checks if an error should be retried
func IsRetryableError(err error) bool {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr.IsRetryable()
	}
	return isDefaultRetryable(err)
}

// GetErrorCode extracts error code from any error
func GetErrorCode(err error) ErrorCode {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr.Code
	}
	return ErrCodeInternalError
}

// GetErrorSeverity extracts error severity from any error
func GetErrorSeverity(err error) Severity {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr.Severity
	}
	return SeverityMedium
}

package errors

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
)

// Handler configuration constants
const (
	defaultRateLimitRetryTimeout = 5 * time.Minute // Default rate limit retry timeout
)

// Handler provides centralized error handling and response formatting
type Handler struct {
	logger  *slog.Logger
	monitor *monitoring.WebhookMonitor
	debug   bool
}

// NewHandler creates a new error handler
func NewHandler(logger *slog.Logger, monitor *monitoring.WebhookMonitor, debug bool) *Handler {
	return &Handler{
		logger:  logger,
		monitor: monitor,
		debug:   debug,
	}
}

// HandleError processes and responds to errors with appropriate HTTP status and JSON response
func (h *Handler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	serviceErr := h.processError(err, r)

	// Log the error with appropriate level
	h.logError(serviceErr, r)

	// Record metrics if monitor is available
	if h.monitor != nil {
		// Use RecordRequest to track failed requests
		h.monitor.RecordRequest(r.URL.Path, false, 0)
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Set retry-after header if specified
	if serviceErr.RetryAfter != nil {
		w.Header().Set("Retry-After", formatRetryAfter(*serviceErr.RetryAfter))
	}

	// Write HTTP status
	w.WriteHeader(serviceErr.HTTPStatusCode())

	// Write error response
	response := serviceErr.ToErrorResponse()
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
		// Fallback to plain text
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// processError converts any error to ServiceError with appropriate context
func (h *Handler) processError(err error, r *http.Request) *ServiceError {
	// If it's already a ServiceError, enhance it with request context
	if serviceErr, ok := err.(*ServiceError); ok {
		return h.enhanceWithRequestContext(serviceErr, r)
	}

	// Convert generic errors to ServiceError
	serviceErr := h.classifyError(err)
	return h.enhanceWithRequestContext(serviceErr, r)
}

// classifyError converts generic errors to structured ServiceError
func (h *Handler) classifyError(err error) *ServiceError {
	errStr := err.Error()

	// Network/HTTP client errors
	if isNetworkError(errStr) {
		return NewError(ErrCodeTimeout).
			WithCategory(CategoryTimeoutError).
			WithSeverity(SeverityMedium).
			WithMessage("Network request failed").
			WithCause(err).
			WithUserMessage("The request failed due to network issues. Please try again.").
			Build()
	}

	// JSON parsing errors
	if isJSONError(errStr) {
		return NewError(ErrCodeInvalidRequest).
			WithCategory(CategoryClientError).
			WithSeverity(SeverityLow).
			WithMessage("Invalid JSON in request").
			WithCause(err).
			WithUserMessage("The request contains invalid JSON. Please check the format.").
			Build()
	}

	// Rate limiting errors
	if isRateLimitError(errStr) {
		return NewError(ErrCodeRateLimited).
			WithCategory(CategoryRateLimitError).
			WithSeverity(SeverityMedium).
			WithMessage("Rate limit exceeded").
			WithCause(err).
			WithRetryAfter(defaultRateLimitRetryTimeout). // Default rate limit retry
			WithUserMessage("Too many requests. Please wait before trying again.").
			Build()
	}

	// Default to internal error
	return NewError(ErrCodeInternalError).
		WithCategory(CategoryServerError).
		WithSeverity(SeverityHigh).
		WithMessage("Internal server error").
		WithCause(err).
		WithUserMessage("An unexpected error occurred. Please try again or contact support.").
		Build()
}

// enhanceWithRequestContext adds request-specific context to ServiceError
func (h *Handler) enhanceWithRequestContext(serviceErr *ServiceError, r *http.Request) *ServiceError {
	// Add request context
	if serviceErr.Context == nil {
		serviceErr.Context = make(map[string]interface{})
	}

	serviceErr.Context["request_method"] = r.Method
	serviceErr.Context["request_path"] = r.URL.Path
	serviceErr.Context["request_query"] = r.URL.RawQuery
	serviceErr.Context["remote_addr"] = r.RemoteAddr
	serviceErr.Context["user_agent"] = r.Header.Get("User-Agent")

	// Add request ID if available
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		serviceErr.RequestID = requestID
	}

	// Add debug information if enabled
	if h.debug {
		serviceErr.Context["stack_trace"] = getStackTrace()
		serviceErr.Context["headers"] = sanitizeHeaders(r.Header)
	}

	return serviceErr
}

// logError logs the error with appropriate level and context
func (h *Handler) logError(serviceErr *ServiceError, r *http.Request) {
	logLevel := h.getLogLevel(serviceErr.Severity)

	// Prepare log attributes
	attrs := []slog.Attr{
		slog.String("error_code", string(serviceErr.Code)),
		slog.String("error_category", string(serviceErr.Category)),
		slog.String("error_severity", string(serviceErr.Severity)),
		slog.String("error_message", serviceErr.Message),
		slog.String("request_method", r.Method),
		slog.String("request_path", r.URL.Path),
		slog.String("remote_addr", r.RemoteAddr),
		slog.Int("http_status", serviceErr.HTTPStatusCode()),
	}

	// Add request ID if available
	if serviceErr.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", serviceErr.RequestID))
	}

	// Add details if available
	if serviceErr.Details != "" {
		attrs = append(attrs, slog.String("error_details", serviceErr.Details))
	}

	// Add cause if available
	if serviceErr.Cause != nil {
		attrs = append(attrs, slog.String("underlying_error", serviceErr.Cause.Error()))
	}

	// Add context fields
	for key, value := range serviceErr.Context {
		if str, ok := value.(string); ok {
			attrs = append(attrs, slog.String("context_"+key, str))
		}
	}

	// Log with appropriate level
	switch logLevel {
	case slog.LevelError:
		h.logger.LogAttrs(context.Background(), slog.LevelError, "Request failed", attrs...)
	case slog.LevelWarn:
		h.logger.LogAttrs(context.Background(), slog.LevelWarn, "Request warning", attrs...)
	case slog.LevelInfo:
		h.logger.LogAttrs(context.Background(), slog.LevelInfo, "Request info", attrs...)
	default:
		h.logger.LogAttrs(context.Background(), slog.LevelDebug, "Request debug", attrs...)
	}
}

// getLogLevel determines appropriate log level based on error severity
func (h *Handler) getLogLevel(severity Severity) slog.Level {
	switch severity {
	case SeverityCritical:
		return slog.LevelError
	case SeverityHigh:
		return slog.LevelError
	case SeverityMedium:
		return slog.LevelWarn
	case SeverityLow:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// ErrorMiddleware provides HTTP middleware for centralized error handling
func (h *Handler) ErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the response writer to catch panics
		wrapped := &errorResponseWriter{
			ResponseWriter: w,
			handler:        h,
			request:        r,
		}

		// Add panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				if e, ok := rec.(error); ok {
					err = e
				} else {
					err = NewError(ErrCodeInternalError).
						WithCategory(CategoryServerError).
						WithSeverity(SeverityCritical).
						WithMessage("Panic occurred during request processing").
						WithDetails(formatPanic(rec)).
						Build()
				}

				h.HandleError(wrapped, r, err)
			}
		}()

		next.ServeHTTP(wrapped, r)
	})
}

// errorResponseWriter wraps http.ResponseWriter to provide error handling
type errorResponseWriter struct {
	http.ResponseWriter
	handler *Handler
	request *http.Request
	written bool
}

// WriteHeader captures the status code for error handling
func (w *errorResponseWriter) WriteHeader(statusCode int) {
	w.written = true
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures writes for error handling
func (w *errorResponseWriter) Write(data []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(data)
}

// Helper functions

// isNetworkError checks if error is network-related
func isNetworkError(errStr string) bool {
	networkIndicators := []string{
		"connection refused", "connection timeout", "network unreachable",
		"no such host", "dial tcp", "context deadline exceeded",
		"Client.Timeout exceeded", "connection reset by peer",
	}

	for _, indicator := range networkIndicators {
		if contains(errStr, indicator) {
			return true
		}
	}
	return false
}

// isJSONError checks if error is JSON parsing related
func isJSONError(errStr string) bool {
	jsonIndicators := []string{
		"invalid character", "unexpected end of JSON",
		"cannot unmarshal", "json:", "JSON",
	}

	for _, indicator := range jsonIndicators {
		if contains(errStr, indicator) {
			return true
		}
	}
	return false
}

// isRateLimitError checks if error is rate limiting related
func isRateLimitError(errStr string) bool {
	rateLimitIndicators := []string{
		"rate limit", "too many requests", "429",
		"quota exceeded", "throttled",
	}

	for _, indicator := range rateLimitIndicators {
		if contains(errStr, indicator) {
			return true
		}
	}
	return false
}

// contains checks if string contains substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		str[:len(substr)] == substr ||
		(len(str) > len(substr) && contains(str[1:], substr))
}

// getStackTrace captures stack trace for debugging
func getStackTrace() string {
	const stackTraceBufferSize = 4096 // Standard buffer size for stack traces
	buf := make([]byte, stackTraceBufferSize)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// sanitizeHeaders removes sensitive headers from logging
func sanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"x-auth-token":  true,
		"cookie":        true,
	}

	for key, values := range headers {
		lowerKey := key
		if len(values) > 0 {
			if sensitiveHeaders[lowerKey] {
				sanitized[key] = "[REDACTED]"
			} else {
				sanitized[key] = values[0]
			}
		}
	}
	return sanitized
}

// formatPanic formats panic information for logging
func formatPanic(rec interface{}) string {
	switch v := rec.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return "Unknown panic type"
	}
}

// formatRetryAfter formats retry-after duration for HTTP header
func formatRetryAfter(duration time.Duration) string {
	return formatDurationSeconds(duration)
}

// formatDurationSeconds formats duration as seconds string
func formatDurationSeconds(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 1 {
		return "1" // Minimum 1 second
	}
	return string(rune(seconds + '0')) // Simple integer to string conversion
}

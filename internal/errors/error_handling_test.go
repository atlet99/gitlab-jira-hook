package errors

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestServiceError_Creation(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *ServiceError
		expected struct {
			code     ErrorCode
			category ErrorCategory
			severity Severity
		}
	}{
		{
			name: "Basic error creation",
			builder: func() *ServiceError {
				return NewError(ErrCodeInvalidRequest).
					WithMessage("Test error").
					Build()
			},
			expected: struct {
				code     ErrorCode
				category ErrorCategory
				severity Severity
			}{
				code:     ErrCodeInvalidRequest,
				category: CategoryClientError,
				severity: SeverityLow,
			},
		},
		{
			name: "Error with custom category and severity",
			builder: func() *ServiceError {
				return NewError(ErrCodeGitLabAPIError).
					WithCategory(CategoryExternalError).
					WithSeverity(SeverityCritical).
					WithMessage("Critical GitLab error").
					Build()
			},
			expected: struct {
				code     ErrorCode
				category ErrorCategory
				severity Severity
			}{
				code:     ErrCodeGitLabAPIError,
				category: CategoryExternalError,
				severity: SeverityCritical,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()

			if err.Code != tt.expected.code {
				t.Errorf("Expected code %s, got %s", tt.expected.code, err.Code)
			}
			if err.Category != tt.expected.category {
				t.Errorf("Expected category %s, got %s", tt.expected.category, err.Category)
			}
			if err.Severity != tt.expected.severity {
				t.Errorf("Expected severity %s, got %s", tt.expected.severity, err.Severity)
			}
		})
	}
}

func TestServiceError_HTTPStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		errorCode    ErrorCode
		expectedHTTP int
	}{
		{"Bad Request", ErrCodeInvalidRequest, http.StatusBadRequest},
		{"Unauthorized", ErrCodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", ErrCodeForbidden, http.StatusForbidden},
		{"Not Found", ErrCodeNotFound, http.StatusNotFound},
		{"Rate Limited", ErrCodeRateLimited, http.StatusTooManyRequests},
		{"Internal Error", ErrCodeInternalError, http.StatusInternalServerError},
		{"Service Unavailable", ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
		{"GitLab API Error", ErrCodeGitLabAPIError, http.StatusBadGateway},
		{"Jira API Error", ErrCodeJiraAPIError, http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.errorCode).Build()
			statusCode := err.HTTPStatusCode()

			if statusCode != tt.expectedHTTP {
				t.Errorf("Expected HTTP status %d, got %d", tt.expectedHTTP, statusCode)
			}
		})
	}
}

func TestServiceError_IsRetryable(t *testing.T) {
	tests := []struct {
		name        string
		errorCode   ErrorCode
		category    ErrorCategory
		isRetryable bool
	}{
		{"Rate Limited Error", ErrCodeRateLimited, CategoryRateLimitError, true},
		{"Timeout Error", ErrCodeTimeout, CategoryTimeoutError, true},
		{"External API Error", ErrCodeGitLabAPIError, CategoryExternalError, true},
		{"Retryable Error", ErrCodeServiceUnavailable, CategoryRetryableError, true},
		{"Client Error", ErrCodeInvalidRequest, CategoryClientError, false},
		{"Permanent Error", ErrCodeNotFound, CategoryClientError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.errorCode).
				WithCategory(tt.category).
				Build()

			isRetryable := err.IsRetryable()

			if isRetryable != tt.isRetryable {
				t.Errorf("Expected retryable %v, got %v", tt.isRetryable, isRetryable)
			}
		})
	}
}

func TestAPIErrorFactory_GitLabAPIError(t *testing.T) {
	factory := NewAPIErrorFactory()

	tests := []struct {
		name         string
		statusCode   int
		operation    string
		expectedCode ErrorCode
	}{
		{"Bad Request", http.StatusBadRequest, "create_issue", ErrCodeInvalidRequest},
		{"Unauthorized", http.StatusUnauthorized, "get_user", ErrCodeGitLabUnauthorized},
		{"Not Found", http.StatusNotFound, "get_project", ErrCodeGitLabNotFound},
		{"Rate Limited", http.StatusTooManyRequests, "list_issues", ErrCodeGitLabRateLimit},
		{"Internal Error", http.StatusInternalServerError, "create_comment", ErrCodeGitLabAPIError},
		{"Timeout", http.StatusGatewayTimeout, "update_issue", ErrCodeGitLabTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.GitLabAPIError(tt.statusCode, "response body", tt.operation)

			if err.Code != tt.expectedCode {
				t.Errorf("Expected error code %s, got %s", tt.expectedCode, err.Code)
			}

			if !strings.Contains(err.Message, tt.operation) {
				t.Errorf("Expected error message to contain operation %s", tt.operation)
			}

			if err.Context["status_code"] != tt.statusCode {
				t.Errorf("Expected status code in context %d, got %v",
					tt.statusCode, err.Context["status_code"])
			}
		})
	}
}

func TestRetryer_Execute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("Success on first attempt", func(t *testing.T) {
		retryer := NewRetryer(DefaultRetryConfig(), logger)
		attempts := 0

		operation := func(ctx context.Context, attempt int) error {
			attempts++
			return nil
		}

		err := retryer.Execute(context.Background(), operation)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Success after retries", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Second,
			Multiplier:   2.0,
			Jitter:       false,
			RetryableFunc: func(err error) bool {
				if serviceErr, ok := err.(*ServiceError); ok {
					return serviceErr.IsRetryable()
				}
				return isDefaultRetryable(err)
			},
		}
		retryer := NewRetryer(config, logger)
		attempts := 0

		operation := func(ctx context.Context, attempt int) error {
			attempts++
			if attempts < 3 {
				return NewError(ErrCodeTimeout).
					WithCategory(CategoryTimeoutError).
					Build()
			}
			return nil
		}

		err := retryer.Execute(context.Background(), operation)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Failure after all retries", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:  2,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Second,
			Multiplier:   2.0,
			Jitter:       false,
			RetryableFunc: func(err error) bool {
				if serviceErr, ok := err.(*ServiceError); ok {
					return serviceErr.IsRetryable()
				}
				return isDefaultRetryable(err)
			},
		}
		retryer := NewRetryer(config, logger)
		attempts := 0

		operation := func(ctx context.Context, attempt int) error {
			attempts++
			return NewError(ErrCodeTimeout).
				WithCategory(CategoryTimeoutError).
				Build()
		}

		err := retryer.Execute(context.Background(), operation)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("Non-retryable error", func(t *testing.T) {
		retryer := NewRetryer(DefaultRetryConfig(), logger)
		attempts := 0

		operation := func(ctx context.Context, attempt int) error {
			attempts++
			return NewError(ErrCodeInvalidRequest).
				WithCategory(CategoryClientError).
				Build()
		}

		err := retryer.Execute(context.Background(), operation)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})
}

func TestCircuitBreaker_Execute(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("Success with closed circuit", func(t *testing.T) {
		cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(), logger)

		operation := func(ctx context.Context, attempt int) error {
			return nil
		}

		err := cb.Execute(func() error {
			return operation(context.Background(), 1)
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		// Note: GetState() is not available in the async package's CircuitBreaker
	})

	t.Run("Circuit opens after failures", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          time.Millisecond,
			MonitoringWindow: time.Second,
		}
		cb := NewCircuitBreaker(config, logger)

		operation := func(ctx context.Context, attempt int) error {
			return errors.New("test error")
		}

		// First failure
		_ = cb.Execute(func() error {
			return operation(context.Background(), 1)
		})
		// Note: GetState() is not available in the async package's CircuitBreaker

		// Second failure should open circuit - add small delay to ensure state update
		time.Sleep(10 * time.Millisecond)
		_ = cb.Execute(func() error {
			return operation(context.Background(), 1)
		})
		// Note: GetState() is not available in the async package's CircuitBreaker

		// Add delay to ensure circuit breaker state is updated
		time.Sleep(10 * time.Millisecond)

		// Next call should be rejected
		err := cb.Execute(func() error {
			return operation(context.Background(), 1)
		})
		if err == nil {
			t.Error("Expected circuit breaker error, got nil")
		}
		if !strings.Contains(err.Error(), "circuit breaker is open") {
			t.Errorf("Expected circuit breaker error message, got %v", err)
		}
	})
}

func TestErrorHandler_HandleError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewHandler(logger, nil, false)

	t.Run("ServiceError with JSON response", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/test", nil)

		serviceErr := NewError(ErrCodeInvalidRequest).
			WithMessage("Test validation error").
			WithUserMessage("Please check your input").
			Build()

		handler.HandleError(recorder, req, serviceErr)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", recorder.Code)
		}

		contentType := recorder.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected JSON content type, got %s", contentType)
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "INVALID_REQUEST") {
			t.Errorf("Expected error code in response body, got %s", body)
		}
		if !strings.Contains(body, "Please check your input") {
			t.Errorf("Expected user message in response body, got %s", body)
		}
	})

	t.Run("Generic error conversion", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		genericErr := errors.New("connection refused")

		handler.HandleError(recorder, req, genericErr)

		if recorder.Code != http.StatusRequestTimeout {
			t.Errorf("Expected status 408 for network error, got %d", recorder.Code)
		}

		body := recorder.Body.String()
		if !strings.Contains(body, "TIMEOUT") {
			t.Errorf("Expected timeout error code, got %s", body)
		}
	})

	t.Run("Rate limit with retry-after", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/test", nil)

		retryAfter := time.Minute * 5
		serviceErr := NewError(ErrCodeRateLimited).
			WithMessage("Rate limit exceeded").
			WithRetryAfter(retryAfter).
			Build()

		handler.HandleError(recorder, req, serviceErr)

		if recorder.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", recorder.Code)
		}

		retryHeader := recorder.Header().Get("Retry-After")
		if retryHeader == "" {
			t.Error("Expected Retry-After header to be set")
		}
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isRetryable bool
	}{
		{
			name: "ServiceError retryable",
			err: NewError(ErrCodeTimeout).
				WithCategory(CategoryTimeoutError).
				Build(),
			isRetryable: true,
		},
		{
			name: "ServiceError not retryable",
			err: NewError(ErrCodeInvalidRequest).
				WithCategory(CategoryClientError).
				Build(),
			isRetryable: false,
		},
		{
			name:        "Network error (generic)",
			err:         errors.New("connection refused"),
			isRetryable: true,
		},
		{
			name:        "JSON error (generic)",
			err:         errors.New("invalid character"),
			isRetryable: false,
		},
		{
			name:        "Rate limit error (generic)",
			err:         errors.New("rate limit exceeded"),
			isRetryable: true,
		},
		{
			name:        "Auth error (generic)",
			err:         errors.New("unauthorized"),
			isRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.isRetryable {
				t.Errorf("Expected IsRetryableError %v, got %v for error: %v",
					tt.isRetryable, result, tt.err)
			}
		})
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	serviceErr := NewError(ErrCodeGitLabAPIError).
		WithMessage("GitLab API failed").
		WithDetails("Connection timeout").
		WithContext("operation", "create_issue").
		WithContext("project_id", "123").
		WithUserMessage("Unable to create issue. Please try again.").
		WithRetryAfter(time.Minute * 2).
		Build()

	response := serviceErr.ToErrorResponse()

	if response.Error != ErrCodeGitLabAPIError {
		t.Errorf("Expected error code %s, got %s", ErrCodeGitLabAPIError, response.Error)
	}

	if response.Message != "GitLab API failed" {
		t.Errorf("Expected message 'GitLab API failed', got %s", response.Message)
	}

	if response.Details != "Connection timeout" {
		t.Errorf("Expected details 'Connection timeout', got %s", response.Details)
	}

	if response.UserMessage != "Unable to create issue. Please try again." {
		t.Errorf("Expected user message, got %s", response.UserMessage)
	}

	if response.RetryAfter == nil || *response.RetryAfter != 120 {
		t.Errorf("Expected retry after 120 seconds, got %v", response.RetryAfter)
	}

	if response.Context["operation"] != "create_issue" {
		t.Errorf("Expected operation context, got %v", response.Context)
	}
}

// Benchmark tests
func BenchmarkServiceError_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewError(ErrCodeInternalError).
			WithMessage("Test error").
			WithContext("key", "value").
			Build()
	}
}

func BenchmarkRetryer_Execute_Success(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	retryer := NewRetryer(DefaultRetryConfig(), logger)

	operation := func(ctx context.Context, attempt int) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryer.Execute(context.Background(), operation)
	}
}

func BenchmarkErrorHandler_HandleError(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewHandler(logger, nil, false)

	serviceErr := NewError(ErrCodeInvalidRequest).
		WithMessage("Test error").
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/test", nil)
		handler.HandleError(recorder, req, serviceErr)
	}
}

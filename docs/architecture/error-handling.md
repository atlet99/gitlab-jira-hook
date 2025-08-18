# Enhanced Error Handling System

The GitLab ↔ Jira Hook service now includes a comprehensive, production-grade error handling system that provides structured error types, intelligent retry logic, circuit breaker protection, and enhanced observability.

## Overview

The enhanced error handling system consists of several key components:

- **Structured Error Types** - Typed errors with codes, categories, and severity levels
- **Intelligent Retry Logic** - Exponential backoff with jitter and smart retry decisions
- **Circuit Breaker Pattern** - Protection against cascading failures
- **Centralized Error Handling** - Unified error processing and HTTP response formatting
- **Enhanced Observability** - Detailed logging with context and metrics

## Core Components

### 1. ServiceError Structure

All errors in the system use the `ServiceError` type which provides:

```go
type ServiceError struct {
    Code        ErrorCode              // Specific error identifier
    Category    ErrorCategory          // Error classification for handling strategy
    Severity    Severity               // Impact level (LOW/MEDIUM/HIGH/CRITICAL)
    Message     string                 // Technical error message
    Details     string                 // Additional error details
    Context     map[string]interface{} // Contextual information
    Cause       error                  // Original underlying error
    Timestamp   time.Time              // When the error occurred
    RequestID   string                 // Request tracking ID
    UserMessage string                 // User-friendly error message
    RetryAfter  *time.Duration         // When to retry (for rate limiting)
}
```

### 2. Error Categories

Errors are classified into categories that determine handling strategy:

- **CLIENT_ERROR** - User/client mistakes (4xx HTTP errors)
- **SERVER_ERROR** - Internal system errors (5xx HTTP errors) 
- **EXTERNAL_ERROR** - External API failures
- **RETRYABLE_ERROR** - Transient errors that can be retried
- **PERMANENT_ERROR** - Permanent failures that should not be retried
- **RATE_LIMIT_ERROR** - Rate limiting errors with retry-after information
- **TIMEOUT_ERROR** - Network timeout and deadline exceeded errors

### 3. Error Codes

Specific error codes for different failure scenarios:

#### Client Errors (4xx)
- `INVALID_REQUEST` - Malformed or invalid request data
- `UNAUTHORIZED` - Authentication failed
- `FORBIDDEN` - Access denied
- `NOT_FOUND` - Resource not found
- `RATE_LIMITED` - Request rate limit exceeded
- `INVALID_SIGNATURE` - Webhook signature validation failed
- `INVALID_WEBHOOK` - Webhook format or content invalid
- `VALIDATION_FAILED` - Data validation errors

#### Server Errors (5xx)
- `INTERNAL_ERROR` - General internal server error
- `SERVICE_UNAVAILABLE` - Service temporarily unavailable
- `TIMEOUT` - Request timeout
- `DATABASE_ERROR` - Database operation failed
- `EXTERNAL_API_ERROR` - External service error

#### GitLab/Jira Specific Errors
- `GITLAB_API_ERROR` / `JIRA_API_ERROR` - API-specific failures
- `GITLAB_TIMEOUT` / `JIRA_TIMEOUT` - API timeout errors
- `GITLAB_RATE_LIMIT` / `JIRA_RATE_LIMIT` - API rate limiting
- `GITLAB_UNAUTHORIZED` / `JIRA_UNAUTHORIZED` - API authentication errors
- `GITLAB_NOT_FOUND` / `JIRA_NOT_FOUND` - API resource not found

#### Processing Errors
- `PROCESSING_FAILED` - Background processing failure
- `QUEUE_FULL` - Job queue is full
- `WORKER_UNAVAILABLE` - No workers available
- `CIRCUIT_BREAKER_OPEN` - Circuit breaker protecting service

## Features

### 1. Intelligent Retry Logic

The retry system provides:

- **Exponential backoff** with configurable base delay and multiplier
- **Jitter** to prevent thundering herd problems
- **Smart retry decisions** based on error type and category
- **Context-aware cancellation** respecting request timeouts
- **Configurable retry policies** for different operations

```go
// Example retry configuration for GitLab API
config := &RetryConfig{
    MaxAttempts:  5,
    InitialDelay: 200 * time.Millisecond,
    MaxDelay:     60 * time.Second,
    Multiplier:   1.5,
    Jitter:       true,
}
```

### 2. Circuit Breaker Protection

Prevents cascading failures with:

- **Failure threshold** - Opens circuit after consecutive failures
- **Success threshold** - Closes circuit after consecutive successes
- **Timeout period** - How long circuit stays open
- **Half-open state** - Gradual recovery testing

```go
// Example circuit breaker configuration
config := &CircuitBreakerConfig{
    FailureThreshold: 5,  // Open after 5 failures
    SuccessThreshold: 3,  // Close after 3 successes
    Timeout:          30 * time.Second,
    MonitoringWindow: 60 * time.Second,
}
```

### 3. Enhanced HTTP Error Responses

All HTTP errors return structured JSON responses:

```json
{
    "error": "GITLAB_API_ERROR",
    "message": "GitLab API create_issue failed",
    "details": "Connection timeout after 30s",
    "context": {
        "operation": "create_issue",
        "status_code": 502,
        "project_id": "123"
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "request_id": "req-abc123",
    "user_message": "Unable to create issue. Please try again later.",
    "retry_after_seconds": 120
}
```

### 4. Comprehensive Logging

Enhanced logging includes:

- **Structured logging** with context fields
- **Appropriate log levels** based on error severity
- **Request tracing** with request IDs
- **Error context** including operation details
- **Performance metrics** and timing information

### 5. Error Recovery Patterns

Built-in support for common error recovery patterns:

- **Graceful degradation** - Continue with reduced functionality
- **Fallback mechanisms** - Alternative processing paths
- **Dead letter queues** - Handle persistently failing jobs
- **Error aggregation** - Collect and report error patterns

## Usage Examples

### Creating Structured Errors

```go
// Create a GitLab API error
err := NewError(ErrCodeGitLabAPIError).
    WithCategory(CategoryExternalError).
    WithSeverity(SeverityHigh).
    WithMessage("Failed to create GitLab issue").
    WithCause(originalError).
    WithContext("project_id", "123").
    WithContext("operation", "create_issue").
    WithUserMessage("Unable to create issue. Please try again.").
    Build()
```

### Using Retry Logic

```go
// Execute operation with retry
retryer := NewRetryer(GitLabRetryConfig(), logger)
err := retryer.Execute(ctx, func(ctx context.Context, attempt int) error {
    return gitlabClient.CreateIssue(ctx, request)
})
```

### Circuit Breaker Protection

```go
// Protect operation with circuit breaker
circuitBreaker := NewCircuitBreaker(DefaultCircuitBreakerConfig(), logger)
err := circuitBreaker.Execute(ctx, func(ctx context.Context, attempt int) error {
    return externalServiceCall()
})
```

### Centralized Error Handling

```go
// Handle errors in HTTP handlers
errorHandler := NewHandler(logger, monitor, debugMode)
errorHandler.HandleError(w, r, err)
```

### Error Middleware

```go
// Add error handling middleware
mux := http.NewServeMux()
mux.Handle("/api/", ErrorHandlingMiddleware(logger, monitor, debug)(apiHandler))
```

## Configuration

### Environment Variables

The error handling system respects existing configuration and adds:

- **Retry behavior** - Configured per API client (GitLab/Jira)
- **Circuit breaker settings** - Configurable thresholds and timeouts
- **Logging levels** - Control error logging verbosity
- **Debug mode** - Enhanced error context in development

### API-Specific Configurations

#### GitLab API Error Handling
- Optimized retry delays for GitLab rate limits
- GitLab-specific error code mapping
- Project and user context in errors

#### Jira API Error Handling  
- Jira Cloud API rate limit handling
- Jira-specific error classifications
- Issue and project context preservation

## Testing

The error handling system includes comprehensive tests:

- **Unit tests** for all error types and classifications
- **Integration tests** for retry and circuit breaker logic
- **HTTP response tests** for error formatting
- **Performance benchmarks** for error creation and handling

Run tests:
```bash
go test ./internal/errors/ -v
```

## Monitoring and Observability

The system provides enhanced observability through:

### Metrics
- Error rate by code and category
- Retry attempt distributions
- Circuit breaker state changes
- Response time percentiles

### Logging
- Structured error logs with context
- Request correlation via request IDs
- Error aggregation and patterns
- Performance and timing information

### Health Checks
- Circuit breaker status
- Error rate thresholds
- External API connectivity
- System resource utilization

## Best Practices

### Error Creation
1. Use specific error codes for different failure scenarios
2. Include relevant context information
3. Provide user-friendly messages for client-facing errors
4. Set appropriate retry-after times for rate limiting

### Retry Logic
1. Use different retry configurations for different APIs
2. Respect context cancellation and timeouts
3. Log retry attempts for debugging
4. Avoid retrying permanent errors

### Circuit Breaker Usage
1. Set appropriate failure thresholds
2. Monitor circuit breaker state changes
3. Implement fallback mechanisms
4. Consider gradual traffic ramping

### Error Handling
1. Use centralized error handling middleware
2. Log errors with appropriate severity levels
3. Include request context in error responses
4. Monitor error patterns and trends

## Migration Notes

The enhanced error handling system is backward compatible:

- Existing error handling continues to work
- Gradual migration to structured errors recommended
- New error types provide additional information
- Monitoring and logging are enhanced automatically

## Performance Considerations

The error handling system is designed for production use:

- **Low latency** - Minimal overhead for error creation
- **Memory efficient** - Structured errors with lazy evaluation
- **Concurrent safe** - Thread-safe circuit breakers and retry logic
- **Scalable** - Suitable for high-throughput webhook processing

## Future Enhancements

Planned improvements include:

- **Error aggregation** - Collect and analyze error patterns
- **Adaptive retry** - ML-based retry delay optimization
- **Error forecasting** - Predict and prevent error scenarios
- **Integration metrics** - API-specific error tracking
- **Distributed tracing** - Full request lifecycle tracking
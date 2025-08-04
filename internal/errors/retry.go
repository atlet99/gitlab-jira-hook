package errors

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	mathRand "math/rand"
	"time"
)

// Retry configuration constants
const (
	// Default retry settings
	defaultMaxAttempts  = 3
	defaultInitialDelay = 100 * time.Millisecond
	defaultMaxDelay     = 30 * time.Second
	defaultMultiplier   = 2.0

	// Fast retry settings
	fastMaxAttempts  = 5
	fastInitialDelay = 200 * time.Millisecond
	fastMaxDelay     = 60 * time.Second
	fastMultiplier   = 1.5

	// Aggressive retry settings
	aggressiveMaxAttempts  = 4
	aggressiveInitialDelay = 300 * time.Millisecond
	aggressiveMaxDelay     = 45 * time.Second
	aggressiveMultiplier   = 2.0

	// Circuit breaker settings
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 3
	defaultCircuitTimeout   = 30 * time.Second
	defaultMonitoringWindow = 60 * time.Second

	// Jitter calculation constants
	jitterPercentage = 0.25 // 25% jitter range
	jitterMultiplier = 2    // For symmetric jitter calculation
	jitterOffset     = 0.5  // Center offset for symmetric distribution
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts   int              `json:"max_attempts"`
	InitialDelay  time.Duration    `json:"initial_delay"`
	MaxDelay      time.Duration    `json:"max_delay"`
	Multiplier    float64          `json:"multiplier"`
	Jitter        bool             `json:"jitter"`
	RetryableFunc func(error) bool `json:"-"` // Function to determine if error is retryable
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  defaultMaxAttempts,
		InitialDelay: defaultInitialDelay,
		MaxDelay:     defaultMaxDelay,
		Multiplier:   defaultMultiplier,
		Jitter:       true,
		RetryableFunc: func(err error) bool {
			if serviceErr, ok := err.(*ServiceError); ok {
				return serviceErr.IsRetryable()
			}
			return isDefaultRetryable(err)
		},
	}
}

// GitLabRetryConfig returns retry configuration optimized for GitLab API
func GitLabRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  fastMaxAttempts,
		InitialDelay: fastInitialDelay,
		MaxDelay:     fastMaxDelay,
		Multiplier:   fastMultiplier,
		Jitter:       true,
		RetryableFunc: func(err error) bool {
			if serviceErr, ok := err.(*ServiceError); ok {
				// Retry on specific GitLab errors
				retryableCodes := map[ErrorCode]bool{
					ErrCodeGitLabTimeout:   true,
					ErrCodeGitLabRateLimit: true,
					ErrCodeTimeout:         true,
					ErrCodeRateLimited:     true,
				}
				return retryableCodes[serviceErr.Code] || serviceErr.IsRetryable()
			}
			return isDefaultRetryable(err)
		},
	}
}

// JiraRetryConfig returns retry configuration optimized for Jira API
func JiraRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  aggressiveMaxAttempts,
		InitialDelay: aggressiveInitialDelay,
		MaxDelay:     aggressiveMaxDelay,
		Multiplier:   aggressiveMultiplier,
		Jitter:       true,
		RetryableFunc: func(err error) bool {
			if serviceErr, ok := err.(*ServiceError); ok {
				// Retry on specific Jira errors
				retryableCodes := map[ErrorCode]bool{
					ErrCodeJiraTimeout:   true,
					ErrCodeJiraRateLimit: true,
					ErrCodeTimeout:       true,
					ErrCodeRateLimited:   true,
				}
				return retryableCodes[serviceErr.Code] || serviceErr.IsRetryable()
			}
			return isDefaultRetryable(err)
		},
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context, attempt int) error

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config *RetryConfig
	logger *slog.Logger
	rand   *mathRand.Rand
}

// generateSecureSeed generates a cryptographically secure seed for math/rand
func generateSecureSeed() int64 {
	var seed int64
	err := binary.Read(rand.Reader, binary.BigEndian, &seed)
	if err != nil {
		// Fallback to time-based seed if crypto/rand fails
		return time.Now().UnixNano()
	}
	return seed
}

// NewRetryer creates a new retryer with the given configuration
func NewRetryer(config *RetryConfig, logger *slog.Logger) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &Retryer{
		config: config,
		logger: logger,
		// Use crypto/rand for seeding to satisfy security requirements
		rand: mathRand.New(mathRand.NewSource(generateSecureSeed())), //nolint:gosec // Using crypto/rand for seed
	}
}

// Execute runs the operation with retry logic
func (r *Retryer) Execute(ctx context.Context, operation RetryableOperation) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return NewError(ErrCodeTimeout).
				WithCategory(CategoryTimeoutError).
				WithSeverity(SeverityMedium).
				WithMessage("Operation canceled").
				WithCause(err).
				Build()
		}

		// Execute the operation
		err := operation(ctx, attempt)
		if err == nil {
			// Success - log if we had previous failures
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					"attempt", attempt,
					"total_attempts", r.config.MaxAttempts)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			r.logger.Debug("Error is not retryable, stopping retry attempts",
				"error", err,
				"attempt", attempt)
			return lastErr
		}

		// Don't wait after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay for next attempt
		delay := r.calculateDelay(attempt)

		r.logger.Warn("Operation failed, retrying",
			"error", err,
			"attempt", attempt,
			"max_attempts", r.config.MaxAttempts,
			"delay_ms", delay.Milliseconds())

		// Wait before retrying
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return NewError(ErrCodeTimeout).
				WithCategory(CategoryTimeoutError).
				WithSeverity(SeverityMedium).
				WithMessage("Operation canceled during retry delay").
				WithCause(ctx.Err()).
				Build()
		case <-timer.C:
			// Continue to next attempt
		}
	}

	// All attempts failed
	r.logger.Error("Operation failed after all retry attempts",
		"error", lastErr,
		"total_attempts", r.config.MaxAttempts)

	// Wrap in processing failed error if not already a ServiceError
	if _, ok := lastErr.(*ServiceError); !ok {
		return NewError(ErrCodeProcessingFailed).
			WithCategory(CategoryServerError).
			WithSeverity(SeverityHigh).
			WithMessage("Operation failed after retries").
			WithCause(lastErr).
			WithContext("attempts", r.config.MaxAttempts).
			Build()
	}

	return lastErr
}

// calculateDelay calculates the delay for the next retry attempt
func (r *Retryer) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: delay = initial_delay * (multiplier ^ (attempt - 1))
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter if enabled (±25% random variation)
	if r.config.Jitter {
		jitterRange := delay * jitterPercentage
		jitter := (r.rand.Float64() - jitterOffset) * jitterMultiplier * jitterRange // -25% to +25%
		delay += jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = float64(r.config.InitialDelay)
	}

	return time.Duration(delay)
}

// isRetryable determines if an error should be retried
func (r *Retryer) isRetryable(err error) bool {
	if r.config.RetryableFunc != nil {
		return r.config.RetryableFunc(err)
	}
	return isDefaultRetryable(err)
}

// isDefaultRetryable provides default retry logic for common error patterns
func isDefaultRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network errors are generally retryable
	retryablePatterns := []string{
		"connection refused",
		"connection timeout",
		"network unreachable",
		"temporary failure",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"rate limit",
		"429", // HTTP 429 Too Many Requests
		"502", // HTTP 502 Bad Gateway
		"503", // HTTP 503 Service Unavailable
		"504", // HTTP 504 Gateway Timeout
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	// Non-retryable errors
	nonRetryablePatterns := []string{
		"400", // HTTP 400 Bad Request
		"401", // HTTP 401 Unauthorized
		"403", // HTTP 403 Forbidden
		"404", // HTTP 404 Not Found
		"invalid json",
		"malformed",
		"unauthorized",
		"forbidden",
		"not found",
	}

	for _, pattern := range nonRetryablePatterns {
		if contains(errStr, pattern) {
			return false
		}
	}

	// Default to not retryable for unknown errors
	return false
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// StateClosed indicates normal operation - requests pass through
	StateClosed CircuitBreakerState = iota
	// StateOpen indicates failure mode - requests are rejected
	StateOpen
	// StateHalfOpen indicates testing mode - limited requests allowed to test recovery
	StateHalfOpen
)

// String returns string representation of circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	MonitoringWindow time.Duration `json:"monitoring_window"`
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: defaultFailureThreshold, // Open after consecutive failures
		SuccessThreshold: defaultSuccessThreshold, // Close after consecutive successes in half-open
		Timeout:          defaultCircuitTimeout,   // Stay open timeout
		MonitoringWindow: defaultMonitoringWindow, // Monitor failures window
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          *CircuitBreakerConfig
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	stateChangeTime time.Time
	logger          *slog.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *slog.Logger) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		stateChangeTime: time.Now(),
		logger:          logger,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation RetryableOperation) error {
	// Check if circuit is open
	if cb.state == StateOpen {
		if time.Since(cb.stateChangeTime) < cb.config.Timeout {
			return NewError(ErrCodeCircuitBreakerOpen).
				WithCategory(CategoryRetryableError).
				WithSeverity(SeverityHigh).
				WithMessage("Circuit breaker is open").
				WithDetails(fmt.Sprintf("Circuit opened due to failures, retry after %v",
					cb.config.Timeout-time.Since(cb.stateChangeTime))).
				WithRetryAfter(cb.config.Timeout - time.Since(cb.stateChangeTime)).
				Build()
		}
		// Timeout elapsed, transition to half-open
		cb.transitionTo(StateHalfOpen)
	}

	// Execute operation
	err := operation(ctx, 1)

	// Record result
	cb.recordResult(err)

	return err
}

// recordResult records the result of an operation and updates circuit breaker state
func (cb *CircuitBreaker) recordResult(err error) {
	now := time.Now()

	if err != nil {
		cb.onFailure(now)
	} else {
		cb.onSuccess(now)
	}
}

// onFailure handles operation failure
func (cb *CircuitBreaker) onFailure(now time.Time) {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = now

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	}
}

// onSuccess handles operation success
func (cb *CircuitBreaker) onSuccess(_ time.Time) {
	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// transitionTo transitions the circuit breaker to a new state
func (cb *CircuitBreaker) transitionTo(newState CircuitBreakerState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.stateChangeTime = time.Now()

	// Reset counters based on new state
	switch newState {
	case StateClosed:
		cb.failureCount = 0
		cb.successCount = 0
	case StateOpen:
		cb.successCount = 0
	case StateHalfOpen:
		cb.successCount = 0
	}

	cb.logger.Info("Circuit breaker state changed",
		"old_state", oldState.String(),
		"new_state", newState.String(),
		"failure_count", cb.failureCount,
		"success_count", cb.successCount)
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"state":             cb.state.String(),
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"last_failure_time": cb.lastFailureTime,
		"state_change_time": cb.stateChangeTime,
		"time_in_state":     time.Since(cb.stateChangeTime),
	}
}

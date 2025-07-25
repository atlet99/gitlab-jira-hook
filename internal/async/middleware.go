package async

import (
	"context"
	"log/slog"
	"time"
)

// JobMiddleware represents a middleware function that processes jobs
type JobMiddleware func(next JobHandler) JobHandler

// JobHandler represents a job processing function
type JobHandler func(ctx context.Context, job *Job) error

// MiddlewareChain represents a chain of middleware functions
type MiddlewareChain struct {
	middlewares []JobMiddleware
	handler     JobHandler
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(handler JobHandler) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]JobMiddleware, 0),
		handler:     handler,
	}
}

// Use adds a middleware to the chain
func (mc *MiddlewareChain) Use(middleware JobMiddleware) *MiddlewareChain {
	mc.middlewares = append(mc.middlewares, middleware)
	return mc
}

// Build creates the final handler with all middleware applied
func (mc *MiddlewareChain) Build() JobHandler {
	handler := mc.handler

	// Apply middleware in order (first added is executed first)
	for i := 0; i < len(mc.middlewares); i++ {
		handler = mc.middlewares[i](handler)
	}

	return handler
}

// LoggingMiddleware creates a middleware that logs job processing
func LoggingMiddleware(logger *slog.Logger) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			start := time.Now()

			logger.Info("Starting job processing",
				"job_id", job.ID,
				"priority", job.Priority,
				"event_type", job.Event.Type,
				"retry_count", job.RetryCount)

			err := next(ctx, job)

			duration := time.Since(start)

			if err != nil {
				logger.Error("Job processing failed",
					"job_id", job.ID,
					"event_type", job.Event.Type,
					"duration", duration,
					"error", err)
			} else {
				logger.Info("Job processing completed",
					"job_id", job.ID,
					"event_type", job.Event.Type,
					"duration", duration)
			}

			return err
		}
	}
}

// MetricsMiddleware creates a middleware that collects metrics
func MetricsMiddleware(metrics *JobMetrics) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			start := time.Now()

			metrics.IncJobsStarted(job.Priority)

			err := next(ctx, job)

			duration := time.Since(start)

			if err != nil {
				metrics.IncJobsFailed(job.Priority)
				metrics.ObserveJobDuration(job.Priority, duration, false)
			} else {
				metrics.IncJobsCompleted(job.Priority)
				metrics.ObserveJobDuration(job.Priority, duration, true)
			}

			return err
		}
	}
}

// RetryMiddleware creates a middleware that handles retries
func RetryMiddleware(maxRetries int, backoff BackoffStrategy) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					// Wait before retry
					delay := backoff.GetDelay(attempt)
					select {
					case <-time.After(delay):
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				err := next(ctx, job)
				if err == nil {
					return nil
				}

				lastErr = err

				// Don't retry if context is canceled
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}

			return lastErr
		}
	}
}

// TimeoutMiddleware creates a middleware that adds timeout to job processing
func TimeoutMiddleware(timeout time.Duration) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Create a channel to receive the result
			resultCh := make(chan error, 1)

			go func() {
				resultCh <- next(ctx, job)
			}()

			select {
			case err := <-resultCh:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// CircuitBreakerMiddleware creates a middleware that implements circuit breaker pattern
func CircuitBreakerMiddleware(
	failureThreshold int,
	timeout time.Duration,
	logger *slog.Logger,
) JobMiddleware {
	breaker := NewCircuitBreaker(failureThreshold, timeout, logger)

	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			return breaker.Execute(func() error {
				return next(ctx, job)
			})
		}
	}
}

// TracingMiddleware creates a middleware that adds tracing to job processing
func TracingMiddleware(tracer JobTracer) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			span := tracer.StartSpan(ctx, "job.process", job.ID)
			defer span.Finish()

			span.SetTag("job.priority", job.Priority)
			span.SetTag("job.event_type", job.Event.Type)
			span.SetTag("job.retry_count", job.RetryCount)

			err := next(ctx, job)

			if err != nil {
				span.SetError(err)
			}

			return err
		}
	}
}

// RateLimitingMiddleware creates a middleware that limits job processing rate
func RateLimitingMiddleware(limiter RateLimiter) JobMiddleware {
	return func(next JobHandler) JobHandler {
		return func(ctx context.Context, job *Job) error {
			if err := limiter.Allow(ctx, job.Priority); err != nil {
				return err
			}

			return next(ctx, job)
		}
	}
}

// JobMetrics represents metrics collection for jobs
type JobMetrics struct {
	// Implementation would depend on the metrics library used
	// For now, we'll use a simple interface
}

// IncJobsStarted increments the counter for started jobs
func (jm *JobMetrics) IncJobsStarted(_ JobPriority) {
	// Implementation
}

// IncJobsCompleted increments the counter for completed jobs
func (jm *JobMetrics) IncJobsCompleted(_ JobPriority) {
	// Implementation
}

// IncJobsFailed increments the counter for failed jobs
func (jm *JobMetrics) IncJobsFailed(_ JobPriority) {
	// Implementation
}

// ObserveJobDuration observes job processing duration
func (jm *JobMetrics) ObserveJobDuration(_ JobPriority, _ time.Duration, _ bool) {
	// Implementation
}

// BackoffStrategy defines the interface for retry backoff strategies
type BackoffStrategy interface {
	GetDelay(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff strategy
type ExponentialBackoff struct {
	baseDelay  time.Duration
	multiplier float64
	maxDelay   time.Duration
}

// NewExponentialBackoff creates a new exponential backoff strategy
func NewExponentialBackoff(baseDelay time.Duration, multiplier float64, maxDelay time.Duration) *ExponentialBackoff {
	return &ExponentialBackoff{
		baseDelay:  baseDelay,
		multiplier: multiplier,
		maxDelay:   maxDelay,
	}
}

// GetDelay returns the delay for the given attempt
func (eb *ExponentialBackoff) GetDelay(attempt int) time.Duration {
	delay := eb.baseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * eb.multiplier)
		if delay > eb.maxDelay {
			delay = eb.maxDelay
			break
		}
	}
	return delay
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	failureThreshold int
	timeout          time.Duration
	logger           *slog.Logger
	state            CircuitBreakerState
	failureCount     int
	lastFailureTime  time.Time
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

// Circuit breaker states
const (
	StateClosed   CircuitBreakerState = iota // Circuit breaker is closed (normal operation)
	StateOpen                                // Circuit breaker is open (blocking requests)
	StateHalfOpen                            // Circuit breaker is half-open (testing recovery)
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, timeout time.Duration, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		logger:           logger,
		state:            StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.logger.Info("Circuit breaker transitioning to half-open")
		} else {
			return ErrCircuitBreakerOpen
		}
	case StateHalfOpen:
		// Allow one request to test if the service is back up
	}

	err := fn()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}

	return err
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen || cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		cb.logger.Warn("Circuit breaker opened", "failure_count", cb.failureCount)
	}
}

func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount = 0
	cb.state = StateClosed
	if cb.state == StateHalfOpen {
		cb.logger.Info("Circuit breaker closed")
	}
}

// JobTracer defines the interface for job tracing
type JobTracer interface {
	StartSpan(ctx context.Context, operation string, jobID string) JobSpan
}

// JobSpan represents a tracing span
type JobSpan interface {
	SetTag(key string, value interface{})
	SetError(err error)
	Finish()
}

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, priority JobPriority) error
}

// ErrCircuitBreakerOpen is returned when the circuit breaker is open
var ErrCircuitBreakerOpen = NewError(ErrCodeCircuitBreakerOpen, "circuit breaker is open", nil)

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct{}

func (e *CircuitBreakerError) Error() string {
	return "circuit breaker is open"
}

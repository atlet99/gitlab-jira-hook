// Package async provides asynchronous webhook processing functionality.
package async

import (
	"errors"
	"fmt"
	"time"
)

// AsyncError represents a base error for async operations
type AsyncError struct {
	Code    string
	Message string
	Err     error
	Context map[string]interface{}
}

// Error returns the error message
func (e *AsyncError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AsyncError) Unwrap() error {
	return e.Err
}

// Is checks if the error is of a specific type
func (e *AsyncError) Is(target error) bool {
	if target == nil {
		return false
	}
	if t, ok := target.(*AsyncError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext adds context to the error
func (e *AsyncError) WithContext(key string, value interface{}) *AsyncError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// GetContext returns the error context
func (e *AsyncError) GetContext() map[string]interface{} {
	return e.Context
}

// NewAsyncError creates a new async error
func NewAsyncError(code, message string, err error) *AsyncError {
	return &AsyncError{
		Code:    code,
		Message: message,
		Err:     err,
		Context: make(map[string]interface{}),
	}
}

// WrapAsyncError wraps an existing error with async context
func WrapAsyncError(err error, code, message string) *AsyncError {
	return NewAsyncError(code, message, err)
}

// JobError represents an error that occurred during job processing
type JobError struct {
	*AsyncError
	JobID       string
	EventType   string
	Priority    JobPriority
	RetryCount  int
	AttemptedAt time.Time
}

// NewJobError creates a new job error
func NewJobError(jobID, eventType string, priority JobPriority, retryCount int, err error) *JobError {
	return &JobError{
		AsyncError:  NewAsyncError("JOB_ERROR", "job processing failed", err),
		JobID:       jobID,
		EventType:   eventType,
		Priority:    priority,
		RetryCount:  retryCount,
		AttemptedAt: time.Now(),
	}
}

// QueueError represents an error that occurred in the job queue
type QueueError struct {
	*AsyncError
	QueueType string
	QueueSize int
	Capacity  int
}

// NewQueueError creates a new queue error
func NewQueueError(queueType string, queueSize, capacity int, err error) *QueueError {
	return &QueueError{
		AsyncError: NewAsyncError("QUEUE_ERROR", "queue operation failed", err),
		QueueType:  queueType,
		QueueSize:  queueSize,
		Capacity:   capacity,
	}
}

// WorkerPoolError represents an error that occurred in the worker pool
type WorkerPoolError struct {
	*AsyncError
	WorkerID    int
	WorkerCount int
	ActiveJobs  int
}

// NewWorkerPoolError creates a new worker pool error
func NewWorkerPoolError(workerID, workerCount, activeJobs int, err error) *WorkerPoolError {
	return &WorkerPoolError{
		AsyncError:  NewAsyncError("WORKER_POOL_ERROR", "worker pool operation failed", err),
		WorkerID:    workerID,
		WorkerCount: workerCount,
		ActiveJobs:  activeJobs,
	}
}

// MiddlewareError represents an error that occurred in middleware
type MiddlewareError struct {
	*AsyncError
	MiddlewareName string
	MiddlewareType string
}

// NewMiddlewareError creates a new middleware error
func NewMiddlewareError(middlewareName, middlewareType string, err error) *MiddlewareError {
	return &MiddlewareError{
		AsyncError:     NewAsyncError("MIDDLEWARE_ERROR", "middleware operation failed", err),
		MiddlewareName: middlewareName,
		MiddlewareType: middlewareType,
	}
}

// ScalingError represents an error that occurred during scaling operations
type ScalingError struct {
	*AsyncError
	ScalingType string
	FromWorkers int
	ToWorkers   int
	Reason      string
}

// NewScalingError creates a new scaling error
func NewScalingError(scalingType string, fromWorkers, toWorkers int, reason string, err error) *ScalingError {
	return &ScalingError{
		AsyncError:  NewAsyncError("SCALING_ERROR", "scaling operation failed", err),
		ScalingType: scalingType,
		FromWorkers: fromWorkers,
		ToWorkers:   toWorkers,
		Reason:      reason,
	}
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	*AsyncError
	Timeout     time.Duration
	Operation   string
	StartedAt   time.Time
	CompletedAt time.Time
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation string, timeout time.Duration, startedAt time.Time, err error) *TimeoutError {
	return &TimeoutError{
		AsyncError:  NewAsyncError("TIMEOUT_ERROR", "operation timed out", err),
		Timeout:     timeout,
		Operation:   operation,
		StartedAt:   startedAt,
		CompletedAt: time.Now(),
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	*AsyncError
	MaxRetries   int
	CurrentRetry int
	RetryDelay   time.Duration
	Retryable    bool
	RetryAfter   time.Time
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, maxRetries, currentRetry int, retryDelay time.Duration, retryable bool) *RetryableError {
	return &RetryableError{
		AsyncError:   NewAsyncError("RETRYABLE_ERROR", "retryable operation failed", err),
		MaxRetries:   maxRetries,
		CurrentRetry: currentRetry,
		RetryDelay:   retryDelay,
		Retryable:    retryable,
		RetryAfter:   time.Now().Add(retryDelay),
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable
	}
	return false
}

// GetRetryCount returns the retry count from an error
func GetRetryCount(err error) int {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.CurrentRetry
	}
	return 0
}

// GetRetryDelay returns the retry delay from an error
func GetRetryDelay(err error) time.Duration {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.RetryDelay
	}
	return 0
}

// Predefined error codes
const (
	ErrCodeQueueFull           = "QUEUE_FULL"
	ErrCodeWorkerPoolStopped   = "WORKER_POOL_STOPPED"
	ErrCodeJobTimeout          = "JOB_TIMEOUT"
	ErrCodeNoJobsAvailable     = "NO_JOBS_AVAILABLE"
	ErrCodeJobProcessingFailed = "JOB_PROCESSING_FAILED"
	ErrCodeMiddlewareFailed    = "MIDDLEWARE_FAILED"
	ErrCodeScalingFailed       = "SCALING_FAILED"
	ErrCodeConfigurationError  = "CONFIGURATION_ERROR"
	ErrCodeValidationError     = "VALIDATION_ERROR"
	ErrCodeResourceExhausted   = "RESOURCE_EXHAUSTED"
	ErrCodeCircuitBreakerOpen  = "CIRCUIT_BREAKER_OPEN"
	ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
)

// Predefined errors
var (
	// ErrQueueFull is returned when the job queue is full
	ErrQueueFull = NewAsyncError(ErrCodeQueueFull, "job queue is full", nil)

	// ErrWorkerPoolStopped is returned when the worker pool is stopped
	ErrWorkerPoolStopped = NewAsyncError(ErrCodeWorkerPoolStopped, "worker pool is stopped", nil)

	// ErrJobTimeout is returned when a job processing times out
	ErrJobTimeout = NewAsyncError(ErrCodeJobTimeout, "job processing timeout", nil)

	// ErrNoJobsAvailable is returned when no jobs are available in the queue
	ErrNoJobsAvailable = NewAsyncError(ErrCodeNoJobsAvailable, "no jobs available", nil)

	// ErrJobProcessingFailed is returned when job processing fails
	ErrJobProcessingFailed = NewAsyncError(ErrCodeJobProcessingFailed, "job processing failed", nil)

	// ErrMiddlewareFailed is returned when middleware fails
	ErrMiddlewareFailed = NewAsyncError(ErrCodeMiddlewareFailed, "middleware failed", nil)

	// ErrScalingFailed is returned when scaling fails
	ErrScalingFailed = NewAsyncError(ErrCodeScalingFailed, "scaling failed", nil)

	// ErrConfigurationError is returned when configuration is invalid
	ErrConfigurationError = NewAsyncError(ErrCodeConfigurationError, "configuration error", nil)

	// ErrValidationError is returned when validation fails
	ErrValidationError = NewAsyncError(ErrCodeValidationError, "validation error", nil)

	// ErrResourceExhausted is returned when resources are exhausted
	ErrResourceExhausted = NewAsyncError(ErrCodeResourceExhausted, "resource exhausted", nil)

	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = NewAsyncError(ErrCodeRateLimitExceeded, "rate limit exceeded", nil)
)

// Error utilities
func IsQueueFull(err error) bool {
	return errors.Is(err, ErrQueueFull)
}

func IsWorkerPoolStopped(err error) bool {
	return errors.Is(err, ErrWorkerPoolStopped)
}

func IsJobTimeout(err error) bool {
	return errors.Is(err, ErrJobTimeout)
}

func IsNoJobsAvailable(err error) bool {
	return errors.Is(err, ErrNoJobsAvailable)
}

func IsJobProcessingFailed(err error) bool {
	return errors.Is(err, ErrJobProcessingFailed)
}

func IsMiddlewareFailed(err error) bool {
	return errors.Is(err, ErrMiddlewareFailed)
}

func IsScalingFailed(err error) bool {
	return errors.Is(err, ErrScalingFailed)
}

func IsConfigurationError(err error) bool {
	return errors.Is(err, ErrConfigurationError)
}

func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidationError)
}

func IsResourceExhausted(err error) bool {
	return errors.Is(err, ErrResourceExhausted)
}

func IsCircuitBreakerOpen(err error) bool {
	return errors.Is(err, ErrCircuitBreakerOpen)
}

func IsRateLimitExceeded(err error) bool {
	return errors.Is(err, ErrRateLimitExceeded)
}

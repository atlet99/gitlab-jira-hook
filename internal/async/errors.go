// Package async provides asynchronous webhook processing functionality.
package async

import (
	"errors"
	"fmt"
	"time"
)

// Error represents a base error for async operations
// Error represents an asynchronous operation error
type Error struct {
	Code    string
	Message string
	Err     error
	Context map[string]interface{}
}

// Error returns the error message
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the error is of a specific type
func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext adds context to the error
func (e *Error) WithContext(key string, value interface{}) *Error {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// GetContext returns the error context
func (e *Error) GetContext() map[string]interface{} {
	return e.Context
}

// NewError creates a new async error
func NewError(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
		Context: make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with async context
func WrapError(err error, code, message string) *Error {
	return NewError(code, message, err)
}

// JobError represents an error that occurred during job processing
type JobError struct {
	BaseError   *Error
	JobID       string
	EventType   string
	Priority    JobPriority
	RetryCount  int
	AttemptedAt time.Time
}

// Error returns the error message
func (e *JobError) Error() string {
	return e.BaseError.Error()
}

// Unwrap returns the underlying error
func (e *JobError) Unwrap() error {
	return e.BaseError.Unwrap()
}

// NewJobError creates a new job error
func NewJobError(jobID, eventType string, priority JobPriority, retryCount int, err error) *JobError {
	return &JobError{
		BaseError:   NewError("JOB_ERROR", "job processing failed", err),
		JobID:       jobID,
		EventType:   eventType,
		Priority:    priority,
		RetryCount:  retryCount,
		AttemptedAt: time.Now(),
	}
}

// QueueError represents an error that occurred in the job queue
type QueueError struct {
	BaseError *Error
	QueueType string
	QueueSize int
	Capacity  int
}

// Error returns the error message
func (e *QueueError) Error() string {
	return e.BaseError.Error()
}

// NewQueueError creates a new queue error
func NewQueueError(queueType string, queueSize, capacity int, err error) *QueueError {
	return &QueueError{
		BaseError: NewError("QUEUE_ERROR", "queue operation failed", err),
		QueueType: queueType,
		QueueSize: queueSize,
		Capacity:  capacity,
	}
}

// WorkerPoolError represents an error that occurred in the worker pool
type WorkerPoolError struct {
	BaseError   *Error
	WorkerID    int
	WorkerCount int
	ActiveJobs  int
}

// Error returns the error message
func (e *WorkerPoolError) Error() string {
	return e.BaseError.Error()
}

// NewWorkerPoolError creates a new worker pool error
func NewWorkerPoolError(workerID, workerCount, activeJobs int, err error) *WorkerPoolError {
	return &WorkerPoolError{
		BaseError:   NewError("WORKER_POOL_ERROR", "worker pool operation failed", err),
		WorkerID:    workerID,
		WorkerCount: workerCount,
		ActiveJobs:  activeJobs,
	}
}

// MiddlewareError represents an error that occurred in middleware
type MiddlewareError struct {
	BaseError      *Error
	MiddlewareName string
	MiddlewareType string
}

// Error returns the error message
func (e *MiddlewareError) Error() string {
	return e.BaseError.Error()
}

// NewMiddlewareError creates a new middleware error
func NewMiddlewareError(middlewareName, middlewareType string, err error) *MiddlewareError {
	return &MiddlewareError{
		BaseError:      NewError("MIDDLEWARE_ERROR", "middleware operation failed", err),
		MiddlewareName: middlewareName,
		MiddlewareType: middlewareType,
	}
}

// ScalingError represents an error that occurred during scaling operations
type ScalingError struct {
	BaseError   *Error
	ScalingType string
	FromWorkers int
	ToWorkers   int
	Reason      string
}

// Error returns the error message
func (e *ScalingError) Error() string {
	return e.BaseError.Error()
}

// NewScalingError creates a new scaling error
func NewScalingError(scalingType string, fromWorkers, toWorkers int, reason string, err error) *ScalingError {
	return &ScalingError{
		BaseError:   NewError("SCALING_ERROR", "scaling operation failed", err),
		ScalingType: scalingType,
		FromWorkers: fromWorkers,
		ToWorkers:   toWorkers,
		Reason:      reason,
	}
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	BaseError   *Error
	Timeout     time.Duration
	Operation   string
	StartedAt   time.Time
	CompletedAt time.Time
}

// Error returns the error message
func (e *TimeoutError) Error() string {
	return e.BaseError.Error()
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation string, timeout time.Duration, startedAt time.Time, err error) *TimeoutError {
	return &TimeoutError{
		BaseError:   NewError("TIMEOUT_ERROR", "operation timed out", err),
		Timeout:     timeout,
		Operation:   operation,
		StartedAt:   startedAt,
		CompletedAt: time.Now(),
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	BaseError    *Error
	MaxRetries   int
	CurrentRetry int
	RetryDelay   time.Duration
	Retryable    bool
	RetryAfter   time.Time
}

// Error returns the error message
func (e *RetryableError) Error() string {
	return e.BaseError.Error()
}

// NewRetryableError creates a new retryable error
func NewRetryableError(
	err error,
	maxRetries, currentRetry int,
	retryDelay time.Duration,
	retryable bool,
) *RetryableError {
	return &RetryableError{
		BaseError:    NewError("RETRYABLE_ERROR", "retryable operation failed", err),
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
	ErrQueueFull = NewError(ErrCodeQueueFull, "job queue is full", nil)

	// ErrWorkerPoolStopped is returned when the worker pool is stopped
	ErrWorkerPoolStopped = NewError(ErrCodeWorkerPoolStopped, "worker pool is stopped", nil)

	// ErrJobTimeout is returned when a job processing times out
	ErrJobTimeout = NewError(ErrCodeJobTimeout, "job processing timeout", nil)

	// ErrNoJobsAvailable is returned when no jobs are available in the queue
	ErrNoJobsAvailable = NewError(ErrCodeNoJobsAvailable, "no jobs available", nil)

	// ErrJobProcessingFailed is returned when job processing fails
	ErrJobProcessingFailed = NewError(ErrCodeJobProcessingFailed, "job processing failed", nil)

	// ErrMiddlewareFailed is returned when middleware fails
	ErrMiddlewareFailed = NewError(ErrCodeMiddlewareFailed, "middleware failed", nil)

	// ErrScalingFailed is returned when scaling fails
	ErrScalingFailed = NewError(ErrCodeScalingFailed, "scaling failed", nil)

	// ErrConfigurationError is returned when configuration is invalid
	ErrConfigurationError = NewError(ErrCodeConfigurationError, "configuration error", nil)

	// ErrValidationError is returned when validation fails
	ErrValidationError = NewError(ErrCodeValidationError, "validation error", nil)

	// ErrResourceExhausted is returned when resources are exhausted
	ErrResourceExhausted = NewError(ErrCodeResourceExhausted, "resource exhausted", nil)

	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = NewError(ErrCodeRateLimitExceeded, "rate limit exceeded", nil)
)

// IsQueueFull checks if the error indicates that the job queue is full
func IsQueueFull(err error) bool {
	return errors.Is(err, ErrQueueFull)
}

// IsWorkerPoolStopped checks if the error indicates that the worker pool is stopped
func IsWorkerPoolStopped(err error) bool {
	return errors.Is(err, ErrWorkerPoolStopped)
}

// IsJobTimeout checks if the error indicates that a job processing timed out
func IsJobTimeout(err error) bool {
	return errors.Is(err, ErrJobTimeout)
}

// IsNoJobsAvailable checks if the error indicates that no jobs are available
func IsNoJobsAvailable(err error) bool {
	return errors.Is(err, ErrNoJobsAvailable)
}

// IsJobProcessingFailed checks if the error indicates that job processing failed
func IsJobProcessingFailed(err error) bool {
	return errors.Is(err, ErrJobProcessingFailed)
}

// IsMiddlewareFailed checks if the error indicates that middleware failed
func IsMiddlewareFailed(err error) bool {
	return errors.Is(err, ErrMiddlewareFailed)
}

// IsScalingFailed checks if the error indicates that scaling failed
func IsScalingFailed(err error) bool {
	return errors.Is(err, ErrScalingFailed)
}

// IsConfigurationError checks if the error indicates a configuration error
func IsConfigurationError(err error) bool {
	return errors.Is(err, ErrConfigurationError)
}

// IsValidationError checks if the error indicates a validation error
func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidationError)
}

// IsResourceExhausted checks if the error indicates that resources are exhausted
func IsResourceExhausted(err error) bool {
	return errors.Is(err, ErrResourceExhausted)
}

// IsCircuitBreakerOpen checks if the error indicates that the circuit breaker is open
func IsCircuitBreakerOpen(err error) bool {
	return errors.Is(err, ErrCircuitBreakerOpen)
}

// IsRateLimitExceeded checks if the error indicates that rate limit was exceeded
func IsRateLimitExceeded(err error) bool {
	return errors.Is(err, ErrRateLimitExceeded)
}

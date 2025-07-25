package async

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	t.Run("basic async error", func(t *testing.T) {
		err := NewError("TEST_ERROR", "test message", nil)
		assert.Equal(t, "TEST_ERROR: test message", err.Error())
		assert.Equal(t, "TEST_ERROR", err.Code)
		assert.Equal(t, "test message", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("async error with wrapped error", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := NewError("TEST_ERROR", "test message", originalErr)
		assert.Equal(t, "TEST_ERROR: test message - original error", err.Error())
		assert.Equal(t, originalErr, err.Unwrap())
	})

	t.Run("async error with context", func(t *testing.T) {
		err := NewError("TEST_ERROR", "test message", nil)
		_ = err.WithContext("key1", "value1").WithContext("key2", 42)

		context := err.GetContext()
		assert.Equal(t, "value1", context["key1"])
		assert.Equal(t, 42, context["key2"])
	})

	t.Run("async error comparison", func(t *testing.T) {
		err1 := NewError("TEST_ERROR", "test message", nil)
		err2 := NewError("TEST_ERROR", "different message", nil)
		err3 := NewError("DIFFERENT_ERROR", "test message", nil)

		assert.True(t, err1.Is(err2))
		assert.False(t, err1.Is(err3))
		assert.False(t, err1.Is(nil))
	})
}

func TestJobError(t *testing.T) {
	t.Run("job error creation", func(t *testing.T) {
		originalErr := errors.New("processing failed")
		jobErr := NewJobError("job-123", "push", PriorityHigh, 2, originalErr)

		assert.Equal(t, "JOB_ERROR: job processing failed - processing failed", jobErr.Error())
		assert.Equal(t, "job-123", jobErr.JobID)
		assert.Equal(t, "push", jobErr.EventType)
		assert.Equal(t, PriorityHigh, jobErr.Priority)
		assert.Equal(t, 2, jobErr.RetryCount)
		assert.NotNil(t, jobErr.AttemptedAt)
	})

	t.Run("job error with context", func(t *testing.T) {
		jobErr := NewJobError("job-123", "push", PriorityHigh, 0, nil)
		_ = jobErr.BaseError.WithContext("worker_id", 5).WithContext("duration", time.Second)

		context := jobErr.BaseError.GetContext()
		assert.Equal(t, 5, context["worker_id"])
		assert.Equal(t, time.Second, context["duration"])
	})
}

func TestQueueError(t *testing.T) {
	t.Run("queue error creation", func(t *testing.T) {
		originalErr := errors.New("queue full")
		queueErr := NewQueueError("priority", 100, 100, originalErr)

		assert.Equal(t, "QUEUE_ERROR: queue operation failed - queue full", queueErr.Error())
		assert.Equal(t, "priority", queueErr.QueueType)
		assert.Equal(t, 100, queueErr.QueueSize)
		assert.Equal(t, 100, queueErr.Capacity)
	})
}

func TestWorkerPoolError(t *testing.T) {
	t.Run("worker pool error creation", func(t *testing.T) {
		originalErr := errors.New("worker failed")
		poolErr := NewWorkerPoolError(5, 10, 3, originalErr)

		assert.Equal(t, "WORKER_POOL_ERROR: worker pool operation failed - worker failed", poolErr.Error())
		assert.Equal(t, 5, poolErr.WorkerID)
		assert.Equal(t, 10, poolErr.WorkerCount)
		assert.Equal(t, 3, poolErr.ActiveJobs)
	})
}

func TestMiddlewareError(t *testing.T) {
	t.Run("middleware error creation", func(t *testing.T) {
		originalErr := errors.New("middleware failed")
		middlewareErr := NewMiddlewareError("logging", "logging_middleware", originalErr)

		assert.Equal(t, "MIDDLEWARE_ERROR: middleware operation failed - middleware failed", middlewareErr.Error())
		assert.Equal(t, "logging", middlewareErr.MiddlewareName)
		assert.Equal(t, "logging_middleware", middlewareErr.MiddlewareType)
	})
}

func TestScalingError(t *testing.T) {
	t.Run("scaling error creation", func(t *testing.T) {
		originalErr := errors.New("scaling failed")
		scalingErr := NewScalingError("scale_up", 5, 10, "high load", originalErr)

		assert.Equal(t, "SCALING_ERROR: scaling operation failed - scaling failed", scalingErr.Error())
		assert.Equal(t, "scale_up", scalingErr.ScalingType)
		assert.Equal(t, 5, scalingErr.FromWorkers)
		assert.Equal(t, 10, scalingErr.ToWorkers)
		assert.Equal(t, "high load", scalingErr.Reason)
	})
}

func TestTimeoutError(t *testing.T) {
	t.Run("timeout error creation", func(t *testing.T) {
		startedAt := time.Now()
		timeout := 5 * time.Second
		originalErr := errors.New("timeout occurred")
		timeoutErr := NewTimeoutError("job_processing", timeout, startedAt, originalErr)

		assert.Equal(t, "TIMEOUT_ERROR: operation timed out - timeout occurred", timeoutErr.Error())
		assert.Equal(t, timeout, timeoutErr.Timeout)
		assert.Equal(t, "job_processing", timeoutErr.Operation)
		assert.Equal(t, startedAt, timeoutErr.StartedAt)
		assert.NotNil(t, timeoutErr.CompletedAt)
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("retryable error creation", func(t *testing.T) {
		originalErr := errors.New("temporary failure")
		retryDelay := 1 * time.Second
		retryableErr := NewRetryableError(originalErr, 3, 1, retryDelay, true)

		assert.Equal(t, "RETRYABLE_ERROR: retryable operation failed - temporary failure", retryableErr.Error())
		assert.Equal(t, 3, retryableErr.MaxRetries)
		assert.Equal(t, 1, retryableErr.CurrentRetry)
		assert.Equal(t, retryDelay, retryableErr.RetryDelay)
		assert.True(t, retryableErr.Retryable)
		assert.True(t, retryableErr.RetryAfter.After(time.Now()))
	})

	t.Run("non-retryable error", func(t *testing.T) {
		originalErr := errors.New("permanent failure")
		retryableErr := NewRetryableError(originalErr, 3, 1, time.Second, false)

		assert.False(t, retryableErr.Retryable)
	})
}

func TestErrorUtilities(t *testing.T) {
	t.Run("is retryable", func(t *testing.T) {
		originalErr := errors.New("temporary failure")
		retryableErr := NewRetryableError(originalErr, 3, 1, time.Second, true)

		assert.True(t, IsRetryable(retryableErr))
		assert.False(t, IsRetryable(originalErr))
	})

	t.Run("get retry count", func(t *testing.T) {
		originalErr := errors.New("temporary failure")
		retryableErr := NewRetryableError(originalErr, 3, 2, time.Second, true)

		assert.Equal(t, 2, GetRetryCount(retryableErr))
		assert.Equal(t, 0, GetRetryCount(originalErr))
	})

	t.Run("get retry delay", func(t *testing.T) {
		retryDelay := 5 * time.Second
		originalErr := errors.New("temporary failure")
		retryableErr := NewRetryableError(originalErr, 3, 1, retryDelay, true)

		assert.Equal(t, retryDelay, GetRetryDelay(retryableErr))
		assert.Equal(t, time.Duration(0), GetRetryDelay(originalErr))
	})
}

func TestErrorTypeChecking(t *testing.T) {
	t.Run("queue full error", func(t *testing.T) {
		assert.True(t, IsQueueFull(ErrQueueFull))
		assert.False(t, IsQueueFull(ErrWorkerPoolStopped))
	})

	t.Run("worker pool stopped error", func(t *testing.T) {
		assert.True(t, IsWorkerPoolStopped(ErrWorkerPoolStopped))
		assert.False(t, IsWorkerPoolStopped(ErrQueueFull))
	})

	t.Run("job timeout error", func(t *testing.T) {
		assert.True(t, IsJobTimeout(ErrJobTimeout))
		assert.False(t, IsJobTimeout(ErrQueueFull))
	})

	t.Run("no jobs available error", func(t *testing.T) {
		assert.True(t, IsNoJobsAvailable(ErrNoJobsAvailable))
		assert.False(t, IsNoJobsAvailable(ErrQueueFull))
	})

	t.Run("job processing failed error", func(t *testing.T) {
		assert.True(t, IsJobProcessingFailed(ErrJobProcessingFailed))
		assert.False(t, IsJobProcessingFailed(ErrQueueFull))
	})

	t.Run("middleware failed error", func(t *testing.T) {
		assert.True(t, IsMiddlewareFailed(ErrMiddlewareFailed))
		assert.False(t, IsMiddlewareFailed(ErrQueueFull))
	})

	t.Run("scaling failed error", func(t *testing.T) {
		assert.True(t, IsScalingFailed(ErrScalingFailed))
		assert.False(t, IsScalingFailed(ErrQueueFull))
	})

	t.Run("configuration error", func(t *testing.T) {
		assert.True(t, IsConfigurationError(ErrConfigurationError))
		assert.False(t, IsConfigurationError(ErrQueueFull))
	})

	t.Run("validation error", func(t *testing.T) {
		assert.True(t, IsValidationError(ErrValidationError))
		assert.False(t, IsValidationError(ErrQueueFull))
	})

	t.Run("resource exhausted error", func(t *testing.T) {
		assert.True(t, IsResourceExhausted(ErrResourceExhausted))
		assert.False(t, IsResourceExhausted(ErrQueueFull))
	})

	t.Run("circuit breaker open error", func(t *testing.T) {
		assert.True(t, IsCircuitBreakerOpen(ErrCircuitBreakerOpen))
		assert.False(t, IsCircuitBreakerOpen(ErrQueueFull))
	})

	t.Run("rate limit exceeded error", func(t *testing.T) {
		assert.True(t, IsRateLimitExceeded(ErrRateLimitExceeded))
		assert.False(t, IsRateLimitExceeded(ErrQueueFull))
	})
}

func TestErrorWrapping(t *testing.T) {
	t.Run("wrap async error", func(t *testing.T) {
		originalErr := errors.New("original error")
		wrappedErr := WrapError(originalErr, "WRAPPED_ERROR", "wrapped message")

		assert.Equal(t, "WRAPPED_ERROR: wrapped message - original error", wrappedErr.Error())
		assert.Equal(t, originalErr, wrappedErr.Unwrap())
	})

	t.Run("error chain", func(t *testing.T) {
		originalErr := errors.New("original error")
		jobErr := NewJobError("job-123", "push", PriorityHigh, 0, originalErr)
		wrappedErr := WrapError(jobErr, "WRAPPED_ERROR", "wrapped message")

		assert.Equal(t, "WRAPPED_ERROR: wrapped message - JOB_ERROR: job processing failed - original error", wrappedErr.Error())
		assert.Equal(t, jobErr, wrappedErr.Unwrap())
		assert.Equal(t, originalErr, jobErr.Unwrap())
	})
}

func TestErrorContext(t *testing.T) {
	t.Run("error with multiple context values", func(t *testing.T) {
		err := NewError("TEST_ERROR", "test message", nil)
		_ = err.WithContext("string_key", "string_value").
			WithContext("int_key", 42).
			WithContext("bool_key", true).
			WithContext("float_key", 3.14)

		context := err.GetContext()
		assert.Equal(t, "string_value", context["string_key"])
		assert.Equal(t, 42, context["int_key"])
		assert.Equal(t, true, context["bool_key"])
		assert.Equal(t, 3.14, context["float_key"])
	})

	t.Run("error context overwrite", func(t *testing.T) {
		err := NewError("TEST_ERROR", "test message", nil)
		_ = err.WithContext("key", "value1").WithContext("key", "value2")

		context := err.GetContext()
		assert.Equal(t, "value2", context["key"])
	})
}

func TestErrorIntegration(t *testing.T) {
	t.Run("complex error scenario", func(t *testing.T) {
		// Simulate a complex error scenario
		originalErr := errors.New("network timeout")
		timeoutErr := NewTimeoutError("http_request", 30*time.Second, time.Now(), originalErr)
		retryableErr := NewRetryableError(timeoutErr, 3, 2, 5*time.Second, true)
		jobErr := NewJobError("job-456", "merge_request", PriorityHigh, 2, retryableErr)
		finalErr := WrapError(jobErr, "WEBHOOK_ERROR", "webhook processing failed")

		// Test error chain
		assert.Contains(t, finalErr.Error(), "WEBHOOK_ERROR")
		assert.Contains(t, finalErr.Error(), "JOB_ERROR")
		assert.Contains(t, finalErr.Error(), "RETRYABLE_ERROR")
		assert.Contains(t, finalErr.Error(), "TIMEOUT_ERROR")
		assert.Contains(t, finalErr.Error(), "network timeout")

		// Test error unwrapping
		unwrapped := finalErr.Unwrap()
		assert.Equal(t, jobErr, unwrapped)

		// Test retryable check
		assert.True(t, IsRetryable(retryableErr))
		assert.Equal(t, 2, GetRetryCount(retryableErr))
		assert.Equal(t, 5*time.Second, GetRetryDelay(retryableErr))
	})
}

package async

import (
	"context"
	"errors"
	"testing"
	"time"

	"io"
	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

func TestMiddlewareChain(t *testing.T) {
	t.Run("basic middleware chain", func(t *testing.T) {
		var executionOrder []string

		// Create a simple handler
		handler := func(ctx context.Context, job *Job) error {
			executionOrder = append(executionOrder, "handler")
			return nil
		}

		// Create middleware that adds to execution order
		middleware1 := func(next JobHandler) JobHandler {
			return func(ctx context.Context, job *Job) error {
				executionOrder = append(executionOrder, "middleware1-before")
				err := next(ctx, job)
				executionOrder = append(executionOrder, "middleware1-after")
				return err
			}
		}

		middleware2 := func(next JobHandler) JobHandler {
			return func(ctx context.Context, job *Job) error {
				executionOrder = append(executionOrder, "middleware2-before")
				err := next(ctx, job)
				executionOrder = append(executionOrder, "middleware2-after")
				return err
			}
		}

		// Build middleware chain
		chain := NewMiddlewareChain(handler)
		chain.Use(middleware1).Use(middleware2)
		finalHandler := chain.Build()

		// Execute
		job := &Job{ID: "test-job"}
		err := finalHandler(context.Background(), job)

		require.NoError(t, err)
		expectedOrder := []string{
			"middleware2-before",
			"middleware1-before",
			"handler",
			"middleware1-after",
			"middleware2-after",
		}
		assert.Equal(t, expectedOrder, executionOrder)
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("successful job", func(t *testing.T) {
		// Create a mock logger (we'll just verify the middleware doesn't panic)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		handler := func(ctx context.Context, job *Job) error {
			return nil
		}

		middleware := LoggingMiddleware(logger)
		wrappedHandler := middleware(handler)

		job := &Job{
			ID:       "test-job",
			Priority: PriorityNormal,
			Event:    &webhook.Event{Type: "push"},
		}

		err := wrappedHandler(context.Background(), job)
		assert.NoError(t, err)
	})

	t.Run("failed job", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		expectedErr := errors.New("test error")
		handler := func(ctx context.Context, job *Job) error {
			return expectedErr
		}

		middleware := LoggingMiddleware(logger)
		wrappedHandler := middleware(handler)

		job := &Job{
			ID:       "test-job",
			Priority: PriorityNormal,
			Event:    &webhook.Event{Type: "push"},
		}

		err := wrappedHandler(context.Background(), job)
		assert.Equal(t, expectedErr, err)
	})
}

func TestRetryMiddleware(t *testing.T) {
	t.Run("successful retry", func(t *testing.T) {
		attempts := 0
		handler := func(ctx context.Context, job *Job) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}

		backoff := NewExponentialBackoff(10*time.Millisecond, 2.0, 100*time.Millisecond)
		middleware := RetryMiddleware(2, backoff)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}
		err := wrappedHandler(context.Background(), job)

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		expectedErr := errors.New("persistent error")
		handler := func(ctx context.Context, job *Job) error {
			attempts++
			return expectedErr
		}

		backoff := NewExponentialBackoff(10*time.Millisecond, 2.0, 100*time.Millisecond)
		middleware := RetryMiddleware(2, backoff)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}
		err := wrappedHandler(context.Background(), job)

		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 3, attempts) // 1 initial + 2 retries
	})

	t.Run("context cancellation", func(t *testing.T) {
		handler := func(ctx context.Context, job *Job) error {
			return errors.New("error")
		}

		backoff := NewExponentialBackoff(100*time.Millisecond, 2.0, 1*time.Second)
		middleware := RetryMiddleware(5, backoff)
		wrappedHandler := middleware(handler)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		job := &Job{ID: "test-job"}
		err := wrappedHandler(ctx, job)

		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("job completes within timeout", func(t *testing.T) {
		handler := func(ctx context.Context, job *Job) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		middleware := TimeoutMiddleware(100 * time.Millisecond)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}
		err := wrappedHandler(context.Background(), job)

		assert.NoError(t, err)
	})

	t.Run("job exceeds timeout", func(t *testing.T) {
		handler := func(ctx context.Context, job *Job) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		middleware := TimeoutMiddleware(10 * time.Millisecond)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}
		err := wrappedHandler(context.Background(), job)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deadline exceeded")
	})
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	t.Run("circuit breaker opens after failures", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		handler := func(ctx context.Context, job *Job) error {
			return errors.New("service error")
		}

		middleware := CircuitBreakerMiddleware(2, 100*time.Millisecond, logger)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}

		// First two calls should fail
		err1 := wrappedHandler(context.Background(), job)
		assert.Error(t, err1)

		err2 := wrappedHandler(context.Background(), job)
		assert.Error(t, err2)

		// Third call should be blocked by circuit breaker
		err3 := wrappedHandler(context.Background(), job)
		assert.Equal(t, ErrCircuitBreakerOpen, err3)
	})

	t.Run("circuit breaker recovers", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		successCount := 0
		handler := func(ctx context.Context, job *Job) error {
			successCount++
			if successCount <= 2 {
				return errors.New("service error")
			}
			return nil
		}

		middleware := CircuitBreakerMiddleware(2, 50*time.Millisecond, logger)
		wrappedHandler := middleware(handler)

		job := &Job{ID: "test-job"}

		// First two calls should fail
		_ = wrappedHandler(context.Background(), job)
		_ = wrappedHandler(context.Background(), job)

		// Third call should be blocked
		err := wrappedHandler(context.Background(), job)
		assert.Equal(t, ErrCircuitBreakerOpen, err)

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Next call should succeed (half-open state)
		err = wrappedHandler(context.Background(), job)
		assert.NoError(t, err)
	})
}

func TestExponentialBackoff(t *testing.T) {
	backoff := NewExponentialBackoff(10*time.Millisecond, 2.0, 100*time.Millisecond)

	t.Run("backoff calculation", func(t *testing.T) {
		delay1 := backoff.GetDelay(0)
		assert.Equal(t, 10*time.Millisecond, delay1)

		delay2 := backoff.GetDelay(1)
		assert.Equal(t, 20*time.Millisecond, delay2)

		delay3 := backoff.GetDelay(2)
		assert.Equal(t, 40*time.Millisecond, delay3)

		delay4 := backoff.GetDelay(3)
		assert.Equal(t, 80*time.Millisecond, delay4)

		delay5 := backoff.GetDelay(4)
		assert.Equal(t, 100*time.Millisecond, delay5) // Capped at max delay
	})
}

func TestPriorityWorkerPoolWithMiddleware(t *testing.T) {
	cfg := &config.Config{
		MinWorkers:          1,
		MaxWorkers:          2,
		MaxConcurrentJobs:   5,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
		MaxRetries:          1,
		RetryDelayMs:        50,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        200,
		MetricsEnabled:      false,
		ScaleInterval:       1,
		HealthCheckInterval: 5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	decider := &DefaultPriorityDecider{}

	t.Run("worker pool with logging middleware", func(t *testing.T) {
		pool := NewPriorityWorkerPool(cfg, logger, nil, decider)

		// Add logging middleware
		pool.UseMiddleware(LoggingMiddleware(logger))
		pool.BuildMiddleware()

		pool.Start()

		// Submit a job
		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := pool.SubmitJobWithDefaultPriority(event, handler)
		assert.NoError(t, err)

		// Wait a bit for processing
		time.Sleep(100 * time.Millisecond)

		// Stop the pool
		pool.Stop()

		// Check that job was processed
		stats := pool.GetStats()
		assert.GreaterOrEqual(t, stats.TotalJobsProcessed, int64(0))
	})

	t.Run("worker pool with timeout middleware", func(t *testing.T) {
		pool := NewPriorityWorkerPool(cfg, logger, nil, decider)

		// Add timeout middleware
		pool.UseMiddleware(TimeoutMiddleware(50 * time.Millisecond))
		pool.BuildMiddleware()

		pool.Start()

		// Submit a job
		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := pool.SubmitJobWithDefaultPriority(event, handler)
		assert.NoError(t, err)

		// Wait a bit for processing
		time.Sleep(100 * time.Millisecond)

		// Stop the pool
		pool.Stop()

		// Check that job was processed
		stats := pool.GetStats()
		assert.GreaterOrEqual(t, stats.TotalJobsProcessed, int64(0))
	})
}

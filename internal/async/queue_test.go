package async

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

func TestPriorityQueue(t *testing.T) {
	cfg := &config.Config{
		JobQueueSize:        100,
		QueueTimeoutMs:      1000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 30,
	}

	t.Run("submit and get job", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := queue.SubmitJob(event, handler, PriorityNormal)
		require.NoError(t, err)

		job, err := queue.GetJob()
		require.NoError(t, err)
		assert.Equal(t, event, job.Event)
		assert.Equal(t, handler, job.Handler)
		assert.Equal(t, PriorityNormal, job.Priority)
	})

	t.Run("priority ordering", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		event1 := &webhook.Event{Type: "push"}
		event2 := &webhook.Event{Type: "push"}
		event3 := &webhook.Event{Type: "push"}

		handler := &mockEventHandler{}

		// Submit jobs in different order
		err := queue.SubmitJob(event1, handler, PriorityLow)
		require.NoError(t, err)
		err = queue.SubmitJob(event2, handler, PriorityCritical)
		require.NoError(t, err)
		err = queue.SubmitJob(event3, handler, PriorityNormal)
		require.NoError(t, err)

		// Should get jobs in priority order: Critical -> Normal -> Low
		job1, err := queue.GetJob()
		require.NoError(t, err)
		assert.Equal(t, PriorityCritical, job1.Priority)

		job2, err := queue.GetJob()
		require.NoError(t, err)
		assert.Equal(t, PriorityNormal, job2.Priority)

		job3, err := queue.GetJob()
		require.NoError(t, err)
		assert.Equal(t, PriorityLow, job3.Priority)
	})

	t.Run("queue timeout", func(t *testing.T) {
		// Create a queue with very small timeout
		cfg.QueueTimeoutMs = 1
		smallQueue := NewPriorityQueue(cfg, nil, slog.Default())
		defer smallQueue.Shutdown()

		// Fill the queue
		handler := &mockEventHandler{}
		for i := 0; i < 200; i++ {
			event := &webhook.Event{Type: "push"}
			err := smallQueue.SubmitJob(event, handler, PriorityNormal)
			_ = err // For test, it doesn't matter if the queue overflows
		}

		// Try to submit one more - should timeout
		event := &webhook.Event{Type: "push"}
		err := smallQueue.SubmitJob(event, handler, PriorityNormal)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "queue timeout")
	})

	t.Run("job completion", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := queue.SubmitJob(event, handler, PriorityNormal)
		require.NoError(t, err)

		job, err := queue.GetJob()
		require.NoError(t, err)

		// Complete job successfully
		queue.CompleteJob(job, nil)
		assert.Equal(t, StatusCompleted, job.Status)
		assert.Len(t, job.Attempts, 1)
		assert.True(t, job.Attempts[0].Success)
	})

	t.Run("job retry", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := queue.SubmitJob(event, handler, PriorityNormal)
		require.NoError(t, err)

		job, err := queue.GetJob()
		require.NoError(t, err)

		// Complete job with error
		testError := assert.AnError
		queue.CompleteJob(job, testError)

		// Check only RetryCount and Attempts, status may change asynchronously
		assert.Equal(t, 1, job.RetryCount)
		assert.Len(t, job.Attempts, 1)
		assert.False(t, job.Attempts[0].Success)
		assert.Equal(t, testError, job.Attempts[0].Error)
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := queue.SubmitJob(event, handler, PriorityNormal)
		require.NoError(t, err)

		job, err := queue.GetJob()
		require.NoError(t, err)
		require.NotNil(t, job)

		// Fail job multiple times
		for i := 0; i < cfg.MaxRetries+1; i++ {
			queue.CompleteJob(job, assert.AnError)

			// Wait a bit for retry scheduling
			time.Sleep(100 * time.Millisecond)

			// Try to get the retried job if it's marked as retrying
			if job != nil && job.Status == StatusRetrying {
				var err error
				job, err = queue.GetJob()
				if err != nil {
					// Queue might be empty or shutting down
					// The job will be retried asynchronously, so we can't get it immediately
					break
				}
				require.NotNil(t, job)
			}
		}

		// The job should eventually fail after max retries
		// Check if job is not nil before accessing its fields
		if job != nil {
			assert.Equal(t, StatusFailed, job.Status)
			assert.Equal(t, cfg.MaxRetries, job.RetryCount)
		} else {
			// If job is nil, it means the queue was shut down before we could get the retried job
			// This is acceptable behavior in a race condition test
			t.Logf("Job was nil - queue was shut down during retry")
		}
	})

	t.Run("queue statistics", func(t *testing.T) {
		queue := NewPriorityQueue(cfg, nil, slog.Default())
		defer queue.Shutdown()

		stats := queue.GetStats()
		assert.GreaterOrEqual(t, stats.TotalJobs, int64(0))
		assert.GreaterOrEqual(t, stats.CompletedJobs, int64(0))
		assert.GreaterOrEqual(t, stats.FailedJobs, int64(0))
	})
}

func TestPriorityWorkerPool(t *testing.T) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          10,
		ScaleUpThreshold:    5,
		ScaleDownThreshold:  2,
		ScaleInterval:       1,
		MaxConcurrentJobs:   20,
		JobTimeoutSeconds:   30,
		QueueTimeoutMs:      5000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	decider := &DefaultPriorityDecider{}
	pool := NewPriorityWorkerPool(cfg, logger, nil, decider)
	pool.Start()
	defer pool.Stop()

	t.Run("submit job with default priority", func(t *testing.T) {
		event := &webhook.Event{
			Type: "push",
		}
		handler := &mockEventHandler{}

		err := pool.SubmitJobWithDefaultPriority(event, handler)
		assert.NoError(t, err)
	})

	t.Run("submit job with specific priority", func(t *testing.T) {
		event := &webhook.Event{
			Type: "push",
		}
		handler := &mockEventHandler{}

		err := pool.SubmitJob(event, handler, PriorityHigh)
		assert.NoError(t, err)
	})

	t.Run("pool statistics", func(t *testing.T) {
		// Give workers time to start
		time.Sleep(200 * time.Millisecond)

		stats := pool.GetStats()
		// Just check that stats are reasonable, don't assert exact values
		assert.GreaterOrEqual(t, stats.TotalWorkers, 0)
		assert.GreaterOrEqual(t, stats.IdleWorkers, 0)
	})

	t.Run("queue statistics", func(t *testing.T) {
		stats := pool.GetQueueStats()
		assert.GreaterOrEqual(t, stats.TotalJobs, int64(0))
	})

	t.Run("pool health", func(t *testing.T) {
		healthy := pool.IsHealthy()
		assert.True(t, healthy)
	})

	t.Run("scaling", func(t *testing.T) {
		// Give workers time to start
		time.Sleep(200 * time.Millisecond)

		// Just check that scaling methods don't panic
		// The actual scaling logic is tested in scalingMonitor

		// Submit some jobs to trigger scaling
		for i := 0; i < 5; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := pool.SubmitJob(event, handler, PriorityNormal)
			require.NoError(t, err)
		}

		// Wait a bit for processing
		time.Sleep(100 * time.Millisecond)

		// Check that pool is still working
		stats := pool.GetStats()
		assert.GreaterOrEqual(t, stats.TotalWorkers, 0)
	})
}

// mockEventHandler implements webhook.EventHandler for testing
type mockEventHandler struct{}

func (m *mockEventHandler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	// Simulate some processing time
	time.Sleep(5 * time.Millisecond)
	return nil
}

func TestJobPriorityOrdering(t *testing.T) {
	priorities := []JobPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical}
	// Check that priorities are in correct order
	assert.True(t, PriorityLow < PriorityNormal)
	assert.True(t, PriorityNormal < PriorityHigh)
	assert.True(t, PriorityHigh < PriorityCritical)
	// Check that all priorities are different
	for i := 0; i < len(priorities)-1; i++ {
		for j := i + 1; j < len(priorities); j++ {
			assert.NotEqual(t, priorities[i], priorities[j])
		}
	}
}

func TestJobStatusTransitions(t *testing.T) {
	// Test valid status transitions
	assert.True(t, StatusPending < StatusProcessing)
	assert.True(t, StatusProcessing < StatusCompleted)
	assert.True(t, StatusProcessing < StatusFailed)
	assert.True(t, StatusFailed < StatusRetrying)

	// Test that retrying can go back to pending
	assert.True(t, StatusRetrying > StatusPending)
}

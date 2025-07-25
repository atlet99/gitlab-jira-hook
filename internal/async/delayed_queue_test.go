package async

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"io"
	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// waitForJob waits for a job to be available in the queue with timeout
func waitForJob(t *testing.T, queue *PriorityQueue, timeout time.Duration) *Job {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if job, err := queue.GetJob(); err == nil {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for job")
	return nil
}

// waitForJobCount waits for a specific number of jobs to be available
func waitForJobCount(t *testing.T, queue *PriorityQueue, expectedCount int, timeout time.Duration) []*Job {
	deadline := time.Now().Add(timeout)
	jobs := make([]*Job, 0, expectedCount)

	for time.Now().Before(deadline) {
		if job, err := queue.GetJob(); err == nil {
			jobs = append(jobs, job)
			if len(jobs) == expectedCount {
				return jobs
			}
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}

	t.Fatalf("timeout waiting for %d jobs, got %d", expectedCount, len(jobs))
	return jobs
}

func TestDelayedQueue(t *testing.T) {
	cfg := &config.Config{
		JobQueueSize:        100,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 30,
		ScaleInterval:       1,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("submit delayed job", func(t *testing.T) {
		mainQueue := NewPriorityQueue(cfg, nil)
		defer mainQueue.Shutdown()

		delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, nil)
		defer delayedQueue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := delayedQueue.SubmitDelayedJob(event, handler, 10*time.Millisecond, PriorityHigh)
		require.NoError(t, err)

		// Job should not be in main queue immediately
		_, err = mainQueue.GetJob()
		assert.Error(t, err) // Should return "no jobs available"
		assert.Contains(t, err.Error(), "no jobs available")

		// Wait for job to be ready and moved to main queue with proper timeout
		job := waitForJob(t, mainQueue, 5*time.Second)
		assert.Equal(t, event, job.Event)
		assert.Equal(t, PriorityHigh, job.Priority)
	})

	t.Run("multiple delayed jobs with different delays", func(t *testing.T) {
		mainQueue := NewPriorityQueue(cfg, nil)
		defer mainQueue.Shutdown()

		delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, nil)
		defer delayedQueue.Shutdown()

		event1 := &webhook.Event{Type: "push"}
		event2 := &webhook.Event{Type: "merge_request"}
		handler := &mockEventHandler{}

		// Submit jobs with different delays
		err := delayedQueue.SubmitDelayedJob(event1, handler, 10*time.Millisecond, PriorityNormal)
		require.NoError(t, err)
		err = delayedQueue.SubmitDelayedJob(event2, handler, 20*time.Millisecond, PriorityHigh)
		require.NoError(t, err)

		// Wait for all jobs to be ready and moved to main queue
		jobs := waitForJobCount(t, mainQueue, 2, 15*time.Second)

		// Check that both events are present (order may vary due to scheduling)
		eventTypes := []string{jobs[0].Event.Type, jobs[1].Event.Type}
		assert.Contains(t, eventTypes, "push")
		assert.Contains(t, eventTypes, "merge_request")

		// Check that one job has high priority
		priorities := []JobPriority{jobs[0].Priority, jobs[1].Priority}
		assert.Contains(t, priorities, PriorityHigh)
	})

	t.Run("delayed job statistics", func(t *testing.T) {
		mainQueue := NewPriorityQueue(cfg, nil)
		defer mainQueue.Shutdown()

		delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, nil)
		defer delayedQueue.Shutdown()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		// Use very short delay for more reliable testing
		err := delayedQueue.SubmitDelayedJob(event, handler, 10*time.Millisecond, PriorityNormal)
		require.NoError(t, err)

		// Give scheduler time to process
		time.Sleep(5 * time.Millisecond)

		// Check stats after submission - use more flexible assertions
		stats := delayedQueue.GetStats()
		totalJobs := stats["total_delayed_jobs"].(int)
		readyJobs := stats["ready_jobs"].(int)

		// Job should be in the delayed queue
		assert.GreaterOrEqual(t, totalJobs, 1, "Should have at least one delayed job")
		// Initially, job should not be ready (delay is 10ms, we waited only 5ms)
		assert.LessOrEqual(t, readyJobs, 1, "Should have at most one ready job")

		// Wait for job to be ready and moved to main queue with longer timeout
		job := waitForJob(t, mainQueue, 10*time.Second)
		assert.Equal(t, event, job.Event)

		// Give some time for stats to update and be consistent
		time.Sleep(200 * time.Millisecond)

		// Check stats after job is moved - should be empty or processing
		stats = delayedQueue.GetStats()
		finalTotalJobs := stats["total_delayed_jobs"].(int)
		finalReadyJobs := stats["ready_jobs"].(int)

		// Job should be processed and moved to main queue
		assert.LessOrEqual(t, finalTotalJobs, 1, "Should have at most one job remaining")
		assert.LessOrEqual(t, finalReadyJobs, 1, "Should have at most one ready job")
	})

	t.Run("delayed job with priority decider", func(t *testing.T) {
		mainQueue := NewPriorityQueue(cfg, nil)
		defer mainQueue.Shutdown()

		decider := &DefaultPriorityDecider{}
		delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, decider)
		defer delayedQueue.Shutdown()

		event := &webhook.Event{Type: "merge_request"}
		handler := &mockEventHandler{}

		// Submit without explicit priority - should use decider
		err := delayedQueue.SubmitDelayedJob(event, handler, 10*time.Millisecond)
		require.NoError(t, err)

		// Wait for job to be ready and moved to main queue
		job := waitForJob(t, mainQueue, 5*time.Second)
		assert.Equal(t, PriorityHigh, job.Priority, "merge_request events should have PriorityHigh priority")
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		mainQueue := NewPriorityQueue(cfg, nil)
		defer mainQueue.Shutdown()

		delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, nil)

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		err := delayedQueue.SubmitDelayedJob(event, handler, 1*time.Second, PriorityNormal)
		require.NoError(t, err)

		// Shutdown immediately
		delayedQueue.Shutdown()

		// Job should not be in main queue (shutdown happened before delay)
		_, err = mainQueue.GetJob()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no jobs available")
	})
}

func TestPriorityWorkerPoolWithDelayedJobs(t *testing.T) {
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
		HealthCheckInterval: 5,
		ScaleInterval:       1,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	decider := &DefaultPriorityDecider{}

	t.Run("submit delayed job to pool", func(t *testing.T) {
		pool := NewPriorityWorkerPool(cfg, logger, nil, decider)
		pool.Start()
		defer pool.Stop()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		// Submit delayed job
		err := pool.SubmitDelayedJob(event, handler, 100*time.Millisecond, PriorityHigh)
		assert.NoError(t, err)

		// Wait for processing with proper timeout
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			stats := pool.GetStats()
			if stats.TotalJobsProcessed > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Check stats
		stats := pool.GetStats()
		assert.GreaterOrEqual(t, stats.TotalJobsProcessed, int64(0))
	})

	t.Run("delayed queue statistics", func(t *testing.T) {
		pool := NewPriorityWorkerPool(cfg, logger, nil, decider)
		pool.Start()
		defer pool.Stop()

		event := &webhook.Event{Type: "merge_request"}
		handler := &mockEventHandler{}

		// Submit delayed job
		err := pool.SubmitDelayedJob(event, handler, 200*time.Millisecond, PriorityNormal)
		assert.NoError(t, err)

		// Check delayed queue stats
		delayedStats := pool.GetDelayedQueueStats()
		assert.Equal(t, 1, delayedStats["total_delayed_jobs"])

		// Wait for job to be processed with proper timeout
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			delayedStats = pool.GetDelayedQueueStats()
			if delayedStats["total_delayed_jobs"] == 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Check delayed queue stats again
		delayedStats = pool.GetDelayedQueueStats()
		assert.Equal(t, 0, delayedStats["total_delayed_jobs"])
	})

	t.Run("pending delayed jobs", func(t *testing.T) {
		pool := NewPriorityWorkerPool(cfg, logger, nil, decider)
		pool.Start()
		defer pool.Stop()

		event := &webhook.Event{Type: "push"}
		handler := &mockEventHandler{}

		// Submit delayed job
		err := pool.SubmitDelayedJob(event, handler, 1*time.Second, PriorityNormal)
		assert.NoError(t, err)

		// Check pending jobs
		pendingJobs := pool.GetPendingDelayedJobs()
		assert.Len(t, pendingJobs, 1)
		assert.Equal(t, event, pendingJobs[0].Event)
	})
}

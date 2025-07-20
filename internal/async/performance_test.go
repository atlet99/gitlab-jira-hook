package async

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// BenchmarkPriorityQueue tests the performance of priority queue operations
func BenchmarkPriorityQueue(b *testing.B) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          8,
		MaxConcurrentJobs:   50,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 5,
	}

	decider := &DefaultPriorityDecider{}
	queue := NewPriorityQueue(cfg, decider)

	b.ResetTimer()
	b.Run("submit_jobs", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := queue.SubmitJob(event, handler, PriorityNormal)
			require.NoError(b, err)
		}
	})

	b.Run("get_jobs", func(b *testing.B) {
		// Pre-fill queue
		for i := 0; i < 1000; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := queue.SubmitJob(event, handler, PriorityNormal)
			require.NoError(b, err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := queue.GetJob()
			if err != nil {
				break
			}
		}
	})

	queue.Shutdown()
}

// BenchmarkDelayedQueue tests the performance of delayed queue operations
func BenchmarkDelayedQueue(b *testing.B) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          8,
		MaxConcurrentJobs:   50,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	decider := &DefaultPriorityDecider{}
	mainQueue := NewPriorityQueue(cfg, decider)
	delayedQueue := NewDelayedQueue(cfg, logger, mainQueue, nil)

	b.ResetTimer()
	b.Run("submit_delayed_jobs", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := delayedQueue.SubmitDelayedJob(event, handler, 10*time.Millisecond, PriorityNormal)
			require.NoError(b, err)
		}
	})

	delayedQueue.Shutdown()
	mainQueue.Shutdown()
}

// BenchmarkPriorityWorkerPool tests the performance of the complete worker pool system
func BenchmarkPriorityWorkerPool(b *testing.B) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          8,
		MaxConcurrentJobs:   50,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 5,
		ScaleInterval:       1,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	decider := &DefaultPriorityDecider{}
	pool := NewPriorityWorkerPool(cfg, logger, nil, decider)
	pool.Start()

	b.ResetTimer()
	b.Run("submit_jobs_to_pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := pool.SubmitJob(event, handler, PriorityNormal)
			require.NoError(b, err)
		}
	})

	b.Run("submit_delayed_jobs_to_pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := pool.SubmitDelayedJob(event, handler, 10*time.Millisecond, PriorityNormal)
			require.NoError(b, err)
		}
	})

	pool.Stop()
}

// BenchmarkResourceAutoDetection tests the performance of resource auto-detection
func BenchmarkResourceAutoDetection(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b.ResetTimer()
	b.Run("auto_detect_config", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := config.NewConfigFromEnv(logger)
			_ = cfg
		}
	})
}

// BenchmarkConcurrentOperations tests concurrent performance
func BenchmarkConcurrentOperations(b *testing.B) {
	cfg := &config.Config{
		MinWorkers:          4,
		MaxWorkers:          16,
		MaxConcurrentJobs:   100,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
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

	b.ResetTimer()
	b.Run("concurrent_job_submission", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				event := &webhook.Event{Type: "push"}
				handler := &mockEventHandler{}
				err := pool.SubmitJob(event, handler, PriorityNormal)
				require.NoError(b, err)
			}
		})
	})

	b.Run("concurrent_delayed_job_submission", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				event := &webhook.Event{Type: "push"}
				handler := &mockEventHandler{}
				err := pool.SubmitDelayedJob(event, handler, 10*time.Millisecond, PriorityNormal)
				require.NoError(b, err)
			}
		})
	})

	pool.Stop()
}

// BenchmarkMemoryUsage tests memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          8,
		MaxConcurrentJobs:   50,
		JobTimeoutSeconds:   10,
		QueueTimeoutMs:      1000,
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

	b.ResetTimer()
	b.Run("memory_under_load", func(b *testing.B) {
		// Submit many jobs to test memory usage
		for i := 0; i < b.N; i++ {
			event := &webhook.Event{Type: "push"}
			handler := &mockEventHandler{}
			err := pool.SubmitJob(event, handler, PriorityNormal)
			require.NoError(b, err)

			if i%100 == 0 {
				// Get stats to trigger memory operations
				_ = pool.GetStats()
			}
		}
	})

	pool.Stop()
}

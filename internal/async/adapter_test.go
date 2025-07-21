package async

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

func TestNewWorkerPoolAdapter(t *testing.T) {
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
	monitor := monitoring.NewWebhookMonitor(cfg, logger)
	priorityDecider := &DefaultPriorityDecider{}

	pool := NewPriorityWorkerPool(cfg, logger, monitor, priorityDecider)
	adapter := NewWorkerPoolAdapter(pool)

	assert.NotNil(t, adapter)
	assert.Equal(t, pool, adapter.pool)
}

func TestWorkerPoolAdapter_GetStats(t *testing.T) {
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
	monitor := monitoring.NewWebhookMonitor(cfg, logger)
	priorityDecider := &DefaultPriorityDecider{}

	pool := NewPriorityWorkerPool(cfg, logger, monitor, priorityDecider)
	adapter := NewWorkerPoolAdapter(pool)

	stats := adapter.GetStats()

	// Verify stats structure
	assert.IsType(t, webhook.PoolStats{}, stats)
	assert.Equal(t, int64(0), stats.TotalJobsProcessed)
	assert.Equal(t, int64(0), stats.SuccessfulJobs)
	assert.Equal(t, int64(0), stats.FailedJobs)
	assert.Equal(t, 0, stats.ActiveWorkers)
	assert.Equal(t, 100, stats.QueueCapacity)
	assert.Equal(t, 0, stats.QueueLength)
	assert.Equal(t, 0, stats.CurrentWorkers)
	assert.Equal(t, 0, stats.ScaleUpEvents)
	assert.Equal(t, 0, stats.ScaleDownEvents)
}

func TestWorkerPoolAdapter_GetDelayedQueueStats(t *testing.T) {
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
	monitor := monitoring.NewWebhookMonitor(cfg, logger)
	priorityDecider := &DefaultPriorityDecider{}

	pool := NewPriorityWorkerPool(cfg, logger, monitor, priorityDecider)
	adapter := NewWorkerPoolAdapter(pool)

	stats := adapter.GetDelayedQueueStats()
	assert.NotNil(t, stats)
	assert.IsType(t, map[string]interface{}{}, stats)
}

func TestWorkerPoolAdapter_GetPendingDelayedJobs(t *testing.T) {
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
	monitor := monitoring.NewWebhookMonitor(cfg, logger)
	priorityDecider := &DefaultPriorityDecider{}

	pool := NewPriorityWorkerPool(cfg, logger, monitor, priorityDecider)
	adapter := NewWorkerPoolAdapter(pool)

	jobs := adapter.GetPendingDelayedJobs()
	assert.NotNil(t, jobs)
	assert.IsType(t, []*DelayedJob{}, jobs)
	assert.Len(t, jobs, 0)
}

func TestWorkerPoolAdapter_WaitForReadyJobs(t *testing.T) {
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
	monitor := monitoring.NewWebhookMonitor(cfg, logger)
	priorityDecider := &DefaultPriorityDecider{}

	pool := NewPriorityWorkerPool(cfg, logger, monitor, priorityDecider)
	adapter := NewWorkerPoolAdapter(pool)

	// Test with timeout
	err := adapter.WaitForReadyJobs(100 * time.Millisecond)
	assert.NoError(t, err)
}

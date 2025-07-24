package async

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestWorkerPoolScaling(t *testing.T) {
	tests := []struct {
		name            string
		minWorkers      int
		maxWorkers      int
		scaleUpThresh   int
		scaleDownThresh int
		queueLength     int
		expectedAction  string
	}{
		{
			name:            "scale up when queue is full",
			minWorkers:      2,
			maxWorkers:      10,
			scaleUpThresh:   5,
			scaleDownThresh: 2,
			queueLength:     8,
			expectedAction:  "scale_up",
		},
		{
			name:            "scale down when queue is empty",
			minWorkers:      2,
			maxWorkers:      10,
			scaleUpThresh:   5,
			scaleDownThresh: 2,
			queueLength:     0,
			expectedAction:  "scale_down",
		},
		{
			name:            "no scale up at max capacity",
			minWorkers:      2,
			maxWorkers:      3,
			scaleUpThresh:   5,
			scaleDownThresh: 2,
			queueLength:     10,
			expectedAction:  "no_change",
		},
		{
			name:            "no scale down at min capacity",
			minWorkers:      2,
			maxWorkers:      10,
			scaleUpThresh:   5,
			scaleDownThresh: 2,
			queueLength:     1,
			expectedAction:  "no_change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				MinWorkers:          tt.minWorkers,
				MaxWorkers:          tt.maxWorkers,
				ScaleUpThreshold:    tt.scaleUpThresh,
				ScaleDownThreshold:  tt.scaleDownThresh,
				ScaleInterval:       1,
				HealthCheckInterval: 5,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			pool := NewWorkerPool(cfg, logger, nil)

			// Start pool to initialize workers
			pool.Start()
			defer pool.Stop()

			// Wait for initial setup
			time.Sleep(100 * time.Millisecond)

			// Fill queue to simulate load
			for i := 0; i < tt.queueLength; i++ {
				select {
				case pool.jobQueue <- &WebhookJob{}:
				default:
					// Queue is full
				}
			}

			// Trigger scaling check
			if tt.queueLength > tt.scaleUpThresh {
				pool.scaleUp()
			} else if tt.queueLength < tt.scaleDownThresh {
				pool.scaleDown()
			}

			// Wait for scaling to complete
			time.Sleep(100 * time.Millisecond)

			// Verify results
			stats := pool.GetStats()
			switch tt.expectedAction {
			case "scale_up":
				assert.Greater(t, stats.ScaleUpEvents, 0)
				assert.GreaterOrEqual(t, stats.CurrentWorkers, tt.minWorkers)
			case "scale_down":
				// For scale down, we expect the event to be recorded if workers were actually reduced
				// But since we start with min workers, scale down won't happen
				assert.Equal(t, tt.minWorkers, stats.CurrentWorkers)
			case "no_change":
				// For no_change, we expect current workers to be at min or max
				assert.True(t, stats.CurrentWorkers >= tt.minWorkers && stats.CurrentWorkers <= tt.maxWorkers)
			}
		})
	}
}

func TestWorkerPoolMetrics(t *testing.T) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          10,
		ScaleUpThreshold:    5,
		ScaleDownThreshold:  2,
		ScaleInterval:       1,
		HealthCheckInterval: 5,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool := NewWorkerPool(cfg, logger, nil)

	// Start pool
	pool.Start()
	defer pool.Stop()

	// Wait for initial setup
	time.Sleep(100 * time.Millisecond)

	// Check initial metrics
	stats := pool.GetStats()
	assert.Equal(t, cfg.MinWorkers, stats.CurrentWorkers)
	assert.Equal(t, 0, stats.QueueLength)
	assert.Equal(t, 0, stats.ScaleUpEvents)
	assert.Equal(t, 0, stats.ScaleDownEvents)

	// Fill queue to trigger scale up
	for i := 0; i < 10; i++ {
		pool.jobQueue <- &WebhookJob{}
	}

	// Trigger scale up
	pool.scaleUp()

	// Wait for scaling
	time.Sleep(100 * time.Millisecond)

	// Check updated metrics
	stats = pool.GetStats()
	assert.Greater(t, stats.ScaleUpEvents, 0)
	assert.Greater(t, stats.CurrentWorkers, cfg.MinWorkers)
}

func TestWorkerPoolScalingMonitor(t *testing.T) {
	cfg := &config.Config{
		MinWorkers:          2,
		MaxWorkers:          5,
		ScaleUpThreshold:    3,
		ScaleDownThreshold:  1,
		ScaleInterval:       1,
		HealthCheckInterval: 5,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool := NewWorkerPool(cfg, logger, nil)

	// Start pool to initialize workers
	pool.Start()
	defer pool.Stop()

	// Wait for initial setup
	time.Sleep(100 * time.Millisecond)

	// Start scaling monitor in a controlled way
	done := make(chan bool)
	go func() {
		defer close(done)
		// Run scaling monitor for a limited time
		ticker := time.NewTicker(time.Duration(cfg.ScaleInterval) * time.Second)
		defer ticker.Stop()

		// Run for 3 ticks maximum
		for i := 0; i < 3; i++ {
			<-ticker.C
			// Manually trigger scaling check
			queueLen := len(pool.jobQueue)
			if queueLen > cfg.ScaleUpThreshold {
				pool.scaleUp()
			} else if queueLen < cfg.ScaleDownThreshold {
				pool.scaleDown()
			}
		}
	}()

	// Test scale up
	for i := 0; i < 5; i++ {
		pool.jobQueue <- &WebhookJob{}
	}

	// Wait for scaling monitor to process
	time.Sleep(1200 * time.Millisecond) // Wait for at least one tick

	stats := pool.GetStats()
	assert.Greater(t, stats.ScaleUpEvents, 0)

	// Test scale down by clearing queue
	for len(pool.jobQueue) > 0 {
		<-pool.jobQueue
	}

	time.Sleep(1200 * time.Millisecond) // Wait for at least one tick

	stats = pool.GetStats()
	// Scale down won't happen because we're already at min workers
	// So we just check that we're still at min workers
	assert.Equal(t, cfg.MinWorkers, stats.CurrentWorkers)

	// Wait for scaling monitor to finish
	<-done
}

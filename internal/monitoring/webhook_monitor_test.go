package monitoring

import (
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestNewWebhookMonitor(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, cfg, monitor.config)
	assert.Equal(t, logger, monitor.logger)
	assert.NotNil(t, monitor.statuses)
	assert.NotNil(t, monitor.metrics)
	assert.NotNil(t, monitor.httpClient)
}

func TestWebhookMonitor_InitializeEndpoints(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)
	monitor.initializeEndpoints()

	// Check that endpoints are initialized
	assert.Contains(t, monitor.statuses, "/gitlab-hook")
	assert.Contains(t, monitor.statuses, "/gitlab-project-hook")
	assert.Contains(t, monitor.statuses, "/health")

	// Check that metrics are initialized
	assert.Contains(t, monitor.metrics, "/gitlab-hook")
	assert.Contains(t, monitor.metrics, "/gitlab-project-hook")
	assert.Contains(t, monitor.metrics, "/health")
}

func TestWebhookMonitor_RecordRequest(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)
	monitor.initializeEndpoints()

	// Record a successful request
	monitor.RecordRequest("/gitlab-hook", true, 100*time.Millisecond)

	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics["/gitlab-hook"].TotalRequests)
	assert.Equal(t, int64(1), metrics["/gitlab-hook"].SuccessfulRequests)
	assert.Equal(t, int64(0), metrics["/gitlab-hook"].FailedRequests)
	assert.Equal(t, 100*time.Millisecond, metrics["/gitlab-hook"].AverageResponseTime)

	// Record a failed request
	monitor.RecordRequest("/gitlab-hook", false, 200*time.Millisecond)

	metrics = monitor.GetMetrics()
	assert.Equal(t, int64(2), metrics["/gitlab-hook"].TotalRequests)
	assert.Equal(t, int64(1), metrics["/gitlab-hook"].SuccessfulRequests)
	assert.Equal(t, int64(1), metrics["/gitlab-hook"].FailedRequests)
	assert.Equal(t, 150*time.Millisecond, metrics["/gitlab-hook"].AverageResponseTime)
}

func TestWebhookMonitor_GetStatus(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)
	monitor.initializeEndpoints()

	status := monitor.GetStatus()

	assert.Contains(t, status, "/gitlab-hook")
	assert.Contains(t, status, "/gitlab-project-hook")
	assert.Contains(t, status, "/health")

	// Check that status is a copy
	originalStatus := monitor.statuses["/gitlab-hook"]
	copiedStatus := status["/gitlab-hook"]

	assert.NotSame(t, originalStatus, copiedStatus)
	assert.Equal(t, originalStatus.Endpoint, copiedStatus.Endpoint)
}

func TestWebhookMonitor_IsHealthy(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)
	monitor.initializeEndpoints()

	// Initially all endpoints are unknown, so not healthy
	assert.False(t, monitor.IsHealthy())

	// Set all endpoints to healthy
	monitor.mu.Lock()
	for _, status := range monitor.statuses {
		status.Status = "healthy"
	}
	monitor.mu.Unlock()

	assert.True(t, monitor.IsHealthy())

	// Set one endpoint to unhealthy
	monitor.mu.Lock()
	monitor.statuses["/gitlab-hook"].Status = "unhealthy"
	monitor.mu.Unlock()

	assert.False(t, monitor.IsHealthy())
}

func TestWebhookMonitor_GetUnhealthyEndpoints(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)
	monitor.initializeEndpoints()

	// Set some endpoints to unhealthy
	monitor.mu.Lock()
	monitor.statuses["/gitlab-hook"].Status = "unhealthy"
	monitor.statuses["/gitlab-project-hook"].Status = "healthy"
	monitor.statuses["/health"].Status = "unknown"
	monitor.mu.Unlock()

	unhealthy := monitor.GetUnhealthyEndpoints()

	assert.Contains(t, unhealthy, "/gitlab-hook")
	assert.Contains(t, unhealthy, "/health")
	assert.NotContains(t, unhealthy, "/gitlab-project-hook")
	assert.Len(t, unhealthy, 2)
}

func TestWebhookMonitor_StartStop(t *testing.T) {
	cfg := &config.Config{
		Port: "8080",
	}
	logger := slog.Default()

	monitor := NewWebhookMonitor(cfg, logger)

	// Start monitoring
	monitor.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop monitoring
	monitor.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Check that context is cancelled
	select {
	case <-monitor.ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after Stop()")
	}
}

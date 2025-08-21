package monitoring

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestHealthMonitor_RegisterChecker(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	checker := NewSimpleHealthChecker("test", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusHealthy, "OK", nil, nil
	})

	monitor.RegisterChecker("test", checker)
	assert.Len(t, monitor.checks, 1)
	assert.Contains(t, monitor.checks, "test")
}

func TestHealthMonitor_RunHealthChecks(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	// Register a healthy checker
	monitor.RegisterChecker("healthy", NewSimpleHealthChecker("healthy", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusHealthy, "OK", map[string]interface{}{"detail": "healthy"}, nil
	}))

	// Register a degraded checker
	monitor.RegisterChecker("degraded", NewSimpleHealthChecker("degraded", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusDegraded, "Warning", map[string]interface{}{"detail": "degraded"}, nil
	}))

	// Register an unhealthy checker
	monitor.RegisterChecker("unhealthy", NewSimpleHealthChecker("unhealthy", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusUnhealthy, "Error", map[string]interface{}{"detail": "unhealthy"}, nil
	}))

	report := monitor.RunHealthChecks(context.Background())

	assert.Equal(t, HealthStatusUnhealthy, report.OverallStatus)
	assert.Len(t, report.Checks, 3)
	assert.Equal(t, HealthStatusHealthy, report.Checks["healthy"].Status)
	assert.Equal(t, HealthStatusDegraded, report.Checks["degraded"].Status)
	assert.Equal(t, HealthStatusUnhealthy, report.Checks["unhealthy"].Status)
}

func TestHealthMonitor_GetHealthStatus(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	checker := NewSimpleHealthChecker("test", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusHealthy, "OK", nil, nil
	})

	monitor.RegisterChecker("test", checker)
	monitor.RunHealthChecks(context.Background())

	status, exists := monitor.GetHealthStatus("test")
	assert.True(t, exists)
	assert.Equal(t, HealthStatusHealthy, status.Status)

	_, exists = monitor.GetHealthStatus("nonexistent")
	assert.False(t, exists)
}

func TestHealthMonitor_IsHealthy(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	// Initially healthy
	assert.True(t, monitor.IsHealthy())

	// Register an unhealthy checker
	monitor.RegisterChecker("unhealthy", NewSimpleHealthChecker("unhealthy", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusUnhealthy, "Error", nil, nil
	}))

	monitor.RunHealthChecks(context.Background())
	assert.False(t, monitor.IsHealthy())
}

func TestHealthMonitor_collectSystemInfo(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	info := monitor.collectSystemInfo()

	assert.Contains(t, info, "memory")
	assert.Contains(t, info, "goroutines")
	assert.Contains(t, info, "cpu")
	assert.Contains(t, info, "uptime")
	assert.Contains(t, info, "app")

	// Check app info
	appInfo := info["app"].(map[string]interface{})
	assert.Equal(t, "1.0.0", appInfo["version"])
	assert.Contains(t, appInfo, "config")
}

func TestSimpleHealthChecker(t *testing.T) {
	checker := NewSimpleHealthChecker("test", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusHealthy, "OK", map[string]interface{}{"test": "value"}, nil
	})

	status, message, details, err := checker.CheckHealth(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, status)
	assert.Equal(t, "OK", message)
	assert.Equal(t, "value", details["test"])
}

func TestCacheHealthChecker(t *testing.T) {
	testCache := cache.NewMemoryCache(100)
	defer testCache.Close()

	checker := NewCacheHealthChecker(testCache)

	// Test with healthy cache
	status, message, details, err := checker.CheckHealth(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, status)
	assert.Equal(t, "Cache is healthy", message)
	assert.Contains(t, details, "tested_operations")

	// Test with nil cache
	checkerNil := NewCacheHealthChecker(nil)
	status, message, _, err = checkerNil.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Equal(t, HealthStatusUnhealthy, status)
	assert.Equal(t, "Cache is nil", message)
}

func TestHTTPHealthHandler_HandleHealth(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	handler := NewHTTPHealthHandler(monitor)

	// Test overall health check
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Test readiness check
	req = httptest.NewRequest("GET", "/health/ready", nil)
	w = httptest.NewRecorder()

	handler.HandleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestHTTPHealthHandler_handleOverall(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	handler := NewHTTPHealthHandler(monitor)

	// Test with healthy system
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.handleOverall(context.Background(), w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test with unhealthy system
	monitor.RegisterChecker("unhealthy", NewSimpleHealthChecker("unhealthy", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusUnhealthy, "Error", nil, nil
	}))

	monitor.RunHealthChecks(context.Background())

	handler.handleOverall(context.Background(), w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHTTPHealthHandler_handleReadiness(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	handler := NewHTTPHealthHandler(monitor)

	// Test with ready system
	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.handleReadiness(context.Background(), w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test with not ready system
	monitor.RegisterChecker("cache", NewSimpleHealthChecker("cache", func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
		return HealthStatusUnhealthy, "Error", nil, nil
	}))

	monitor.RunHealthChecks(context.Background())

	handler.handleReadiness(context.Background(), w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHTTPHealthHandler_writeJSONResponse(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{}))
	monitor := NewHealthMonitor(cfg, logger, "1.0.0")

	handler := NewHTTPHealthHandler(monitor)

	w := httptest.NewRecorder()

	testData := map[string]interface{}{
		"status": "ok",
		"time":   time.Now(),
	}

	handler.writeJSONResponse(w, http.StatusOK, testData)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// Helper test writer
type testWriter struct{}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

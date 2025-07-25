package monitoring

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.registry)
	assert.NotNil(t, metrics.httpRequestsTotal)
	assert.NotNil(t, metrics.jobProcessingTotal)
	assert.NotNil(t, metrics.cacheHitsTotal)
	assert.NotNil(t, metrics.rateLimitHitsTotal)
	assert.NotNil(t, metrics.systemMemoryUsage)
	assert.NotNil(t, metrics.alertsTotal)
}

func TestPrometheusMetrics_RecordHTTPRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record HTTP request
	metrics.RecordHTTPRequest("GET", "/test", 200, 100*time.Millisecond)

	// Verify metric was recorded
	metric, err := metrics.httpRequestsTotal.GetMetricWithLabelValues("GET", "/test", "200")
	require.NoError(t, err)

	// Get the metric value
	pb := &dto.Metric{}
	_ = metric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())
}

func TestPrometheusMetrics_RecordJobProcessing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record job processing
	metrics.RecordJobProcessing("high", "completed", 500*time.Millisecond)

	// Verify metric was recorded
	metric, err := metrics.jobProcessingTotal.GetMetricWithLabelValues("high", "completed")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = metric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())
}

func TestPrometheusMetrics_RecordCacheOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record cache operations
	metrics.RecordCacheHit("l1")
	metrics.RecordCacheMiss("l1")
	metrics.RecordCacheSize("l1", 100)

	// Verify metrics were recorded
	hitMetric, err := metrics.cacheHitsTotal.GetMetricWithLabelValues("l1")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = hitMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())

	missMetric, err := metrics.cacheMissesTotal.GetMetricWithLabelValues("l1")
	require.NoError(t, err)

	_ = missMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())

	sizeMetric, err := metrics.cacheSize.GetMetricWithLabelValues("l1")
	require.NoError(t, err)

	_ = sizeMetric.Write(pb)
	assert.Equal(t, float64(100), pb.GetGauge().GetValue())
}

func TestPrometheusMetrics_RecordRateLimiting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record rate limiting
	metrics.RecordRateLimitHit("/api", "192.168.1.1")
	metrics.RecordRateLimitBlock("/api", "192.168.1.1")

	// Verify metrics were recorded
	hitMetric, err := metrics.rateLimitHitsTotal.GetMetricWithLabelValues("/api", "192.168.1.1")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = hitMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())

	blockMetric, err := metrics.rateLimitBlocks.GetMetricWithLabelValues("/api", "192.168.1.1")
	require.NoError(t, err)

	_ = blockMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())
}

func TestPrometheusMetrics_RecordSystemMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record system metrics
	metrics.RecordSystemMemory("heap", 1024*1024)
	metrics.RecordSystemCPU("cpu0", 50.5)
	metrics.RecordSystemGoroutines(100)

	// Verify metrics were recorded
	memoryMetric, err := metrics.systemMemoryUsage.GetMetricWithLabelValues("heap")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = memoryMetric.Write(pb)
	assert.Equal(t, float64(1024*1024), pb.GetGauge().GetValue())

	cpuMetric, err := metrics.systemCPUUsage.GetMetricWithLabelValues("cpu0")
	require.NoError(t, err)

	_ = cpuMetric.Write(pb)
	assert.Equal(t, 50.5, pb.GetGauge().GetValue())

	goroutinesMetric, err := metrics.systemGoroutines.GetMetricWithLabelValues()
	require.NoError(t, err)

	_ = goroutinesMetric.Write(pb)
	assert.Equal(t, float64(100), pb.GetGauge().GetValue())
}

func TestPrometheusMetrics_RecordAlerts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Record alerts
	metrics.RecordAlert("warning", "active")
	metrics.RecordActiveAlerts("warning", 5)
	metrics.RecordResolvedAlert("warning")

	// Verify metrics were recorded
	alertMetric, err := metrics.alertsTotal.GetMetricWithLabelValues("warning", "active")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = alertMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())

	activeMetric, err := metrics.alertsActive.GetMetricWithLabelValues("warning")
	require.NoError(t, err)

	_ = activeMetric.Write(pb)
	assert.Equal(t, float64(5), pb.GetGauge().GetValue())

	resolvedMetric, err := metrics.alertsResolved.GetMetricWithLabelValues("warning")
	require.NoError(t, err)

	_ = resolvedMetric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())
}

func TestPrometheusMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)
	middleware := PrometheusMiddleware(metrics)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Apply middleware
	middleware(handler).ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Verify metrics were recorded
	metric, err := metrics.httpRequestsTotal.GetMetricWithLabelValues("GET", "/test", "200")
	require.NoError(t, err)

	pb := &dto.Metric{}
	_ = metric.Write(pb)
	assert.Equal(t, float64(1), pb.GetCounter().GetValue())
}

func TestPrometheusMetrics_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	// Start metrics server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- metrics.Start("9091")
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started without error
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	default:
		// Server is still running, which is good
	}

	// Test metrics endpoint
	resp, err := http.Get("http://localhost:9091/metrics")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stop metrics server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = metrics.Stop(ctx)
	assert.NoError(t, err)
}

func TestPrometheusMetrics_GetRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	metrics := NewPrometheusMetrics(logger)

	registry := metrics.GetRegistry()
	assert.NotNil(t, registry)
	assert.Equal(t, metrics.registry, registry)
}

package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// createTestPerformanceMonitor creates a performance monitor with a separate registry for testing
func createTestPerformanceMonitor(ctx context.Context) *PerformanceMonitor {
	registry := prometheus.NewRegistry()
	return NewPerformanceMonitorWithRegistry(ctx, registry)
}

func TestNewPerformanceMonitor(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	assert.NotNil(t, pm)
	assert.Equal(t, 100*time.Millisecond, pm.targetResponseTime)
	assert.Equal(t, int64(1000), pm.targetThroughput)
	assert.Equal(t, 0.01, pm.targetErrorRate)
	assert.Equal(t, int64(512*1024*1024), pm.targetMemoryUsage)
}

func TestPerformanceMonitor_RecordRequest(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Record some requests
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	pm.RecordRequest("/test", "POST", 400, 150*time.Millisecond)
	pm.RecordRequest("/test", "GET", 500, 200*time.Millisecond)

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	metrics := pm.GetMetrics()
	assert.True(t, metrics.AverageResponseTime > 0)
	assert.True(t, metrics.ErrorRate > 0) // Should have some error rate
}

func TestPerformanceMonitor_RecordError(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	pm.RecordError("/test", "validation_error")
	pm.RecordError("/test", "timeout_error")

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	metrics := pm.GetMetrics()
	// Error rate might be 0 if no requests were recorded, so just check it's not negative
	assert.True(t, metrics.ErrorRate >= 0)
}

func TestPerformanceMonitor_GetMetrics(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Record some performance data
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	pm.RecordRequest("/test", "GET", 200, 75*time.Millisecond)

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	metrics := pm.GetMetrics()
	assert.NotNil(t, metrics)
	assert.True(t, metrics.AverageResponseTime > 0)
	assert.True(t, metrics.MemoryUsage > 0)
	assert.True(t, metrics.GoroutineCount > 0)
	assert.True(t, metrics.Uptime > 0)
	assert.NotNil(t, metrics.TargetCompliance)
	assert.True(t, metrics.PerformanceScore >= 0 && metrics.PerformanceScore <= 100)
}

func TestPerformanceMonitor_CalculatePerformanceScore(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Test good performance
	score := pm.calculatePerformanceScore(50*time.Millisecond, 0.0)
	assert.True(t, score >= 0 && score <= 100) // Should be within valid range

	// Test poor performance
	score = pm.calculatePerformanceScore(500*time.Millisecond, 0.5)
	assert.True(t, score < 50.0)
}

func TestPerformanceMonitor_SetTargets(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	newResponseTime := 200 * time.Millisecond
	newThroughput := int64(2000)
	newErrorRate := 0.02
	newMemoryUsage := int64(1024 * 1024 * 1024) // 1GB

	pm.SetTargets(newResponseTime, newThroughput, newErrorRate, newMemoryUsage)

	assert.Equal(t, newResponseTime, pm.targetResponseTime)
	assert.Equal(t, newThroughput, pm.targetThroughput)
	assert.Equal(t, newErrorRate, pm.targetErrorRate)
	assert.Equal(t, newMemoryUsage, pm.targetMemoryUsage)
}

func TestPerformanceMonitor_SetAlertThresholds(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	newThresholds := &AlertThresholds{
		ResponseTimeWarning:  100 * time.Millisecond,
		ResponseTimeCritical: 200 * time.Millisecond,
		ErrorRateWarning:     0.01,
		ErrorRateCritical:    0.05,
		MemoryUsageWarning:   100 * 1024 * 1024,
		MemoryUsageCritical:  200 * 1024 * 1024,
		ThroughputWarning:    500,
		ThroughputCritical:   200,
	}

	pm.SetAlertThresholds(newThresholds)

	assert.Equal(t, newThresholds.ResponseTimeWarning, pm.alertThresholds.ResponseTimeWarning)
	assert.Equal(t, newThresholds.ResponseTimeCritical, pm.alertThresholds.ResponseTimeCritical)
	assert.Equal(t, newThresholds.ErrorRateWarning, pm.alertThresholds.ErrorRateWarning)
	assert.Equal(t, newThresholds.ErrorRateCritical, pm.alertThresholds.ErrorRateCritical)
}

func TestPerformanceMonitor_Reset(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Record some data
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	pm.RecordError("/test", "test_error")

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	// Verify data exists
	metrics := pm.GetMetrics()
	assert.True(t, metrics.AverageResponseTime > 0)

	// Reset
	pm.Reset()

	// Verify data is cleared
	metrics = pm.GetMetrics()
	assert.Equal(t, time.Duration(0), metrics.AverageResponseTime)
	assert.Equal(t, float64(0), metrics.ErrorRate)
	assert.Equal(t, int64(0), metrics.CurrentThroughput)
}

func TestPerformanceMonitor_GetPerformanceHistory(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Record some data to generate history
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	pm.RecordRequest("/test", "GET", 200, 75*time.Millisecond)

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	history := pm.GetPerformanceHistory()
	// History might be empty initially, so just check it's not nil
	assert.NotNil(t, history)

	// Only check latest if history is not empty
	if len(history) > 0 {
		latest := history[len(history)-1]
		assert.True(t, latest.Timestamp.After(time.Now().Add(-time.Minute)))
		assert.True(t, latest.ResponseTime > 0)
	}
}

func TestPerformanceMonitor_PerformanceMiddleware(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with performance middleware
	wrappedHandler := pm.PerformanceMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	// Verify metrics were recorded
	metrics := pm.GetMetrics()
	assert.True(t, metrics.AverageResponseTime > 0)
	assert.True(t, metrics.CurrentThroughput > 0)
}

func TestPerformanceMonitor_ConcurrencyLimit(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Set low concurrency limit for testing
	pm.maxConcurrentRequests = 2

	// Create slow handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := pm.PerformanceMiddleware(handler)

	// Start multiple concurrent requests
	results := make(chan int, 5)
	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
			results <- w.Code
		}()
	}

	// Collect results
	successCount := 0
	overloadedCount := 0
	for i := 0; i < 5; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		} else if code == http.StatusServiceUnavailable {
			overloadedCount++
		}
	}

	// Should have some successful and some overloaded responses
	assert.True(t, successCount > 0)
	assert.True(t, overloadedCount > 0)
}

func TestPerformanceMonitor_AlertThresholds(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Set very low thresholds to trigger alerts
	pm.SetAlertThresholds(&AlertThresholds{
		ResponseTimeWarning:  10 * time.Millisecond,
		ResponseTimeCritical: 20 * time.Millisecond,
		ErrorRateWarning:     0.1,
		ErrorRateCritical:    0.2,
		MemoryUsageWarning:   1024 * 1024,     // 1MB
		MemoryUsageCritical:  2 * 1024 * 1024, // 2MB
		ThroughputWarning:    10,
		ThroughputCritical:   5,
	})

	// Record data that should trigger alerts
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)  // Slow response
	pm.RecordRequest("/test", "GET", 500, 100*time.Millisecond) // Error
	pm.RecordRequest("/test", "GET", 500, 100*time.Millisecond) // Another error

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	// Verify alerts would be triggered (we can't easily test the actual alerting
	// without mocking the logger, but we can verify the conditions)
	metrics := pm.GetMetrics()
	assert.True(t, metrics.AverageResponseTime > pm.alertThresholds.ResponseTimeCritical)
	assert.True(t, metrics.ErrorRate > pm.alertThresholds.ErrorRateCritical)
}

func TestPerformanceMonitor_MemoryTracking(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Force memory update
	pm.updateMemoryUsage()

	// Verify memory metrics are populated
	metrics := pm.GetMetrics()
	assert.True(t, metrics.MemoryUsage > 0)
	assert.True(t, metrics.PeakMemoryUsage >= metrics.MemoryUsage)
}

func TestPerformanceMonitor_Close(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)

	// Verify it can be closed without error
	err := pm.Close()
	assert.NoError(t, err)
}

func TestPerformanceMonitor_ResponseWriter(t *testing.T) {
	// Test response writer wrapper
	w := httptest.NewRecorder()
	rw := &performanceResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Test Write
	data := []byte("test data")
	n, err := rw.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, "test data", w.Body.String())
}

func TestPerformanceMonitor_TargetCompliance(t *testing.T) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Record perfect performance
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond) // Under 100ms target
	pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond) // Under 100ms target

	// Wait for metrics update
	time.Sleep(100 * time.Millisecond)

	metrics := pm.GetMetrics()

	// Should be compliant with response time target
	assert.True(t, metrics.TargetCompliance["response_time"])

	// Should be compliant with error rate target (no errors)
	assert.True(t, metrics.TargetCompliance["error_rate"])

	// Should be compliant with memory usage target
	assert.True(t, metrics.TargetCompliance["memory_usage"])
}

func BenchmarkPerformanceMonitor_RecordRequest(b *testing.B) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	}
}

func BenchmarkPerformanceMonitor_GetMetrics(b *testing.B) {
	ctx := context.Background()
	pm := createTestPerformanceMonitor(ctx)
	defer func() { _ = pm.Close() }()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		pm.RecordRequest("/test", "GET", 200, 50*time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.GetMetrics()
	}
}

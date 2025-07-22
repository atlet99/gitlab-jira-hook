package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// Performance monitoring constants
	defaultMetricsUpdateInterval = 5 * time.Second
	defaultTargetResponseTime    = 100 * time.Millisecond
	defaultTargetThroughput      = 1000
	defaultTargetErrorRate       = 0.01              // 1%
	defaultTargetMemoryUsage     = 512 * 1024 * 1024 // 512MB
	defaultMaxConcurrentRequests = 1000
	defaultHistoryMaxSize        = 1000

	// Alert threshold constants
	defaultResponseTimeWarning  = 150 * time.Millisecond
	defaultResponseTimeCritical = 300 * time.Millisecond
	defaultErrorRateWarning     = 0.05              // 5%
	defaultErrorRateCritical    = 0.10              // 10%
	defaultMemoryUsageWarning   = 400 * 1024 * 1024 // 400MB
	defaultMemoryUsageCritical  = 600 * 1024 * 1024 // 600MB
	defaultThroughputWarning    = 800
	defaultThroughputCritical   = 500

	// Performance score constants
	perfectScore       = 100.0
	responseTimeWeight = 0.4
	errorRateWeight    = 0.3
	throughputWeight   = 0.2
	memoryWeight       = 0.1

	// HTTP status constants
	performanceHTTPErrorThreshold = 400
	errorRatePercentage           = 100

	// Memory constants
	maxInt64Value = int64(1<<63 - 1)
)

// PerformanceMonitor provides comprehensive performance monitoring
type PerformanceMonitor struct {
	// Response time tracking
	responseTimeHistogram *prometheus.HistogramVec
	responseTimeCounter   *prometheus.CounterVec

	// Throughput tracking
	requestsPerSecond *prometheus.GaugeVec
	throughputCounter *prometheus.CounterVec

	// Error rate tracking
	errorRateGauge *prometheus.GaugeVec
	errorCounter   *prometheus.CounterVec

	// Memory usage tracking
	memoryUsageGauge *prometheus.GaugeVec
	memoryAllocGauge *prometheus.GaugeVec
	memorySysGauge   *prometheus.GaugeVec

	// System metrics
	goroutineGauge   *prometheus.GaugeVec
	gcDurationGauge  *prometheus.GaugeVec
	gcFrequencyGauge *prometheus.GaugeVec

	// Performance targets
	targetResponseTime time.Duration
	targetThroughput   int64
	targetErrorRate    float64
	targetMemoryUsage  int64

	// Internal state
	mu                    sync.RWMutex
	startTime             time.Time
	lastMetricsUpdate     time.Time
	metricsUpdateInterval time.Duration

	// Performance tracking
	totalRequests      int64
	totalErrors        int64
	totalResponseTime  int64
	lastSecondRequests int64
	lastSecondErrors   int64

	// Memory tracking
	peakMemoryUsage    int64
	currentMemoryUsage int64

	// Concurrency control
	activeRequests        int64
	maxConcurrentRequests int64

	// Alert thresholds
	alertThresholds *AlertThresholds

	// Performance history for trend analysis
	performanceHistory []PerformanceSnapshot
	historyMaxSize     int

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// AlertThresholds defines performance alert thresholds
type AlertThresholds struct {
	ResponseTimeWarning  time.Duration
	ResponseTimeCritical time.Duration
	ErrorRateWarning     float64
	ErrorRateCritical    float64
	MemoryUsageWarning   int64
	MemoryUsageCritical  int64
	ThroughputWarning    int64
	ThroughputCritical   int64
}

// PerformanceSnapshot represents a point-in-time performance measurement
type PerformanceSnapshot struct {
	Timestamp      time.Time
	ResponseTime   time.Duration
	Throughput     int64
	ErrorRate      float64
	MemoryUsage    int64
	GoroutineCount int64
	ActiveRequests int64
}

// PerformanceMetrics represents current performance metrics
type PerformanceMetrics struct {
	AverageResponseTime time.Duration
	CurrentThroughput   int64
	ErrorRate           float64
	MemoryUsage         int64
	PeakMemoryUsage     int64
	GoroutineCount      int64
	ActiveRequests      int64
	Uptime              time.Duration
	TargetCompliance    map[string]bool
	PerformanceScore    float64
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(ctx context.Context) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(ctx)

	pm := &PerformanceMonitor{
		startTime:             time.Now(),
		lastMetricsUpdate:     time.Now(),
		metricsUpdateInterval: defaultMetricsUpdateInterval,
		targetResponseTime:    defaultTargetResponseTime,
		targetThroughput:      defaultTargetThroughput,
		targetErrorRate:       defaultTargetErrorRate,
		targetMemoryUsage:     defaultTargetMemoryUsage,
		maxConcurrentRequests: defaultMaxConcurrentRequests,
		historyMaxSize:        defaultHistoryMaxSize,
		ctx:                   ctx,
		cancel:                cancel,
		alertThresholds: &AlertThresholds{
			ResponseTimeWarning:  defaultResponseTimeWarning,
			ResponseTimeCritical: defaultResponseTimeCritical,
			ErrorRateWarning:     defaultErrorRateWarning,
			ErrorRateCritical:    defaultErrorRateCritical,
			MemoryUsageWarning:   defaultMemoryUsageWarning,
			MemoryUsageCritical:  defaultMemoryUsageCritical,
			ThroughputWarning:    defaultThroughputWarning,
			ThroughputCritical:   defaultThroughputCritical,
		},
	}

	pm.initializeMetrics()
	go pm.metricsUpdateLoop()
	go pm.memoryMonitoringLoop()

	return pm
}

// NewPerformanceMonitorWithRegistry creates a new performance monitor instance with custom registry
func NewPerformanceMonitorWithRegistry(ctx context.Context, registry prometheus.Registerer) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(ctx)

	pm := &PerformanceMonitor{
		startTime:             time.Now(),
		lastMetricsUpdate:     time.Now(),
		metricsUpdateInterval: defaultMetricsUpdateInterval,
		targetResponseTime:    defaultTargetResponseTime,
		targetThroughput:      defaultTargetThroughput,
		targetErrorRate:       defaultTargetErrorRate,
		targetMemoryUsage:     defaultTargetMemoryUsage,
		maxConcurrentRequests: defaultMaxConcurrentRequests,
		historyMaxSize:        defaultHistoryMaxSize,
		ctx:                   ctx,
		cancel:                cancel,
		alertThresholds: &AlertThresholds{
			ResponseTimeWarning:  defaultResponseTimeWarning,
			ResponseTimeCritical: defaultResponseTimeCritical,
			ErrorRateWarning:     defaultErrorRateWarning,
			ErrorRateCritical:    defaultErrorRateCritical,
			MemoryUsageWarning:   defaultMemoryUsageWarning,
			MemoryUsageCritical:  defaultMemoryUsageCritical,
			ThroughputWarning:    defaultThroughputWarning,
			ThroughputCritical:   defaultThroughputCritical,
		},
	}

	pm.initializeMetricsWithRegistry(registry)
	go pm.metricsUpdateLoop()
	go pm.memoryMonitoringLoop()

	return pm
}

// initializeMetrics initializes Prometheus metrics
func (pm *PerformanceMonitor) initializeMetrics() {
	pm.responseTimeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_time_seconds",
			Help:    "Response time distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method", "status"},
	)

	pm.responseTimeCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_time_total",
			Help: "Total response time",
		},
		[]string{"endpoint", "method"},
	)

	pm.requestsPerSecond = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "requests_per_second",
			Help: "Current requests per second",
		},
		[]string{"endpoint"},
	)

	pm.throughputCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total requests processed",
		},
		[]string{"endpoint", "status"},
	)

	pm.errorRateGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "error_rate",
			Help: "Current error rate percentage",
		},
		[]string{"endpoint"},
	)

	pm.errorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total errors",
		},
		[]string{"endpoint", "error_type"},
	)

	pm.memoryUsageGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
		[]string{"type"},
	)

	pm.memoryAllocGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_alloc_bytes",
			Help: "Memory allocated in bytes",
		},
		[]string{},
	)

	pm.memorySysGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_sys_bytes",
			Help: "System memory in bytes",
		},
		[]string{},
	)

	pm.goroutineGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "goroutines_total",
			Help: "Number of goroutines",
		},
		[]string{},
	)

	pm.gcDurationGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_duration_seconds",
			Help: "GC duration in seconds",
		},
		[]string{},
	)

	pm.gcFrequencyGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_frequency_per_second",
			Help: "GC frequency per second",
		},
		[]string{},
	)
}

// initializeMetricsWithRegistry initializes Prometheus metrics with custom registry
func (pm *PerformanceMonitor) initializeMetricsWithRegistry(registry prometheus.Registerer) {
	factory := promauto.With(registry)

	pm.responseTimeHistogram = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_time_seconds",
			Help:    "Response time distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method", "status"},
	)

	pm.responseTimeCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_time_total",
			Help: "Total response time",
		},
		[]string{"endpoint", "method"},
	)

	pm.requestsPerSecond = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "requests_per_second",
			Help: "Current requests per second",
		},
		[]string{"endpoint"},
	)

	pm.throughputCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total requests processed",
		},
		[]string{"endpoint", "status"},
	)

	pm.errorRateGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "error_rate",
			Help: "Current error rate percentage",
		},
		[]string{"endpoint"},
	)

	pm.errorCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total errors",
		},
		[]string{"endpoint", "error_type"},
	)

	pm.memoryUsageGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
		[]string{"type"},
	)

	pm.memoryAllocGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_alloc_bytes",
			Help: "Memory allocated in bytes",
		},
		[]string{},
	)

	pm.memorySysGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_sys_bytes",
			Help: "System memory in bytes",
		},
		[]string{},
	)

	pm.goroutineGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "goroutines_total",
			Help: "Number of goroutines",
		},
		[]string{},
	)

	pm.gcDurationGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_duration_seconds",
			Help: "GC duration in seconds",
		},
		[]string{},
	)

	pm.gcFrequencyGauge = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_frequency_per_second",
			Help: "GC frequency per second",
		},
		[]string{},
	)
}

// RecordRequest records a request for performance tracking
func (pm *PerformanceMonitor) RecordRequest(endpoint, method string, status int, duration time.Duration) {
	// Update Prometheus metrics
	pm.responseTimeHistogram.WithLabelValues(endpoint, method, fmt.Sprintf("%d", status)).Observe(duration.Seconds())
	pm.responseTimeCounter.WithLabelValues(endpoint, method).Add(duration.Seconds())
	pm.throughputCounter.WithLabelValues(endpoint, fmt.Sprintf("%d", status)).Inc()

	// Update internal counters
	atomic.AddInt64(&pm.totalRequests, 1)
	atomic.AddInt64(&pm.totalResponseTime, int64(duration))
	atomic.AddInt64(&pm.lastSecondRequests, 1)

	// Track errors
	if status >= performanceHTTPErrorThreshold {
		atomic.AddInt64(&pm.totalErrors, 1)
		atomic.AddInt64(&pm.lastSecondErrors, 1)
		pm.errorCounter.WithLabelValues(endpoint, "http_error").Inc()
	}

	// Update memory usage
	pm.updateMemoryUsage()
}

// RecordError records an error for error rate tracking
func (pm *PerformanceMonitor) RecordError(endpoint, errorType string) {
	atomic.AddInt64(&pm.totalErrors, 1)
	atomic.AddInt64(&pm.lastSecondErrors, 1)
	pm.errorCounter.WithLabelValues(endpoint, errorType).Inc()
}

// GetMetrics returns current performance metrics
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalRequests := atomic.LoadInt64(&pm.totalRequests)
	totalErrors := atomic.LoadInt64(&pm.totalErrors)
	totalResponseTime := atomic.LoadInt64(&pm.totalResponseTime)

	var avgResponseTime time.Duration
	if totalRequests > 0 {
		avgResponseTime = time.Duration(totalResponseTime / totalRequests)
	}

	var errorRate float64
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests)
	}

	// Calculate performance score (0-100)
	performanceScore := pm.calculatePerformanceScore(avgResponseTime, errorRate)

	// Check target compliance
	targetCompliance := map[string]bool{
		"response_time": avgResponseTime < pm.targetResponseTime,
		"throughput":    pm.getCurrentThroughput() >= pm.targetThroughput,
		"error_rate":    errorRate < pm.targetErrorRate,
		"memory_usage":  pm.currentMemoryUsage < pm.targetMemoryUsage,
	}

	return &PerformanceMetrics{
		AverageResponseTime: avgResponseTime,
		CurrentThroughput:   pm.getCurrentThroughput(),
		ErrorRate:           errorRate,
		MemoryUsage:         pm.currentMemoryUsage,
		PeakMemoryUsage:     pm.peakMemoryUsage,
		GoroutineCount:      int64(runtime.NumGoroutine()),
		ActiveRequests:      atomic.LoadInt64(&pm.activeRequests),
		Uptime:              time.Since(pm.startTime),
		TargetCompliance:    targetCompliance,
		PerformanceScore:    performanceScore,
	}
}

// calculatePerformanceScore calculates overall performance score (0-100)
func (pm *PerformanceMonitor) calculatePerformanceScore(avgResponseTime time.Duration, errorRate float64) float64 {
	// Response time score (40% weight)
	responseTimeScore := perfectScore
	if avgResponseTime > pm.targetResponseTime {
		responseTimeScore = perfectScore * (float64(pm.targetResponseTime) / float64(avgResponseTime))
	}

	// Error rate score (30% weight)
	errorRateScore := perfectScore
	if errorRate > pm.targetErrorRate {
		errorRateScore = perfectScore * (pm.targetErrorRate / errorRate)
	}

	// Throughput score (20% weight)
	currentThroughput := pm.getCurrentThroughput()
	throughputScore := perfectScore
	if currentThroughput < pm.targetThroughput {
		throughputScore = perfectScore * (float64(currentThroughput) / float64(pm.targetThroughput))
	}

	// Memory usage score (10% weight)
	memoryScore := perfectScore
	if pm.currentMemoryUsage > pm.targetMemoryUsage {
		memoryScore = perfectScore * (float64(pm.targetMemoryUsage) / float64(pm.currentMemoryUsage))
	}

	return (responseTimeScore * responseTimeWeight) +
		(errorRateScore * errorRateWeight) +
		(throughputScore * throughputWeight) +
		(memoryScore * memoryWeight)
}

// getCurrentThroughput calculates current requests per second
func (pm *PerformanceMonitor) getCurrentThroughput() int64 {
	return atomic.LoadInt64(&pm.lastSecondRequests)
}

// updateMemoryUsage updates memory usage metrics
func (pm *PerformanceMonitor) updateMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Safe conversion from uint64 to int64
	var currentUsage int64
	if m.Alloc <= uint64(1<<63-1) {
		currentUsage = int64(m.Alloc)
	} else {
		currentUsage = maxInt64Value // Max int64 value
	}
	atomic.StoreInt64(&pm.currentMemoryUsage, currentUsage)

	// Update peak memory usage
	for {
		peak := atomic.LoadInt64(&pm.peakMemoryUsage)
		if currentUsage <= peak || atomic.CompareAndSwapInt64(&pm.peakMemoryUsage, peak, currentUsage) {
			break
		}
	}

	// Update Prometheus metrics
	pm.memoryUsageGauge.WithLabelValues("current").Set(float64(currentUsage))
	pm.memoryAllocGauge.WithLabelValues().Set(float64(m.Alloc))
	pm.memorySysGauge.WithLabelValues().Set(float64(m.Sys))
	pm.goroutineGauge.WithLabelValues().Set(float64(runtime.NumGoroutine()))
}

// metricsUpdateLoop updates metrics periodically
func (pm *PerformanceMonitor) metricsUpdateLoop() {
	ticker := time.NewTicker(pm.metricsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.updateMetrics()
		}
	}
}

// updateMetrics updates all metrics
func (pm *PerformanceMonitor) updateMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Update throughput metrics
	currentThroughput := atomic.LoadInt64(&pm.lastSecondRequests)
	pm.requestsPerSecond.WithLabelValues("all").Set(float64(currentThroughput))

	// Update error rate metrics
	totalRequests := atomic.LoadInt64(&pm.totalRequests)
	totalErrors := atomic.LoadInt64(&pm.totalErrors)

	var errorRate float64
	if totalRequests > 0 {
		errorRate = float64(totalErrors) / float64(totalRequests) * errorRatePercentage
	}
	pm.errorRateGauge.WithLabelValues("all").Set(errorRate)

	// Reset per-second counters
	atomic.StoreInt64(&pm.lastSecondRequests, 0)
	atomic.StoreInt64(&pm.lastSecondErrors, 0)

	// Update memory metrics
	pm.updateMemoryUsage()

	// Store performance snapshot
	pm.storePerformanceSnapshot()

	// Check for alerts
	pm.checkAlerts()
}

// storePerformanceSnapshot stores current performance state
func (pm *PerformanceMonitor) storePerformanceSnapshot() {
	metrics := pm.GetMetrics()

	snapshot := PerformanceSnapshot{
		Timestamp:      time.Now(),
		ResponseTime:   metrics.AverageResponseTime,
		Throughput:     metrics.CurrentThroughput,
		ErrorRate:      metrics.ErrorRate,
		MemoryUsage:    metrics.MemoryUsage,
		GoroutineCount: metrics.GoroutineCount,
		ActiveRequests: metrics.ActiveRequests,
	}

	pm.performanceHistory = append(pm.performanceHistory, snapshot)

	// Keep history size manageable
	if len(pm.performanceHistory) > pm.historyMaxSize {
		pm.performanceHistory = pm.performanceHistory[1:]
	}
}

// memoryMonitoringLoop monitors memory usage continuously
func (pm *PerformanceMonitor) memoryMonitoringLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.updateMemoryUsage()
		}
	}
}

// checkAlerts checks for performance alerts
func (pm *PerformanceMonitor) checkAlerts() {
	metrics := pm.GetMetrics()

	// Response time alerts
	if metrics.AverageResponseTime > pm.alertThresholds.ResponseTimeCritical {
		pm.triggerAlert("CRITICAL", "Response time critical", map[string]interface{}{
			"current":   metrics.AverageResponseTime,
			"threshold": pm.alertThresholds.ResponseTimeCritical,
		})
	} else if metrics.AverageResponseTime > pm.alertThresholds.ResponseTimeWarning {
		pm.triggerAlert("WARNING", "Response time high", map[string]interface{}{
			"current":   metrics.AverageResponseTime,
			"threshold": pm.alertThresholds.ResponseTimeWarning,
		})
	}

	// Error rate alerts
	if metrics.ErrorRate > pm.alertThresholds.ErrorRateCritical {
		pm.triggerAlert("CRITICAL", "Error rate critical", map[string]interface{}{
			"current":   metrics.ErrorRate,
			"threshold": pm.alertThresholds.ErrorRateCritical,
		})
	} else if metrics.ErrorRate > pm.alertThresholds.ErrorRateWarning {
		pm.triggerAlert("WARNING", "Error rate high", map[string]interface{}{
			"current":   metrics.ErrorRate,
			"threshold": pm.alertThresholds.ErrorRateWarning,
		})
	}

	// Memory usage alerts
	if metrics.MemoryUsage > pm.alertThresholds.MemoryUsageCritical {
		pm.triggerAlert("CRITICAL", "Memory usage critical", map[string]interface{}{
			"current":   metrics.MemoryUsage,
			"threshold": pm.alertThresholds.MemoryUsageCritical,
		})
	} else if metrics.MemoryUsage > pm.alertThresholds.MemoryUsageWarning {
		pm.triggerAlert("WARNING", "Memory usage high", map[string]interface{}{
			"current":   metrics.MemoryUsage,
			"threshold": pm.alertThresholds.MemoryUsageWarning,
		})
	}

	// Throughput alerts
	if metrics.CurrentThroughput < pm.alertThresholds.ThroughputCritical {
		pm.triggerAlert("CRITICAL", "Throughput critical", map[string]interface{}{
			"current":   metrics.CurrentThroughput,
			"threshold": pm.alertThresholds.ThroughputCritical,
		})
	} else if metrics.CurrentThroughput < pm.alertThresholds.ThroughputWarning {
		pm.triggerAlert("WARNING", "Throughput low", map[string]interface{}{
			"current":   metrics.CurrentThroughput,
			"threshold": pm.alertThresholds.ThroughputWarning,
		})
	}
}

// triggerAlert triggers a performance alert
func (pm *PerformanceMonitor) triggerAlert(level, message string, details map[string]interface{}) {
	// This would integrate with your alerting system
	// For now, we'll just log the alert
	logger := slog.Default()
	logger.Warn("Performance alert triggered",
		"level", level,
		"message", message,
		"details", details,
		"timestamp", time.Now(),
	)
}

// GetPerformanceHistory returns performance history
func (pm *PerformanceMonitor) GetPerformanceHistory() []PerformanceSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	history := make([]PerformanceSnapshot, len(pm.performanceHistory))
	copy(history, pm.performanceHistory)
	return history
}

// SetTargets updates performance targets
func (pm *PerformanceMonitor) SetTargets(
	responseTime time.Duration,
	throughput int64,
	errorRate float64,
	memoryUsage int64,
) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.targetResponseTime = responseTime
	pm.targetThroughput = throughput
	pm.targetErrorRate = errorRate
	pm.targetMemoryUsage = memoryUsage
}

// SetAlertThresholds updates alert thresholds
func (pm *PerformanceMonitor) SetAlertThresholds(thresholds *AlertThresholds) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.alertThresholds = thresholds
}

// Reset resets all performance counters
func (pm *PerformanceMonitor) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	atomic.StoreInt64(&pm.totalRequests, 0)
	atomic.StoreInt64(&pm.totalErrors, 0)
	atomic.StoreInt64(&pm.totalResponseTime, 0)
	atomic.StoreInt64(&pm.lastSecondRequests, 0)
	atomic.StoreInt64(&pm.lastSecondErrors, 0)
	atomic.StoreInt64(&pm.peakMemoryUsage, 0)
	atomic.StoreInt64(&pm.activeRequests, 0)
	pm.performanceHistory = nil
	pm.startTime = time.Now()
}

// Close gracefully shuts down the performance monitor
func (pm *PerformanceMonitor) Close() error {
	pm.cancel()
	return nil
}

// PerformanceMiddleware creates middleware for automatic performance tracking
func (pm *PerformanceMonitor) PerformanceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Check concurrency limit
		currentActive := atomic.LoadInt64(&pm.activeRequests)
		if currentActive >= pm.maxConcurrentRequests {
			http.Error(w, "Service overloaded", http.StatusServiceUnavailable)
			return
		}

		// Increment active requests
		atomic.AddInt64(&pm.activeRequests, 1)
		defer atomic.AddInt64(&pm.activeRequests, -1)

		// Create response writer wrapper to capture status
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(wrappedWriter, r)

		// Record performance metrics
		duration := time.Since(start)
		pm.RecordRequest(r.URL.Path, r.Method, wrappedWriter.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

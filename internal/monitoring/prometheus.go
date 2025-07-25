package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Default timeout for HTTP server
	defaultReadHeaderTimeout = 30 * time.Second
)

// PrometheusMetrics provides Prometheus integration for monitoring
type PrometheusMetrics struct {
	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight *prometheus.GaugeVec

	// Job processing metrics
	jobProcessingTotal    *prometheus.CounterVec
	jobProcessingDuration *prometheus.HistogramVec
	jobQueueSize          *prometheus.GaugeVec
	jobProcessingErrors   *prometheus.CounterVec

	// Cache metrics
	cacheHitsTotal   *prometheus.CounterVec
	cacheMissesTotal *prometheus.CounterVec
	cacheSize        *prometheus.GaugeVec

	// Rate limiting metrics
	rateLimitHitsTotal *prometheus.CounterVec
	rateLimitBlocks    *prometheus.CounterVec

	// System metrics
	systemMemoryUsage *prometheus.GaugeVec
	systemCPUUsage    *prometheus.GaugeVec
	systemGoroutines  *prometheus.GaugeVec

	// Alert metrics
	alertsTotal    *prometheus.CounterVec
	alertsActive   *prometheus.GaugeVec
	alertsResolved *prometheus.CounterVec

	// Registry
	registry *prometheus.Registry

	// HTTP server
	server *http.Server
	logger *slog.Logger
}

// NewPrometheusMetrics creates a new Prometheus metrics collector
func NewPrometheusMetrics(logger *slog.Logger) *PrometheusMetrics {
	registry := prometheus.NewRegistry()

	pm := &PrometheusMetrics{
		registry: registry,
		logger:   logger,
	}

	// Initialize metrics
	pm.initHTTPMetrics()
	pm.initJobMetrics()
	pm.initCacheMetrics()
	pm.initRateLimitMetrics()
	pm.initSystemMetrics()
	pm.initAlertMetrics()

	// Register all metrics
	pm.registerMetrics()

	return pm
}

// initHTTPMetrics initializes HTTP-related metrics
func (pm *PrometheusMetrics) initHTTPMetrics() {
	pm.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	pm.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	pm.httpRequestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
		[]string{"endpoint"},
	)
}

// initJobMetrics initializes job processing metrics
func (pm *PrometheusMetrics) initJobMetrics() {
	pm.jobProcessingTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_processing_total",
			Help: "Total number of jobs processed",
		},
		[]string{"priority", "status"},
	)

	pm.jobProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "job_processing_duration_seconds",
			Help:    "Job processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"priority"},
	)

	pm.jobQueueSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "job_queue_size",
			Help: "Current number of jobs in queue",
		},
		[]string{"priority"},
	)

	pm.jobProcessingErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_processing_errors_total",
			Help: "Total number of job processing errors",
		},
		[]string{"priority", "error_type"},
	)
}

// initCacheMetrics initializes cache metrics
func (pm *PrometheusMetrics) initCacheMetrics() {
	pm.cacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_level"},
	)

	pm.cacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_level"},
	)

	pm.cacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_size",
			Help: "Current number of items in cache",
		},
		[]string{"cache_level"},
	)
}

// initRateLimitMetrics initializes rate limiting metrics
func (pm *PrometheusMetrics) initRateLimitMetrics() {
	pm.rateLimitHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"endpoint", "ip"},
	)

	pm.rateLimitBlocks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_blocks_total",
			Help: "Total number of requests blocked by rate limiting",
		},
		[]string{"endpoint", "ip"},
	)
}

// initSystemMetrics initializes system metrics
func (pm *PrometheusMetrics) initSystemMetrics() {
	pm.systemMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
		[]string{"type"},
	)

	pm.systemCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_cpu_usage_percent",
			Help: "Current CPU usage percentage",
		},
		[]string{"core"},
	)

	pm.systemGoroutines = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_goroutines",
			Help: "Current number of goroutines",
		},
		[]string{},
	)
}

// initAlertMetrics initializes alert metrics
func (pm *PrometheusMetrics) initAlertMetrics() {
	pm.alertsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_total",
			Help: "Total number of alerts",
		},
		[]string{"level", "status"},
	)

	pm.alertsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alerts_active",
			Help: "Current number of active alerts",
		},
		[]string{"level"},
	)

	pm.alertsResolved = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_resolved_total",
			Help: "Total number of resolved alerts",
		},
		[]string{"level"},
	)
}

// registerMetrics registers all metrics with the registry
func (pm *PrometheusMetrics) registerMetrics() {
	// HTTP metrics
	pm.registry.MustRegister(pm.httpRequestsTotal)
	pm.registry.MustRegister(pm.httpRequestDuration)
	pm.registry.MustRegister(pm.httpRequestsInFlight)

	// Job metrics
	pm.registry.MustRegister(pm.jobProcessingTotal)
	pm.registry.MustRegister(pm.jobProcessingDuration)
	pm.registry.MustRegister(pm.jobQueueSize)
	pm.registry.MustRegister(pm.jobProcessingErrors)

	// Cache metrics
	pm.registry.MustRegister(pm.cacheHitsTotal)
	pm.registry.MustRegister(pm.cacheMissesTotal)
	pm.registry.MustRegister(pm.cacheSize)

	// Rate limiting metrics
	pm.registry.MustRegister(pm.rateLimitHitsTotal)
	pm.registry.MustRegister(pm.rateLimitBlocks)

	// System metrics
	pm.registry.MustRegister(pm.systemMemoryUsage)
	pm.registry.MustRegister(pm.systemCPUUsage)
	pm.registry.MustRegister(pm.systemGoroutines)

	// Alert metrics
	pm.registry.MustRegister(pm.alertsTotal)
	pm.registry.MustRegister(pm.alertsActive)
	pm.registry.MustRegister(pm.alertsResolved)
}

// Start starts the Prometheus metrics server
func (pm *PrometheusMetrics) Start(port string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{}))

	pm.server = &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout, // Protection against Slowloris attack
	}

	pm.logger.Info("Starting Prometheus metrics server", "port", port)
	return pm.server.ListenAndServe()
}

// Stop stops the Prometheus metrics server
func (pm *PrometheusMetrics) Stop(ctx context.Context) error {
	if pm.server != nil {
		return pm.server.Shutdown(ctx)
	}
	return nil
}

// RecordHTTPRequest records an HTTP request
func (pm *PrometheusMetrics) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	pm.httpRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", statusCode)).Inc()
	pm.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordHTTPRequestInFlight records an in-flight HTTP request
func (pm *PrometheusMetrics) RecordHTTPRequestInFlight(endpoint string, inFlight int) {
	pm.httpRequestsInFlight.WithLabelValues(endpoint).Set(float64(inFlight))
}

// RecordJobProcessing records job processing metrics
func (pm *PrometheusMetrics) RecordJobProcessing(priority, status string, duration time.Duration) {
	pm.jobProcessingTotal.WithLabelValues(priority, status).Inc()
	pm.jobProcessingDuration.WithLabelValues(priority).Observe(duration.Seconds())
}

// RecordJobQueueSize records the current queue size
func (pm *PrometheusMetrics) RecordJobQueueSize(priority string, size int) {
	pm.jobQueueSize.WithLabelValues(priority).Set(float64(size))
}

// RecordJobError records a job processing error
func (pm *PrometheusMetrics) RecordJobError(priority, errorType string) {
	pm.jobProcessingErrors.WithLabelValues(priority, errorType).Inc()
}

// RecordCacheHit records a cache hit
func (pm *PrometheusMetrics) RecordCacheHit(level string) {
	pm.cacheHitsTotal.WithLabelValues(level).Inc()
}

// RecordCacheMiss records a cache miss
func (pm *PrometheusMetrics) RecordCacheMiss(level string) {
	pm.cacheMissesTotal.WithLabelValues(level).Inc()
}

// RecordCacheSize records the current cache size
func (pm *PrometheusMetrics) RecordCacheSize(level string, size int) {
	pm.cacheSize.WithLabelValues(level).Set(float64(size))
}

// RecordRateLimitHit records a rate limit hit
func (pm *PrometheusMetrics) RecordRateLimitHit(endpoint, ip string) {
	pm.rateLimitHitsTotal.WithLabelValues(endpoint, ip).Inc()
}

// RecordRateLimitBlock records a rate limit block
func (pm *PrometheusMetrics) RecordRateLimitBlock(endpoint, ip string) {
	pm.rateLimitBlocks.WithLabelValues(endpoint, ip).Inc()
}

// RecordSystemMemory records system memory usage
func (pm *PrometheusMetrics) RecordSystemMemory(memoryType string, bytes int64) {
	pm.systemMemoryUsage.WithLabelValues(memoryType).Set(float64(bytes))
}

// RecordSystemCPU records system CPU usage
func (pm *PrometheusMetrics) RecordSystemCPU(core string, percent float64) {
	pm.systemCPUUsage.WithLabelValues(core).Set(percent)
}

// RecordSystemGoroutines records the number of goroutines
func (pm *PrometheusMetrics) RecordSystemGoroutines(count int) {
	pm.systemGoroutines.WithLabelValues().Set(float64(count))
}

// RecordAlert records an alert
func (pm *PrometheusMetrics) RecordAlert(level, status string) {
	pm.alertsTotal.WithLabelValues(level, status).Inc()
}

// RecordActiveAlerts records the number of active alerts
func (pm *PrometheusMetrics) RecordActiveAlerts(level string, count int) {
	pm.alertsActive.WithLabelValues(level).Set(float64(count))
}

// RecordResolvedAlert records a resolved alert
func (pm *PrometheusMetrics) RecordResolvedAlert(level string) {
	pm.alertsResolved.WithLabelValues(level).Inc()
}

// GetRegistry returns the Prometheus registry
func (pm *PrometheusMetrics) GetRegistry() *prometheus.Registry {
	return pm.registry
}

// PrometheusMiddleware creates a middleware that records Prometheus metrics
func PrometheusMiddleware(metrics *PrometheusMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Record in-flight request
			metrics.RecordHTTPRequestInFlight(r.URL.Path, 1)
			defer metrics.RecordHTTPRequestInFlight(r.URL.Path, 0)

			// Create a response writer that captures the status code
			rw := &ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			metrics.RecordHTTPRequest(r.Method, r.URL.Path, rw.StatusCode, duration)
		})
	}
}

// Package monitoring provides comprehensive monitoring and metrics collection
package monitoring

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Default constants for monitoring
const (
	defaultShutdownTimeout = 5 * time.Second
)

// System provides a comprehensive monitoring system
type System struct {
	config             *Config
	logger             *slog.Logger
	webhookMonitor     *WebhookMonitor
	performanceMonitor *PerformanceMonitor
	prometheusMetrics  *PrometheusMetrics
	handler            *Handler
	// mu sync.RWMutex // unused field, commented out
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// Config holds monitoring configuration
type Config struct {
	Enabled                   bool
	Port                      string
	WebhookCheckInterval      time.Duration
	PerformanceUpdateInterval time.Duration
	PrometheusPort            string
	EnableDetailedMetrics     bool
	EnableAlerts              bool
	AlertThresholds           *AlertThresholds
}

// NewSystem creates a new monitoring system
func NewSystem(cfg *Config, logger *slog.Logger) *System {
	ctx, cancel := context.WithCancel(context.Background())

	ms := &System{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize components
	if cfg.Enabled {
		ms.initializeComponents()
	}

	return ms
}

// initializeComponents initializes all monitoring components
func (ms *System) initializeComponents() {
	// Initialize webhook monitor
	ms.webhookMonitor = NewWebhookMonitor(&config.Config{}, ms.logger)

	// Initialize performance monitor
	ms.performanceMonitor = NewPerformanceMonitor(ms.ctx)

	// Initialize Prometheus metrics
	ms.prometheusMetrics = NewPrometheusMetrics(ms.logger)

	// Initialize handler
	ms.handler = NewHandler(ms.webhookMonitor, ms.performanceMonitor, ms.logger)

	// Configure alert thresholds if provided
	if ms.config.AlertThresholds != nil {
		ms.performanceMonitor.SetAlertThresholds(ms.config.AlertThresholds)
	}
}

// Start starts the monitoring system
func (ms *System) Start() error {
	if !ms.config.Enabled {
		ms.logger.Info("Monitoring system disabled")
		return nil
	}

	ms.logger.Info("Starting monitoring system", "port", ms.config.Port)

	// Start webhook monitor
	ms.webhookMonitor.Start()

	// Start Prometheus metrics server if port is configured
	if ms.config.PrometheusPort != "" {
		go ms.startPrometheusServer()
	}

	// Start HTTP server for monitoring endpoints
	ms.startHTTPServer()

	return nil
}

// Stop stops the monitoring system
func (ms *System) Stop() error {
	if !ms.config.Enabled {
		return nil
	}

	ms.logger.Info("Stopping monitoring system")

	// Stop webhook monitor
	ms.webhookMonitor.Stop()

	// Stop performance monitor
	if ms.performanceMonitor != nil {
		if err := ms.performanceMonitor.Close(); err != nil {
			ms.logger.Error("Failed to close performance monitor", "error", err)
		}
	}

	// Stop HTTP server
	if ms.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := ms.server.Shutdown(ctx); err != nil {
			ms.logger.Error("Failed to shutdown server", "error", err)
		}
	}

	// Cancel context
	ms.cancel()

	return nil
}

// startHTTPServer starts the HTTP server for monitoring endpoints
func (ms *System) startHTTPServer() {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", ms.handler.HandleHealth)

	// Status endpoints
	mux.HandleFunc("/monitoring/status", ms.handler.HandleStatus)
	mux.HandleFunc("/monitoring/detailed-status", ms.handler.HandleDetailedStatus)

	// Metrics endpoints
	mux.HandleFunc("/monitoring/metrics", ms.handler.HandleMetrics)
	mux.HandleFunc("/monitoring/performance", ms.handler.HandlePerformance)
	mux.HandleFunc("/monitoring/performance/history", ms.handler.HandlePerformanceHistory)

	// Control endpoints
	mux.HandleFunc("/monitoring/reconnect", ms.handler.HandleReconnect)
	mux.HandleFunc("/monitoring/performance/targets", ms.handler.HandlePerformanceTargets)
	mux.HandleFunc("/monitoring/performance/reset", ms.handler.HandlePerformanceReset)

	// Additional endpoints if detailed metrics are enabled
	if ms.config.EnableDetailedMetrics {
		mux.HandleFunc("/monitoring/system", ms.handleSystemInfo)
		mux.HandleFunc("/monitoring/alerts", ms.handleAlerts)
		mux.HandleFunc("/monitoring/config", ms.handleConfig)
	}

	ms.server = &http.Server{
		Addr:         ":" + ms.config.Port,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ms.logger.Error("Monitoring server error", "error", err)
		}
	}()
}

// startPrometheusServer starts the Prometheus metrics server
func (ms *System) startPrometheusServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(ms.prometheusMetrics.GetRegistry(), promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:         ":" + ms.config.PrometheusPort,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	ms.logger.Info("Starting Prometheus metrics server", "port", ms.config.PrometheusPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ms.logger.Error("Prometheus server error", "error", err)
	}
}

// handleSystemInfo handles system information requests
func (ms *System) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get system information
	systemInfo := map[string]interface{}{
		"version":        "1.0.0",
		"uptime":         time.Since(time.Now()).String(),
		"go_version":     runtime.Version(),
		"num_goroutines": runtime.NumGoroutine(),
		"timestamp":      time.Now().UTC(),
	}

	// Add memory info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	systemInfo["memory"] = map[string]interface{}{
		"allocated":   m.Alloc,
		"total_alloc": m.TotalAlloc,
		"system":      m.Sys,
		"gc_count":    m.NumGC,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"system_info": systemInfo,
	}); err != nil {
		ms.logger.Error("Failed to encode system info response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAlerts handles alert information requests
func (ms *System) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current alerts (this would be implemented based on your alerting system)
	alerts := []map[string]interface{}{
		{
			"id":        "alert-001",
			"level":     "warning",
			"message":   "High response time detected",
			"timestamp": time.Now().UTC(),
			"resolved":  false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"alerts": alerts,
	}); err != nil {
		ms.logger.Error("Failed to encode alerts response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleConfig handles configuration requests
func (ms *System) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := map[string]interface{}{
		"enabled":                     ms.config.Enabled,
		"port":                        ms.config.Port,
		"prometheus_port":             ms.config.PrometheusPort,
		"enable_detailed_metrics":     ms.config.EnableDetailedMetrics,
		"enable_alerts":               ms.config.EnableAlerts,
		"webhook_check_interval":      ms.config.WebhookCheckInterval.String(),
		"performance_update_interval": ms.config.PerformanceUpdateInterval.String(),
		"alert_thresholds":            ms.config.AlertThresholds,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"config": cfg,
	}); err != nil {
		ms.logger.Error("Failed to encode config response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RecordWebhookRequest records a webhook request
func (ms *System) RecordWebhookRequest(endpoint string, success bool, responseTime time.Duration) {
	if ms.webhookMonitor != nil {
		ms.webhookMonitor.RecordRequest(endpoint, success, responseTime)
	}

	if ms.prometheusMetrics != nil {
		ms.prometheusMetrics.RecordHTTPRequest("POST", endpoint, http.StatusOK, responseTime)
	}
}

// RecordHTTPRequest records an HTTP request for metrics
func (ms *System) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	if ms.prometheusMetrics != nil {
		ms.prometheusMetrics.RecordHTTPRequest(method, endpoint, statusCode, duration)
	}

	if ms.performanceMonitor != nil {
		ms.performanceMonitor.RecordRequest(endpoint, method, statusCode, duration)
	}
}

// GetSystemHealth returns overall system health
func (ms *System) GetSystemHealth() map[string]interface{} {
	health := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"status":    "healthy",
	}

	if ms.webhookMonitor != nil {
		webhookStatus := ms.webhookMonitor.GetStatus()
		health["webhooks"] = webhookStatus
		health["webhook_healthy"] = ms.webhookMonitor.IsHealthy()
	}

	if ms.performanceMonitor != nil {
		perfMetrics := ms.performanceMonitor.GetMetrics()
		health["performance"] = perfMetrics
		health["performance_score"] = perfMetrics.PerformanceScore
	}

	return health
}

// GetMetrics returns all available metrics
func (ms *System) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"timestamp": time.Now().UTC(),
	}

	if ms.webhookMonitor != nil {
		metrics["webhooks"] = ms.webhookMonitor.GetMetrics()
	}

	if ms.performanceMonitor != nil {
		metrics["performance"] = ms.performanceMonitor.GetMetrics()
	}

	if ms.prometheusMetrics != nil {
		metrics["prometheus"] = "available"
	}

	return metrics
}

// Middleware creates monitoring middleware
func (ms *System) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Record request
		ms.RecordHTTPRequest(r.Method, r.URL.Path, http.StatusOK, 0)

		// Create response writer wrapper
		wrapped := &monitoringResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Record final metrics
		duration := time.Since(start)
		ms.RecordHTTPRequest(r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// monitoringResponseWriter wraps http.ResponseWriter to capture status code
type monitoringResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *monitoringResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *monitoringResponseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

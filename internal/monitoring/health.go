package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheck represents a health check for a specific component
type HealthCheck struct {
	Name        string                 `json:"name"`
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration_ms"`
}

// HealthReport represents the overall health report
type HealthReport struct {
	OverallStatus HealthStatus           `json:"overall_status"`
	Timestamp     time.Time              `json:"timestamp"`
	Version       string                 `json:"version"`
	Checks        map[string]HealthCheck `json:"checks"`
	SystemInfo    map[string]interface{} `json:"system_info"`
}

// HealthChecker interface for implementing health checks
type HealthChecker interface {
	CheckHealth(ctx context.Context) (HealthStatus, string, map[string]interface{}, error)
}

// HealthMonitor manages health checks for all components
type HealthMonitor struct {
	config    *config.Config
	logger    *slog.Logger
	checks    map[string]HealthChecker
	results   map[string]HealthCheck
	mu        sync.RWMutex
	version   string
	startTime time.Time
	cache     cache.Cache
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(cfg *config.Config, logger *slog.Logger, version string) *HealthMonitor {
	return &HealthMonitor{
		config:    cfg,
		logger:    logger,
		checks:    make(map[string]HealthChecker),
		results:   make(map[string]HealthCheck),
		version:   version,
		startTime: time.Now(),
		cache:     nil, // Can be injected if needed
	}
}

// RegisterChecker registers a health checker for a component
func (hm *HealthMonitor) RegisterChecker(name string, checker HealthChecker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checks[name] = checker
	hm.logger.Info("Registered health checker", "checker", name)
}

// RunHealthChecks runs all registered health checks
func (hm *HealthMonitor) RunHealthChecks(ctx context.Context) HealthReport {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	report := HealthReport{
		OverallStatus: HealthStatusHealthy,
		Timestamp:     time.Now(),
		Version:       hm.version,
		Checks:        make(map[string]HealthCheck),
		SystemInfo:    hm.collectSystemInfo(),
	}

	// Run all health checks
	for name, checker := range hm.checks {
		start := time.Now()
		status, message, details, err := checker.CheckHealth(ctx)
		duration := time.Since(start)

		check := HealthCheck{
			Name:        name,
			Status:      status,
			Message:     message,
			Details:     details,
			LastChecked: time.Now(),
			Duration:    duration,
		}

		if err != nil {
			check.Message = fmt.Sprintf("Health check failed: %v", err)
			check.Status = HealthStatusUnhealthy
		}

		report.Checks[name] = check
		hm.results[name] = check

		// Update overall status
		if status == HealthStatusUnhealthy {
			report.OverallStatus = HealthStatusUnhealthy
		} else if status == HealthStatusDegraded && report.OverallStatus == HealthStatusHealthy {
			report.OverallStatus = HealthStatusDegraded
		}
	}

	return report
}

// GetHealthStatus returns the health status of a specific component
func (hm *HealthMonitor) GetHealthStatus(component string) (HealthCheck, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	check, exists := hm.results[component]
	return check, exists
}

// IsHealthy checks if the overall system is healthy
func (hm *HealthMonitor) IsHealthy() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for _, check := range hm.results {
		if check.Status == HealthStatusUnhealthy {
			return false
		}
	}
	return true
}

// collectSystemInfo collects system information for health reports
func (hm *HealthMonitor) collectSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info["memory"] = map[string]interface{}{
		"allocated_mb":       m.Alloc / 1024 / 1024,
		"total_allocated_mb": m.TotalAlloc / 1024 / 1024,
		"system_memory_mb":   m.Sys / 1024 / 1024,
		"gc_count":           m.NumGC,
		"gc_pause_total_ns":  m.PauseTotalNs,
	}

	// Goroutine count
	info["goroutines"] = runtime.NumGoroutine()

	// CPU info
	info["cpu"] = map[string]interface{}{
		"num_goroutines": runtime.NumGoroutine(),
		"num_cpu":        runtime.NumCPU(),
	}

	// Uptime
	info["uptime"] = time.Since(hm.startTime).String()

	// Application info
	info["app"] = map[string]interface{}{
		"version":    hm.version,
		"start_time": hm.startTime,
		"config": map[string]interface{}{
			"port":                  hm.config.Port,
			"log_level":             hm.config.LogLevel,
			"debug_mode":            hm.config.DebugMode,
			"bidirectional_enabled": hm.config.BidirectionalEnabled,
		},
	}

	return info
}

// SimpleHealthChecker provides a basic health checker implementation
type SimpleHealthChecker struct {
	name      string
	checkFunc func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error)
}

// NewSimpleHealthChecker creates a new simple health checker
func NewSimpleHealthChecker(name string, checkFunc func(ctx context.Context) (HealthStatus, string, map[string]interface{}, error)) *SimpleHealthChecker {
	return &SimpleHealthChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// CheckHealth performs the health check
func (s *SimpleHealthChecker) CheckHealth(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
	return s.checkFunc(ctx)
}

// CacheHealthChecker checks cache health
type CacheHealthChecker struct {
	cache cache.Cache
}

// NewCacheHealthChecker creates a new cache health checker
func NewCacheHealthChecker(cache cache.Cache) *CacheHealthChecker {
	return &CacheHealthChecker{cache: cache}
}

// CheckHealth checks cache connectivity and performance
func (c *CacheHealthChecker) CheckHealth(ctx context.Context) (HealthStatus, string, map[string]interface{}, error) {
	if c.cache == nil {
		return HealthStatusUnhealthy, "Cache is nil", nil, nil
	}

	// Test cache operations
	testKey := "health_check"
	testValue := "test_value"

	// Set value
	c.cache.Set(testKey, testValue, time.Minute)

	// Get value
	result, found := c.cache.Get(testKey)
	if !found {
		return HealthStatusUnhealthy, "Failed to read from cache", nil, fmt.Errorf("cache item not found")
	}

	// Verify the value
	if result != testValue {
		return HealthStatusUnhealthy, "Cache returned incorrect value", nil, fmt.Errorf("expected %s, got %v", testValue, result)
	}

	// Delete value
	c.cache.Delete(testKey)

	// Get cache stats
	stats := c.cache.GetStats()
	details := map[string]interface{}{
		"hits":              stats.Hits,
		"misses":            stats.Misses,
		"evictions":         stats.Evictions,
		"size":              stats.Size,
		"max_size":          stats.MaxSize,
		"hit_rate":          stats.HitRate,
		"memory_usage":      stats.MemoryUsage,
		"tested_operations": []string{"set", "get", "delete"},
	}

	return HealthStatusHealthy, "Cache is healthy", details, nil
}

// HTTPHealthHandler handles HTTP health check requests
type HTTPHealthHandler struct {
	monitor *HealthMonitor
}

// NewHTTPHealthHandler creates a new HTTP health handler
func NewHTTPHealthHandler(monitor *HealthMonitor) *HTTPHealthHandler {
	return &HTTPHealthHandler{monitor: monitor}
}

// HandleHealth handles HTTP health check requests
func (h *HTTPHealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if health check endpoint is requested
	if r.URL.Path == "/health/ready" {
		h.handleReadiness(w, r, ctx)
		return
	}

	// Default to overall health check
	h.handleOverall(w, r, ctx)
}

// handleOverall handles overall health check requests
func (h *HTTPHealthHandler) handleOverall(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	report := h.monitor.RunHealthChecks(ctx)

	statusCode := http.StatusOK
	if report.OverallStatus == HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if report.OverallStatus == HealthStatusDegraded {
		statusCode = http.StatusOK // Or http.StatusPartialContent if preferred
	}

	h.writeJSONResponse(w, statusCode, report)
}

// handleReadiness handles readiness check requests
func (h *HTTPHealthHandler) handleReadiness(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	// For readiness, we might only check critical components
	criticalChecks := []string{
		"cache",
	}

	ready := true
	results := make(map[string]HealthCheck)

	for _, name := range criticalChecks {
		if check, exists := h.monitor.GetHealthStatus(name); exists {
			results[name] = check
			if check.Status == HealthStatusUnhealthy {
				ready = false
			}
		}
	}

	response := map[string]interface{}{
		"ready":   ready,
		"checks":  results,
		"version": h.monitor.version,
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	h.writeJSONResponse(w, statusCode, response)
}

// writeJSONResponse writes a JSON response
func (h *HTTPHealthHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.monitor.logger.Error("Failed to encode health response", "error", err)
	}
}

// Package monitoring provides webhook monitoring and health check functionality.
package monitoring

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const (
	// ReconnectWaitTime is the time to wait for reconnection check to complete
	ReconnectWaitTime = 2 * time.Second

	// StatusHealthy represents a healthy system status
	StatusHealthy = "healthy"
	// StatusUnhealthy represents an unhealthy system status
	StatusUnhealthy = "unhealthy"
	// StatusOK represents a successful operation status
	StatusOK = "ok"
)

// Handler handles monitoring-related HTTP requests
type Handler struct {
	monitor            *WebhookMonitor
	performanceMonitor *PerformanceMonitor
	logger             *slog.Logger
}

// NewHandler creates a new monitoring handler
func NewHandler(monitor *WebhookMonitor, performanceMonitor *PerformanceMonitor, logger *slog.Logger) *Handler {
	return &Handler{
		monitor:            monitor,
		performanceMonitor: performanceMonitor,
		logger:             logger,
	}
}

// HandleStatus handles requests for webhook status information
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.monitor.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    StatusOK,
		"timestamp": time.Now().UTC(),
		"endpoints": status,
		"healthy":   h.monitor.IsHealthy(),
	}); err != nil {
		h.logger.Error("Failed to encode status response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleMetrics handles requests for webhook metrics
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := h.monitor.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    StatusOK,
		"timestamp": time.Now().UTC(),
		"metrics":   metrics,
	}); err != nil {
		h.logger.Error("Failed to encode metrics response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleHealth handles detailed health check requests
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.monitor.GetStatus()
	metrics := h.monitor.GetMetrics()
	unhealthy := h.monitor.GetUnhealthyEndpoints()

	// Determine overall health status
	overallStatus := StatusHealthy
	statusCode := http.StatusOK

	if len(unhealthy) > 0 {
		overallStatus = StatusUnhealthy
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":              overallStatus,
		"timestamp":           time.Now().UTC(),
		"endpoints":           status,
		"metrics":             metrics,
		"unhealthy_endpoints": unhealthy,
		"total_endpoints":     len(status),
		"healthy_endpoints":   len(status) - len(unhealthy),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleDetailedStatus handles requests for detailed status of a specific endpoint
func (h *Handler) HandleDetailedStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract endpoint from query parameter
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		http.Error(w, "endpoint parameter is required", http.StatusBadRequest)
		return
	}

	status := h.monitor.GetStatus()
	endpointStatus, exists := status[endpoint]

	if !exists {
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	metrics := h.monitor.GetMetrics()
	endpointMetrics, exists := metrics[endpoint]

	response := map[string]interface{}{
		"status":          StatusOK,
		"timestamp":       time.Now().UTC(),
		"endpoint":        endpoint,
		"endpoint_status": endpointStatus,
	}

	if exists {
		response["metrics"] = endpointMetrics
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode detailed status response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleReconnect handles manual reconnection requests
func (h *Handler) HandleReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract endpoint from query parameter
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		http.Error(w, "endpoint parameter is required", http.StatusBadRequest)
		return
	}

	// Force a check of the specific endpoint
	go h.monitor.checkEndpoint(endpoint)

	// Wait a bit for the check to complete
	time.Sleep(ReconnectWaitTime)

	// Get updated status
	status := h.monitor.GetStatus()
	endpointStatus, exists := status[endpoint]

	if !exists {
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"status":          StatusOK,
		"timestamp":       time.Now().UTC(),
		"endpoint":        endpoint,
		"endpoint_status": endpointStatus,
		"message":         "Reconnection check completed",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode reconnect response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handlePerformanceResponse is a helper function to reduce code duplication
func (h *Handler) handlePerformanceResponse(w http.ResponseWriter, data interface{}, dataKey string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":    StatusOK,
		"timestamp": time.Now().UTC(),
		dataKey:     data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode performance response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandlePerformance handles requests for performance metrics
func (h *Handler) HandlePerformance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.performanceMonitor == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	metrics := h.performanceMonitor.GetMetrics()
	h.handlePerformanceResponse(w, metrics, "metrics")
}

// HandlePerformanceHistory handles requests for performance history
func (h *Handler) HandlePerformanceHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.performanceMonitor == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	history := h.performanceMonitor.GetPerformanceHistory()
	h.handlePerformanceResponse(w, history, "history")
}

// HandlePerformanceTargets handles requests to update performance targets
func (h *Handler) HandlePerformanceTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.performanceMonitor == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	var request struct {
		ResponseTime time.Duration `json:"response_time"`
		Throughput   int64         `json:"throughput"`
		ErrorRate    float64       `json:"error_rate"`
		MemoryUsage  int64         `json:"memory_usage"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.performanceMonitor.SetTargets(
		request.ResponseTime,
		request.Throughput,
		request.ErrorRate,
		request.MemoryUsage,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    StatusOK,
		"timestamp": time.Now().UTC(),
		"message":   "Performance targets updated successfully",
	}); err != nil {
		h.logger.Error("Failed to encode performance targets response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandlePerformanceReset handles requests to reset performance counters
func (h *Handler) HandlePerformanceReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.performanceMonitor == nil {
		http.Error(w, "Performance monitoring not available", http.StatusServiceUnavailable)
		return
	}

	h.performanceMonitor.Reset()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    StatusOK,
		"timestamp": time.Now().UTC(),
		"message":   "Performance counters reset successfully",
	}); err != nil {
		h.logger.Error("Failed to encode performance reset response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

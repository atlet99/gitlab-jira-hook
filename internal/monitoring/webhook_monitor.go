// Package monitoring provides webhook monitoring and health check functionality.
package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests
	DefaultHTTPTimeout = 30 * time.Second
	// DefaultCheckInterval is the default interval for health checks
	DefaultCheckInterval = 30 * time.Second
	// AverageResponseTimeDivisor is used for calculating average response time
	AverageResponseTimeDivisor = 2

	// StatusUnknown represents an unknown endpoint status
	StatusUnknown = "unknown"
)

// WebhookStatus represents the status of a webhook endpoint
type WebhookStatus struct {
	Endpoint     string        `json:"endpoint"`
	Status       string        `json:"status"` // StatusHealthy, StatusUnhealthy, StatusUnknown
	LastCheck    time.Time     `json:"last_check"`
	LastSuccess  time.Time     `json:"last_success,omitempty"`
	LastFailure  time.Time     `json:"last_failure,omitempty"`
	FailureCount int           `json:"failure_count"`
	ResponseTime time.Duration `json:"response_time,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// WebhookMetrics represents metrics for webhook monitoring
type WebhookMetrics struct {
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastRequestTime     time.Time     `json:"last_request_time"`
	Uptime              time.Duration `json:"uptime"`
}

// WebhookMonitor monitors webhook endpoints and provides health checks
type WebhookMonitor struct {
	config     *config.Config
	logger     *slog.Logger
	statuses   map[string]*WebhookStatus
	metrics    map[string]*WebhookMetrics
	mu         sync.RWMutex
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWebhookMonitor creates a new webhook monitor
func NewWebhookMonitor(cfg *config.Config, logger *slog.Logger) *WebhookMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebhookMonitor{
		config:   cfg,
		logger:   logger,
		statuses: make(map[string]*WebhookStatus),
		metrics:  make(map[string]*WebhookMetrics),
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the webhook monitoring
func (m *WebhookMonitor) Start() {
	m.logger.Info("Starting webhook monitor")

	// Initialize statuses for known endpoints
	m.initializeEndpoints()

	// Start monitoring goroutine
	go m.monitorLoop()
}

// Stop stops the webhook monitoring
func (m *WebhookMonitor) Stop() {
	m.logger.Info("Stopping webhook monitor")
	m.cancel()
}

// initializeEndpoints initializes monitoring for known webhook endpoints
func (m *WebhookMonitor) initializeEndpoints() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// System hook endpoint
	m.statuses["/gitlab-hook"] = &WebhookStatus{
		Endpoint:  "/gitlab-hook",
		Status:    StatusUnknown,
		LastCheck: time.Now(),
	}

	// Project hook endpoint
	m.statuses["/gitlab-project-hook"] = &WebhookStatus{
		Endpoint:  "/gitlab-project-hook",
		Status:    StatusUnknown,
		LastCheck: time.Now(),
	}

	// Health check endpoint
	m.statuses["/health"] = &WebhookStatus{
		Endpoint:  "/health",
		Status:    StatusUnknown,
		LastCheck: time.Now(),
	}

	// Initialize metrics
	for endpoint := range m.statuses {
		m.metrics[endpoint] = &WebhookMetrics{
			LastRequestTime: time.Now(),
		}
	}
}

// monitorLoop runs the main monitoring loop
func (m *WebhookMonitor) monitorLoop() {
	ticker := time.NewTicker(DefaultCheckInterval) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllEndpoints()
		}
	}
}

// checkAllEndpoints checks the health of all monitored endpoints
func (m *WebhookMonitor) checkAllEndpoints() {
	m.mu.RLock()
	endpoints := make([]string, 0, len(m.statuses))
	for endpoint := range m.statuses {
		endpoints = append(endpoints, endpoint)
	}
	m.mu.RUnlock()

	for _, endpoint := range endpoints {
		go m.checkEndpoint(endpoint)
	}
}

// checkEndpoint checks the health of a specific endpoint
func (m *WebhookMonitor) checkEndpoint(endpoint string) {
	start := time.Now()

	// Create request to local health check
	url := fmt.Sprintf("http://localhost:%s%s", m.config.Port, endpoint)
	req, err := http.NewRequestWithContext(m.ctx, "GET", url, http.NoBody)
	if err != nil {
		m.updateStatus(endpoint, "unhealthy", time.Since(start), err.Error())
		return
	}

	// Add headers for webhook endpoints
	if endpoint == "/gitlab-hook" || endpoint == "/gitlab-project-hook" {
		req.Header.Set("X-Gitlab-Token", m.config.GitLabSecret)
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.updateStatus(endpoint, "unhealthy", time.Since(start), err.Error())
		return
	}

	// Ensure response body is closed
	if resp.Body != nil {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				m.logger.Warn("Failed to close response body", "error", closeErr)
			}
		}()
	}

	responseTime := time.Since(start)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		m.updateStatus(endpoint, "healthy", responseTime, "")
	} else {
		m.updateStatus(endpoint, "unhealthy", responseTime, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

// updateStatus updates the status of an endpoint
func (m *WebhookMonitor) updateStatus(endpoint, status string, responseTime time.Duration, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	webhookStatus, exists := m.statuses[endpoint]
	if !exists {
		return
	}

	webhookStatus.Status = status
	webhookStatus.LastCheck = time.Now()
	webhookStatus.ResponseTime = responseTime

	if status == StatusHealthy {
		webhookStatus.LastSuccess = time.Now()
		webhookStatus.Error = ""
	} else {
		webhookStatus.LastFailure = time.Now()
		webhookStatus.FailureCount++
		webhookStatus.Error = errorMsg
	}

	// Update metrics
	metrics, exists := m.metrics[endpoint]
	if !exists {
		metrics = &WebhookMetrics{}
		m.metrics[endpoint] = metrics
	}

	metrics.TotalRequests++
	metrics.LastRequestTime = time.Now()

	if status == StatusHealthy {
		metrics.SuccessfulRequests++
	} else {
		metrics.FailedRequests++
	}

	// Update average response time
	if metrics.AverageResponseTime == 0 {
		metrics.AverageResponseTime = responseTime
	} else {
		metrics.AverageResponseTime = (metrics.AverageResponseTime + responseTime) / AverageResponseTimeDivisor
	}

	m.logger.Debug("Updated webhook status",
		"endpoint", endpoint,
		"status", status,
		"responseTime", responseTime,
		"error", errorMsg)
}

// RecordRequest records a webhook request for metrics
func (m *WebhookMonitor) RecordRequest(endpoint string, success bool, responseTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[endpoint]
	if !exists {
		metrics = &WebhookMetrics{}
		m.metrics[endpoint] = metrics
	}

	metrics.TotalRequests++
	metrics.LastRequestTime = time.Now()

	if success {
		metrics.SuccessfulRequests++
	} else {
		metrics.FailedRequests++
	}

	// Update average response time
	if metrics.AverageResponseTime == 0 {
		metrics.AverageResponseTime = responseTime
	} else {
		metrics.AverageResponseTime = (metrics.AverageResponseTime + responseTime) / AverageResponseTimeDivisor
	}
}

// GetStatus returns the current status of all endpoints
func (m *WebhookMonitor) GetStatus() map[string]*WebhookStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*WebhookStatus)
	for endpoint, status := range m.statuses {
		// Create a copy to avoid race conditions
		statusCopy := *status
		result[endpoint] = &statusCopy
	}

	return result
}

// GetMetrics returns the current metrics for all endpoints
func (m *WebhookMonitor) GetMetrics() map[string]*WebhookMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*WebhookMetrics)
	for endpoint, metrics := range m.metrics {
		// Create a copy to avoid race conditions
		metricsCopy := *metrics
		metricsCopy.Uptime = time.Since(metrics.LastRequestTime)
		result[endpoint] = &metricsCopy
	}

	return result
}

// IsHealthy returns true if all endpoints are healthy
func (m *WebhookMonitor) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, status := range m.statuses {
		if status.Status != StatusHealthy {
			return false
		}
	}

	return true
}

// GetUnhealthyEndpoints returns a list of unhealthy endpoints
func (m *WebhookMonitor) GetUnhealthyEndpoints() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var unhealthy []string
	for endpoint, status := range m.statuses {
		if status.Status != StatusHealthy {
			unhealthy = append(unhealthy, endpoint)
		}
	}

	return unhealthy
}

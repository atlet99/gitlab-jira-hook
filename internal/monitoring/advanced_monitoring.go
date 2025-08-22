// Package monitoring provides comprehensive monitoring and metrics collection
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Default monitoring constants
const (
	defaultReadTimeout         = 30 * time.Second
	defaultWriteTimeout        = 30 * time.Second
	defaultAlertChanSize       = 100
	defaultAlertHistoryMaxSize = 1000
)

// AdvancedMonitor provides advanced monitoring capabilities
type AdvancedMonitor struct {
	// Configuration
	config *AdvancedConfig

	// Core components
	logger *slog.Logger

	// Metrics
	metrics *AdvancedMetrics

	// Alerts
	alerts *AlertManager

	// Dashboards
	dashboards map[string]*Dashboard

	// Test mode
	testMode bool

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	mu sync.RWMutex
}

// AdvancedConfig holds configuration for advanced monitoring
type AdvancedConfig struct {
	// Basic settings
	Enabled          bool
	Port             string
	PrometheusPort   string
	EnableAlerting   bool
	EnableDashboards bool

	// Performance settings
	MetricsRetention   time.Duration
	AlertCheckInterval time.Duration

	// Export settings
	EnableExport bool
	ExportFormat string // "json", "prometheus", "csv"
}

// AdvancedMetrics provides advanced metrics collection
type AdvancedMetrics struct {
	// Metric storage
	metrics map[string]*Metric

	// Prometheus registry
	registry *prometheus.Registry

	// Synchronization
	mu sync.RWMutex
}

// Metric represents a single metric
type Metric struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// MetricType represents the type of a metric
type MetricType int

const (
	// MetricTypeCounter represents a counter metric type
	MetricTypeCounter MetricType = iota
	// MetricTypeGauge represents a gauge metric type
	MetricTypeGauge
	// MetricTypeHistogram represents a histogram metric type
	MetricTypeHistogram
	// MetricTypeSummary represents a summary metric type
	MetricTypeSummary
)

// AlertManager manages alerts
type AlertManager struct {
	// Active alerts
	activeAlerts map[string]*Alert

	// Alert rules
	alertRules map[string]*AlertRule

	// Alert history
	alertHistory []Alert

	// Alert channel for real-time notifications
	alertChan chan *Alert

	// Synchronization
	mu sync.RWMutex
}

// Alert represents an active alert
type Alert struct {
	ID           string
	Name         string
	Description  string
	Level        AlertLevel
	Metric       string
	CurrentValue float64
	Threshold    float64
	Condition    string
	Timestamp    time.Time
	Resolved     bool
	ResolvedAt   time.Time
}

// AlertLevel represents the severity level of an alert
type AlertLevel int

const (
	// AlertLevelInfo represents an informational alert level
	AlertLevelInfo AlertLevel = iota
	// AlertLevelWarning represents a warning alert level
	AlertLevelWarning
	// AlertLevelError represents an error alert level
	AlertLevelError
	// AlertLevelCritical represents a critical alert level
	AlertLevelCritical
)

// AlertRule represents an alert rule
type AlertRule struct {
	ID          string
	Name        string
	Description string
	Metric      string
	Condition   string
	Threshold   float64
	Level       AlertLevel
	Enabled     bool
}

// Dashboard represents a monitoring dashboard
type Dashboard struct {
	ID          string
	Name        string
	Description string
	Panels      []*DashboardPanel
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// DashboardPanel represents a panel in a dashboard
type DashboardPanel struct {
	ID       string
	Title    string
	Type     string
	Metrics  []string
	Config   map[string]interface{}
	Position PanelPosition
}

// PanelPosition represents the position and size of a panel
type PanelPosition struct {
	X      int
	Y      int
	Width  int
	Height int
}

// NewAdvancedMonitor creates a new advanced monitor
func NewAdvancedMonitor(config *AdvancedConfig, logger *slog.Logger) *AdvancedMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	am := &AdvancedMonitor{
		config:     config,
		logger:     logger,
		metrics:    NewAdvancedMetrics(),
		alerts:     NewAlertManager(),
		dashboards: make(map[string]*Dashboard),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize components
	if config.Enabled {
		am.initializeComponents()
	}

	return am
}

// initializeComponents initializes all monitoring components
func (am *AdvancedMonitor) initializeComponents() {
	// Initialize metrics
	am.metrics = NewAdvancedMetrics()

	// Initialize alerts
	am.alerts = NewAlertManager()

	// Initialize dashboards
	am.dashboards = make(map[string]*Dashboard)

	// Start background processes
	go am.metricsCleanupLoop()
	go am.alertProcessingLoop()
}

// Start starts the advanced monitor
func (am *AdvancedMonitor) Start() error {
	if !am.config.Enabled {
		am.logger.Info("Advanced monitoring disabled")
		return nil
	}

	am.logger.Info("Starting advanced monitoring system", "port", am.config.Port)

	// Start HTTP server
	go am.startHTTPServer()

	return nil
}

// Stop stops the advanced monitor
func (am *AdvancedMonitor) Stop() error {
	if !am.config.Enabled {
		return nil
	}

	am.logger.Info("Stopping advanced monitoring system")

	// Cancel context
	am.cancel()

	return nil
}

// SetTestMode enables or disables test mode
func (am *AdvancedMonitor) SetTestMode(enabled bool) {
	am.testMode = enabled
}

// Metrics recording methods

// RecordCounter records a counter metric
func (am *AdvancedMonitor) RecordCounter(name string, value float64, labels map[string]string) {
	am.metrics.RecordCounter(name, value, labels)
}

// RecordGauge records a gauge metric
func (am *AdvancedMonitor) RecordGauge(name string, value float64, labels map[string]string) {
	am.metrics.RecordGauge(name, value, labels)
}

// RecordHistogram records a histogram metric
func (am *AdvancedMonitor) RecordHistogram(name string, value float64, labels map[string]string) {
	am.metrics.RecordHistogram(name, value, labels)
}

// GetMetric retrieves a specific metric
func (am *AdvancedMonitor) GetMetric(name string, labels map[string]string) *Metric {
	return am.metrics.GetMetric(name, labels)
}

// GetMetrics retrieves all metrics
func (am *AdvancedMonitor) GetMetrics() []*Metric {
	return am.metrics.GetAllMetrics()
}

// Alert management methods

// AddAlertRule adds an alert rule
func (am *AdvancedMonitor) AddAlertRule(rule *AlertRule) {
	am.alerts.AddAlertRule(rule)
}

// RemoveAlertRule removes an alert rule
func (am *AdvancedMonitor) RemoveAlertRule(id string) {
	am.alerts.RemoveAlertRule(id)
}

// GetAlertRules returns all alert rules
func (am *AdvancedMonitor) GetAlertRules() []*AlertRule {
	return am.alerts.GetAlertRules()
}

// GetActiveAlerts returns all active alerts
func (am *AdvancedMonitor) GetActiveAlerts() []*Alert {
	return am.alerts.GetActiveAlerts()
}

// GetAlertChannel returns the alert notification channel
func (am *AdvancedMonitor) GetAlertChannel() chan *Alert {
	return am.alerts.GetAlertChannel()
}

// Dashboard management methods

// CreateDashboard creates a new dashboard
func (am *AdvancedMonitor) CreateDashboard(name, description string) *Dashboard {
	dashboard := &Dashboard{
		ID:          generateDashboardID(),
		Name:        name,
		Description: description,
		Panels:      make([]*DashboardPanel, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	am.dashboards[dashboard.ID] = dashboard
	return dashboard
}

// GetDashboard retrieves a dashboard by ID
func (am *AdvancedMonitor) GetDashboard(id string) *Dashboard {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return am.dashboards[id]
}

// GetDashboards returns all dashboards
func (am *AdvancedMonitor) GetDashboards() []*Dashboard {
	am.mu.RLock()
	defer am.mu.RUnlock()

	dashboards := make([]*Dashboard, 0, len(am.dashboards))
	for _, dashboard := range am.dashboards {
		dashboards = append(dashboards, dashboard)
	}

	return dashboards
}

// AddPanel adds a panel to a dashboard
func (am *AdvancedMonitor) AddPanel(dashboardID string, panel *DashboardPanel) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	dashboard, exists := am.dashboards[dashboardID]
	if !exists {
		return fmt.Errorf("dashboard not found")
	}

	panel.ID = generatePanelID()
	dashboard.Panels = append(dashboard.Panels, panel)
	dashboard.UpdatedAt = time.Now()

	return nil
}

// Export methods

// ExportMetrics exports metrics in the configured format
func (am *AdvancedMonitor) ExportMetrics() string {
	switch am.config.ExportFormat {
	case "json":
		return am.exportMetricsJSON()
	case "prometheus":
		return am.exportMetricsPrometheus()
	case "csv":
		return am.exportMetricsCSV()
	default:
		return am.exportMetricsJSON()
	}
}

// ExportAlerts exports alerts in JSON format
func (am *AdvancedMonitor) ExportAlerts() ([]byte, error) {
	return json.Marshal(am.alerts.GetAlertHistory())
}

// Health status methods

// GetHealthStatus returns the current health status
func (am *AdvancedMonitor) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":           "healthy",
		"active_alerts":    0,
		"critical_alerts":  0,
		"total_metrics":    len(am.metrics.GetAllMetrics()),
		"total_rules":      len(am.alerts.GetAlertRules()),
		"total_dashboards": len(am.dashboards),
	}

	// Check for active alerts
	activeAlerts := am.alerts.GetActiveAlerts()
	status["active_alerts"] = len(activeAlerts)

	// Check for critical alerts
	criticalCount := 0
	for _, alert := range activeAlerts {
		if alert.Level == AlertLevelCritical {
			criticalCount++
		}
	}
	status["critical_alerts"] = criticalCount

	// Update status based on alert levels
	if criticalCount > 0 {
		status["status"] = "critical"
	} else if len(activeAlerts) > 0 {
		status["status"] = "warning"
	}

	return status
}

// Background processing methods

// metricsCleanupLoop cleans up old metrics periodically
func (am *AdvancedMonitor) metricsCleanupLoop() {
	ticker := time.NewTicker(am.config.MetricsRetention)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.metrics.CleanupOldMetrics(am.config.MetricsRetention)
		}
	}
}

// alertProcessingLoop processes alerts periodically
func (am *AdvancedMonitor) alertProcessingLoop() {
	ticker := time.NewTicker(am.config.AlertCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.alerts.CheckAlertRules(am.metrics)
		}
	}
}

// HTTP server methods

// startHTTPServer starts the HTTP server for monitoring endpoints
func (am *AdvancedMonitor) startHTTPServer() {
	mux := http.NewServeMux()

	// Metrics endpoints
	mux.HandleFunc("/metrics", am.handleMetrics)
	mux.HandleFunc("/metrics/prometheus", am.handlePrometheusMetrics)

	// Alert endpoints
	mux.HandleFunc("/alerts", am.handleAlerts)
	mux.HandleFunc("/alerts/rules", am.handleAlertRules)

	// Dashboard endpoints
	mux.HandleFunc("/dashboards", am.handleDashboards)
	mux.HandleFunc("/dashboards/", am.handleDashboard)

	// Export endpoints
	if am.config.EnableExport {
		mux.HandleFunc("/export/metrics", am.handleExportMetrics)
		mux.HandleFunc("/export/alerts", am.handleExportAlerts)
	}

	// Health endpoint
	mux.HandleFunc("/health", am.handleHealth)

	server := &http.Server{
		Addr:         ":" + am.config.Port,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	am.logger.Info("Starting advanced monitoring server", "port", am.config.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		am.logger.Error("Advanced monitoring server error", "error", err)
	}
}

// HTTP handlers

// handleBasicGetRequest is a shared function for handling basic GET requests
func (am *AdvancedMonitor) handleBasicGetRequest(
	w http.ResponseWriter, r *http.Request, endpointName string,
	_ interface{}, getData func() interface{},
) {
	// Basic handler with standard logging
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	am.logger.Debug(fmt.Sprintf("Processing %s request", endpointName), "remote_addr", r.RemoteAddr)

	// Get data using the provided function
	// Get data using the provided function
	result := getData()
	// Use the result from getData function
	_ = result

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		am.logger.Error(fmt.Sprintf("Failed to encode %s response", endpointName), "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (am *AdvancedMonitor) handleMetrics(w http.ResponseWriter, r *http.Request) {
	am.handleBasicGetRequest(w, r, "metrics", am.GetMetrics(), func() interface{} { return am.GetMetrics() })
}

func (am *AdvancedMonitor) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve Prometheus metrics
	registry := am.metrics.GetRegistry()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(w, r)
}

func (am *AdvancedMonitor) handleAlerts(w http.ResponseWriter, r *http.Request) {
	am.handleBasicGetRequest(w, r, "alerts", am.GetActiveAlerts(), func() interface{} { return am.GetActiveAlerts() })
}

func (am *AdvancedMonitor) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	am.handleBasicGetRequest(w, r, "alert rules", am.GetAlertRules(), func() interface{} { return am.GetAlertRules() })
}

func (am *AdvancedMonitor) handleDashboards(w http.ResponseWriter, r *http.Request) {
	am.handleBasicGetRequest(w, r, "dashboards", am.GetDashboards(), func() interface{} { return am.GetDashboards() })
}

func (am *AdvancedMonitor) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract dashboard ID from path
	// This is a simplified implementation - in production you'd use a proper router
	dashboardID := r.URL.Path[len("/dashboards/"):]
	if dashboardID == "" {
		http.Error(w, "Dashboard ID required", http.StatusBadRequest)
		return
	}

	dashboard := am.GetDashboard(dashboardID)
	if dashboard == nil {
		http.Error(w, "Dashboard not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(dashboard); err != nil {
		am.logger.Error("Failed to encode dashboard response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (am *AdvancedMonitor) handleExportMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	export := am.ExportMetrics()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(export)); err != nil {
		am.logger.Error("Failed to write export response", "error", err)
	}
}

func (am *AdvancedMonitor) handleExportAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	export, err := am.ExportAlerts()
	if err != nil {
		am.logger.Error("Failed to export alerts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(export); err != nil {
		am.logger.Error("Failed to write export response", "error", err)
	}
}

func (am *AdvancedMonitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	am.handleBasicGetRequest(w, r, "health", am.GetHealthStatus(), func() interface{} { return am.GetHealthStatus() })
}

// Export implementations

func (am *AdvancedMonitor) exportMetricsJSON() string {
	metrics := am.GetMetrics()

	data := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"metrics":   metrics,
		"total":     len(metrics),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		am.logger.Error("Failed to marshal metrics for export", "error", err)
		return fmt.Sprintf(`{"error": "Failed to export metrics: %v"}`, err)
	}
	return string(jsonData)
}

func (am *AdvancedMonitor) exportMetricsPrometheus() string {
	// This would implement Prometheus format export
	// For now, return a placeholder
	return "# Prometheus metrics export not implemented"
}

func (am *AdvancedMonitor) exportMetricsCSV() string {
	// This would implement CSV format export
	// For now, return a placeholder
	return "name,type,value,timestamp\n"
}

// Alert condition evaluation

func (am *AdvancedMonitor) evaluateCondition(value, condition string, threshold float64) bool {
	valueFloat := parseFloat(value)

	switch condition {
	case ">":
		return valueFloat > threshold
	case ">=":
		return valueFloat >= threshold
	case "<":
		return valueFloat < threshold
	case "<=":
		return valueFloat <= threshold
	case "==":
		return valueFloat == threshold
	case "!=":
		return valueFloat != threshold
	default:
		return false
	}
}

// Helper functions

func parseFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		return 0
	}
	return f
}

func generateDashboardID() string {
	return fmt.Sprintf("dashboard-%d", time.Now().UnixNano())
}

func generatePanelID() string {
	return fmt.Sprintf("panel-%d", time.Now().UnixNano())
}

// AdvancedMetrics methods

// NewAdvancedMetrics creates a new advanced metrics collector
func NewAdvancedMetrics() *AdvancedMetrics {
	return &AdvancedMetrics{
		metrics:  make(map[string]*Metric),
		registry: prometheus.NewRegistry(),
	}
}

// RecordCounter records a counter metric
// recordMetric is a shared function to record metrics with common logic
func (am *AdvancedMetrics) recordMetric(name string, value float64, labels map[string]string, metricType MetricType) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.generateMetricKey(name, labels)

	if existing, exists := am.metrics[key]; exists && existing.Type == metricType {
		if metricType == MetricTypeCounter {
			existing.Value += value
		} else {
			existing.Value = value
		}
		existing.Timestamp = time.Now()
	} else {
		am.metrics[key] = &Metric{
			Name:      name,
			Type:      metricType,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// RecordCounter records a counter metric with the given name, value, and labels
func (am *AdvancedMetrics) RecordCounter(name string, value float64, labels map[string]string) {
	// Record counter metric with atomic operation and thread safety
	am.recordMetric(name, value, labels, MetricTypeCounter)
}

// RecordGauge records a gauge metric
func (am *AdvancedMetrics) RecordGauge(name string, value float64, labels map[string]string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.generateMetricKey(name, labels)
	am.metrics[key] = &Metric{
		Name:      name,
		Type:      MetricTypeGauge,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// RecordHistogram records a histogram metric
func (am *AdvancedMetrics) RecordHistogram(name string, value float64, labels map[string]string) {
	// Record histogram metric with atomic operation and thread safety
	am.recordMetric(name, value, labels, MetricTypeHistogram)
}

// GetMetric retrieves a specific metric
func (am *AdvancedMetrics) GetMetric(name string, labels map[string]string) *Metric {
	am.mu.RLock()
	defer am.mu.RUnlock()

	key := am.generateMetricKey(name, labels)
	return am.metrics[key]
}

// GetAllMetrics retrieves all metrics
func (am *AdvancedMetrics) GetAllMetrics() []*Metric {
	am.mu.RLock()
	defer am.mu.RUnlock()

	metrics := make([]*Metric, 0, len(am.metrics))
	for _, metric := range am.metrics {
		metrics = append(metrics, metric)
	}

	return metrics
}

// CleanupOldMetrics removes metrics older than the specified duration
func (am *AdvancedMetrics) CleanupOldMetrics(retention time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().Add(-retention)

	for key, metric := range am.metrics {
		if metric.Timestamp.Before(cutoff) {
			delete(am.metrics, key)
		}
	}
}

// GetRegistry returns the Prometheus registry
func (am *AdvancedMetrics) GetRegistry() *prometheus.Registry {
	return am.registry
}

// generateMetricKey generates a unique key for a metric
func (am *AdvancedMetrics) generateMetricKey(name string, labels map[string]string) string {
	// Simple key generation - in production you'd want something more robust
	return fmt.Sprintf("%s-%v", name, labels)
}

// AlertManager methods

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		activeAlerts: make(map[string]*Alert),
		alertRules:   make(map[string]*AlertRule),
		alertHistory: make([]Alert, 0),
		alertChan:    make(chan *Alert, defaultAlertChanSize),
	}
}

// AddAlertRule adds an alert rule
func (am *AlertManager) AddAlertRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.alertRules[rule.ID] = rule
}

// RemoveAlertRule removes an alert rule
func (am *AlertManager) RemoveAlertRule(id string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.alertRules, id)
}

// GetAlertRules returns all alert rules
func (am *AlertManager) GetAlertRules() []*AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(am.alertRules))
	for _, rule := range am.alertRules {
		rules = append(rules, rule)
	}

	return rules
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, alert)
	}

	return alerts
}

// GetAlertChannel returns the alert notification channel
func (am *AlertManager) GetAlertChannel() chan *Alert {
	return am.alertChan
}

// GetAlertHistory returns alert history
func (am *AlertManager) GetAlertHistory() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Return a copy to avoid external modifications
	history := make([]Alert, len(am.alertHistory))
	copy(history, am.alertHistory)
	return history
}

// CheckAlertRules checks all alert rules and triggers alerts if needed
func (am *AlertManager) CheckAlertRules(metrics *AdvancedMetrics) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, rule := range am.alertRules {
		if !rule.Enabled {
			continue
		}

		// Get current metric value
		metric := metrics.GetMetric(rule.Metric, nil)
		if metric == nil {
			continue
		}

		// Check condition
		shouldAlert := false
		switch rule.Condition {
		case ">":
			shouldAlert = metric.Value > rule.Threshold
		case ">=":
			shouldAlert = metric.Value >= rule.Threshold
		case "<":
			shouldAlert = metric.Value < rule.Threshold
		case "<=":
			shouldAlert = metric.Value <= rule.Threshold
		case "==":
			shouldAlert = metric.Value == rule.Threshold
		case "!=":
			shouldAlert = metric.Value != rule.Threshold
		}

		// Create or update alert
		alertID := fmt.Sprintf("%s-%s", rule.ID, rule.Metric)
		if shouldAlert {
			if existing, exists := am.activeAlerts[alertID]; exists {
				// Update existing alert
				existing.CurrentValue = metric.Value
				existing.Timestamp = time.Now()
			} else {
				// Create new alert
				alert := &Alert{
					ID:           alertID,
					Name:         rule.Name,
					Description:  rule.Description,
					Level:        rule.Level,
					Metric:       rule.Metric,
					CurrentValue: metric.Value,
					Threshold:    rule.Threshold,
					Condition:    rule.Condition,
					Timestamp:    time.Now(),
					Resolved:     false,
				}
				am.activeAlerts[alertID] = alert

				// Send notification
				select {
				case am.alertChan <- alert:
				default:
					// Channel full, drop the notification
				}
			}
		} else {
			// Resolve alert if it exists
			if existing, exists := am.activeAlerts[alertID]; exists {
				existing.Resolved = true
				existing.ResolvedAt = time.Now()

				// Move to history
				am.alertHistory = append(am.alertHistory, *existing)
				delete(am.activeAlerts, alertID)
			}
		}
	}

	// Keep history size manageable
	if len(am.alertHistory) > defaultAlertHistoryMaxSize {
		am.alertHistory = am.alertHistory[1:]
	}
}

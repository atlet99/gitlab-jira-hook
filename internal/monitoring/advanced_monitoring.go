// Package monitoring provides comprehensive monitoring and metrics collection
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

// processMetric processes a single metric
func (am *AdvancedMonitor) processMetric(metric *Metric) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.getMetricKey(metric.Name, metric.Labels)
	am.metrics[key] = metric

	am.logger.Debug("Processed metric", "name", metric.Name, "value", metric.Value)
}

// checkAlertRules checks all alert rules against current metrics
func (am *AdvancedMonitor) checkAlertRules() {
	am.mu.RLock()
	rules := make([]*AlertRule, 0, len(am.alertRules))
	for _, rule := range am.alertRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	am.mu.RUnlock()

	for _, rule := range rules {
		am.checkAlertRule(rule)
	}
}

// checkAlertRule checks a single alert rule
func (am *AdvancedMonitor) checkAlertRule(rule *AlertRule) {
	// Find the metric this rule applies to
	metric := am.GetMetric(rule.Metric, rule.Labels)
	if metric == nil {
		return
	}

	// Check if the condition is met
	conditionMet := am.evaluateCondition(metric.Value, rule.Condition, rule.Threshold)
	if conditionMet {
		am.triggerAlert(rule, metric)
	} else {
		am.resolveAlert(rule, metric)
	}
}

// evaluateCondition evaluates if a condition is met
func (am *AdvancedMonitor) evaluateCondition(value float64, condition string, threshold float64) bool {
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

// triggerAlert triggers an alert
func (am *AdvancedMonitor) triggerAlert(rule *AlertRule, metric *Metric) {
	alertID := fmt.Sprintf("%s-%s", rule.ID, metric.Name)

	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if alert already exists and is active
	if alert, exists := am.alerts[alertID]; exists && !alert.Resolved {
		return
	}

	alert := &Alert{
		ID:    alertID,
		Name:  rule.Name,
		Level: rule.Level,
		Message: fmt.Sprintf("Metric %s is %s %f (current: %f)",
			rule.Metric, rule.Condition, rule.Threshold, metric.Value),
		Metric:       rule.Metric,
		Threshold:    rule.Threshold,
		CurrentValue: metric.Value,
		Labels:       rule.Labels,
		Timestamp:    time.Now(),
		Resolved:     false,
	}

	am.alerts[alertID] = alert

	// Send alert to channel
	select {
	case am.alertChan <- alert:
		am.logger.Warn("Alert triggered", "alert_id", alertID, "level", alert.Level, "message", alert.Message)
	default:
		am.logger.Warn("Alert channel full, dropping alert", "alert_id", alertID)
	}
}

// resolveAlert resolves an alert when condition is no longer met
func (am *AdvancedMonitor) resolveAlert(rule *AlertRule, metric *Metric) {
	alertID := fmt.Sprintf("%s-%s", rule.ID, metric.Name)

	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, exists := am.alerts[alertID]; exists && !alert.Resolved {
		alert.Resolved = true
		now := time.Now()
		alert.ResolvedAt = &now
		am.logger.Info("Alert resolved", "alert_id", alertID)
	}
}

// getMetricKey generates a unique key for a metric
func (am *AdvancedMonitor) getMetricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	// Create a deterministic key by sorting labels
	labelPairs := make([]string, 0, len(labels))
	for k, v := range labels {
		labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
	}

	// In a real implementation, you'd sort the label pairs
	return fmt.Sprintf("%s{%s}", name, fmt.Sprintf("%v", labelPairs))
}

// generateID generates a unique ID
func (am *AdvancedMonitor) generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ExportMetrics exports metrics in Prometheus format
func (am *AdvancedMonitor) ExportMetrics() string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result string
	for _, metric := range am.metrics {
		labels := ""
		if len(metric.Labels) > 0 {
			labelPairs := make([]string, 0, len(metric.Labels))
			for k, v := range metric.Labels {
				labelPairs = append(labelPairs, fmt.Sprintf(`%s=%q`, k, v))
			}
			labels = fmt.Sprintf("{%s}", fmt.Sprintf("%v", labelPairs))
		}

		result += fmt.Sprintf("%s%s %f %d\n", metric.Name, labels, metric.Value, metric.Timestamp.Unix())
	}

	return result
}

// ExportAlerts exports alerts in JSON format
func (am *AdvancedMonitor) ExportAlerts() ([]byte, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		alerts = append(alerts, alert)
	}

	return json.Marshal(alerts)
}

// GetHealthStatus returns the overall health status
func (am *AdvancedMonitor) GetHealthStatus() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	activeAlerts := 0
	criticalAlerts := 0
	for _, alert := range am.alerts {
		if !alert.Resolved {
			activeAlerts++
			if alert.Level == AlertLevelCritical {
				criticalAlerts++
			}
		}
	}

	return map[string]interface{}{
		"status":           am.getHealthStatusString(activeAlerts, criticalAlerts),
		"active_alerts":    activeAlerts,
		"critical_alerts":  criticalAlerts,
		"total_metrics":    len(am.metrics),
		"total_rules":      len(am.alertRules),
		"total_dashboards": len(am.dashboards),
		"timestamp":        time.Now(),
	}
}

// isTestMode returns true if the monitor is in test mode
func (am *AdvancedMonitor) isTestMode() bool {
	return am.testMode
}

// SetTestMode sets the test mode flag
func (am *AdvancedMonitor) SetTestMode(enabled bool) {
	am.testMode = enabled
}

// getHealthStatusString returns a health status string
func (am *AdvancedMonitor) getHealthStatusString(activeAlerts, criticalAlerts int) string {
	if criticalAlerts > 0 {
		return "critical"
	}
	if activeAlerts > 0 {
		return "warning"
	}
	return "healthy"
}

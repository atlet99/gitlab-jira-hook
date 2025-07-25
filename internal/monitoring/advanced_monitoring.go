package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"log/slog"
)

const (
	// Channel buffer sizes
	alertChannelBuffer  = 100
	metricChannelBuffer = 1000

	// Processing intervals
	defaultProcessingInterval = 30 * time.Second
	testProcessingInterval    = 100 * time.Millisecond
)

// MetricType defines the type of metric
type MetricType int

const (
	// MetricTypeCounter represents a cumulative metric that increases over time
	MetricTypeCounter MetricType = iota
	// MetricTypeGauge represents a metric that can go up and down
	MetricTypeGauge
	// MetricTypeHistogram represents a metric with buckets for distribution analysis
	MetricTypeHistogram
	// MetricTypeSummary represents a metric with quantiles
	MetricTypeSummary
)

// Metric represents a monitoring metric
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Value       float64           `json:"value"`
	Labels      map[string]string `json:"labels"`
	Timestamp   time.Time         `json:"timestamp"`
	Description string            `json:"description"`
	Unit        string            `json:"unit"`
}

// AlertLevel defines the severity level of an alert
type AlertLevel int

const (
	// AlertLevelInfo represents informational alerts
	AlertLevelInfo AlertLevel = iota
	// AlertLevelWarning represents warning alerts
	AlertLevelWarning
	// AlertLevelError represents error alerts
	AlertLevelError
	// AlertLevelCritical represents critical alerts
	AlertLevelCritical
)

// Alert represents a monitoring alert
type Alert struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Level        AlertLevel        `json:"level"`
	Message      string            `json:"message"`
	Metric       string            `json:"metric"`
	Threshold    float64           `json:"threshold"`
	CurrentValue float64           `json:"current_value"`
	Labels       map[string]string `json:"labels"`
	Timestamp    time.Time         `json:"timestamp"`
	Resolved     bool              `json:"resolved"`
	ResolvedAt   *time.Time        `json:"resolved_at"`
}

// AlertRule defines when an alert should be triggered
type AlertRule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Metric    string            `json:"metric"`
	Condition string            `json:"condition"` // e.g., ">", "<", "==", ">=", "<="
	Threshold float64           `json:"threshold"`
	Duration  time.Duration     `json:"duration"` // How long the condition must be true
	Level     AlertLevel        `json:"level"`
	Labels    map[string]string `json:"labels"`
	Enabled   bool              `json:"enabled"`
}

// Dashboard represents a monitoring dashboard
type Dashboard struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Panels      []DashboardPanel `json:"panels"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// DashboardPanel represents a panel in a dashboard
type DashboardPanel struct {
	ID       string                 `json:"id"`
	Title    string                 `json:"title"`
	Type     string                 `json:"type"` // "graph", "stat", "table", "heatmap"
	Metrics  []string               `json:"metrics"`
	Config   map[string]interface{} `json:"config"`
	Position PanelPosition          `json:"position"`
}

// PanelPosition defines the position of a panel in the dashboard
type PanelPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AdvancedMonitor provides advanced monitoring capabilities
type AdvancedMonitor struct {
	metrics    map[string]*Metric
	alerts     map[string]*Alert
	alertRules map[string]*AlertRule
	dashboards map[string]*Dashboard
	mu         sync.RWMutex
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	alertChan  chan *Alert
	metricChan chan *Metric
	testMode   bool
}

// NewAdvancedMonitor creates a new advanced monitor
func NewAdvancedMonitor(logger *slog.Logger) *AdvancedMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &AdvancedMonitor{
		metrics:    make(map[string]*Metric),
		alerts:     make(map[string]*Alert),
		alertRules: make(map[string]*AlertRule),
		dashboards: make(map[string]*Dashboard),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		alertChan:  make(chan *Alert, alertChannelBuffer),
		metricChan: make(chan *Metric, metricChannelBuffer),
	}
}

// Start starts the advanced monitor
func (am *AdvancedMonitor) Start() {
	am.logger.Info("Starting advanced monitor")
	go am.metricProcessor()
	go am.alertProcessor()
}

// Stop stops the advanced monitor
func (am *AdvancedMonitor) Stop() {
	am.logger.Info("Stopping advanced monitor")
	am.cancel()
}

// RecordMetric records a new metric
func (am *AdvancedMonitor) RecordMetric(name string, value float64, metricType MetricType, labels map[string]string) {
	metric := &Metric{
		Name:      name,
		Type:      metricType,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}

	select {
	case am.metricChan <- metric:
	case <-am.ctx.Done():
		am.logger.Warn("Failed to record metric, monitor stopped", "metric", name)
	}
}

// RecordCounter increments a counter metric
func (am *AdvancedMonitor) RecordCounter(name string, increment float64, labels map[string]string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.getMetricKey(name, labels)
	if metric, exists := am.metrics[key]; exists {
		metric.Value += increment
		metric.Timestamp = time.Now()
	} else {
		am.metrics[key] = &Metric{
			Name:      name,
			Type:      MetricTypeCounter,
			Value:     increment,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// RecordGauge sets a gauge metric value
func (am *AdvancedMonitor) RecordGauge(name string, value float64, labels map[string]string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.getMetricKey(name, labels)
	am.metrics[key] = &Metric{
		Name:      name,
		Type:      MetricTypeGauge,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// RecordHistogram records a histogram metric
func (am *AdvancedMonitor) RecordHistogram(name string, value float64, labels map[string]string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := am.getMetricKey(name, labels)
	if metric, exists := am.metrics[key]; exists {
		// For simplicity, we'll just track the current value
		// In a real implementation, you'd track buckets and quantiles
		metric.Value = value
		metric.Timestamp = time.Now()
	} else {
		am.metrics[key] = &Metric{
			Name:      name,
			Type:      MetricTypeHistogram,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// AddAlertRule adds a new alert rule
func (am *AdvancedMonitor) AddAlertRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.alertRules[rule.ID] = rule
	am.logger.Info("Added alert rule", "rule_id", rule.ID, "metric", rule.Metric)
}

// RemoveAlertRule removes an alert rule
func (am *AdvancedMonitor) RemoveAlertRule(ruleID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.alertRules, ruleID)
	am.logger.Info("Removed alert rule", "rule_id", ruleID)
}

// GetAlertRules returns all alert rules
func (am *AdvancedMonitor) GetAlertRules() []*AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(am.alertRules))
	for _, rule := range am.alertRules {
		rules = append(rules, rule)
	}
	return rules
}

// GetAlerts returns all alerts
func (am *AdvancedMonitor) GetAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// GetActiveAlerts returns only active (unresolved) alerts
func (am *AdvancedMonitor) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range am.alerts {
		if !alert.Resolved {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// ResolveAlert resolves an alert
func (am *AdvancedMonitor) ResolveAlert(alertID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, exists := am.alerts[alertID]; exists {
		alert.Resolved = true
		now := time.Now()
		alert.ResolvedAt = &now
		am.logger.Info("Alert resolved", "alert_id", alertID)
	}
}

// GetMetrics returns all metrics
func (am *AdvancedMonitor) GetMetrics() []*Metric {
	am.mu.RLock()
	defer am.mu.RUnlock()

	metrics := make([]*Metric, 0, len(am.metrics))
	for _, metric := range am.metrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

// GetMetric returns a specific metric
func (am *AdvancedMonitor) GetMetric(name string, labels map[string]string) *Metric {
	am.mu.RLock()
	defer am.mu.RUnlock()

	key := am.getMetricKey(name, labels)
	return am.metrics[key]
}

// CreateDashboard creates a new dashboard
func (am *AdvancedMonitor) CreateDashboard(name, description string) *Dashboard {
	am.mu.Lock()
	defer am.mu.Unlock()

	dashboard := &Dashboard{
		ID:          am.generateID(),
		Name:        name,
		Description: description,
		Panels:      make([]DashboardPanel, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	am.dashboards[dashboard.ID] = dashboard
	am.logger.Info("Created dashboard", "dashboard_id", dashboard.ID, "name", name)

	return dashboard
}

// GetDashboard returns a dashboard by ID
func (am *AdvancedMonitor) GetDashboard(dashboardID string) *Dashboard {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return am.dashboards[dashboardID]
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
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}

	panel.ID = am.generateID()
	dashboard.Panels = append(dashboard.Panels, *panel)
	dashboard.UpdatedAt = time.Now()

	am.logger.Info("Added panel to dashboard", "dashboard_id", dashboardID, "panel_id", panel.ID)

	return nil
}

// GetAlertChannel returns the channel for receiving alerts
func (am *AdvancedMonitor) GetAlertChannel() <-chan *Alert {
	return am.alertChan
}

// metricProcessor processes incoming metrics
func (am *AdvancedMonitor) metricProcessor() {
	for {
		select {
		case metric := <-am.metricChan:
			am.processMetric(metric)
		case <-am.ctx.Done():
			return
		}
	}
}

// alertProcessor processes alerts and checks rules
func (am *AdvancedMonitor) alertProcessor() {
	// Use shorter interval for testing, longer for production
	interval := defaultProcessingInterval
	if am.isTestMode() {
		interval = testProcessingInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			am.checkAlertRules()
		case <-am.ctx.Done():
			return
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
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		am.logger.Warn("Unknown condition", "condition", condition)
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

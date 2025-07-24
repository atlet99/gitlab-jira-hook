package monitoring

import (
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/stretchr/testify/assert"
)

func TestAdvancedMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("new_advanced_monitor", func(t *testing.T) {
		monitor := NewAdvancedMonitor(logger)
		assert.NotNil(t, monitor)
		assert.NotNil(t, monitor.metrics)
		assert.NotNil(t, monitor.alerts)
		assert.NotNil(t, monitor.alertRules)
		assert.NotNil(t, monitor.dashboards)
	})

	t.Run("start_stop", func(t *testing.T) {
		monitor := NewAdvancedMonitor(logger)
		monitor.Start()
		time.Sleep(100 * time.Millisecond)
		monitor.Stop()
	})
}

func TestAdvancedMonitorMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	defer monitor.Stop()

	t.Run("record_counter", func(t *testing.T) {
		monitor.RecordCounter("test_counter", 1.0, map[string]string{"label1": "value1"})
		monitor.RecordCounter("test_counter", 2.0, map[string]string{"label1": "value1"})

		metric := monitor.GetMetric("test_counter", map[string]string{"label1": "value1"})
		assert.NotNil(t, metric)
		assert.Equal(t, MetricTypeCounter, metric.Type)
		assert.Equal(t, 3.0, metric.Value)
	})

	t.Run("record_gauge", func(t *testing.T) {
		monitor.RecordGauge("test_gauge", 10.5, map[string]string{"label1": "value1"})
		monitor.RecordGauge("test_gauge", 15.2, map[string]string{"label1": "value1"})

		metric := monitor.GetMetric("test_gauge", map[string]string{"label1": "value1"})
		assert.NotNil(t, metric)
		assert.Equal(t, MetricTypeGauge, metric.Type)
		assert.Equal(t, 15.2, metric.Value) // Should be the last value
	})

	t.Run("record_histogram", func(t *testing.T) {
		monitor.RecordHistogram("test_histogram", 100.0, map[string]string{"label1": "value1"})
		monitor.RecordHistogram("test_histogram", 200.0, map[string]string{"label1": "value1"})

		metric := monitor.GetMetric("test_histogram", map[string]string{"label1": "value1"})
		assert.NotNil(t, metric)
		assert.Equal(t, MetricTypeHistogram, metric.Type)
		assert.Equal(t, 200.0, metric.Value) // Should be the last value
	})

	t.Run("get_metrics", func(t *testing.T) {
		monitor.RecordGauge("test_metric1", 1.0, nil)
		monitor.RecordGauge("test_metric2", 2.0, nil)

		metrics := monitor.GetMetrics()
		assert.GreaterOrEqual(t, len(metrics), 2)

		// Find our test metrics
		found1, found2 := false, false
		for _, metric := range metrics {
			if metric.Name == "test_metric1" && metric.Value == 1.0 {
				found1 = true
			}
			if metric.Name == "test_metric2" && metric.Value == 2.0 {
				found2 = true
			}
		}
		assert.True(t, found1)
		assert.True(t, found2)
	})
}

func TestAdvancedMonitorAlerts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	monitor.SetTestMode(true)
	monitor.Start()
	defer monitor.Stop()

	t.Run("add_alert_rule", func(t *testing.T) {
		rule := &AlertRule{
			ID:        "test_rule",
			Name:      "Test Alert",
			Metric:    "test_metric",
			Condition: ">",
			Threshold: 10.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}

		monitor.AddAlertRule(rule)

		rules := monitor.GetAlertRules()
		assert.Len(t, rules, 1)
		assert.Equal(t, "test_rule", rules[0].ID)
	})

	t.Run("remove_alert_rule", func(t *testing.T) {
		rule := &AlertRule{
			ID:        "test_rule2",
			Name:      "Test Alert 2",
			Metric:    "test_metric2",
			Condition: ">",
			Threshold: 10.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}

		monitor.AddAlertRule(rule)
		assert.Len(t, monitor.GetAlertRules(), 2)

		monitor.RemoveAlertRule("test_rule2")
		assert.Len(t, monitor.GetAlertRules(), 1)
	})

	t.Run("trigger_alert", func(t *testing.T) {
		// Add alert rule
		rule := &AlertRule{
			ID:        "high_value_rule",
			Name:      "High Value Alert",
			Metric:    "test_gauge",
			Condition: ">",
			Threshold: 5.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}
		monitor.AddAlertRule(rule)

		// Set metric value that should trigger alert
		monitor.RecordGauge("test_gauge", 10.0, nil)

		// Wait for alert processing - need more time for the alert processor
		time.Sleep(200 * time.Millisecond)

		alerts := monitor.GetActiveAlerts()
		assert.GreaterOrEqual(t, len(alerts), 1)

		// Find our alert
		found := false
		for _, alert := range alerts {
			if alert.Name == "High Value Alert" && !alert.Resolved {
				found = true
				assert.Equal(t, AlertLevelWarning, alert.Level)
				assert.Equal(t, 10.0, alert.CurrentValue)
				assert.Equal(t, 5.0, alert.Threshold)
			}
		}
		assert.True(t, found)
	})

	t.Run("resolve_alert", func(t *testing.T) {
		// Set metric value that should resolve alert
		monitor.RecordGauge("test_gauge", 3.0, nil)

		// Wait for alert processing - need more time for the alert processor
		time.Sleep(200 * time.Millisecond)

		alerts := monitor.GetActiveAlerts()
		// The alert should be resolved now
		found := false
		for _, alert := range alerts {
			if alert.Name == "High Value Alert" {
				found = true
			}
		}
		assert.False(t, found)
	})
}

func TestAdvancedMonitorDashboards(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	defer monitor.Stop()

	t.Run("create_dashboard", func(t *testing.T) {
		dashboard := monitor.CreateDashboard("Test Dashboard", "A test dashboard")
		assert.NotNil(t, dashboard)
		assert.Equal(t, "Test Dashboard", dashboard.Name)
		assert.Equal(t, "A test dashboard", dashboard.Description)
		assert.NotEmpty(t, dashboard.ID)
		assert.NotNil(t, dashboard.Panels)
		assert.Len(t, dashboard.Panels, 0)
	})

	t.Run("get_dashboard", func(t *testing.T) {
		dashboard := monitor.CreateDashboard("Test Dashboard 2", "Another test dashboard")
		retrieved := monitor.GetDashboard(dashboard.ID)
		assert.NotNil(t, retrieved)
		assert.Equal(t, dashboard.ID, retrieved.ID)
		assert.Equal(t, dashboard.Name, retrieved.Name)
	})

	t.Run("get_dashboards", func(t *testing.T) {
		dashboards := monitor.GetDashboards()
		assert.GreaterOrEqual(t, len(dashboards), 2)
	})

	t.Run("add_panel", func(t *testing.T) {
		dashboard := monitor.CreateDashboard("Panel Test", "Testing panels")

		panel := &DashboardPanel{
			Title:   "Test Panel",
			Type:    "graph",
			Metrics: []string{"test_metric1", "test_metric2"},
			Config: map[string]interface{}{
				"type": "line",
			},
			Position: PanelPosition{
				X: 0, Y: 0, Width: 6, Height: 4,
			},
		}

		err := monitor.AddPanel(dashboard.ID, panel)
		assert.NoError(t, err)

		retrieved := monitor.GetDashboard(dashboard.ID)
		assert.Len(t, retrieved.Panels, 1)
		assert.Equal(t, "Test Panel", retrieved.Panels[0].Title)
		assert.Equal(t, "graph", retrieved.Panels[0].Type)
	})

	t.Run("add_panel_invalid_dashboard", func(t *testing.T) {
		panel := &DashboardPanel{
			Title: "Test Panel",
			Type:  "graph",
		}

		err := monitor.AddPanel("invalid_id", panel)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dashboard not found")
	})
}

func TestAdvancedMonitorExport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	monitor.SetTestMode(true)
	monitor.Start()
	defer monitor.Stop()

	t.Run("export_metrics", func(t *testing.T) {
		monitor.RecordGauge("export_test", 42.5, map[string]string{"label1": "value1"})
		monitor.RecordCounter("export_counter", 10.0, nil)

		export := monitor.ExportMetrics()
		assert.Contains(t, export, "export_test")
		assert.Contains(t, export, "export_counter")
		assert.Contains(t, export, "42.5")
		assert.Contains(t, export, "10")
	})

	t.Run("export_alerts", func(t *testing.T) {
		// Create an alert
		rule := &AlertRule{
			ID:        "export_rule",
			Name:      "Export Test Alert",
			Metric:    "export_test",
			Condition: ">",
			Threshold: 40.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}
		monitor.AddAlertRule(rule)

		// Trigger alert
		monitor.RecordGauge("export_test", 50.0, nil)
		time.Sleep(200 * time.Millisecond)

		export, err := monitor.ExportAlerts()
		assert.NoError(t, err)
		assert.Contains(t, string(export), "Export Test Alert")
	})
}

func TestAdvancedMonitorHealthStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	monitor.SetTestMode(true)
	monitor.Start()
	defer monitor.Stop()

	t.Run("healthy_status", func(t *testing.T) {
		status := monitor.GetHealthStatus()
		assert.Equal(t, "healthy", status["status"])
		assert.Equal(t, 0, status["active_alerts"])
		assert.Equal(t, 0, status["critical_alerts"])
		assert.Equal(t, 0, status["total_metrics"])
		assert.Equal(t, 0, status["total_rules"])
		assert.Equal(t, 0, status["total_dashboards"])
	})

	t.Run("warning_status", func(t *testing.T) {
		// Add a warning alert
		rule := &AlertRule{
			ID:        "warning_rule",
			Name:      "Warning Alert",
			Metric:    "warning_metric",
			Condition: ">",
			Threshold: 1.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}
		monitor.AddAlertRule(rule)
		monitor.RecordGauge("warning_metric", 5.0, nil)

		time.Sleep(200 * time.Millisecond)

		status := monitor.GetHealthStatus()
		assert.Equal(t, "warning", status["status"])
		assert.Greater(t, status["active_alerts"], 0)
	})

	t.Run("critical_status", func(t *testing.T) {
		// Add a critical alert
		rule := &AlertRule{
			ID:        "critical_rule",
			Name:      "Critical Alert",
			Metric:    "critical_metric",
			Condition: ">",
			Threshold: 1.0,
			Level:     AlertLevelCritical,
			Enabled:   true,
		}
		monitor.AddAlertRule(rule)
		monitor.RecordGauge("critical_metric", 5.0, nil)

		time.Sleep(200 * time.Millisecond)

		status := monitor.GetHealthStatus()
		assert.Equal(t, "critical", status["status"])
		assert.Greater(t, status["critical_alerts"], 0)
	})
}

func TestAdvancedMonitorAlertChannel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	monitor.SetTestMode(true)
	monitor.Start()
	defer monitor.Stop()

	t.Run("alert_channel", func(t *testing.T) {
		alertChan := monitor.GetAlertChannel()
		assert.NotNil(t, alertChan)

		// Add alert rule
		rule := &AlertRule{
			ID:        "channel_rule",
			Name:      "Channel Test Alert",
			Metric:    "channel_metric",
			Condition: ">",
			Threshold: 1.0,
			Level:     AlertLevelWarning,
			Enabled:   true,
		}
		monitor.AddAlertRule(rule)

		// Trigger alert
		monitor.RecordGauge("channel_metric", 5.0, nil)

		// Wait for alert
		select {
		case alert := <-alertChan:
			assert.Equal(t, "Channel Test Alert", alert.Name)
			assert.Equal(t, AlertLevelWarning, alert.Level)
			assert.False(t, alert.Resolved)
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for alert")
		}
	})
}

func TestAdvancedMonitorConditionEvaluation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewAdvancedMonitor(logger)
	defer monitor.Stop()

	t.Run("condition_evaluation", func(t *testing.T) {
		testCases := []struct {
			condition string
			value     float64
			threshold float64
			expected  bool
		}{
			{">", 5.0, 3.0, true},
			{">", 2.0, 3.0, false},
			{">=", 3.0, 3.0, true},
			{">=", 2.0, 3.0, false},
			{"<", 2.0, 3.0, true},
			{"<", 5.0, 3.0, false},
			{"<=", 3.0, 3.0, true},
			{"<=", 5.0, 3.0, false},
			{"==", 3.0, 3.0, true},
			{"==", 5.0, 3.0, false},
			{"!=", 5.0, 3.0, true},
			{"!=", 3.0, 3.0, false},
		}

		for _, tc := range testCases {
			result := monitor.evaluateCondition(tc.value, tc.condition, tc.threshold)
			assert.Equal(t, tc.expected, result,
				"Condition %s: %f %s %f should be %v",
				tc.condition, tc.value, tc.condition, tc.threshold, tc.expected)
		}
	})

	t.Run("unknown_condition", func(t *testing.T) {
		result := monitor.evaluateCondition(5.0, "unknown", 3.0)
		assert.False(t, result)
	})
}

func TestAlertLevelString(t *testing.T) {
	t.Run("alert_level_strings", func(t *testing.T) {
		assert.Equal(t, "Info", AlertLevelInfo.String())
		assert.Equal(t, "Warning", AlertLevelWarning.String())
		assert.Equal(t, "Error", AlertLevelError.String())
		assert.Equal(t, "Critical", AlertLevelCritical.String())
	})
}

// Add String method for AlertLevel
func (al AlertLevel) String() string {
	switch al {
	case AlertLevelInfo:
		return "Info"
	case AlertLevelWarning:
		return "Warning"
	case AlertLevelError:
		return "Error"
	case AlertLevelCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

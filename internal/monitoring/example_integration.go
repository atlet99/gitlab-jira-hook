// Package monitoring provides example integration of the monitoring system
package monitoring

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Default constants for example integration
const (
	defaultPerformanceUpdateInterval = 5 * time.Second
	defaultSleepDuration             = 100 * time.Millisecond
	defaultSuccessRate               = 20
	defaultDemoDuration              = 30 * time.Second
	defaultMemoryAllocation          = 1024 * 1024 // 1MB

	// Magic number constants
	defaultSleepFactor        = 10
	defaultResponseTimeFactor = 5
	defaultMaxResponseTime    = 20
	defaultMinResponseTime    = 3
)

// ExampleIntegration demonstrates how to integrate the monitoring system
func ExampleIntegration() {
	// Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create monitoring configuration
	monitoringConfig := &Config{
		Enabled:                   true,
		Port:                      "8080",
		PrometheusPort:            "9090",
		EnableDetailedMetrics:     true,
		EnableAlerts:              true,
		WebhookCheckInterval:      DefaultCheckInterval,
		PerformanceUpdateInterval: defaultPerformanceUpdateInterval,
		AlertThresholds: &AlertThresholds{
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

	// Create monitoring system
	monitoringSystem := NewSystem(monitoringConfig, logger)

	// Create application configuration
	appConfig := &config.Config{
		Port:                 "8081",
		LogLevel:             "info",
		DebugMode:            true,
		BidirectionalEnabled: true,
	}

	// Create HTTP server with monitoring middleware
	mux := http.NewServeMux()

	// Add example endpoints
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("Hello, World!")); err != nil {
			logger.Error("Failed to write response", "error", err)
		}
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		health := monitoringSystem.GetSystemHealth()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(health); err != nil {
			logger.Error("Failed to encode health response", "error", err)
		}
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		metrics := monitoringSystem.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			logger.Error("Failed to encode metrics response", "error", err)
		}
	})

	// Create server with monitoring middleware
	server := &http.Server{
		Addr:              ":" + appConfig.Port,
		Handler:           monitoringSystem.Middleware(mux),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	// Start monitoring system
	if err := monitoringSystem.Start(); err != nil {
		logger.Error("Failed to start monitoring system", "error", err)
		return
	}

	logger.Info("Application started", "port", appConfig.Port, "monitoring_port", monitoringConfig.Port)

	// Graceful shutdown
	go func() {
		// Wait for interrupt signal
		<-context.Background().Done()

		logger.Info("Shutting down application...")

		// Stop monitoring system
		if err := monitoringSystem.Stop(); err != nil {
			logger.Error("Failed to stop monitoring system", "error", err)
		}

		// Shutdown HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Failed to shutdown server", "error", err)
		}
	}()

	// Start HTTP server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server error", "error", err)
	}
}

// WebhookIntegration demonstrates webhook monitoring integration
func WebhookIntegration() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create monitoring configuration
	monitoringConfig := &Config{
		Enabled:                   true,
		Port:                      "8080",
		WebhookCheckInterval:      DefaultCheckInterval,
		PerformanceUpdateInterval: defaultPerformanceUpdateInterval,
		EnableDetailedMetrics:     true,
	}

	// Create monitoring system
	monitoringSystem := NewSystem(monitoringConfig, logger)

	// Start monitoring system
	if err := monitoringSystem.Start(); err != nil {
		logger.Error("Failed to start monitoring system", "error", err)
		return
	}

	// Simulate webhook processing
	go func() {
		for i := 0; i < 100; i++ {
			// Simulate webhook processing
			start := time.Now()

			// Simulate work
			time.Sleep(time.Duration(i%defaultSleepFactor) * defaultSleepDuration)

			// Record webhook request
			success := i%defaultSuccessRate != 0 // 90% success rate
			responseTime := time.Since(start)

			monitoringSystem.RecordWebhookRequest("/webhook/gitlab", success, responseTime)

			time.Sleep(defaultSleepDuration)
		}
	}()

	// Keep running for demonstration
	time.Sleep(defaultDemoDuration)

	// Stop monitoring system
	if err := monitoringSystem.Stop(); err != nil {
		logger.Error("Failed to stop monitoring system", "error", err)
	}
}

// PerformanceIntegration demonstrates performance monitoring integration
func PerformanceIntegration() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create monitoring configuration
	monitoringConfig := &Config{
		Enabled:                   true,
		Port:                      "8080",
		PerformanceUpdateInterval: 1 * time.Second,
		EnableDetailedMetrics:     true,
	}

	// Create monitoring system
	monitoringSystem := NewSystem(monitoringConfig, logger)

	// Start monitoring system
	if err := monitoringSystem.Start(); err != nil {
		logger.Error("Failed to start monitoring system", "error", err)
		return
	}

	// Simulate various load patterns
	go func() {
		// Normal load
		for i := 0; i < 50; i++ {
			monitoringSystem.RecordHTTPRequest(
				"GET", "/api/data", http.StatusOK, time.Duration(i%defaultResponseTimeFactor)*defaultSleepDuration)
			time.Sleep(defaultSleepDuration)
		}

		// High load with some errors
		for i := 0; i < 100; i++ {
			status := http.StatusOK
			if i%10 == 0 {
				status = http.StatusInternalServerError // 10% error rate
			}
			monitoringSystem.RecordHTTPRequest(
				"POST", "/api/process", status, time.Duration(i%defaultMaxResponseTime)*defaultSleepDuration)
			time.Sleep(defaultSleepDuration)
		}

		// Memory intensive operations
		for i := 0; i < 20; i++ {
			// Simulate memory allocation
			data := make([]byte, defaultMemoryAllocation) // 1MB
			_ = data

			monitoringSystem.RecordHTTPRequest(
				"GET", "/api/large-data", http.StatusOK, time.Duration(i%defaultMinResponseTime)*defaultSleepDuration)
			time.Sleep(defaultSleepDuration)
		}
	}()

	// Keep running for demonstration
	time.Sleep(defaultDemoDuration)

	// Stop monitoring system
	if err := monitoringSystem.Stop(); err != nil {
		logger.Error("Failed to stop monitoring system", "error", err)
	}
}

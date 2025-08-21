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
		WebhookCheckInterval:      30 * time.Second,
		PerformanceUpdateInterval: 5 * time.Second,
		AlertThresholds: &AlertThresholds{
			ResponseTimeWarning:  100 * time.Millisecond,
			ResponseTimeCritical: 500 * time.Millisecond,
			ErrorRateWarning:     0.05,
			ErrorRateCritical:    0.10,
			MemoryUsageWarning:   100 * 1024 * 1024, // 100MB
			MemoryUsageCritical:  500 * 1024 * 1024, // 500MB
			ThroughputWarning:    100,
			ThroughputCritical:   50,
		},
	}

	// Create monitoring system
	monitoringSystem := NewMonitoringSystem(monitoringConfig, logger)

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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := monitoringSystem.GetSystemHealth()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := monitoringSystem.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})

	// Create server with monitoring middleware
	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: monitoringSystem.Middleware(mux),
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
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
		WebhookCheckInterval:      30 * time.Second,
		PerformanceUpdateInterval: 5 * time.Second,
		EnableDetailedMetrics:     true,
	}

	// Create monitoring system
	monitoringSystem := NewMonitoringSystem(monitoringConfig, logger)

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
			time.Sleep(time.Duration(i%10) * 10 * time.Millisecond)

			// Record webhook request
			success := i%20 != 0 // 90% success rate
			responseTime := time.Since(start)

			monitoringSystem.RecordWebhookRequest("/webhook/gitlab", success, responseTime)

			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Keep running for demonstration
	time.Sleep(30 * time.Second)

	// Stop monitoring system
	monitoringSystem.Stop()
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
	monitoringSystem := NewMonitoringSystem(monitoringConfig, logger)

	// Start monitoring system
	if err := monitoringSystem.Start(); err != nil {
		logger.Error("Failed to start monitoring system", "error", err)
		return
	}

	// Simulate various load patterns
	go func() {
		// Normal load
		for i := 0; i < 50; i++ {
			monitoringSystem.RecordHTTPRequest("GET", "/api/data", 200, time.Duration(i%5)*10*time.Millisecond)
			time.Sleep(50 * time.Millisecond)
		}

		// High load with some errors
		for i := 0; i < 100; i++ {
			status := 200
			if i%10 == 0 {
				status = 500 // 10% error rate
			}
			monitoringSystem.RecordHTTPRequest("POST", "/api/process", status, time.Duration(i%20)*5*time.Millisecond)
			time.Sleep(20 * time.Millisecond)
		}

		// Memory intensive operations
		for i := 0; i < 20; i++ {
			// Simulate memory allocation
			data := make([]byte, 1024*1024) // 1MB
			_ = data

			monitoringSystem.RecordHTTPRequest("GET", "/api/large-data", 200, time.Duration(i%3)*50*time.Millisecond)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Keep running for demonstration
	time.Sleep(30 * time.Second)

	// Stop monitoring system
	monitoringSystem.Stop()
}

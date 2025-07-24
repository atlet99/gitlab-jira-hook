package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// createTestConfig creates a minimal test configuration
func createTestConfig(port string) *config.Config {
	return &config.Config{
		Port:          port,
		GitLabSecret:  "test-secret",
		JiraEmail:     "test@example.com",
		JiraToken:     "test-token",
		JiraBaseURL:   "https://jira.example.com",
		MinWorkers:    2,
		MaxWorkers:    10,
		JobQueueSize:  100,
		ScaleInterval: 10,
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		logger      *slog.Logger
		expectError bool
	}{
		{
			name:        "create server with valid config",
			config:      createTestConfig("8080"),
			logger:      slog.Default(),
			expectError: false,
		},
		{
			name: "create server with invalid config",
			config: func() *config.Config {
				cfg := createTestConfig("invalid-port")
				return cfg
			}(),
			logger:      slog.Default(),
			expectError: false, // Server will be created but may fail to start
		},
		{
			name:        "create server with minimal config",
			config:      createTestConfig("8080"),
			logger:      slog.Default(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create separate registry for each test to avoid conflicts
			registry := prometheus.NewRegistry()
			server := NewWithRegistry(tt.config, tt.logger, registry)

			if tt.expectError {
				assert.Nil(t, server)
				return
			}

			require.NotNil(t, server)

			// Verify server components
			assert.NotNil(t, server.config)
			assert.NotNil(t, server.logger)
			assert.NotNil(t, server.monitor)
			assert.NotNil(t, server.workerPool)
			assert.NotNil(t, server.adapter)
		})
	}
}

func TestServerStart(t *testing.T) {
	t.Run("start server successfully", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing (random port)

		// Create separate registry for each test to avoid conflicts
		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start server in background
		errChan := make(chan error, 1)
		go func() {
			errChan <- server.Start()
		}()

		// Wait a bit for server to start
		time.Sleep(100 * time.Millisecond)

		// Check if server started without error
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		default:
			// Server is still running, which is good
		}

		// Stop server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("start server with invalid port", func(t *testing.T) {
		cfg := func() *config.Config {
			cfg := createTestConfig("99999") // Invalid port
			return cfg
		}()

		// Create separate registry for each test to avoid conflicts
		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start server should fail
		err := server.Start()
		assert.Error(t, err)
	})
}

func TestServerShutdown(t *testing.T) {
	t.Run("shutdown server gracefully", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start server in background
		go func() {
			_ = server.Start()
		}()

		// Wait a bit for server to start
		time.Sleep(100 * time.Millisecond)

		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("shutdown with timeout", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start server in background
		go func() {
			_ = server.Start()
		}()

		// Wait a bit for server to start
		time.Sleep(100 * time.Millisecond)

		// Shutdown with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		err := server.Shutdown(ctx)
		// Should not error even with short timeout
		assert.NoError(t, err)
	})

	t.Run("shutdown without starting", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Shutdown without starting should not error
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("health check endpoint", func(t *testing.T) {
		// Create test request
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		// Call health check handler
		handleHealth(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})

	t.Run("health check with different methods", func(t *testing.T) {
		// Test GET method (should work)
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handleHealth(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")

		// Test other methods (should return 405)
		methods := []string{"POST", "PUT", "DELETE", "HEAD", "OPTIONS"}
		for _, method := range methods {
			t.Run(method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/health", nil)
				w := httptest.NewRecorder()
				handleHealth(w, req)
				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			})
		}
	})

	t.Run("health check with invalid method", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/health", nil)
		w := httptest.NewRecorder()

		handleHealth(w, req)

		// Should return method not allowed
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestServerComponents(t *testing.T) {
	t.Run("verify server components", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Verify all components are properly initialized
		assert.Equal(t, cfg, server.config)
		assert.NotNil(t, server.logger)
		assert.NotNil(t, server.monitor)
		assert.NotNil(t, server.workerPool)
		assert.NotNil(t, server.adapter)
	})

	t.Run("verify worker pool configuration", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start the worker pool to initialize workers
		server.workerPool.Start()
		defer server.workerPool.Stop()

		// Wait a bit for workers to start
		time.Sleep(100 * time.Millisecond)

		// Verify worker pool is properly configured
		stats := server.workerPool.GetStats()
		assert.NotNil(t, stats)
		assert.Equal(t, cfg.MinWorkers, stats.TotalWorkers) // Total workers should match MinWorkers
		assert.LessOrEqual(t, stats.TotalWorkers, cfg.MaxWorkers)
	})

	t.Run("verify monitoring setup", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing
		cfg.MetricsEnabled = true

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Verify monitor is properly initialized
		assert.NotNil(t, server.monitor)
	})
}

func TestServerIntegration(t *testing.T) {
	t.Run("full server lifecycle", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing
		cfg.GitLabSecret = "integration-secret"
		cfg.JiraEmail = "integration@example.com"
		cfg.JiraToken = "integration-token"
		cfg.JiraBaseURL = "https://integration-jira.example.com"
		cfg.MinWorkers = 2
		cfg.MaxWorkers = 5
		cfg.MetricsEnabled = true

		// Create server
		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Verify initial state
		assert.NotNil(t, server.config)
		assert.NotNil(t, server.logger)
		assert.NotNil(t, server.monitor)
		assert.NotNil(t, server.workerPool)

		// Start server
		errChan := make(chan error, 1)
		go func() {
			errChan <- server.Start()
		}()

		// Wait for server to start
		time.Sleep(200 * time.Millisecond)

		// Check if server started successfully
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		default:
			// Server is running, which is good
		}

		// Test health check
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handleHealth(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestServerErrorHandling(t *testing.T) {
	t.Run("server with invalid configuration", func(t *testing.T) {
		// Test with invalid config
		cfg := func() *config.Config {
			cfg := createTestConfig("99999") // Invalid port
			return cfg
		}()

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		require.NotNil(t, server)

		// Start should fail
		err := server.Start()
		assert.Error(t, err)
	})

	t.Run("server with missing required fields", func(t *testing.T) {
		// Test with missing required fields
		cfg := &config.Config{
			Port: "8080",
			// Missing GitLabSecret, JiraEmail, JiraToken, JiraBaseURL
		}

		registry := prometheus.NewRegistry()
		server := NewWithRegistry(cfg, slog.Default(), registry)
		// Should still create server (validation happens later)
		require.NotNil(t, server)
	})
}

func TestServerPerformance(t *testing.T) {
	t.Run("server creation performance", func(t *testing.T) {
		cfg := createTestConfig("0") // Use port 0 for testing
		cfg.GitLabSecret = "perf-secret"
		cfg.JiraEmail = "perf@example.com"
		cfg.JiraToken = "perf-token"
		cfg.JiraBaseURL = "https://perf-jira.example.com"

		// Measure server creation performance
		start := time.Now()
		for i := 0; i < 100; i++ {
			registry := prometheus.NewRegistry()
			server := NewWithRegistry(cfg, slog.Default(), registry)
			require.NotNil(t, server)
		}
		duration := time.Since(start)

		// Should complete within reasonable time
		assert.Less(t, duration, 10*time.Second, "Server creation should be fast")
		t.Logf("Created 100 servers in %v", duration)
	})

	t.Run("health check performance", func(t *testing.T) {
		// Measure health check performance
		start := time.Now()
		for i := 0; i < 1000; i++ {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			handleHealth(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
		duration := time.Since(start)

		// Should complete within reasonable time
		assert.Less(t, duration, 5*time.Second, "Health check should be fast")
		t.Logf("Performed 1000 health checks in %v", duration)
	})
}

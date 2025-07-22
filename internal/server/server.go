// Package server provides HTTP server functionality for the GitLab-Jira Hook application.
// It handles webhook endpoints and health checks.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/async"
	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
)

const (
	// Server configuration constants
	serverDefaultRate  = 20.0 // 20 requests per second
	serverDefaultBurst = 40   // burst of 40 requests

	// Cache configuration constants
	cacheL1Size = 1000  // L1 cache size
	cacheL2Size = 10000 // L2 cache size

	// Server timeout constants
	readHeaderTimeout = 30 * time.Second
)

// Server represents the main application server
type Server struct {
	*http.Server
	config             *config.Config
	logger             *slog.Logger
	monitor            *monitoring.WebhookMonitor
	performanceMonitor *monitoring.PerformanceMonitor
	workerPool         *async.PriorityWorkerPool
	adapter            *async.WorkerPoolAdapter
	rateLimiter        *HTTPRateLimiter
	cache              cache.Cache
}

// New creates a new server instance
func New(cfg *config.Config, logger *slog.Logger) *Server {
	// Create GitLab handler
	gitlabHandler := gitlab.NewHandler(cfg, logger)
	// Create Project Hook handler
	projectHookHandler := gitlab.NewProjectHookHandler(cfg, logger)

	// Create webhook monitor
	webhookMonitor := monitoring.NewWebhookMonitor(cfg, logger)

	// Set monitor in handlers for metrics recording
	gitlabHandler.SetMonitor(webhookMonitor)
	projectHookHandler.SetMonitor(webhookMonitor)

	// Create priority worker pool for async processing
	workerPool := async.NewPriorityWorkerPool(cfg, logger, webhookMonitor, nil)

	// Create adapter for compatibility with webhook.WorkerPoolInterface
	adapter := async.NewWorkerPoolAdapter(workerPool)

	// Set worker pool in handlers using adapter
	gitlabHandler.SetWorkerPool(adapter)
	projectHookHandler.SetWorkerPool(adapter)

	// Create performance monitor
	performanceMonitor := monitoring.NewPerformanceMonitor(context.Background())

	// Create monitoring handler
	monitoringHandler := monitoring.NewHandler(webhookMonitor, performanceMonitor, logger)

	// Create rate limiter
	rateLimiter := NewHTTPRateLimiter(&RateLimiterConfig{
		DefaultRate:  serverDefaultRate,
		DefaultBurst: serverDefaultBurst,
		PerIP:        true,
		PerEndpoint:  true,
	})

	// Create cache (multi-level for better performance)
	appCache := cache.NewMultiLevelCache(cacheL1Size, cacheL2Size)

	// Create mux
	mux := http.NewServeMux()

	// Register routes with rate limiting and performance monitoring
	mux.Handle("/gitlab-hook",
		performanceMonitor.PerformanceMiddleware(
			RateLimitMiddleware(rateLimiter)(http.HandlerFunc(gitlabHandler.HandleWebhook))))
	mux.Handle("/gitlab-project-hook",
		performanceMonitor.PerformanceMiddleware(
			RateLimitMiddleware(rateLimiter)(http.HandlerFunc(projectHookHandler.HandleProjectHook))))
	mux.HandleFunc("/health", handleHealth)

	// Register monitoring routes (no rate limiting for internal monitoring)
	mux.HandleFunc("/monitoring/status", monitoringHandler.HandleStatus)
	mux.HandleFunc("/monitoring/metrics", monitoringHandler.HandleMetrics)
	mux.HandleFunc("/monitoring/health", monitoringHandler.HandleHealth)
	mux.HandleFunc("/monitoring/detailed", monitoringHandler.HandleDetailedStatus)
	mux.HandleFunc("/monitoring/reconnect", monitoringHandler.HandleReconnect)

	// Register performance monitoring routes
	mux.HandleFunc("/performance", monitoringHandler.HandlePerformance)
	mux.HandleFunc("/performance/history", monitoringHandler.HandlePerformanceHistory)
	mux.HandleFunc("/performance/targets", monitoringHandler.HandlePerformanceTargets)
	mux.HandleFunc("/performance/reset", monitoringHandler.HandlePerformanceReset)

	// Create server
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return &Server{
		Server:             srv,
		config:             cfg,
		logger:             logger,
		monitor:            webhookMonitor,
		performanceMonitor: performanceMonitor,
		workerPool:         workerPool,
		adapter:            adapter,
		rateLimiter:        rateLimiter,
		cache:              appCache,
	}
}

// Start starts the server and monitoring
func (s *Server) Start() error {
	// Start webhook monitoring
	s.monitor.Start()

	// Start worker pool
	s.workerPool.Start()

	s.logger.Info("Starting HTTP server", "port", s.config.Port)
	return s.ListenAndServe()
}

// Shutdown gracefully shuts down the server and monitoring
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop webhook monitoring
	s.monitor.Stop()

	// Stop performance monitoring
	if s.performanceMonitor != nil {
		if err := s.performanceMonitor.Close(); err != nil {
			s.logger.Error("Failed to close performance monitor", "error", err)
		}
	}

	// Stop worker pool
	s.workerPool.Stop()

	// Close cache
	if s.cache != nil {
		if mlc, ok := s.cache.(*cache.MultiLevelCache); ok {
			mlc.Close()
		}
	}

	// Shutdown HTTP server
	return s.Server.Shutdown(ctx)
}

// GetCache returns the cache instance
func (s *Server) GetCache() cache.Cache {
	return s.cache
}

// GetRateLimiter returns the rate limiter instance
func (s *Server) GetRateLimiter() *HTTPRateLimiter {
	return s.rateLimiter
}

// GetPerformanceMonitor returns the performance monitor instance
func (s *Server) GetPerformanceMonitor() *monitoring.PerformanceMonitor {
	return s.performanceMonitor
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		// Log error but don't fail the request - this is a health check
		// In a real application, you might want to log this error
		_ = err // explicitly ignore the error
	}
}

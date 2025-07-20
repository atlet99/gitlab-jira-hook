// Package server provides HTTP server functionality for the GitLab-Jira Hook application.
// It handles webhook endpoints and health checks.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/async"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
)

// Server represents the HTTP server
type Server struct {
	*http.Server
	config     *config.Config
	logger     *slog.Logger
	monitor    *monitoring.WebhookMonitor
	workerPool *async.PriorityWorkerPool
	adapter    *async.WorkerPoolAdapter
}

// New creates a new HTTP server
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

	// Create monitoring handler
	monitoringHandler := monitoring.NewHandler(webhookMonitor, logger)

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/gitlab-hook", gitlabHandler.HandleWebhook)
	mux.HandleFunc("/gitlab-project-hook", projectHookHandler.HandleProjectHook)
	mux.HandleFunc("/health", handleHealth)

	// Register monitoring routes
	mux.HandleFunc("/monitoring/status", monitoringHandler.HandleStatus)
	mux.HandleFunc("/monitoring/metrics", monitoringHandler.HandleMetrics)
	mux.HandleFunc("/monitoring/health", monitoringHandler.HandleHealth)
	mux.HandleFunc("/monitoring/detailed", monitoringHandler.HandleDetailedStatus)
	mux.HandleFunc("/monitoring/reconnect", monitoringHandler.HandleReconnect)

	// Create server
	const readHeaderTimeout = 30 * time.Second
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return &Server{
		Server:     srv,
		config:     cfg,
		logger:     logger,
		monitor:    webhookMonitor,
		workerPool: workerPool,
		adapter:    adapter,
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

	// Stop worker pool
	s.workerPool.Stop()

	// Shutdown HTTP server
	return s.Server.Shutdown(ctx)
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

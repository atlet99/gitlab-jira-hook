// Package server provides HTTP server functionality for the GitLab-Jira Hook application.
// It handles webhook endpoints and health checks.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/atlet99/gitlab-jira-hook/internal/async"
	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/sync"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
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
	// Create handlers and clients
	gitlabHandler := gitlab.NewHandler(cfg, logger)
	projectHookHandler := gitlab.NewProjectHookHandler(cfg, logger)
	jiraAPIClient := jira.NewClient(cfg)
	jiraWebhookHandler := jira.NewWebhookHandler(cfg, logger)

	// Configure Jira webhook handler
	configureJiraWebhookHandler(jiraWebhookHandler, jiraAPIClient, cfg, logger)

	// Create monitoring components
	webhookMonitor := monitoring.NewWebhookMonitor(cfg, logger)
	performanceMonitor := monitoring.NewPerformanceMonitor(context.Background())

	// Create worker pool and adapter
	workerPool := async.NewPriorityWorkerPool(cfg, logger, webhookMonitor, nil)
	adapter := async.NewWorkerPoolAdapter(workerPool)

	// Configure handlers
	configureHandlers(gitlabHandler, projectHookHandler, jiraWebhookHandler, webhookMonitor, adapter)

	// Create server components
	rateLimiter := NewHTTPRateLimiter(&RateLimiterConfig{
		DefaultRate:  serverDefaultRate,
		DefaultBurst: serverDefaultBurst,
		PerIP:        true,
		PerEndpoint:  true,
	})

	appCache := cache.NewMultiLevelCache(cacheL1Size, cacheL2Size)
	mux := createMux(
		cfg, logger, gitlabHandler, projectHookHandler,
		jiraWebhookHandler, webhookMonitor, performanceMonitor,
	)

	// Create HTTP server
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

// configureJiraWebhookHandler configures the Jira webhook handler with clients and settings
func configureJiraWebhookHandler(
	handler *jira.WebhookHandler,
	jiraClient *jira.Client,
	cfg *config.Config,
	logger *slog.Logger,
) {
	handler.SetJiraClient(jiraClient)

	// Configure JWT validation if enabled
	if cfg.JWTEnabled {
		handler.ConfigureJWTValidation(cfg.JWTExpectedAudience, cfg.JWTAllowedIssuers)
		logger.Info("JWT validation configured for Jira webhook handler",
			"audience", cfg.JWTExpectedAudience,
			"allowed_issuers", cfg.JWTAllowedIssuers)
	}

	// Create and set GitLab API client via adapter
	gitlabAPIClient := gitlab.NewAPIClient(cfg, logger)
	gitlabAdapter := gitlab.NewJiraAPIAdapter(gitlabAPIClient)
	handler.SetGitLabClient(gitlabAdapter)

	// Create sync manager for bidirectional sync with conflict resolution
	syncAdapter := sync.NewGitLabSyncAdapter(gitlabAPIClient)
	jiraAdapter := sync.NewJiraSyncAdapter(jiraClient)
	syncManager := sync.NewManager(cfg, syncAdapter, jiraAdapter, logger)
	handler.SetManager(syncManager)
}

// configureHandlers configures all handlers with monitoring and worker pool
func configureHandlers(
	gitlabHandler, projectHookHandler, jiraWebhookHandler interface{},
	monitor *monitoring.WebhookMonitor,
	adapter webhook.WorkerPoolInterface,
) {
	// Set monitor in handlers for metrics recording
	if h, ok := gitlabHandler.(interface {
		SetMonitor(*monitoring.WebhookMonitor)
	}); ok {
		h.SetMonitor(monitor)
	}
	if h, ok := projectHookHandler.(interface {
		SetMonitor(*monitoring.WebhookMonitor)
	}); ok {
		h.SetMonitor(monitor)
	}
	if h, ok := jiraWebhookHandler.(interface {
		SetMonitor(*monitoring.WebhookMonitor)
	}); ok {
		h.SetMonitor(monitor)
	}

	// Set worker pool in handlers using adapter
	if h, ok := gitlabHandler.(interface {
		SetWorkerPool(webhook.WorkerPoolInterface)
	}); ok {
		h.SetWorkerPool(adapter)
	}
	if h, ok := projectHookHandler.(interface {
		SetWorkerPool(webhook.WorkerPoolInterface)
	}); ok {
		h.SetWorkerPool(adapter)
	}
	if h, ok := jiraWebhookHandler.(interface {
		SetWorkerPool(webhook.WorkerPoolInterface)
	}); ok {
		h.SetWorkerPool(adapter)
	}
}

// createMux creates and configures the HTTP serve mux with all routes
func createMux(
	cfg *config.Config,
	logger *slog.Logger,
	gitlabHandler, projectHookHandler, jiraWebhookHandler interface{},
	webhookMonitor *monitoring.WebhookMonitor,
	performanceMonitor *monitoring.PerformanceMonitor,
) *http.ServeMux {
	mux := http.NewServeMux()

	// Register main webhook routes with rate limiting and performance monitoring
	registerWebhookRoutes(mux, gitlabHandler, projectHookHandler, jiraWebhookHandler, webhookMonitor, performanceMonitor)

	// Register monitoring routes (no rate limiting for internal monitoring)
	registerMonitoringRoutes(mux, webhookMonitor, performanceMonitor, logger)

	// Register OAuth 2.0 routes (only if OAuth 2.0 is configured)
	if cfg.JiraAuthMethod == config.JiraAuthMethodOAuth2 {
		registerOAuthRoutes(mux, cfg, logger)
	}

	return mux
}

// registerWebhookRoutes registers the main webhook endpoints
func registerWebhookRoutes(
	mux *http.ServeMux,
	gitlabHandler, projectHookHandler, jiraWebhookHandler interface{},
	_ *monitoring.WebhookMonitor,
	performanceMonitor *monitoring.PerformanceMonitor,
) {
	rateLimiter := NewHTTPRateLimiter(&RateLimiterConfig{
		DefaultRate:  serverDefaultRate,
		DefaultBurst: serverDefaultBurst,
		PerIP:        true,
		PerEndpoint:  true,
	})

	// GitLab webhook
	if h, ok := gitlabHandler.(interface {
		HandleWebhook(http.ResponseWriter, *http.Request)
	}); ok {
		mux.Handle("/gitlab-hook",
			performanceMonitor.PerformanceMiddleware(
				RateLimitMiddleware(rateLimiter)(http.HandlerFunc(h.HandleWebhook))))
	}

	// GitLab project hook
	if h, ok := projectHookHandler.(interface {
		HandleProjectHook(http.ResponseWriter, *http.Request)
	}); ok {
		mux.Handle("/gitlab-project-hook",
			performanceMonitor.PerformanceMiddleware(
				RateLimitMiddleware(rateLimiter)(http.HandlerFunc(h.HandleProjectHook))))
	}

	// Jira webhook
	if h, ok := jiraWebhookHandler.(interface {
		HandleWebhook(http.ResponseWriter, *http.Request)
	}); ok {
		mux.Handle("/jira-webhook",
			performanceMonitor.PerformanceMiddleware(
				RateLimitMiddleware(rateLimiter)(http.HandlerFunc(h.HandleWebhook))))
	}

	// Health check
	mux.HandleFunc("/health", handleHealth)
}

// registerMonitoringRoutes registers monitoring and performance endpoints
func registerMonitoringRoutes(
	mux *http.ServeMux,
	webhookMonitor *monitoring.WebhookMonitor,
	performanceMonitor *monitoring.PerformanceMonitor,
	logger *slog.Logger,
) {
	monitoringHandler := monitoring.NewHandler(webhookMonitor, performanceMonitor, logger)

	// Monitoring routes
	mux.HandleFunc("/monitoring/status", monitoringHandler.HandleStatus)
	mux.HandleFunc("/monitoring/metrics", monitoringHandler.HandleMetrics)
	mux.HandleFunc("/monitoring/health", monitoringHandler.HandleHealth)
	mux.HandleFunc("/monitoring/detailed", monitoringHandler.HandleDetailedStatus)
	mux.HandleFunc("/monitoring/reconnect", monitoringHandler.HandleReconnect)

	// Performance monitoring routes
	mux.HandleFunc("/performance", monitoringHandler.HandlePerformance)
	mux.HandleFunc("/performance/history", monitoringHandler.HandlePerformanceHistory)
	mux.HandleFunc("/performance/targets", monitoringHandler.HandlePerformanceTargets)
	mux.HandleFunc("/performance/reset", monitoringHandler.HandlePerformanceReset)
}

// registerOAuthRoutes registers OAuth 2.0 authentication endpoints
func registerOAuthRoutes(mux *http.ServeMux, cfg *config.Config, logger *slog.Logger) {
	oauth2Handlers := jira.NewOAuth2Handlers(cfg, logger)
	mux.HandleFunc("/auth/jira/authorize", oauth2Handlers.HandleAuthorize)
	mux.HandleFunc("/auth/jira/callback", oauth2Handlers.HandleCallback)
	mux.HandleFunc("/auth/jira/status", oauth2Handlers.HandleStatus)
	logger.Info("OAuth 2.0 endpoints registered",
		"authorize_url", "/auth/jira/authorize",
		"callback_url", "/auth/jira/callback",
		"status_url", "/auth/jira/status")
}

// NewWithRegistry creates a new server with a custom Prometheus registry for testing
func NewWithRegistry(cfg *config.Config, logger *slog.Logger, registry prometheus.Registerer) *Server {
	// Create webhook monitor
	webhookMonitor := monitoring.NewWebhookMonitor(cfg, logger)

	// Create handlers
	gitlabHandler := gitlab.NewHandler(cfg, logger)
	projectHookHandler := gitlab.NewProjectHookHandler(cfg, logger)
	jiraWebhookHandler := jira.NewWebhookHandler(cfg, logger)
	// Create and set GitLab API client via adapter
	gitlabAPIClient := gitlab.NewAPIClient(cfg, logger)
	gitlabAdapter := gitlab.NewJiraAPIAdapter(gitlabAPIClient)
	jiraWebhookHandler.SetGitLabClient(gitlabAdapter)

	// Create Jira API client for conflict resolution
	jiraAPIClient := jira.NewClient(cfg)
	// Set Jira client in webhook handler
	jiraWebhookHandler.SetJiraClient(jiraAPIClient)

	// Create sync manager for bidirectional sync
	syncAdapter := sync.NewGitLabSyncAdapter(gitlabAPIClient)
	jiraAdapter := sync.NewJiraSyncAdapter(jiraAPIClient)
	syncManager := sync.NewManager(cfg, syncAdapter, jiraAdapter, logger)
	jiraWebhookHandler.SetManager(syncManager)

	// Create worker pool
	workerPool := async.NewPriorityWorkerPool(cfg, logger, webhookMonitor, nil)

	// Set monitor in handlers for metrics recording
	gitlabHandler.SetMonitor(webhookMonitor)
	projectHookHandler.SetMonitor(webhookMonitor)
	jiraWebhookHandler.SetMonitor(webhookMonitor)

	// Create adapter for compatibility with webhook.WorkerPoolInterface
	adapter := async.NewWorkerPoolAdapter(workerPool)

	// Set worker pool in handlers using adapter
	gitlabHandler.SetWorkerPool(adapter)
	projectHookHandler.SetWorkerPool(adapter)
	jiraWebhookHandler.SetWorkerPool(adapter)

	// Create performance monitor with custom registry
	performanceMonitor := monitoring.NewPerformanceMonitorWithRegistry(context.Background(), registry)

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
	mux.Handle("/jira-webhook",
		performanceMonitor.PerformanceMiddleware(
			RateLimitMiddleware(rateLimiter)(http.HandlerFunc(jiraWebhookHandler.HandleWebhook))))
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

	// Register OAuth 2.0 routes (only if OAuth 2.0 is configured)
	if cfg.JiraAuthMethod == config.JiraAuthMethodOAuth2 {
		oauth2Handlers := jira.NewOAuth2Handlers(cfg, logger)
		mux.HandleFunc("/auth/jira/authorize", oauth2Handlers.HandleAuthorize)
		mux.HandleFunc("/auth/jira/callback", oauth2Handlers.HandleCallback)
		mux.HandleFunc("/auth/jira/status", oauth2Handlers.HandleStatus)
		logger.Info("OAuth 2.0 endpoints registered",
			"authorize_url", "/auth/jira/authorize",
			"callback_url", "/auth/jira/callback",
			"status_url", "/auth/jira/status")
	}

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

// GetWorkerPool returns the worker pool instance
func (s *Server) GetWorkerPool() *async.PriorityWorkerPool {
	return s.workerPool
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

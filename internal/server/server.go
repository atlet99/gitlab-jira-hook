// Package server provides HTTP server functionality for the GitLab-Jira Hook application.
// It handles webhook endpoints and health checks.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
)

// Server represents the HTTP server
type Server struct {
	*http.Server
	config *config.Config
	logger *slog.Logger
}

// New creates a new HTTP server
func New(cfg *config.Config, logger *slog.Logger) *Server {
	// Create GitLab handler
	gitlabHandler := gitlab.NewHandler(cfg, logger)
	// Create Project Hook handler
	projectHookHandler := gitlab.NewProjectHookHandler(cfg, logger)

	// Create mux
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/gitlab-hook", gitlabHandler.HandleWebhook)
	mux.HandleFunc("/gitlab-project-hook", projectHookHandler.HandleProjectHook)
	mux.HandleFunc("/health", handleHealth)

	// Create server
	const readHeaderTimeout = 30 * time.Second
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return &Server{
		Server: srv,
		config: cfg,
		logger: logger,
	}
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

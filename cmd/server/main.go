// Package main provides the entry point for the GitLab-Jira Hook webhook server.
// This server listens to GitLab System Hook events and posts comments to Jira issues.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/server"
	ver "github.com/atlet99/gitlab-jira-hook/internal/version"
	"github.com/atlet99/gitlab-jira-hook/pkg/logger"
)

func main() {
	// Parse command line flags
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	// Show version and exit if requested
	if showVersion {
		fmt.Println(ver.GetFullVersionInfo())
		os.Exit(0)
	}

	// Initialize logger
	log := logger.NewLogger()

	// Log startup information
	log.Info("Starting GitLab-Jira Hook",
		"version", ver.GetVersion(),
		"commit", ver.Commit,
		"builtBy", ver.BuiltBy,
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create server
	srv := server.New(cfg, log)

	// Start server in a goroutine
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create a deadline for server shutdown
	const shutdownTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}

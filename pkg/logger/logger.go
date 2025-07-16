// Package logger provides logging utilities for the GitLab-Jira Hook application.
// It includes structured logging setup and configuration.
package logger

import (
	"log/slog"
	"os"
)

// NewLogger creates a new structured logger
func NewLogger() *slog.Logger {
	// Get log level from environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Parse log level
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create logger options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	// Create handler
	handler := slog.NewJSONHandler(os.Stdout, opts)

	// Create logger
	logger := slog.New(handler)

	// Set as default logger
	slog.SetDefault(logger)

	return logger
}

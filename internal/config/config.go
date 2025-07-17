// Package config provides configuration management for the GitLab-Jira Hook application.
// It handles loading and validation of environment variables and configuration settings.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Default configuration values
const (
	DefaultPort                 = "8080"
	DefaultLogLevel             = "info"
	DefaultJiraRateLimit        = 10
	DefaultJiraRetryMaxAttempts = 3
	DefaultJiraRetryBaseDelayMs = 200
)

// Config holds all configuration for the application
type Config struct {
	Port                 string
	GitLabSecret         string
	GitLabBaseURL        string
	JiraEmail            string
	JiraToken            string
	JiraBaseURL          string
	LogLevel             string
	AllowedProjects      []string
	AllowedGroups        []string
	JiraRateLimit        int
	JiraRetryMaxAttempts int
	JiraRetryBaseDelayMs int
	PushBranchFilter     []string // comma-separated list of branch names to filter
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist - we'll use environment variables
		_ = err // explicitly ignore the error
	}

	cfg := &Config{
		Port:                 getEnv("PORT", DefaultPort),
		GitLabSecret:         getEnv("GITLAB_SECRET", ""),
		GitLabBaseURL:        getEnv("GITLAB_BASE_URL", ""),
		JiraEmail:            getEnv("JIRA_EMAIL", ""),
		JiraToken:            getEnv("JIRA_TOKEN", ""),
		JiraBaseURL:          getEnv("JIRA_BASE_URL", ""),
		LogLevel:             getEnv("LOG_LEVEL", DefaultLogLevel),
		AllowedProjects:      parseCSVEnv("ALLOWED_PROJECTS"),
		AllowedGroups:        parseCSVEnv("ALLOWED_GROUPS"),
		JiraRateLimit:        parseIntEnv("JIRA_RATE_LIMIT", DefaultJiraRateLimit),
		JiraRetryMaxAttempts: parseIntEnv("JIRA_RETRY_MAX_ATTEMPTS", DefaultJiraRetryMaxAttempts),
		JiraRetryBaseDelayMs: parseIntEnv("JIRA_RETRY_BASE_DELAY_MS", DefaultJiraRetryBaseDelayMs),
		PushBranchFilter:     parseCSVEnv("PUSH_BRANCH_FILTER"),
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// validate checks if all required configuration fields are set
func (c *Config) validate() error {
	if c.GitLabSecret == "" {
		return fmt.Errorf("GITLAB_SECRET is required")
	}
	if c.GitLabBaseURL == "" {
		return fmt.Errorf("GITLAB_BASE_URL is required")
	}
	if c.JiraEmail == "" {
		return fmt.Errorf("JIRA_EMAIL is required")
	}
	if c.JiraToken == "" {
		return fmt.Errorf("JIRA_TOKEN is required")
	}
	if c.JiraBaseURL == "" {
		return fmt.Errorf("JIRA_BASE_URL is required")
	}

	// Validate port is a valid number
	if _, err := strconv.Atoi(c.Port); err != nil {
		return fmt.Errorf("PORT must be a valid number: %w", err)
	}

	return nil
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// parseIntEnv parses an integer environment variable with a fallback value
func parseIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

// parseCSVEnv parses a comma-separated environment variable into a string slice
func parseCSVEnv(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

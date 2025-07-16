// Package config provides configuration management for the GitLab-Jira Hook application.
// It handles loading and validation of environment variables and configuration settings.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Port         string
	GitLabSecret string
	JiraEmail    string
	JiraToken    string
	JiraBaseURL  string
	LogLevel     string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist - we'll use environment variables
		_ = err // explicitly ignore the error
	}

	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		GitLabSecret: getEnv("GITLAB_SECRET", ""),
		JiraEmail:    getEnv("JIRA_EMAIL", ""),
		JiraToken:    getEnv("JIRA_TOKEN", ""),
		JiraBaseURL:  getEnv("JIRA_BASE_URL", ""),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
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

// Package config provides configuration management for the GitLab-Jira Hook application.
// It handles loading and validation of environment variables and configuration settings.
package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"log/slog"

	"github.com/joho/godotenv"
)

const (
	// DefaultWorkerMultiplier is the multiplier for CPU count to determine worker count
	DefaultWorkerMultiplier = 2
	// DefaultMemoryPerWorkerMB is the estimated memory per worker in MB
	DefaultMemoryPerWorkerMB = 128
	// AutoMaxWorkers is the maximum number of workers for auto-detection
	AutoMaxWorkers = 32
	// DefaultQueueSizeMultiplier is the multiplier for workers to determine queue size
	DefaultQueueSizeMultiplier = 100
	// DefaultMemoryPerJobMB is the estimated memory per job in MB
	DefaultMemoryPerJobMB = 2
	// DefaultMinQueueSize is the minimum queue size
	DefaultMinQueueSize = 100
	// DefaultMaxQueueSize is the maximum queue size
	DefaultMaxQueueSize = 10000
	// BytesPerMB is the number of bytes in 1 MB
	BytesPerMB = 1024 * 1024
	// KBPerMB is the number of KB in 1 MB
	KBPerMB = 1024
	// MinFieldsForMemInfo is the minimum number of fields in /proc/meminfo
	MinFieldsForMemInfo = 2
)

// Default configuration values
const (
	DefaultPort                 = "8080"
	DefaultLogLevel             = "info"
	DefaultJiraRateLimit        = 10
	DefaultJiraRetryMaxAttempts = 3
	DefaultJiraRetryBaseDelayMs = 200
	DefaultTimezone             = "Etc/GMT-5" // UTC+5
	DefaultWorkerPoolSize       = 10
	DefaultJobQueueSize         = 100

	// Queue and throughput defaults
	DefaultMinWorkers         = 2
	DefaultMaxWorkers         = 32
	DefaultScaleUpThreshold   = 10
	DefaultScaleDownThreshold = 2
	DefaultScaleInterval      = 10

	// Rate limiting defaults
	DefaultMaxConcurrentJobs = 50
	DefaultJobTimeoutSeconds = 30
	DefaultQueueTimeoutMs    = 5000

	// Retry and backoff defaults
	DefaultMaxRetries        = 3
	DefaultRetryDelayMs      = 1000
	DefaultBackoffMultiplier = 2.0
	DefaultMaxBackoffMs      = 30000

	// Monitoring defaults
	DefaultMetricsEnabled      = true
	DefaultHealthCheckInterval = 30
)

// Config holds all configuration for the application
type Config struct {
	// Server Configuration
	Port     string
	LogLevel string
	Timezone string // timezone for date formatting and container

	// GitLab Configuration
	GitLabSecret     string
	GitLabBaseURL    string
	AllowedProjects  []string
	AllowedGroups    []string
	PushBranchFilter []string // comma-separated list of branch names to filter

	// Jira Configuration
	JiraEmail            string
	JiraToken            string
	JiraBaseURL          string
	JiraRateLimit        int
	JiraRetryMaxAttempts int
	JiraRetryBaseDelayMs int

	// Queue and Throughput Configuration
	WorkerPoolSize     int // number of workers in the pool (legacy)
	JobQueueSize       int // size of the job queue (legacy)
	MinWorkers         int // minimum number of workers in the pool
	MaxWorkers         int // maximum number of workers in the pool
	ScaleUpThreshold   int // queue length above which the pool will scale up
	ScaleDownThreshold int // queue length below which the pool will scale down
	ScaleInterval      int // interval (in seconds) between scaling checks

	// Rate Limiting and Throughput
	MaxConcurrentJobs int // maximum concurrent jobs processing
	JobTimeoutSeconds int // timeout for individual job processing
	QueueTimeoutMs    int // timeout for queue operations

	// Retry and Backoff Configuration
	MaxRetries        int     // maximum retry attempts for failed jobs
	RetryDelayMs      int     // base delay between retries in milliseconds
	BackoffMultiplier float64 // exponential backoff multiplier
	MaxBackoffMs      int     // maximum backoff delay in milliseconds

	// Monitoring and Health
	MetricsEnabled      bool // enable metrics collection
	HealthCheckInterval int  // health check interval in seconds
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist - we'll use environment variables
		_ = err // explicitly ignore the error
	}

	cfg := &Config{
		// Server Configuration
		Port:     getEnv("PORT", DefaultPort),
		LogLevel: getEnv("LOG_LEVEL", DefaultLogLevel),
		Timezone: getEnv("TIMEZONE", DefaultTimezone),

		// GitLab Configuration
		GitLabSecret:     getEnv("GITLAB_SECRET", ""),
		GitLabBaseURL:    getEnv("GITLAB_BASE_URL", ""),
		AllowedProjects:  parseCSVEnv("ALLOWED_PROJECTS"),
		AllowedGroups:    parseCSVEnv("ALLOWED_GROUPS"),
		PushBranchFilter: parseCSVEnv("PUSH_BRANCH_FILTER"),

		// Jira Configuration
		JiraEmail:            getEnv("JIRA_EMAIL", ""),
		JiraToken:            getEnv("JIRA_TOKEN", ""),
		JiraBaseURL:          getEnv("JIRA_BASE_URL", ""),
		JiraRateLimit:        parseIntEnv("JIRA_RATE_LIMIT", DefaultJiraRateLimit),
		JiraRetryMaxAttempts: parseIntEnv("JIRA_RETRY_MAX_ATTEMPTS", DefaultJiraRetryMaxAttempts),
		JiraRetryBaseDelayMs: parseIntEnv("JIRA_RETRY_BASE_DELAY_MS", DefaultJiraRetryBaseDelayMs),

		// Queue and Throughput Configuration (legacy support)
		WorkerPoolSize: parseIntEnv("WORKER_POOL_SIZE", DefaultWorkerPoolSize),
		JobQueueSize:   parseIntEnv("JOB_QUEUE_SIZE", DefaultJobQueueSize),

		// New Queue Configuration
		MinWorkers:         parseIntEnv("MIN_WORKERS", DefaultMinWorkers),
		MaxWorkers:         parseIntEnv("MAX_WORKERS", DefaultMaxWorkers),
		ScaleUpThreshold:   parseIntEnv("SCALE_UP_THRESHOLD", DefaultScaleUpThreshold),
		ScaleDownThreshold: parseIntEnv("SCALE_DOWN_THRESHOLD", DefaultScaleDownThreshold),
		ScaleInterval:      parseIntEnv("SCALE_INTERVAL", DefaultScaleInterval),

		// Rate Limiting and Throughput
		MaxConcurrentJobs: parseIntEnv("MAX_CONCURRENT_JOBS", DefaultMaxConcurrentJobs),
		JobTimeoutSeconds: parseIntEnv("JOB_TIMEOUT_SECONDS", DefaultJobTimeoutSeconds),
		QueueTimeoutMs:    parseIntEnv("QUEUE_TIMEOUT_MS", DefaultQueueTimeoutMs),

		// Retry and Backoff Configuration
		MaxRetries:        parseIntEnv("MAX_RETRIES", DefaultMaxRetries),
		RetryDelayMs:      parseIntEnv("RETRY_DELAY_MS", DefaultRetryDelayMs),
		BackoffMultiplier: parseFloatEnv("BACKOFF_MULTIPLIER", DefaultBackoffMultiplier),
		MaxBackoffMs:      parseIntEnv("MAX_BACKOFF_MS", DefaultMaxBackoffMs),

		// Monitoring and Health
		MetricsEnabled:      parseBoolEnv("METRICS_ENABLED", DefaultMetricsEnabled),
		HealthCheckInterval: parseIntEnv("HEALTH_CHECK_INTERVAL", DefaultHealthCheckInterval),
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// NewConfigFromEnv loads config from environment variables or uses defaults/auto-detects
func NewConfigFromEnv(logger *slog.Logger) *Config {
	cfg := &Config{}
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist - we'll use environment variables
		_ = err // explicitly ignore the error
	}

	// Server Configuration
	cfg.Port = getEnv("PORT", DefaultPort)
	cfg.LogLevel = getEnv("LOG_LEVEL", DefaultLogLevel)
	cfg.Timezone = getEnv("TIMEZONE", DefaultTimezone)

	// GitLab Configuration
	cfg.GitLabSecret = getEnv("GITLAB_SECRET", "")
	cfg.GitLabBaseURL = getEnv("GITLAB_BASE_URL", "")
	cfg.AllowedProjects = parseCSVEnv("ALLOWED_PROJECTS")
	cfg.AllowedGroups = parseCSVEnv("ALLOWED_GROUPS")
	cfg.PushBranchFilter = parseCSVEnv("PUSH_BRANCH_FILTER")

	// Jira Configuration
	cfg.JiraEmail = getEnv("JIRA_EMAIL", "")
	cfg.JiraToken = getEnv("JIRA_TOKEN", "")
	cfg.JiraBaseURL = getEnv("JIRA_BASE_URL", "")
	cfg.JiraRateLimit = parseIntEnv("JIRA_RATE_LIMIT", DefaultJiraRateLimit)
	cfg.JiraRetryMaxAttempts = parseIntEnv("JIRA_RETRY_MAX_ATTEMPTS", DefaultJiraRetryMaxAttempts)
	cfg.JiraRetryBaseDelayMs = parseIntEnv("JIRA_RETRY_BASE_DELAY_MS", DefaultJiraRetryBaseDelayMs)

	// Queue and Throughput Configuration (legacy support)
	cfg.WorkerPoolSize = parseIntEnv("WORKER_POOL_SIZE", DefaultWorkerPoolSize)
	cfg.JobQueueSize = parseIntEnv("JOB_QUEUE_SIZE", DefaultJobQueueSize)

	// New Queue Configuration
	cfg.MinWorkers = parseIntEnv("MIN_WORKERS", DefaultMinWorkers)
	cfg.MaxWorkers = parseIntEnv("MAX_WORKERS", DefaultMaxWorkers)
	cfg.ScaleUpThreshold = parseIntEnv("SCALE_UP_THRESHOLD", DefaultScaleUpThreshold)
	cfg.ScaleDownThreshold = parseIntEnv("SCALE_DOWN_THRESHOLD", DefaultScaleDownThreshold)
	cfg.ScaleInterval = parseIntEnv("SCALE_INTERVAL", DefaultScaleInterval)

	// Rate Limiting and Throughput
	cfg.MaxConcurrentJobs = parseIntEnv("MAX_CONCURRENT_JOBS", DefaultMaxConcurrentJobs)
	cfg.JobTimeoutSeconds = parseIntEnv("JOB_TIMEOUT_SECONDS", DefaultJobTimeoutSeconds)
	cfg.QueueTimeoutMs = parseIntEnv("QUEUE_TIMEOUT_MS", DefaultQueueTimeoutMs)

	// Retry and Backoff Configuration
	cfg.MaxRetries = parseIntEnv("MAX_RETRIES", DefaultMaxRetries)
	cfg.RetryDelayMs = parseIntEnv("RETRY_DELAY_MS", DefaultRetryDelayMs)
	cfg.BackoffMultiplier = parseFloatEnv("BACKOFF_MULTIPLIER", DefaultBackoffMultiplier)
	cfg.MaxBackoffMs = parseIntEnv("MAX_BACKOFF_MS", DefaultMaxBackoffMs)

	// Monitoring and Health
	cfg.MetricsEnabled = parseBoolEnv("METRICS_ENABLED", DefaultMetricsEnabled)
	cfg.HealthCheckInterval = parseIntEnv("HEALTH_CHECK_INTERVAL", DefaultHealthCheckInterval)

	// Auto-detect worker/queue params if not set
	cpuCount := runtime.NumCPU()
	memLimitMB := detectMemoryLimitMB()

	if cfg.MaxWorkers == 0 {
		cfg.MaxWorkers = autoMaxWorkers(cpuCount, memLimitMB)
		logger.Info("Auto-detected MaxWorkers", "value", cfg.MaxWorkers, "cpu_count", cpuCount, "mem_limit_mb", memLimitMB)
	}
	if cfg.MinWorkers == 0 {
		cfg.MinWorkers = 1
	}
	if cfg.JobQueueSize == 0 {
		cfg.JobQueueSize = autoQueueSize(cfg.MaxWorkers, memLimitMB)
		logger.Info("Auto-detected JobQueueSize",
			"value", cfg.JobQueueSize,
			"max_workers", cfg.MaxWorkers,
			"mem_limit_mb", memLimitMB)
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		logger.Error("Configuration validation failed", "error", err)
		// Optionally, you might want to return a default or error state
		return &Config{ // Return a default config with errors
			Port:                DefaultPort,
			LogLevel:            DefaultLogLevel,
			Timezone:            DefaultTimezone,
			MetricsEnabled:      DefaultMetricsEnabled,
			HealthCheckInterval: DefaultHealthCheckInterval,
		}
	}

	return cfg
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

// parseFloatEnv parses a float64 environment variable with a fallback value
func parseFloatEnv(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// parseBoolEnv parses a boolean environment variable with a fallback value
func parseBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}

// autoMaxWorkers returns optimal worker count based on CPU/memory
func autoMaxWorkers(cpuCount, memLimitMB int) int {
	w := cpuCount * DefaultWorkerMultiplier
	if memLimitMB > 0 && w > memLimitMB/DefaultMemoryPerWorkerMB {
		w = memLimitMB / DefaultMemoryPerWorkerMB
	}
	if w < 1 {
		w = 1
	}
	if w > AutoMaxWorkers {
		w = AutoMaxWorkers // safeguard
	}
	return w
}

// autoQueueSize returns optimal queue size based on workers/memory
func autoQueueSize(workers, memLimitMB int) int {
	q := workers * DefaultQueueSizeMultiplier
	if memLimitMB > 0 && q > memLimitMB*DefaultMemoryPerJobMB {
		q = memLimitMB * DefaultMemoryPerJobMB
	}
	if q < DefaultMinQueueSize {
		q = DefaultMinQueueSize
	}
	if q > DefaultMaxQueueSize {
		q = DefaultMaxQueueSize // safeguard
	}
	return q
}

// detectMemoryLimitMB tries to detect memory limit in MB (cgroup/docker/linux only)
func detectMemoryLimitMB() int {
	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		if v, err := strconv.ParseInt(string(data), 10, 64); err == nil && v > 0 {
			return int(v / BytesPerMB)
		}
	}
	// fallback: try /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= MinFieldsForMemInfo {
					if kb, err := strconv.Atoi(fields[1]); err == nil {
						return kb / KBPerMB
					}
				}
			}
		}
	}
	return 0 // unknown
}

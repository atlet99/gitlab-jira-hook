// Package config provides configuration management for the GitLab-Jira Hook application.
// It handles loading and validation of environment variables and configuration settings.
package config

import (
	"fmt"
	"net/url"
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

	// MinGitLabTokenLength is the minimum required length for GitLab API tokens
	MinGitLabTokenLength = 8
	// MinJiraTokenLength is the minimum required length for Jira API tokens
	MinJiraTokenLength = 10
	// MaxWorkersSafetyLimit is the maximum allowed number of workers for safety
	MaxWorkersSafetyLimit = 1000
	// MinJobTimeoutSeconds is the minimum job timeout in seconds
	MinJobTimeoutSeconds = 10
	// MaxJobTimeoutSeconds is the maximum job timeout in seconds (1 hour)
	MaxJobTimeoutSeconds = 3600
	// MaxJiraRateLimit is the maximum allowed Jira API rate limit
	MaxJiraRateLimit = 100
	// MaxRetryAttempts is the maximum allowed retry attempts
	MaxRetryAttempts = 10
	// MinRetryDelayMs is the minimum retry delay in milliseconds
	MinRetryDelayMs = 50
	// MaxConcurrentJobsLimit is the maximum allowed concurrent jobs
	MaxConcurrentJobsLimit = 1000

	// JiraAuthMethodBasic represents basic authentication method for Jira
	JiraAuthMethodBasic = "basic"
	// JiraAuthMethodOAuth2 represents OAuth 2.0 authentication method for Jira
	JiraAuthMethodOAuth2 = "oauth2"

	// Bidirectional sync default values
	defaultBidirectionalSyncInterval = 30
	defaultSyncBatchSize             = 10
	defaultSyncRateLimit             = 60
	defaultSyncRetryAttempts         = 3
	defaultMaxEventAge               = 24

	// Bidirectional sync validation limits
	minSyncInterval        = 5
	maxSyncInterval        = 3600
	maxSyncBatchSize       = 100
	maxSyncRateLimit       = 300
	maxSyncRetryAttempts   = 10
	maxEventAgeHours       = 168 // 1 week
	keyValueSeparatorParts = 2
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
	DefaultJobTimeoutSeconds = 120 // Increased from 30 to 120 seconds
	DefaultQueueTimeoutMs    = 5000

	// Retry and backoff defaults
	DefaultMaxRetries        = 3
	DefaultRetryDelayMs      = 1000
	DefaultBackoffMultiplier = 2.0
	DefaultMaxBackoffMs      = 30000

	// Monitoring defaults
	DefaultMetricsEnabled      = true
	DefaultHealthCheckInterval = 30

	// Debug defaults
	DefaultDebugMode = false
)

// Config holds all configuration for the application
type Config struct {
	// Server Configuration
	Port      string
	LogLevel  string
	Timezone  string // timezone for date formatting and container
	DebugMode bool   // enable debug logging and verbose output

	// GitLab Configuration
	GitLabSecret     string
	GitLabBaseURL    string
	GitLabAPIToken   string // GitLab API token for creating issues and comments
	GitLabNamespace  string // Default GitLab namespace for Jira projects
	AllowedProjects  []string
	AllowedGroups    []string
	PushBranchFilter []string          // comma-separated list of branch names to filter
	ProjectMappings  map[string]string // Jira project key -> GitLab project mapping

	// Jira Configuration
	JiraEmail            string
	JiraToken            string
	JiraBaseURL          string
	JiraWebhookSecret    string // Secret for validating incoming Jira webhooks
	JiraRateLimit        int
	JiraRetryMaxAttempts int
	JiraRetryBaseDelayMs int

	// Jira OAuth 2.0 Configuration
	JiraAuthMethod         string // "basic" or "oauth2"
	JiraOAuth2ClientID     string // OAuth 2.0 client ID
	JiraOAuth2ClientSecret string // OAuth 2.0 client secret
	JiraOAuth2Scope        string // OAuth 2.0 scope (default: "read:jira-work write:jira-work")
	JiraOAuth2TokenURL     string // OAuth 2.0 token endpoint
	JiraOAuth2AuthURL      string // OAuth 2.0 authorization endpoint
	JiraOAuth2RedirectURL  string // OAuth 2.0 redirect URL
	JiraAccessToken        string // Current OAuth 2.0 access token
	JiraRefreshToken       string // OAuth 2.0 refresh token
	JiraTokenExpiry        int64  // Token expiry timestamp (Unix seconds)

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

	// Bidirectional Sync Configuration
	BidirectionalEnabled bool     // enable bidirectional sync (Jira → GitLab)
	BidirectionalEvents  []string // list of Jira events to process
	// (e.g., "issue_created", "issue_updated", "comment_created")
	BidirectionalSyncInterval     int    // interval in seconds for sync operations
	BidirectionalConflictStrategy string // conflict resolution strategy: "last_write_wins", "merge", "manual"

	// Status Mapping (Jira → GitLab)
	StatusMappingEnabled bool              // enable status synchronization
	StatusMapping        map[string]string // Jira status → GitLab label mapping (e.g., "In Progress" → "in-progress")
	DefaultGitLabLabel   string            // default label when Jira status has no mapping

	// Comment Sync Configuration
	CommentSyncEnabled          bool   // enable comment synchronization (Jira → GitLab)
	CommentSyncDirection        string // sync direction: "jira_to_gitlab", "gitlab_to_jira", "bidirectional"
	CommentTemplateJiraToGitLab string // template for Jira → GitLab comments
	CommentTemplateGitLabToJira string // template for GitLab → Jira comments

	// Assignee Sync Configuration
	AssigneeSyncEnabled   bool              // enable assignee synchronization
	UserMapping           map[string]string // Jira user → GitLab user mapping (e.g., "john.doe@company.com" → "jdoe")
	DefaultGitLabAssignee string            // default GitLab assignee when Jira user has no mapping

	// Sync Limits and Safety
	SyncBatchSize     int  // maximum number of sync operations per batch
	SyncRateLimit     int  // maximum sync operations per minute
	SyncRetryAttempts int  // number of retry attempts for failed sync operations
	SkipOldEvents     bool // skip events older than specified duration
	MaxEventAge       int  // maximum event age in hours to process
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
		Port:      getEnv("PORT", DefaultPort),
		LogLevel:  getEnv("LOG_LEVEL", DefaultLogLevel),
		Timezone:  getEnv("TIMEZONE", DefaultTimezone),
		DebugMode: parseBoolEnv("DEBUG_MODE", DefaultDebugMode),

		// GitLab Configuration
		GitLabSecret:     getEnv("GITLAB_SECRET", ""),
		GitLabBaseURL:    getEnv("GITLAB_BASE_URL", ""),
		GitLabAPIToken:   getEnv("GITLAB_API_TOKEN", ""),
		GitLabNamespace:  getEnv("GITLAB_NAMESPACE", "jira-sync"),
		AllowedProjects:  parseCSVEnv("ALLOWED_PROJECTS"),
		AllowedGroups:    parseCSVEnv("ALLOWED_GROUPS"),
		PushBranchFilter: parseCSVEnv("PUSH_BRANCH_FILTER"),
		ProjectMappings:  parseProjectMappings("PROJECT_MAPPINGS"),

		// Jira Configuration
		JiraEmail:            getEnv("JIRA_EMAIL", ""),
		JiraToken:            getEnv("JIRA_TOKEN", ""),
		JiraBaseURL:          getEnv("JIRA_BASE_URL", ""),
		JiraWebhookSecret:    getEnv("JIRA_WEBHOOK_SECRET", ""),
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
//
//nolint:funlen // configuration function naturally has many statements
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
	cfg.DebugMode = parseBoolEnv("DEBUG_MODE", DefaultDebugMode)

	// GitLab Configuration
	cfg.GitLabSecret = getEnv("GITLAB_SECRET", "")
	cfg.GitLabBaseURL = getEnv("GITLAB_BASE_URL", "")
	cfg.GitLabAPIToken = getEnv("GITLAB_API_TOKEN", "")
	cfg.GitLabNamespace = getEnv("GITLAB_NAMESPACE", "jira-sync")
	cfg.AllowedProjects = parseCSVEnv("ALLOWED_PROJECTS")
	cfg.AllowedGroups = parseCSVEnv("ALLOWED_GROUPS")
	cfg.PushBranchFilter = parseCSVEnv("PUSH_BRANCH_FILTER")
	cfg.ProjectMappings = parseProjectMappings("PROJECT_MAPPINGS")

	// Jira Configuration
	cfg.JiraEmail = getEnv("JIRA_EMAIL", "")
	cfg.JiraToken = getEnv("JIRA_TOKEN", "")
	cfg.JiraBaseURL = getEnv("JIRA_BASE_URL", "")
	cfg.JiraWebhookSecret = getEnv("JIRA_WEBHOOK_SECRET", "")
	cfg.JiraRateLimit = parseIntEnv("JIRA_RATE_LIMIT", DefaultJiraRateLimit)
	cfg.JiraRetryMaxAttempts = parseIntEnv("JIRA_RETRY_MAX_ATTEMPTS", DefaultJiraRetryMaxAttempts)
	cfg.JiraRetryBaseDelayMs = parseIntEnv("JIRA_RETRY_BASE_DELAY_MS", DefaultJiraRetryBaseDelayMs)

	// Jira OAuth 2.0 Configuration
	cfg.JiraAuthMethod = getEnv("JIRA_AUTH_METHOD", JiraAuthMethodBasic)
	// Ensure auth method is not empty (fallback to basic if it is)
	if cfg.JiraAuthMethod == "" {
		cfg.JiraAuthMethod = JiraAuthMethodBasic
	}
	cfg.JiraOAuth2ClientID = getEnv("JIRA_OAUTH2_CLIENT_ID", "")
	cfg.JiraOAuth2ClientSecret = getEnv("JIRA_OAUTH2_CLIENT_SECRET", "")
	cfg.JiraOAuth2Scope = getEnv("JIRA_OAUTH2_SCOPE", "read:jira-work write:jira-work")
	cfg.JiraOAuth2TokenURL = getEnv("JIRA_OAUTH2_TOKEN_URL", "")
	cfg.JiraOAuth2AuthURL = getEnv("JIRA_OAUTH2_AUTH_URL", "")
	cfg.JiraOAuth2RedirectURL = getEnv("JIRA_OAUTH2_REDIRECT_URL", "")
	cfg.JiraAccessToken = getEnv("JIRA_ACCESS_TOKEN", "")
	cfg.JiraRefreshToken = getEnv("JIRA_REFRESH_TOKEN", "")
	cfg.JiraTokenExpiry = parseInt64Env("JIRA_TOKEN_EXPIRY", 0)

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

	// Bidirectional Sync Configuration
	cfg.BidirectionalEnabled = parseBoolEnv("BIDIRECTIONAL_ENABLED", false)
	cfg.BidirectionalEvents = parseCSVEnv("BIDIRECTIONAL_EVENTS")
	cfg.BidirectionalSyncInterval = parseIntEnv("BIDIRECTIONAL_SYNC_INTERVAL", defaultBidirectionalSyncInterval)
	cfg.BidirectionalConflictStrategy = getEnv("BIDIRECTIONAL_CONFLICT_STRATEGY", "last_write_wins")

	// Status Mapping (Jira → GitLab)
	cfg.StatusMappingEnabled = parseBoolEnv("STATUS_MAPPING_ENABLED", false)
	cfg.StatusMapping = parseKeyValueMapping("STATUS_MAPPING")
	cfg.DefaultGitLabLabel = getEnv("DEFAULT_GITLAB_LABEL", "external-sync")

	// Comment Sync Configuration
	cfg.CommentSyncEnabled = parseBoolEnv("COMMENT_SYNC_ENABLED", false)
	cfg.CommentSyncDirection = getEnv("COMMENT_SYNC_DIRECTION", "jira_to_gitlab")
	cfg.CommentTemplateJiraToGitLab = getEnv("COMMENT_TEMPLATE_JIRA_TO_GITLAB", "**From Jira**: {content}")
	cfg.CommentTemplateGitLabToJira = getEnv("COMMENT_TEMPLATE_GITLAB_TO_JIRA", "**From GitLab**: {content}")

	// Assignee Sync Configuration
	cfg.AssigneeSyncEnabled = parseBoolEnv("ASSIGNEE_SYNC_ENABLED", false)
	cfg.UserMapping = parseKeyValueMapping("USER_MAPPING")
	cfg.DefaultGitLabAssignee = getEnv("DEFAULT_GITLAB_ASSIGNEE", "")

	// Sync Limits and Safety
	cfg.SyncBatchSize = parseIntEnv("SYNC_BATCH_SIZE", defaultSyncBatchSize)
	cfg.SyncRateLimit = parseIntEnv("SYNC_RATE_LIMIT", defaultSyncRateLimit)
	cfg.SyncRetryAttempts = parseIntEnv("SYNC_RETRY_ATTEMPTS", defaultSyncRetryAttempts)
	cfg.SkipOldEvents = parseBoolEnv("SKIP_OLD_EVENTS", true)
	cfg.MaxEventAge = parseIntEnv("MAX_EVENT_AGE", defaultMaxEventAge)

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
//
//nolint:gocyclo // validation function naturally has high complexity
func (c *Config) validate() error {
	// Required fields validation
	if c.GitLabSecret == "" {
		return fmt.Errorf("GITLAB_SECRET is required")
	}
	if c.GitLabBaseURL == "" {
		return fmt.Errorf("GITLAB_BASE_URL is required")
	}
	if c.GitLabAPIToken == "" {
		return fmt.Errorf("GITLAB_API_TOKEN is required for bidirectional sync")
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

	// Validate port is a valid number and in valid range
	port, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("PORT must be a valid number: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535, got: %d", port)
	}

	// Validate URLs format
	if err := validateURL(c.GitLabBaseURL, "GITLAB_BASE_URL"); err != nil {
		return err
	}
	if err := validateURL(c.JiraBaseURL, "JIRA_BASE_URL"); err != nil {
		return err
	}

	// Validate email format
	if err := validateEmail(c.JiraEmail, "JIRA_EMAIL"); err != nil {
		return err
	}

	// Validate GitLab API token format (basic check)
	if err := validateGitLabToken(c.GitLabAPIToken); err != nil {
		return err
	}

	// Validate Jira authentication configuration
	if err := c.validateJiraAuth(); err != nil {
		return err
	}

	// Validate worker pool configuration
	if err := c.validateWorkerPool(); err != nil {
		return err
	}

	// Validate project mappings format
	if err := c.validateProjectMappings(); err != nil {
		return err
	}

	// Validate rate limiting configuration
	if err := c.validateRateLimiting(); err != nil {
		return err
	}

	// Validate bidirectional sync configuration
	if err := c.validateBidirectionalSync(); err != nil {
		return err
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

// parseInt64Env parses an int64 environment variable with a fallback value
func parseInt64Env(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
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

// parseProjectMappings parses project mappings from environment variable
// Format: "JIRA_KEY1=gitlab/project1,JIRA_KEY2=gitlab/project2"
func parseProjectMappings(key string) map[string]string {
	return parseKeyValueMapping(key)
}

// parseKeyValueMapping parses environment variable into map[string]string
// Format: "KEY1=value1,KEY2=value2,KEY3=value3"
func parseKeyValueMapping(key string) map[string]string {
	value := getEnv(key, "")
	mappings := make(map[string]string)

	if value == "" {
		return mappings
	}

	// Split by comma to get individual mappings
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		// Trim whitespace
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split by = to get key-value pair
		parts := strings.SplitN(pair, "=", keyValueSeparatorParts)
		if len(parts) != keyValueSeparatorParts {
			continue
		}

		mapKey := strings.TrimSpace(parts[0])
		mapValue := strings.TrimSpace(parts[1])

		if mapKey != "" && mapValue != "" {
			mappings[mapKey] = mapValue
		}
	}

	return mappings
}

// validateURL validates that a string is a valid URL format
func validateURL(urlStr, fieldName string) error {
	if urlStr == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Check if it starts with http:// or https://
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("%s must start with http:// or https://, got: %s", fieldName, urlStr)
	}

	// Parse the URL to validate its format
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", fieldName, err)
	}

	// Additional validation
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%s must use http or https scheme, got: %s", fieldName, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s must have a valid host, got: %s", fieldName, urlStr)
	}

	return nil
}

// validateEmail validates basic email format
func validateEmail(email, fieldName string) error {
	if email == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Basic email validation - contains @ and . after @
	if !strings.Contains(email, "@") {
		return fmt.Errorf("%s must be a valid email address, got: %s", fieldName, email)
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("%s must be a valid email address, got: %s", fieldName, email)
	}

	// Check domain part has at least one dot
	if !strings.Contains(parts[1], ".") {
		return fmt.Errorf("%s must have a valid domain, got: %s", fieldName, email)
	}

	return nil
}

// validateGitLabToken validates GitLab API token format
func validateGitLabToken(token string) error {
	if token == "" {
		return fmt.Errorf("GITLAB_API_TOKEN cannot be empty")
	}

	// GitLab tokens typically start with glpat- (personal access tokens)
	// or glpat- for project access tokens, but can also be other formats
	// Just check minimum length and no spaces
	if len(token) < MinGitLabTokenLength {
		return fmt.Errorf("GITLAB_API_TOKEN appears too short (minimum %d characters)", MinGitLabTokenLength)
	}

	if strings.Contains(token, " ") {
		return fmt.Errorf("GITLAB_API_TOKEN cannot contain spaces")
	}

	return nil
}

// validateJiraToken validates Jira API token format
func validateJiraToken(token string) error {
	if token == "" {
		return fmt.Errorf("JIRA_TOKEN cannot be empty")
	}

	// Jira API tokens are typically base64-encoded, minimum length check
	if len(token) < MinJiraTokenLength {
		return fmt.Errorf("JIRA_TOKEN appears too short (minimum %d characters)", MinJiraTokenLength)
	}

	if strings.Contains(token, " ") {
		return fmt.Errorf("JIRA_TOKEN cannot contain spaces")
	}

	return nil
}

// validateJiraAuth validates Jira authentication configuration
func (c *Config) validateJiraAuth() error {
	// Validate authentication method
	if c.JiraAuthMethod != JiraAuthMethodBasic && c.JiraAuthMethod != JiraAuthMethodOAuth2 {
		return fmt.Errorf("JIRA_AUTH_METHOD must be either '%s' or '%s', got: %s",
			JiraAuthMethodBasic, JiraAuthMethodOAuth2, c.JiraAuthMethod)
	}

	// Validate Basic Auth configuration
	if c.JiraAuthMethod == JiraAuthMethodBasic {
		if err := validateJiraToken(c.JiraToken); err != nil {
			return err
		}
		if c.JiraEmail == "" {
			return fmt.Errorf("JIRA_EMAIL cannot be empty when using basic authentication")
		}
	}

	// Validate OAuth 2.0 configuration
	if c.JiraAuthMethod == JiraAuthMethodOAuth2 {
		if c.JiraOAuth2ClientID == "" {
			return fmt.Errorf("JIRA_OAUTH2_CLIENT_ID cannot be empty when using OAuth 2.0")
		}
		if c.JiraOAuth2ClientSecret == "" {
			return fmt.Errorf("JIRA_OAUTH2_CLIENT_SECRET cannot be empty when using OAuth 2.0")
		}
		if c.JiraOAuth2TokenURL == "" {
			return fmt.Errorf("JIRA_OAUTH2_TOKEN_URL cannot be empty when using OAuth 2.0")
		}
		if c.JiraOAuth2AuthURL == "" {
			return fmt.Errorf("JIRA_OAUTH2_AUTH_URL cannot be empty when using OAuth 2.0")
		}
		if c.JiraOAuth2RedirectURL == "" {
			return fmt.Errorf("JIRA_OAUTH2_REDIRECT_URL cannot be empty when using OAuth 2.0")
		}

		// Validate OAuth 2.0 scope format
		if c.JiraOAuth2Scope == "" {
			return fmt.Errorf("JIRA_OAUTH2_SCOPE cannot be empty when using OAuth 2.0")
		}
	}

	return nil
}

// validateWorkerPool validates worker pool configuration
func (c *Config) validateWorkerPool() error {
	// Check basic constraints
	if c.MinWorkers < 1 {
		return fmt.Errorf("MIN_WORKERS must be at least 1, got: %d", c.MinWorkers)
	}

	if c.MaxWorkers < c.MinWorkers {
		return fmt.Errorf("MAX_WORKERS (%d) must be greater than or equal to MIN_WORKERS (%d)",
			c.MaxWorkers, c.MinWorkers)
	}

	if c.MaxWorkers > MaxWorkersSafetyLimit {
		return fmt.Errorf("MAX_WORKERS should not exceed %d for safety, got: %d", MaxWorkersSafetyLimit, c.MaxWorkers)
	}

	// Check scaling thresholds
	if c.ScaleUpThreshold < 1 {
		return fmt.Errorf("SCALE_UP_THRESHOLD must be at least 1, got: %d", c.ScaleUpThreshold)
	}

	if c.ScaleDownThreshold < 0 {
		return fmt.Errorf("SCALE_DOWN_THRESHOLD must be non-negative, got: %d", c.ScaleDownThreshold)
	}

	if c.ScaleUpThreshold <= c.ScaleDownThreshold {
		return fmt.Errorf("SCALE_UP_THRESHOLD (%d) must be greater than SCALE_DOWN_THRESHOLD (%d)",
			c.ScaleUpThreshold, c.ScaleDownThreshold)
	}

	// Check timeouts
	if c.JobTimeoutSeconds < MinJobTimeoutSeconds {
		return fmt.Errorf("JOB_TIMEOUT_SECONDS should be at least %d seconds, got: %d",
			MinJobTimeoutSeconds, c.JobTimeoutSeconds)
	}

	if c.JobTimeoutSeconds > MaxJobTimeoutSeconds {
		return fmt.Errorf("JOB_TIMEOUT_SECONDS should not exceed 1 hour (%d seconds), got: %d",
			MaxJobTimeoutSeconds, c.JobTimeoutSeconds)
	}

	return nil
}

// validateProjectMappings validates project mappings configuration
func (c *Config) validateProjectMappings() error {
	for jiraKey, gitlabProject := range c.ProjectMappings {
		// Validate Jira project key
		if jiraKey == "" {
			return fmt.Errorf("PROJECT_MAPPINGS: Jira project key cannot be empty")
		}

		if strings.Contains(jiraKey, " ") || strings.Contains(jiraKey, "/") {
			return fmt.Errorf("PROJECT_MAPPINGS: invalid Jira project key '%s' (no spaces or slashes allowed)", jiraKey)
		}

		// Validate GitLab project path
		if gitlabProject == "" {
			return fmt.Errorf("PROJECT_MAPPINGS: GitLab project cannot be empty for Jira key '%s'", jiraKey)
		}

		if !strings.Contains(gitlabProject, "/") {
			return fmt.Errorf("PROJECT_MAPPINGS: GitLab project '%s' should be in format 'namespace/project'", gitlabProject)
		}

		// Check for valid namespace/project format
		parts := strings.Split(gitlabProject, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("PROJECT_MAPPINGS: GitLab project '%s' must be in format 'namespace/project'", gitlabProject)
		}
	}

	// Validate default namespace if no explicit mappings
	if len(c.ProjectMappings) == 0 && c.GitLabNamespace == "" {
		return fmt.Errorf("either PROJECT_MAPPINGS or GITLAB_NAMESPACE must be configured")
	}

	if c.GitLabNamespace != "" {
		if strings.Contains(c.GitLabNamespace, "/") || strings.Contains(c.GitLabNamespace, " ") {
			return fmt.Errorf("GITLAB_NAMESPACE '%s' should be a simple namespace name "+
				"(no slashes or spaces)", c.GitLabNamespace)
		}
	}

	return nil
}

// validateRateLimiting validates rate limiting configuration
func (c *Config) validateRateLimiting() error {
	// Jira rate limiting
	if c.JiraRateLimit < 1 {
		return fmt.Errorf("JIRA_RATE_LIMIT must be at least 1 request per second, got: %d", c.JiraRateLimit)
	}

	if c.JiraRateLimit > MaxJiraRateLimit {
		return fmt.Errorf("JIRA_RATE_LIMIT should not exceed %d requests per second, got: %d",
			MaxJiraRateLimit, c.JiraRateLimit)
	}

	// Retry configuration
	if c.JiraRetryMaxAttempts < 0 {
		return fmt.Errorf("JIRA_RETRY_MAX_ATTEMPTS must be non-negative, got: %d", c.JiraRetryMaxAttempts)
	}

	if c.JiraRetryMaxAttempts > MaxRetryAttempts {
		return fmt.Errorf("JIRA_RETRY_MAX_ATTEMPTS should not exceed %d, got: %d", MaxRetryAttempts, c.JiraRetryMaxAttempts)
	}

	if c.JiraRetryBaseDelayMs < MinRetryDelayMs {
		return fmt.Errorf("JIRA_RETRY_BASE_DELAY_MS should be at least %dms, got: %d",
			MinRetryDelayMs, c.JiraRetryBaseDelayMs)
	}

	// Concurrent jobs
	if c.MaxConcurrentJobs < 1 {
		return fmt.Errorf("MAX_CONCURRENT_JOBS must be at least 1, got: %d", c.MaxConcurrentJobs)
	}

	if c.MaxConcurrentJobs > MaxConcurrentJobsLimit {
		return fmt.Errorf("MAX_CONCURRENT_JOBS should not exceed %d, got: %d", MaxConcurrentJobsLimit, c.MaxConcurrentJobs)
	}

	return nil
}

// validateBidirectionalSync validates bidirectional sync configuration
//
//nolint:gocyclo // This function coordinates multiple validation steps
func (c *Config) validateBidirectionalSync() error {
	// If bidirectional sync is disabled, skip validation
	if !c.BidirectionalEnabled {
		return nil
	}

	// Run all validation steps
	validators := []func() error{
		c.validateConflictStrategy,
		c.validateSyncInterval,
		c.validateBidirectionalEvents,
		c.validateCommentSyncDirection,
		c.validateSyncLimits,
		c.validateEventAge,
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return err
		}
	}

	return nil
}

// validateConflictStrategy validates the conflict resolution strategy
func (c *Config) validateConflictStrategy() error {
	validStrategies := map[string]bool{
		"last_write_wins": true,
		"merge":           true,
		"manual":          true,
	}
	if !validStrategies[c.BidirectionalConflictStrategy] {
		return fmt.Errorf("BIDIRECTIONAL_CONFLICT_STRATEGY must be one of: last_write_wins, merge, manual, got: %s",
			c.BidirectionalConflictStrategy)
	}
	return nil
}

// validateSyncInterval validates the sync interval settings
func (c *Config) validateSyncInterval() error {
	if c.BidirectionalSyncInterval < minSyncInterval {
		return fmt.Errorf("BIDIRECTIONAL_SYNC_INTERVAL must be at least %d seconds, got: %d",
			minSyncInterval, c.BidirectionalSyncInterval)
	}
	if c.BidirectionalSyncInterval > maxSyncInterval {
		return fmt.Errorf("BIDIRECTIONAL_SYNC_INTERVAL should not exceed %d seconds (1 hour), got: %d",
			maxSyncInterval, c.BidirectionalSyncInterval)
	}
	return nil
}

// validateBidirectionalEvents validates the configured events
func (c *Config) validateBidirectionalEvents() error {
	if len(c.BidirectionalEvents) == 0 {
		return nil
	}

	validEvents := map[string]bool{
		"issue_created":   true,
		"issue_updated":   true,
		"issue_deleted":   true,
		"comment_created": true,
		"comment_updated": true,
		"comment_deleted": true,
		"worklog_created": true,
		"worklog_updated": true,
		"worklog_deleted": true,
	}
	for _, event := range c.BidirectionalEvents {
		if !validEvents[event] {
			return fmt.Errorf("unsupported bidirectional event: %s", event)
		}
	}
	return nil
}

// validateCommentSyncDirection validates comment sync direction
func (c *Config) validateCommentSyncDirection() error {
	if !c.CommentSyncEnabled {
		return nil
	}

	validDirections := map[string]bool{
		"jira_to_gitlab": true,
		"gitlab_to_jira": true,
		"bidirectional":  true,
	}
	if !validDirections[c.CommentSyncDirection] {
		return fmt.Errorf("COMMENT_SYNC_DIRECTION must be one of: jira_to_gitlab, gitlab_to_jira, bidirectional, got: %s",
			c.CommentSyncDirection)
	}
	return nil
}

// validateSyncLimits validates sync batch and rate limits
func (c *Config) validateSyncLimits() error {
	if c.SyncBatchSize < 1 {
		return fmt.Errorf("SYNC_BATCH_SIZE must be at least 1, got: %d", c.SyncBatchSize)
	}
	if c.SyncBatchSize > maxSyncBatchSize {
		return fmt.Errorf("SYNC_BATCH_SIZE should not exceed %d, got: %d", maxSyncBatchSize, c.SyncBatchSize)
	}

	if c.SyncRateLimit < 1 {
		return fmt.Errorf("SYNC_RATE_LIMIT must be at least 1 operation per minute, got: %d", c.SyncRateLimit)
	}
	if c.SyncRateLimit > maxSyncRateLimit {
		return fmt.Errorf("SYNC_RATE_LIMIT should not exceed %d operations per minute, got: %d",
			maxSyncRateLimit, c.SyncRateLimit)
	}

	if c.SyncRetryAttempts < 0 {
		return fmt.Errorf("SYNC_RETRY_ATTEMPTS must be non-negative, got: %d", c.SyncRetryAttempts)
	}
	if c.SyncRetryAttempts > maxSyncRetryAttempts {
		return fmt.Errorf("SYNC_RETRY_ATTEMPTS should not exceed %d, got: %d", maxSyncRetryAttempts, c.SyncRetryAttempts)
	}
	return nil
}

// validateEventAge validates event age settings
func (c *Config) validateEventAge() error {
	if !c.SkipOldEvents {
		return nil
	}

	if c.MaxEventAge < 1 {
		return fmt.Errorf("MAX_EVENT_AGE must be at least 1 hour when SKIP_OLD_EVENTS is enabled, got: %d", c.MaxEventAge)
	}
	if c.MaxEventAge > maxEventAgeHours { // 1 week
		return fmt.Errorf("MAX_EVENT_AGE should not exceed %d hours (1 week), got: %d", maxEventAgeHours, c.MaxEventAge)
	}
	return nil
}

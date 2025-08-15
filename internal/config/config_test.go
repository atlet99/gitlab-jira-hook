package config

import (
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		checkConfig func(*testing.T, *Config)
	}{
		{
			name: "load with default values",
			envVars: map[string]string{
				"GITLAB_SECRET":   "test-secret",
				"GITLAB_BASE_URL": "https://gitlab.example.com",
				"JIRA_EMAIL":      "test@example.com",
				"JIRA_TOKEN":      "test-token",
				"JIRA_BASE_URL":   "https://jira.example.com",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-secret", cfg.GitLabSecret)
				assert.Equal(t, "test@example.com", cfg.JiraEmail)
				assert.Equal(t, "test-token", cfg.JiraToken)
				assert.Equal(t, "https://jira.example.com", cfg.JiraBaseURL)
				assert.Equal(t, DefaultPort, cfg.Port)
				assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
				assert.Equal(t, DefaultTimezone, cfg.Timezone)
			},
		},
		{
			name: "load with custom values",
			envVars: map[string]string{
				"GITLAB_SECRET":      "custom-secret",
				"GITLAB_BASE_URL":    "https://custom-gitlab.example.com",
				"JIRA_EMAIL":         "custom@example.com",
				"JIRA_API_TOKEN":     "custom-token",
				"JIRA_BASE_URL":      "https://custom-jira.example.com",
				"PORT":               "9090",
				"LOG_LEVEL":          "debug",
				"TIMEZONE":           "UTC",
				"WORKER_POOL_SIZE":   "20",
				"JOB_QUEUE_SIZE":     "200",
				"MIN_WORKERS":        "5",
				"MAX_WORKERS":        "50",
				"SCALE_UP_THRESHOLD": "15",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom-secret", cfg.GitLabSecret)
				assert.Equal(t, "custom@example.com", cfg.JiraEmail)
				assert.Equal(t, "custom-token", cfg.JiraToken)
				assert.Equal(t, "https://custom-jira.example.com", cfg.JiraBaseURL)
				assert.Equal(t, "9090", cfg.Port)
				assert.Equal(t, "debug", cfg.LogLevel)
				assert.Equal(t, "UTC", cfg.Timezone)
				assert.Equal(t, 20, cfg.WorkerPoolSize)
				assert.Equal(t, 200, cfg.JobQueueSize)
				assert.Equal(t, 5, cfg.MinWorkers)
				assert.Equal(t, 50, cfg.MaxWorkers)
				assert.Equal(t, 15, cfg.ScaleUpThreshold)
			},
		},
		{
			name: "load with CSV arrays",
			envVars: map[string]string{
				"GITLAB_SECRET":      "test-secret",
				"GITLAB_BASE_URL":    "https://gitlab.example.com",
				"JIRA_EMAIL":         "test@example.com",
				"JIRA_TOKEN":         "test-token",
				"JIRA_BASE_URL":      "https://jira.example.com",
				"ALLOWED_PROJECTS":   "project1,project2,project3",
				"ALLOWED_GROUPS":     "group1,group2",
				"PUSH_BRANCH_FILTER": "main,develop,release-*",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, []string{"project1", "project2", "project3"}, cfg.AllowedProjects)
				assert.Equal(t, []string{"group1", "group2"}, cfg.AllowedGroups)
				assert.Equal(t, []string{"main", "develop", "release-*"}, cfg.PushBranchFilter)
			},
		},
		{
			name: "load with float values",
			envVars: map[string]string{
				"GITLAB_SECRET":      "test-secret",
				"GITLAB_BASE_URL":    "https://gitlab.example.com",
				"JIRA_EMAIL":         "test@example.com",
				"JIRA_TOKEN":         "test-token",
				"JIRA_BASE_URL":      "https://jira.example.com",
				"BACKOFF_MULTIPLIER": "2.5",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 2.5, cfg.BackoffMultiplier)
			},
		},
		{
			name: "load with boolean values",
			envVars: map[string]string{
				"GITLAB_SECRET":   "test-secret",
				"GITLAB_BASE_URL": "https://gitlab.example.com",
				"JIRA_EMAIL":      "test@example.com",
				"JIRA_API_TOKEN":  "test-token",
				"JIRA_BASE_URL":   "https://jira.example.com",
				"METRICS_ENABLED": "false",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.False(t, cfg.MetricsEnabled)
			},
		},
		{
			name: "missing required fields",
			envVars: map[string]string{
				"GITLAB_SECRET":   "test-secret",
				"GITLAB_BASE_URL": "https://gitlab.example.com",
				// Missing JIRA_EMAIL, JIRA_TOKEN, JIRA_BASE_URL
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Remove .env file to prevent interference
			if err := os.Remove(".env"); err != nil && !os.IsNotExist(err) {
				t.Fatalf("Failed to remove .env file: %v", err)
			}
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				// Clean up environment variables
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Test Load function
			cfg, err := Load()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}
		})
	}
}

func TestNewConfigFromEnv(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		checkConfig func(*testing.T, *Config)
	}{
		{
			name: "create config with default values",
			envVars: map[string]string{
				"GITLAB_SECRET":   "test-secret",
				"GITLAB_BASE_URL": "https://gitlab.example.com",
				"JIRA_EMAIL":      "test@example.com",
				"JIRA_API_TOKEN":  "test-token",
				"JIRA_BASE_URL":   "https://jira.example.com",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-secret", cfg.GitLabSecret)
				assert.Equal(t, "test@example.com", cfg.JiraEmail)
				assert.Equal(t, "test-token", cfg.JiraToken)
				assert.Equal(t, "https://jira.example.com", cfg.JiraBaseURL)
				assert.Equal(t, DefaultPort, cfg.Port)
				assert.Equal(t, DefaultLogLevel, cfg.LogLevel)
				assert.Equal(t, DefaultTimezone, cfg.Timezone)
				assert.Equal(t, DefaultWorkerPoolSize, cfg.WorkerPoolSize)
				assert.Equal(t, DefaultJobQueueSize, cfg.JobQueueSize)
				assert.Equal(t, DefaultMinWorkers, cfg.MinWorkers)
				assert.Equal(t, DefaultMaxWorkers, cfg.MaxWorkers)
				assert.Equal(t, DefaultScaleUpThreshold, cfg.ScaleUpThreshold)
				assert.Equal(t, DefaultScaleDownThreshold, cfg.ScaleDownThreshold)
				assert.Equal(t, DefaultScaleInterval, cfg.ScaleInterval)
				assert.Equal(t, DefaultMaxConcurrentJobs, cfg.MaxConcurrentJobs)
				assert.Equal(t, DefaultJobTimeoutSeconds, cfg.JobTimeoutSeconds)
				assert.Equal(t, DefaultQueueTimeoutMs, cfg.QueueTimeoutMs)
				assert.Equal(t, DefaultMaxRetries, cfg.MaxRetries)
				assert.Equal(t, DefaultRetryDelayMs, cfg.RetryDelayMs)
				assert.Equal(t, DefaultBackoffMultiplier, cfg.BackoffMultiplier)
				assert.Equal(t, DefaultMaxBackoffMs, cfg.MaxBackoffMs)
				assert.Equal(t, DefaultMetricsEnabled, cfg.MetricsEnabled)
				assert.Equal(t, DefaultHealthCheckInterval, cfg.HealthCheckInterval)
			},
		},
		{
			name: "create config with custom values",
			envVars: map[string]string{
				"GITLAB_SECRET":         "custom-secret",
				"GITLAB_BASE_URL":       "https://custom-gitlab.example.com",
				"JIRA_EMAIL":            "custom@example.com",
				"JIRA_TOKEN":            "custom-token",
				"JIRA_BASE_URL":         "https://custom-jira.example.com",
				"PORT":                  "9090",
				"LOG_LEVEL":             "debug",
				"TIMEZONE":              "UTC",
				"WORKER_POOL_SIZE":      "20",
				"JOB_QUEUE_SIZE":        "200",
				"MIN_WORKERS":           "5",
				"MAX_WORKERS":           "50",
				"SCALE_UP_THRESHOLD":    "15",
				"SCALE_DOWN_THRESHOLD":  "3",
				"SCALE_INTERVAL":        "20",
				"MAX_CONCURRENT_JOBS":   "100",
				"JOB_TIMEOUT_SECONDS":   "60",
				"QUEUE_TIMEOUT_MS":      "10000",
				"MAX_RETRIES":           "5",
				"RETRY_DELAY_MS":        "2000",
				"BACKOFF_MULTIPLIER":    "3.0",
				"MAX_BACKOFF_MS":        "60000",
				"METRICS_ENABLED":       "false",
				"HEALTH_CHECK_INTERVAL": "60",
			},
			expectError: false,
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom-secret", cfg.GitLabSecret)
				assert.Equal(t, "custom@example.com", cfg.JiraEmail)
				assert.Equal(t, "custom-token", cfg.JiraToken)
				assert.Equal(t, "https://custom-jira.example.com", cfg.JiraBaseURL)
				assert.Equal(t, "9090", cfg.Port)
				assert.Equal(t, "debug", cfg.LogLevel)
				assert.Equal(t, "UTC", cfg.Timezone)
				assert.Equal(t, 20, cfg.WorkerPoolSize)
				assert.Equal(t, 200, cfg.JobQueueSize)
				assert.Equal(t, 5, cfg.MinWorkers)
				assert.Equal(t, 50, cfg.MaxWorkers)
				assert.Equal(t, 15, cfg.ScaleUpThreshold)
				assert.Equal(t, 3, cfg.ScaleDownThreshold)
				assert.Equal(t, 20, cfg.ScaleInterval)
				assert.Equal(t, 100, cfg.MaxConcurrentJobs)
				assert.Equal(t, 60, cfg.JobTimeoutSeconds)
				assert.Equal(t, 10000, cfg.QueueTimeoutMs)
				assert.Equal(t, 5, cfg.MaxRetries)
				assert.Equal(t, 2000, cfg.RetryDelayMs)
				assert.Equal(t, 3.0, cfg.BackoffMultiplier)
				assert.Equal(t, 60000, cfg.MaxBackoffMs)
				assert.False(t, cfg.MetricsEnabled)
				assert.Equal(t, 60, cfg.HealthCheckInterval)
			},
		},
		{
			name: "missing required fields",
			envVars: map[string]string{
				"GITLAB_SECRET":   "test-secret",
				"GITLAB_BASE_URL": "https://gitlab.example.com",
				// Missing JIRA_EMAIL, JIRA_TOKEN, JIRA_BASE_URL
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				// Clean up environment variables
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Test NewConfigFromEnv function
			cfg := NewConfigFromEnv(logger)

			if tt.expectError {
				assert.Error(t, cfg.validate())
				return
			}

			require.NoError(t, cfg.validate())

			if tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				GitLabSecret:  "test-secret",
				GitLabBaseURL: "https://gitlab.example.com",
				JiraEmail:     "test@example.com",
				JiraToken:     "test-token",
				JiraBaseURL:   "https://jira.example.com",
				Port:          "8080",
			},
			expectError: false,
		},
		{
			name: "missing GitLab secret",
			config: &Config{
				JiraEmail:   "test@example.com",
				JiraToken:   "test-token",
				JiraBaseURL: "https://jira.example.com",
			},
			expectError: true,
		},
		{
			name: "missing Jira email",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraToken:    "test-token",
				JiraBaseURL:  "https://jira.example.com",
			},
			expectError: true,
		},
		{
			name: "missing Jira token",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraBaseURL:  "https://jira.example.com",
			},
			expectError: true,
		},
		{
			name: "missing Jira base URL",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
			},
			expectError: true,
		},
		{
			name: "invalid Jira base URL",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
				JiraBaseURL:  "not-a-url",
			},
			expectError: true,
		},
		{
			name: "invalid port",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
				JiraBaseURL:  "https://jira.example.com",
				Port:         "not-a-port",
			},
			expectError: true,
		},
		{
			name: "invalid worker pool size",
			config: &Config{
				GitLabSecret:   "test-secret",
				JiraEmail:      "test@example.com",
				JiraToken:      "test-token",
				JiraBaseURL:    "https://jira.example.com",
				WorkerPoolSize: -1,
			},
			expectError: true,
		},
		{
			name: "invalid job queue size",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
				JiraBaseURL:  "https://jira.example.com",
				JobQueueSize: -1,
			},
			expectError: true,
		},
		{
			name: "min workers greater than max workers",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
				JiraBaseURL:  "https://jira.example.com",
				MinWorkers:   10,
				MaxWorkers:   5,
			},
			expectError: true,
		},
		{
			name: "invalid retry delay",
			config: &Config{
				GitLabSecret: "test-secret",
				JiraEmail:    "test@example.com",
				JiraToken:    "test-token",
				JiraBaseURL:  "https://jira.example.com",
				RetryDelayMs: -1,
			},
			expectError: true,
		},
		{
			name: "invalid backoff multiplier",
			config: &Config{
				GitLabSecret:      "test-secret",
				JiraEmail:         "test@example.com",
				JiraToken:         "test-token",
				JiraBaseURL:       "https://jira.example.com",
				BackoffMultiplier: -1.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("getEnv with default", func(t *testing.T) {
		os.Unsetenv("TEST_VAR")
		result := getEnv("TEST_VAR", "default-value")
		assert.Equal(t, "default-value", result)
	})

	t.Run("getEnv with value", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test-value")
		defer os.Unsetenv("TEST_VAR")
		result := getEnv("TEST_VAR", "default-value")
		assert.Equal(t, "test-value", result)
	})

	t.Run("parseIntEnv with default", func(t *testing.T) {
		os.Unsetenv("TEST_INT")
		result := parseIntEnv("TEST_INT", 42)
		assert.Equal(t, 42, result)
	})

	t.Run("parseIntEnv with valid value", func(t *testing.T) {
		os.Setenv("TEST_INT", "123")
		defer os.Unsetenv("TEST_INT")
		result := parseIntEnv("TEST_INT", 42)
		assert.Equal(t, 123, result)
	})

	t.Run("parseIntEnv with invalid value", func(t *testing.T) {
		os.Setenv("TEST_INT", "not-a-number")
		defer os.Unsetenv("TEST_INT")
		result := parseIntEnv("TEST_INT", 42)
		assert.Equal(t, 42, result)
	})

	t.Run("parseCSVEnv with empty", func(t *testing.T) {
		os.Unsetenv("TEST_CSV")
		result := parseCSVEnv("TEST_CSV")
		assert.Empty(t, result)
	})

	t.Run("parseCSVEnv with values", func(t *testing.T) {
		os.Setenv("TEST_CSV", "value1,value2,value3")
		defer os.Unsetenv("TEST_CSV")
		result := parseCSVEnv("TEST_CSV")
		assert.Equal(t, []string{"value1", "value2", "value3"}, result)
	})

	t.Run("parseFloatEnv with default", func(t *testing.T) {
		os.Unsetenv("TEST_FLOAT")
		result := parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, 3.14, result)
	})

	t.Run("parseFloatEnv with valid value", func(t *testing.T) {
		os.Setenv("TEST_FLOAT", "2.718")
		defer os.Unsetenv("TEST_FLOAT")
		result := parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, 2.718, result)
	})

	t.Run("parseFloatEnv with invalid value", func(t *testing.T) {
		os.Setenv("TEST_FLOAT", "not-a-float")
		defer os.Unsetenv("TEST_FLOAT")
		result := parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, 3.14, result)
	})

	t.Run("parseBoolEnv with default", func(t *testing.T) {
		os.Unsetenv("TEST_BOOL")
		result := parseBoolEnv("TEST_BOOL", true)
		assert.True(t, result)
	})

	t.Run("parseBoolEnv with true", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")
		result := parseBoolEnv("TEST_BOOL", false)
		assert.True(t, result)
	})

	t.Run("parseBoolEnv with false", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "false")
		defer os.Unsetenv("TEST_BOOL")
		result := parseBoolEnv("TEST_BOOL", true)
		assert.False(t, result)
	})

	t.Run("parseBoolEnv with invalid value", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "not-a-bool")
		defer os.Unsetenv("TEST_BOOL")
		result := parseBoolEnv("TEST_BOOL", true)
		assert.True(t, result)
	})
}

func TestAutoConfiguration(t *testing.T) {
	t.Run("autoMaxWorkers", func(t *testing.T) {
		// Test with reasonable values
		result := autoMaxWorkers(4, 1024) // 4 CPU cores, 1GB memory
		assert.Greater(t, result, 0)
		assert.LessOrEqual(t, result, AutoMaxWorkers)

		// Test with high values
		result = autoMaxWorkers(32, 8192) // 32 CPU cores, 8GB memory
		assert.Equal(t, AutoMaxWorkers, result)

		// Test with low values
		result = autoMaxWorkers(1, 256) // 1 CPU core, 256MB memory
		assert.Greater(t, result, 0)
		assert.LessOrEqual(t, result, AutoMaxWorkers)
	})

	t.Run("autoQueueSize", func(t *testing.T) {
		// Test with reasonable values
		result := autoQueueSize(10, 1024) // 10 workers, 1GB memory
		assert.Greater(t, result, 0)
		assert.LessOrEqual(t, result, DefaultMaxQueueSize)

		// Test with high values
		result = autoQueueSize(100, 8192) // 100 workers, 8GB memory
		assert.Equal(t, DefaultMaxQueueSize, result)

		// Test with low values
		result = autoQueueSize(2, 256) // 2 workers, 256MB memory
		assert.GreaterOrEqual(t, result, DefaultMinQueueSize)
	})

	t.Run("detectMemoryLimitMB", func(t *testing.T) {
		result := detectMemoryLimitMB()
		// Should be reasonable for most systems
		assert.Less(t, result, 100000) // Less than 100GB
		// Note: result might be 0 on some systems, which is acceptable
	})
}

func TestConfigurationIntegration(t *testing.T) {
	t.Run("full configuration lifecycle", func(t *testing.T) {
		// Set up environment
		envVars := map[string]string{
			"GITLAB_SECRET":         "integration-secret",
			"GITLAB_BASE_URL":       "https://integration-gitlab.example.com",
			"JIRA_EMAIL":            "integration@example.com",
			"JIRA_TOKEN":            "integration-token",
			"JIRA_BASE_URL":         "https://integration-jira.example.com",
			"PORT":                  "9090",
			"LOG_LEVEL":             "debug",
			"TIMEZONE":              "UTC",
			"WORKER_POOL_SIZE":      "15",
			"JOB_QUEUE_SIZE":        "150",
			"MIN_WORKERS":           "3",
			"MAX_WORKERS":           "30",
			"ALLOWED_PROJECTS":      "project1,project2",
			"ALLOWED_GROUPS":        "group1,group2",
			"PUSH_BRANCH_FILTER":    "main,develop,release-*",
			"METRICS_ENABLED":       "true",
			"HEALTH_CHECK_INTERVAL": "45",
		}

		for key, value := range envVars {
			os.Setenv(key, value)
		}
		defer func() {
			for key := range envVars {
				os.Unsetenv(key)
			}
		}()

		// Test Load function
		cfg, err := Load()
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify configuration
		assert.Equal(t, "integration-secret", cfg.GitLabSecret)
		assert.Equal(t, "integration@example.com", cfg.JiraEmail)
		assert.Equal(t, "integration-token", cfg.JiraToken)
		assert.Equal(t, "https://integration-jira.example.com", cfg.JiraBaseURL)
		assert.Equal(t, "9090", cfg.Port)
		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, "UTC", cfg.Timezone)
		assert.Equal(t, 15, cfg.WorkerPoolSize)
		assert.Equal(t, 150, cfg.JobQueueSize)
		assert.Equal(t, 3, cfg.MinWorkers)
		assert.Equal(t, 30, cfg.MaxWorkers)
		assert.Equal(t, []string{"project1", "project2"}, cfg.AllowedProjects)
		assert.Equal(t, []string{"group1", "group2"}, cfg.AllowedGroups)
		assert.Equal(t, []string{"main", "develop", "release-*"}, cfg.PushBranchFilter)
		assert.True(t, cfg.MetricsEnabled)
		assert.Equal(t, 45, cfg.HealthCheckInterval)

		// Test validation
		err = cfg.validate()
		assert.NoError(t, err)
	})
}

func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("empty CSV values", func(t *testing.T) {
		os.Setenv("TEST_CSV", "")
		defer os.Unsetenv("TEST_CSV")
		result := parseCSVEnv("TEST_CSV")
		assert.Empty(t, result)
	})

	t.Run("CSV with spaces", func(t *testing.T) {
		os.Setenv("TEST_CSV", " value1 , value2 , value3 ")
		defer os.Unsetenv("TEST_CSV")
		result := parseCSVEnv("TEST_CSV")
		assert.Equal(t, []string{"value1", "value2", "value3"}, result)
	})

	t.Run("CSV with empty values", func(t *testing.T) {
		os.Setenv("TEST_CSV", "value1,,value3")
		defer os.Unsetenv("TEST_CSV")
		result := parseCSVEnv("TEST_CSV")
		assert.Equal(t, []string{"value1", "", "value3"}, result)
	})

	t.Run("boolean edge cases", func(t *testing.T) {
		testCases := []struct {
			value    string
			default_ bool
			expected bool
		}{
			{"1", false, true},
			{"0", true, false},
			{"true", false, true},
			{"false", true, false},
			{"TRUE", false, true},
			{"FALSE", true, false},
		}

		for _, tc := range testCases {
			t.Run(tc.value, func(t *testing.T) {
				os.Setenv("TEST_BOOL", tc.value)
				defer os.Unsetenv("TEST_BOOL")
				result := parseBoolEnv("TEST_BOOL", tc.default_)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("integer edge cases", func(t *testing.T) {
		// Test with very large numbers
		os.Setenv("TEST_INT", "999999999")
		defer os.Unsetenv("TEST_INT")
		result := parseIntEnv("TEST_INT", 42)
		assert.Equal(t, 999999999, result)

		// Test with zero
		os.Setenv("TEST_INT", "0")
		result = parseIntEnv("TEST_INT", 42)
		assert.Equal(t, 0, result)

		// Test with negative numbers
		os.Setenv("TEST_INT", "-123")
		result = parseIntEnv("TEST_INT", 42)
		assert.Equal(t, -123, result)
	})

	t.Run("float edge cases", func(t *testing.T) {
		// Test with scientific notation
		os.Setenv("TEST_FLOAT", "1.23e-4")
		defer os.Unsetenv("TEST_FLOAT")
		result := parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, 1.23e-4, result)

		// Test with zero
		os.Setenv("TEST_FLOAT", "0.0")
		result = parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, 0.0, result)

		// Test with negative numbers
		os.Setenv("TEST_FLOAT", "-2.718")
		result = parseFloatEnv("TEST_FLOAT", 3.14)
		assert.Equal(t, -2.718, result)
	})
}

func TestConfigurationPerformance(t *testing.T) {
	t.Run("load performance", func(t *testing.T) {
		// Set up environment
		envVars := map[string]string{
			"GITLAB_SECRET":   "perf-secret",
			"GITLAB_BASE_URL": "https://perf-gitlab.example.com",
			"JIRA_EMAIL":      "perf@example.com",
			"JIRA_TOKEN":      "perf-token",
			"JIRA_BASE_URL":   "https://perf-jira.example.com",
		}

		for key, value := range envVars {
			os.Setenv(key, value)
		}
		defer func() {
			for key := range envVars {
				os.Unsetenv(key)
			}
		}()

		// Measure performance
		start := time.Now()
		for i := 0; i < 1000; i++ {
			cfg := NewConfigFromEnv(slog.Default())
			require.NoError(t, cfg.validate())
		}
		duration := time.Since(start)

		// Should complete within reasonable time
		assert.Less(t, duration, 5*time.Second, "Configuration loading should be fast")
		t.Logf("Loaded 1000 configurations in %v", duration)
	})
}

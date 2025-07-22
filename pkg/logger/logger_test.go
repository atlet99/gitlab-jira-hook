package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_DefaultLevel(t *testing.T) {
	// Clear environment variable
	os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_DebugLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_InfoLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_WarnLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "warn")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_ErrorLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "invalid")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default (should default to info)
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_EmptyLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Test that logger is set as default
	defaultLogger := slog.Default()
	assert.Equal(t, logger, defaultLogger)
}

func TestNewLogger_LoggingFunctionality(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	// Create logger
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Log a message
	logger.Info("test message", "key", "value")

	// Close pipe and read output
	w.Close()
	os.Stdout = originalStdout
	_, _ = buf.ReadFrom(r)

	// Parse JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify log entry structure
	assert.Contains(t, logEntry, "time")
	assert.Contains(t, logEntry, "level")
	assert.Contains(t, logEntry, "msg")
	assert.Contains(t, logEntry, "key")
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "value", logEntry["key"])
}

func TestNewLogger_LogLevels(t *testing.T) {
	testCases := []struct {
		name     string
		envLevel string
		expected string
	}{
		{"debug level", "debug", "DEBUG"},
		{"info level", "info", "INFO"},
		{"invalid level", "invalid", "INFO"},
		{"empty level", "", "INFO"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment and create logger
			if tc.envLevel != "" {
				os.Setenv("LOG_LEVEL", tc.envLevel)
			} else {
				os.Unsetenv("LOG_LEVEL")
			}
			defer os.Unsetenv("LOG_LEVEL")

			logger := NewLogger()
			assert.NotNil(t, logger)

			// Verify that logger was created successfully
			defaultLogger := slog.Default()
			assert.Equal(t, logger, defaultLogger)
		})
	}
}

func TestNewLogger_StructuredLogging(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	// Create logger
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Log structured data
	logger.Info("user action",
		"user_id", 123,
		"action", "login",
		"ip", "192.168.1.1",
		"success", true,
	)

	// Close pipe and read output
	w.Close()
	os.Stdout = originalStdout
	_, _ = buf.ReadFrom(r)

	// Parse JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify structured data
	assert.Equal(t, "user action", logEntry["msg"])
	assert.Equal(t, float64(123), logEntry["user_id"])
	assert.Equal(t, "login", logEntry["action"])
	assert.Equal(t, "192.168.1.1", logEntry["ip"])
	assert.Equal(t, true, logEntry["success"])
}

func TestNewLogger_ErrorLogging(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	// Create logger
	os.Setenv("LOG_LEVEL", "error")
	defer os.Unsetenv("LOG_LEVEL")

	logger := NewLogger()
	assert.NotNil(t, logger)

	// Log error
	logger.Error("database connection failed",
		"error", "connection timeout",
		"retry_count", 3,
	)

	// Close pipe and read output
	w.Close()
	os.Stdout = originalStdout
	_, _ = buf.ReadFrom(r)

	// Parse JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	// Verify error log
	assert.Equal(t, "database connection failed", logEntry["msg"])
	assert.Equal(t, "connection timeout", logEntry["error"])
	assert.Equal(t, float64(3), logEntry["retry_count"])
}

func TestNewLogger_DefaultLoggerReplacement(t *testing.T) {
	// Create initial default logger
	initialLogger := slog.Default()

	// Create new logger
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	newLogger := NewLogger()
	assert.NotNil(t, newLogger)

	// Verify that default logger was replaced
	currentLogger := slog.Default()
	assert.Equal(t, newLogger, currentLogger)
	assert.NotEqual(t, initialLogger, currentLogger)
}

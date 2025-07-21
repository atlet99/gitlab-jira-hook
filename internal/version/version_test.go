package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		buildNumber string
		expected    string
	}{
		{"version with build number", "1.0.0", "123", "1.0.0 (build 123)"},
		{"version without build number", "1.0.0", "0", "1.0.0"},
		{"version with empty build number", "1.0.0", "", "1.0.0"},
		{"dev version", "dev", "0", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originalVersion := Version
			originalBuildNumber := BuildNumber

			// Set test values
			Version = tt.version
			BuildNumber = tt.buildNumber

			// Test
			result := GetVersion()
			assert.Equal(t, tt.expected, result)

			// Restore original values
			Version = originalVersion
			BuildNumber = originalBuildNumber
		})
	}
}

func TestGetFullVersionInfo(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalBuiltBy := BuiltBy

	// Set test values
	Version = "1.0.0"
	Commit = "abcdef123456789"
	Date = "2023_12_25_10_30_00"
	BuiltBy = "test-user"

	// Test
	result := GetFullVersionInfo()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook 1.0.0")
	assert.Contains(t, result, "(abcdef1)")
	assert.Contains(t, result, "built 2023 12 25 10 30 00")
	assert.Contains(t, result, "by test-user")
	assert.Contains(t, result, "with go")
	assert.Contains(t, result, "for ")

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
	Date = originalDate
	BuiltBy = originalBuiltBy
}

func TestGetFullVersionInfo_WithUnknownValues(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalBuiltBy := BuiltBy

	// Set test values with unknown values
	Version = "1.0.0"
	Commit = UnknownValue
	Date = UnknownValue
	BuiltBy = UnknownValue

	// Test
	result := GetFullVersionInfo()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook 1.0.0")
	assert.NotContains(t, result, "(unknown)")
	assert.NotContains(t, result, "built unknown")
	assert.NotContains(t, result, "by unknown")
	assert.Contains(t, result, "with go")
	assert.Contains(t, result, "for ")

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
	Date = originalDate
	BuiltBy = originalBuiltBy
}

func TestGetFullVersionInfo_WithShortCommit(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit

	// Set test values with short commit
	Version = "1.0.0"
	Commit = "abc123"

	// Test
	result := GetFullVersionInfo()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook 1.0.0")
	assert.Contains(t, result, "(abc123)")

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
}

func TestGetFullVersionInfo_WithLongCommit(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit

	// Set test values with long commit
	Version = "1.0.0"
	Commit = "abcdef123456789abcdef123456789abcdef12"

	// Test
	result := GetFullVersionInfo()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook 1.0.0")
	assert.Contains(t, result, "(abcdef1)") // Should be truncated

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
}

func TestGet(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalBuiltBy := BuiltBy

	// Set test values
	Version = "1.0.0"
	Commit = "abcdef123456789"
	Date = "2023-12-25"
	BuiltBy = "test-user"

	// Test
	buildInfo := Get()

	// Verify the result
	assert.Equal(t, "1.0.0", buildInfo.Version)
	assert.Equal(t, "abcdef123456789", buildInfo.Commit)
	assert.Equal(t, "2023-12-25", buildInfo.Date)
	assert.Equal(t, "test-user", buildInfo.BuiltBy)
	assert.Contains(t, buildInfo.GoVersion, "go")
	assert.Contains(t, buildInfo.Platform, "/")

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
	Date = originalDate
	BuiltBy = originalBuiltBy
}

func TestBuildInfo_String(t *testing.T) {
	buildInfo := &BuildInfo{
		Version:   "1.0.0",
		Commit:    "abcdef123456789",
		Date:      "2023-12-25",
		BuiltBy:   "test-user",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
	}

	result := buildInfo.String()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook version 1.0.0")
	assert.Contains(t, result, "(abcdef123456789)")
	assert.Contains(t, result, "built on 2023-12-25")
	assert.Contains(t, result, "by test-user")
	assert.Contains(t, result, "using go1.21.0")
	assert.Contains(t, result, "for linux/amd64")
}

func TestBuildInfo_Short(t *testing.T) {
	buildInfo := &BuildInfo{
		Version: "1.0.0",
	}

	result := buildInfo.Short()

	assert.Equal(t, "gitlab-jira-hook 1.0.0", result)
}

func TestConstants(t *testing.T) {
	// Test constants
	assert.Equal(t, 7, ShortCommitHashLength)
	assert.Equal(t, "unknown", UnknownValue)
}

func TestGetFullVersionInfo_Formatting(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalBuiltBy := BuiltBy

	// Set test values
	Version = "1.0.0"
	Commit = "abcdef123456789"
	Date = "2023_12_25_10_30_00"
	BuiltBy = "test-user"

	// Test
	result := GetFullVersionInfo()

	// Verify formatting
	lines := strings.Split(result, "\n")
	assert.GreaterOrEqual(t, len(lines), 1)

	// First line should contain version and commit
	firstLine := lines[0]
	assert.Contains(t, firstLine, "gitlab-jira-hook 1.0.0")
	assert.Contains(t, firstLine, "(abcdef1)")

	// Second line should contain build info
	if len(lines) > 1 {
		secondLine := lines[1]
		assert.Contains(t, secondLine, "built 2023 12 25 10 30 00")
		assert.Contains(t, secondLine, "by test-user")
		assert.Contains(t, secondLine, "with go")
		assert.Contains(t, secondLine, "for ")
	}

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
	Date = originalDate
	BuiltBy = originalBuiltBy
}

func TestGetFullVersionInfo_EmptyValues(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalBuiltBy := BuiltBy

	// Set test values with empty values
	Version = "1.0.0"
	Commit = ""
	Date = ""
	BuiltBy = ""

	// Test
	result := GetFullVersionInfo()

	// Verify the result contains expected information
	assert.Contains(t, result, "gitlab-jira-hook 1.0.0")
	assert.NotContains(t, result, "()")     // Should not have empty commit
	assert.NotContains(t, result, "built ") // Should not have build info
	assert.NotContains(t, result, "by ")    // Should not have built by info
	assert.Contains(t, result, "with go")
	assert.Contains(t, result, "for ")

	// Restore original values
	Version = originalVersion
	Commit = originalCommit
	Date = originalDate
	BuiltBy = originalBuiltBy
}

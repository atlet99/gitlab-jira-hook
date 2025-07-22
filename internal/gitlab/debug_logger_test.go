package gitlab

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDebugLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	debugLogger := NewDebugLogger(logger)

	assert.NotNil(t, debugLogger)
	assert.Equal(t, logger, debugLogger.logger)
}

func TestDebugLogger_LogWebhookRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	// Create a test request
	req, err := http.NewRequest("POST", "/webhook", strings.NewReader(`{"test":"data"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "test-token")
	req.Header.Set("X-Gitlab-Event", "push")

	// Create a test event
	event := &Event{
		ObjectKind:  "push",
		Type:        "push",
		UserID:      123,
		UserName:    "test-user",
		ProjectID:   456,
		ProjectName: "test-project",
	}

	// Test debug logging
	debugLogger.LogWebhookRequest(req, []byte(`{"test":"data"}`), event)

	output := buf.String()
	assert.Contains(t, output, "=== GITLAB WEBHOOK DEBUG START ===")
	assert.Contains(t, output, "=== GITLAB WEBHOOK DEBUG END ===")
	assert.Contains(t, output, "Request Headers")
	assert.Contains(t, output, "Request Body")
	assert.Contains(t, output, "Parsed Webhook Event")
}

func TestDebugLogger_LogWebhookRequest_NoEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	req, err := http.NewRequest("POST", "/webhook", strings.NewReader(`{"test":"data"}`))
	require.NoError(t, err)

	// Test with nil event
	debugLogger.LogWebhookRequest(req, []byte(`{"test":"data"}`), nil)

	output := buf.String()
	assert.Contains(t, output, "=== GITLAB WEBHOOK DEBUG START ===")
	assert.Contains(t, output, "=== GITLAB WEBHOOK DEBUG END ===")
	assert.Contains(t, output, "Request Headers")
	assert.Contains(t, output, "Request Body")
	// Should not contain parsed event info when event is nil
	assert.NotContains(t, output, "Parsed Webhook Event")
}

func TestDebugLogger_LogWebhookEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	event := &Event{
		ObjectKind:  "push",
		Type:        "push",
		UserID:      123,
		UserName:    "test-user",
		ProjectID:   456,
		ProjectName: "test-project",
	}

	debugLogger.LogWebhookEvent(event)

	output := buf.String()
	assert.Contains(t, output, "Parsed Webhook Event")
	assert.Contains(t, output, "object_kind")
	assert.Contains(t, output, "push")
	assert.Contains(t, output, "user_name")
	assert.Contains(t, output, "test-user")
}

func TestDebugLogger_LogWebhookEvent_NilEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	// Test with nil event
	debugLogger.LogWebhookEvent(nil)

	output := buf.String()
	assert.Empty(t, output)
}

func TestDebugLogger_LogPushEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	event := &Event{
		ObjectKind:  "push",
		Type:        "push",
		Before:      "abc123",
		After:       "def456",
		Ref:         "refs/heads/main",
		UserID:      123,
		UserName:    "test-user",
		ProjectID:   456,
		ProjectName: "test-project",
		Commits: []Commit{
			{
				ID:      "commit1",
				Message: "test commit",
				Author: Author{
					Name:  "test author",
					Email: "test@example.com",
				},
			},
		},
	}

	debugLogger.LogWebhookEvent(event)

	output := buf.String()
	assert.Contains(t, output, "Push Event Details")
	assert.Contains(t, output, "before")
	assert.Contains(t, output, "abc123")
	assert.Contains(t, output, "after")
	assert.Contains(t, output, "def456")
	assert.Contains(t, output, "ref")
	assert.Contains(t, output, "refs/heads/main")
	assert.Contains(t, output, "commits_count")
	assert.Contains(t, output, "1")
}

func TestDebugLogger_LogMergeRequestEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	event := &Event{
		ObjectKind: "merge_request",
		Type:       "merge_request",
		User: &User{
			ID:    123,
			Name:  "test-user",
			Email: "test@example.com",
		},
		Project: &Project{
			ID:   456,
			Name: "test-project",
		},
		MergeRequest: &MergeRequest{
			ID:           789,
			Title:        "Test MR",
			State:        "opened",
			TargetBranch: "main",
			SourceBranch: "feature/test",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:           789,
			Title:        "Test MR",
			State:        "opened",
			TargetBranch: "main",
			SourceBranch: "feature/test",
		},
	}

	debugLogger.LogWebhookEvent(event)

	output := buf.String()
	assert.Contains(t, output, "Merge Request Event Details")
	assert.Contains(t, output, "mr_id")
	assert.Contains(t, output, "789")
	assert.Contains(t, output, "mr_title")
	assert.Contains(t, output, "Test MR")
	assert.Contains(t, output, "mr_state")
	assert.Contains(t, output, "opened")
}

func TestDebugLogger_LogIssueEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	event := &Event{
		ObjectKind: "issue",
		Type:       "issue",
		User: &User{
			ID:    123,
			Name:  "test-user",
			Email: "test@example.com",
		},
		Project: &Project{
			ID:   456,
			Name: "test-project",
		},
		Issue: &Issue{
			ID:    789,
			Title: "Test Issue",
			State: "opened",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:     789,
			Title:  "Test Issue",
			State:  "opened",
			Action: "open",
		},
	}

	debugLogger.LogWebhookEvent(event)

	output := buf.String()
	assert.Contains(t, output, "Issue Event Details")
	assert.Contains(t, output, "issue_id")
	assert.Contains(t, output, "789")
	assert.Contains(t, output, "issue_title")
	assert.Contains(t, output, "Test Issue")
	assert.Contains(t, output, "issue_state")
	assert.Contains(t, output, "opened")
}

func TestDebugLogger_TruncateString(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	// Test short string
	result := debugLogger.truncateString("short", 10)
	assert.Equal(t, "short", result)

	// Test long string
	longString := "this is a very long string that should be truncated"
	result = debugLogger.truncateString(longString, 20)
	assert.Equal(t, "this is a very long...", result)
	assert.Len(t, result, 22) // 19 + 3 for "..." (avoided space at position 20)

	// Test exact length
	result = debugLogger.truncateString("exactly ten", 10)
	assert.Equal(t, "exactly te...", result)
}

func TestDebugLogger_MaskToken(t *testing.T) {

	// Test short token
	result := maskToken("short")
	assert.Equal(t, "***", result)

	// Test long token
	result = maskToken("very-long-token-12345")
	assert.Equal(t, "very...2345", result)

	// Test medium token
	result = maskToken("medium123")
	assert.Equal(t, "medi...m123", result)
}

func TestDebugLogger_LogWebhookRequest_JSONPrettyPrint(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	// Create JSON data
	jsonData := map[string]interface{}{
		"object_kind": "push",
		"user": map[string]interface{}{
			"id":   123,
			"name": "test-user",
		},
		"project": map[string]interface{}{
			"id":   456,
			"name": "test-project",
		},
	}

	body, err := json.Marshal(jsonData)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/webhook", strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	debugLogger.LogWebhookRequest(req, body, nil)

	output := buf.String()
	assert.Contains(t, output, "Request Body (formatted)")
	assert.Contains(t, output, "object_kind")
	assert.Contains(t, output, "push")
	assert.Contains(t, output, "user")
	assert.Contains(t, output, "test-user")
}

func TestDebugLogger_LogWebhookRequest_NonJSONContent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	debugLogger := NewDebugLogger(logger)

	req, err := http.NewRequest("POST", "/webhook", strings.NewReader("plain text"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	debugLogger.LogWebhookRequest(req, []byte("plain text"), nil)

	output := buf.String()
	assert.Contains(t, output, "Request Body (raw)")
	assert.Contains(t, output, "plain text")
	// Should not contain formatted JSON
	assert.NotContains(t, output, "Request Body (formatted)")
}

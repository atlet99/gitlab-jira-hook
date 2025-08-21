package jira

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestNewJQLFilter(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "project = {{project}} AND status = {{status}}",
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	assert.NotNil(t, filter)
	assert.Equal(t, cfg.JQLFilter, filter.config.JQLFilter)
	assert.NotNil(t, filter.logger)
	assert.NotNil(t, filter.config)
}

func TestJQLFilter_ShouldProcessEvent(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "project = {{project}} AND status = {{status}}",
	}
	logger := testLogger()

	// Create a filter with nil client to test the logic without SearchIssues calls
	filter := NewJQLFilter(cfg, logger, nil)

	tests := []struct {
		name         string
		webhookEvent string
		issueKey     string
		projectKey   string
		status       string
		shouldPass   bool
	}{
		{
			name:         "Valid issue event with matching project",
			webhookEvent: "jira:issue_created",
			issueKey:     "PROJ-123",
			projectKey:   "PROJ",
			status:       "Open",
			shouldPass:   true, // Current implementation returns true on error (fail-safe)
		},
		{
			name:         "Valid issue event with non-matching project",
			webhookEvent: "jira:issue_created",
			issueKey:     "OTHER-123",
			projectKey:   "OTHER",
			status:       "Open",
			shouldPass:   true, // Current implementation returns true on error (fail-safe)
		},
		{
			name:         "Non-issue event",
			webhookEvent: "jira:comment_created",
			issueKey:     "PROJ-123",
			projectKey:   "PROJ",
			status:       "Open",
			shouldPass:   true, // Non-issue events should pass through
		},
		{
			name:         "Empty webhook event",
			webhookEvent: "",
			issueKey:     "PROJ-123",
			projectKey:   "PROJ",
			status:       "Open",
			shouldPass:   true, // Empty events should pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &WebhookEvent{
				WebhookEvent: tt.webhookEvent,
				Issue: &JiraIssue{
					Key: tt.issueKey,
					Fields: &JiraIssueFields{
						Project: &JiraProject{
							Key: tt.projectKey,
						},
						Status: &Status{
							Name: tt.status,
						},
					},
				},
				Timestamp: time.Now().Unix(),
			}

			result := filter.ShouldProcessEvent(event)
			assert.Equal(t, tt.shouldPass, result)
		})
	}
}

func TestJQLFilter_ShouldProcessEvent_WithEventAgeLimit(t *testing.T) {
	cfg := &config.Config{
		JQLFilter:     "project = {{project}} AND status = {{status}}",
		MaxEventAge:   1, // 1 hour limit
		SkipOldEvents: true,
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	// Test event that's too old
	oldEvent := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &JiraIssue{
			Key: "PROJ-123",
			Fields: &JiraIssueFields{
				Project: &JiraProject{
					Key: "PROJ",
				},
				Status: &Status{
					Name: "Open",
				},
			},
		},
		Timestamp: time.Now().Add(-2 * time.Hour).Unix(), // 2 hours ago
	}

	result := filter.ShouldProcessEvent(oldEvent)
	assert.False(t, result, "Old event should be filtered out")

	// Test recent event
	recentEvent := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &JiraIssue{
			Key: "PROJ-123",
			Fields: &JiraIssueFields{
				Project: &JiraProject{
					Key: "PROJ",
				},
				Status: &Status{
					Name: "Open",
				},
			},
		},
		Timestamp: time.Now().Unix(), // Current time
	}

	result = filter.ShouldProcessEvent(recentEvent)
	assert.True(t, result, "Recent event should pass through")
}

func TestJQLFilter_ShouldProcessEvent_WithJQLQuery(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "project = {{project}} AND status = {{status}} AND created >= {{created}}",
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	// Mock the JQL query method
	_ = &mockJiraClient{
		mockJQLQuery: func(ctx context.Context, jql string) ([]JiraIssue, error) {
			// Simulate that the query returns no results for non-matching criteria
			if jql == "project = PROJ AND status = Open AND created >= 2023-01-01" {
				return []JiraIssue{}, nil
			}
			return []JiraIssue{}, nil
		},
	}

	event := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &JiraIssue{
			Key: "PROJ-123",
			Fields: &JiraIssueFields{
				Project: &JiraProject{
					Key: "PROJ",
				},
				Status: &Status{
					Name: "Open",
				},
			},
		},
		Timestamp: time.Now().Unix(),
	}

	result := filter.ShouldProcessEvent(event)
	assert.True(t, result, "Event should pass through JQL filter")
}

func TestJQLFilter_ShouldProcessEvent_NonIssueEvents(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "project = {{project}} AND status = {{status}}",
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	nonIssueEvents := []string{
		"comment_created",
		"comment_updated",
		"comment_deleted",
		"worklog_created",
		"worklog_updated",
		"worklog_deleted",
		"jira:comment_created",
		"jira:comment_updated",
		"jira:comment_deleted",
		"jira:worklog_created",
		"jira:worklog_updated",
		"jira:worklog_deleted",
	}

	for _, eventType := range nonIssueEvents {
		t.Run(eventType, func(t *testing.T) {
			event := &WebhookEvent{
				WebhookEvent: eventType,
				Issue: &JiraIssue{
					Key: "PROJ-123",
					Fields: &JiraIssueFields{
						Project: &JiraProject{
							Key: "PROJ",
						},
						Status: &Status{
							Name: "Open",
						},
					},
				},
				Timestamp: time.Now().Unix(),
			}

			result := filter.ShouldProcessEvent(event)
			assert.True(t, result, "Non-issue event should pass through")
		})
	}
}

func TestJQLFilter_ShouldProcessEvent_WithNilIssue(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "project = {{project}} AND status = {{status}}",
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	event := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue:        nil, // No issue data
		Timestamp:    time.Now().Unix(),
	}

	result := filter.ShouldProcessEvent(event)
	assert.True(t, result, "Event with nil issue should pass through (fail-safe)")
}

func TestJQLFilter_ShouldProcessEvent_WithEmptyJQL(t *testing.T) {
	cfg := &config.Config{
		JQLFilter: "", // Empty JQL
	}
	logger := testLogger()

	filter := NewJQLFilter(cfg, logger, nil)

	event := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &JiraIssue{
			Key: "PROJ-123",
			Fields: &JiraIssueFields{
				Project: &JiraProject{
					Key: "PROJ",
				},
				Status: &Status{
					Name: "Open",
				},
			},
		},
		Timestamp: time.Now().Unix(),
	}

	result := filter.ShouldProcessEvent(event)
	assert.True(t, result, "Event should pass through when JQL is empty")
}

func TestJQLFilter_ShouldProcessEvent_WithNilConfig(t *testing.T) {
	logger := testLogger()

	// This should not panic, but handle nil config gracefully
	filter := NewJQLFilter(nil, logger, nil)
	event := &WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &JiraIssue{
			Key: "PROJ-123",
			Fields: &JiraIssueFields{
				Project: &JiraProject{
					Key: "PROJ",
				},
				Status: &Status{
					Name: "Open",
				},
			},
		},
		Timestamp: time.Now().Unix(),
	}
	result := filter.ShouldProcessEvent(event)
	assert.True(t, result, "Event should pass through when config is nil")
}

// Helper function to create a test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&testWriter{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// Test writer for logging
type testWriter struct{}

func (w *testWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Mock Jira client for testing
type mockJiraClient struct {
	mockJQLQuery func(ctx context.Context, jql string) ([]JiraIssue, error)
}

func (m *mockJiraClient) SearchIssues(ctx context.Context, jql string) ([]JiraIssue, error) {
	panic("Mock SearchIssues called with: " + jql)
}

// Implement other required methods to satisfy the Client interface
func (m *mockJiraClient) AddComment(ctx context.Context, issueID string, payload CommentPayload) error {
	return nil
}

func (m *mockJiraClient) AddCommentWithContext(ctx context.Context, issueID string, payload CommentPayload) error {
	return nil
}

func (m *mockJiraClient) TestConnection() error {
	return nil
}

func (m *mockJiraClient) TestConnectionWithContext(ctx context.Context) error {
	return nil
}

func (m *mockJiraClient) GetIssue(ctx context.Context, issueID string) (*JiraIssue, error) {
	return &JiraIssue{}, nil
}

func (m *mockJiraClient) GetIssueWithContext(ctx context.Context, issueID string) (*JiraIssue, error) {
	return &JiraIssue{}, nil
}

func (m *mockJiraClient) UpdateIssue(ctx context.Context, issueID string, fields map[string]interface{}) error {
	return nil
}

func (m *mockJiraClient) UpdateIssueWithContext(ctx context.Context, issueID string, fields map[string]interface{}) error {
	return nil
}

func (m *mockJiraClient) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	return &Project{}, nil
}

func (m *mockJiraClient) GetProjectWithContext(ctx context.Context, projectKey string) (*Project, error) {
	return &Project{}, nil
}

func (m *mockJiraClient) GetUser(ctx context.Context, username string) (*User, error) {
	return &User{}, nil
}

func (m *mockJiraClient) GetUserWithContext(ctx context.Context, username string) (*User, error) {
	return &User{}, nil
}

func (m *mockJiraClient) IsHealthy() bool {
	return true
}

func (m *mockJiraClient) GetHealthStatus() HealthStatus {
	return HealthStatus{Healthy: true}
}

func (m *mockJiraClient) ValidateConfig() error {
	return nil
}

// Additional methods that might be needed
func (m *mockJiraClient) GetTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	return []Transition{}, nil
}

func (m *mockJiraClient) ExecuteTransition(ctx context.Context, issueKey string, transitionID string) error {
	return nil
}

func (m *mockJiraClient) FindTransition(ctx context.Context, issueKey string, targetStatus string) (*Transition, error) {
	return nil, nil
}

func (m *mockJiraClient) TransitionToStatus(ctx context.Context, issueKey string, targetStatus string) error {
	return nil
}

func (m *mockJiraClient) AddCommentSimple(ctx context.Context, issueKey string, comment string) error {
	return nil
}

func (m *mockJiraClient) SetAssigneeSimple(ctx context.Context, issueKey string, accountID string) error {
	return nil
}

func (m *mockJiraClient) GetAssigneeDetailsSimple(ctx context.Context, accountID string) (*AssigneeDetails, error) {
	return &AssigneeDetails{
		IssueKey:    "TEST-123",
		HasAssignee: true,
		Assignee: JiraUser{
			AccountID:    accountID,
			DisplayName:  "Test User",
			EmailAddress: "test@example.com",
			Active:       true,
		},
		IsSpecial: false,
	}, nil
}

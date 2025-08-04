package gitlab

import (
	"context"
	"io"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// EventProcessorMockJiraClient implements the jira client interface for testing
type EventProcessorMockJiraClient struct {
	commentsAdded map[string]int
	connectionOk  bool
}

func (m *EventProcessorMockJiraClient) AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error {
	if m.commentsAdded == nil {
		m.commentsAdded = make(map[string]int)
	}
	m.commentsAdded[issueID]++
	return nil
}

func (m *EventProcessorMockJiraClient) TestConnection(ctx context.Context) error {
	if m.connectionOk {
		return nil
	}
	return assert.AnError
}

func TestNewEventProcessor(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	mockConfig := &config.Config{Timezone: "Etc/GMT-5"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, mockConfig, logger)

	assert.NotNil(t, processor)
	assert.Equal(t, mockJira, processor.jiraClient)
	assert.Equal(t, urlBuilder, processor.urlBuilder)
	assert.Equal(t, logger, processor.logger)
	assert.NotNil(t, processor.parser)
}

func TestEventProcessor_ProcessEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	tests := []struct {
		name        string
		event       *Event
		expectError bool
	}{
		{
			name: "push event",
			event: &Event{
				ObjectKind: "push",
				Project: &Project{
					ID:   1,
					Name: "test-project",
				},
				Commits: []Commit{
					{
						ID:      "abc123",
						Message: "Fix ABC-123",
						Author: Author{
							Name:  "Test User",
							Email: "test@example.com",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge_request event",
			event: &Event{
				ObjectKind: "merge_request",
				Project: &Project{
					ID:   1,
					Name: "test-project",
				},
				ObjectAttributes: &ObjectAttributes{
					ID:          123,
					Title:       "Test MR for ABC-123",
					Description: "This MR addresses ABC-123",
					State:       "opened",
					Action:      "open",
					URL:         "https://gitlab.com/test/project/merge_requests/123",
				},
			},
			expectError: false,
		},
		{
			name: "unsupported event type",
			event: &Event{
				ObjectKind: "unsupported",
				Project: &Project{
					ID:   1,
					Name: "test-project",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ProcessEvent(context.Background(), tt.event)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEventProcessor_ProcessPushEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "push",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		Commits: []Commit{
			{
				ID:      "abc123",
				Message: "Fix ABC-123 issue",
				Author: Author{
					Name:  "Test User",
					Email: "test@example.com",
				},
				URL: "https://gitlab.com/test/project/commit/abc123",
			},
		},
	}

	err := processor.processPushEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123
	assert.Equal(t, 1, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessMergeRequestEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "merge_request",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          123,
			Title:       "Test MR for ABC-123",
			Description: "This MR addresses ABC-123",
			State:       "opened",
			Action:      "open",
			URL:         "https://gitlab.com/test/project/merge_requests/123",
		},
	}

	err := processor.processMergeRequestEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123 (should be 2 because title and description both contain ABC-123)
	assert.Equal(t, 2, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessIssueEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "issue",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          456,
			Title:       "Bug related to ABC-123",
			Description: "This issue is related to ABC-123",
			State:       "opened",
			Action:      "open",
			URL:         "https://gitlab.com/test/project/issues/456",
		},
	}

	err := processor.processIssueEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123 (should be 2 because title and description both contain ABC-123)
	assert.Equal(t, 2, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessNoteEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "note",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:   101,
			Note: "This comment addresses ABC-123",
			URL:  "https://gitlab.com/test/project/notes/101",
		},
	}

	err := processor.processNoteEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123
	assert.Equal(t, 1, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessPipelineEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "pipeline",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:       789,
			Ref:      "main",
			Status:   "success",
			Duration: 120,
			URL:      "https://gitlab.com/test/project/pipelines/789",
		},
	}

	err := processor.processPipelineEvent(context.Background(), event)
	assert.NoError(t, err)
}

func TestEventProcessor_ProcessJobEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "job",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		Build: &Build{
			ID:       999,
			Name:     "test-job",
			Stage:    "test",
			Status:   "success",
			Duration: 60,
			URL:      "https://gitlab.com/test/project/jobs/999",
		},
	}

	err := processor.processJobEvent(context.Background(), event)
	assert.NoError(t, err)
}

func TestEventProcessor_ProcessDeploymentEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "deployment",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          555,
			Name:        "production",
			Environment: "production",
			Status:      "success",
			URL:         "https://gitlab.com/test/project/deployments/555",
		},
	}

	err := processor.processDeploymentEvent(context.Background(), event)
	assert.NoError(t, err)
}

func TestEventProcessor_ProcessReleaseEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "release",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		Release: &Release{
			ID:          777,
			Name:        "Release v1.0.0",
			Description: "Release for ABC-123",
			URL:         "https://gitlab.com/test/project/releases/777",
		},
	}

	err := processor.processReleaseEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123
	assert.Equal(t, 1, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessWikiPageEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "wiki_page",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		WikiPage: &WikiPage{
			Title:   "Wiki page for ABC-123",
			Content: "Content related to ABC-123",
			URL:     "https://gitlab.com/test/project/wikis/888",
		},
	}

	err := processor.processWikiPageEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123 (should be 2 because title and content both contain ABC-123)
	assert.Equal(t, 2, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_ProcessFeatureFlagEvent(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	event := &Event{
		ObjectKind: "feature_flag",
		Project: &Project{
			ID:     1,
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		FeatureFlag: &FeatureFlag{
			ID:          666,
			Name:        "feature-ABC-123",
			Description: "Feature flag for ABC-123",
		},
	}

	err := processor.processFeatureFlagEvent(context.Background(), event)
	assert.NoError(t, err)

	// Check that comment was added for ABC-123 (should be 2 because name and description both contain ABC-123)
	assert.Equal(t, 2, mockJira.commentsAdded["ABC-123"])
}

func TestEventProcessor_BuildSimpleComment(t *testing.T) {
	mockJira := &EventProcessorMockJiraClient{connectionOk: true}
	urlBuilder := &URLBuilder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processor := NewEventProcessor(mockJira, urlBuilder, &config.Config{Timezone: "Etc/GMT-5"}, logger)

	comment := processor.buildSimpleComment("Test Event", "Test Title", "create")

	assert.NotNil(t, comment)
	assert.Equal(t, "doc", comment.Body.Type)
	assert.Equal(t, 1, comment.Body.Version)
	assert.Len(t, comment.Body.Content, 1)
	assert.Equal(t, "paragraph", comment.Body.Content[0].Type)
	assert.Len(t, comment.Body.Content[0].Content, 1)
	assert.Equal(t, "Test Event create: Test Title", comment.Body.Content[0].Content[0].Text)
}

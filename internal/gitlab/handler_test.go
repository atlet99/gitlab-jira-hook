package gitlab

import (
	"context"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// testJiraClient is a mock implementation of Jira client for testing
type testJiraClient struct{}

func (m *testJiraClient) AddComment(ctx context.Context, issueID string, comment jira.CommentPayload) error {
	return nil
}

func (m *testJiraClient) TestConnection(ctx context.Context) error {
	return nil
}

func TestProcessRepositoryUpdateEvent(t *testing.T) {
	handler := &Handler{
		config: &config.Config{
			GitLabBaseURL: "https://gitlab.example.com",
			Timezone:      "UTC",
		},
		parser: NewParser(),
		jira:   &testJiraClient{},
		logger: slog.Default(),
	}

	event := &Event{
		Type: "repository_update",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.example.com/test/test-project",
		},
		UserName: "John Doe",
		Changes: []Change{
			{
				Before: "abc123456789",
				After:  "def987654321",
				Ref:    "refs/heads/main",
			},
		},
	}

	err := handler.processRepositoryUpdateEvent(context.Background(), event)
	assert.NoError(t, err)
}

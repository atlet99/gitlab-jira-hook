package gitlab

import (
	"log/slog"
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

type mockJiraClient struct {
	comments map[string][]string
}

func newMockJiraClient() *mockJiraClient {
	return &mockJiraClient{comments: make(map[string][]string)}
}

func (m *mockJiraClient) AddComment(issueID string, payload jira.CommentPayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m.comments[issueID] = append(m.comments[issueID], string(b))
	return nil
}

func (m *mockJiraClient) TestConnection() error { return nil }

func (m *mockJiraClient) GetComments(issueID string) []string {
	return m.comments[issueID]
}

func TestProcessTagPushEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Ref:    "ABC-123-v1.0.0",
			URL:    "https://gitlab.example.com/tag/ABC-123-v1.0.0",
			Action: "create",
			Name:   "v1.0.0",
		},
	}
	err := h.processTagPushEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("ABC-123")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "tag push")
}

func TestProcessReleaseEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Name:        "ABC-456 Release",
			Description: "Release for ABC-456",
			URL:         "https://gitlab.example.com/release/ABC-456",
		},
	}
	err := h.processReleaseEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("ABC-456")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "release")
}

func TestProcessDeploymentEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Ref:         "feature/DEF-789",
			URL:         "https://gitlab.example.com/deployment/DEF-789",
			Environment: "production",
			Status:      "success",
			Sha:         "abc123ef456",
			Name:        "John Doe",
		},
	}
	err := h.processDeploymentEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("DEF-789")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "deployment")
}

func TestProcessFeatureFlagEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Name:        "GHI-321 Feature Flag",
			Description: "Feature flag for GHI-321",
			URL:         "https://gitlab.example.com/feature-flag/GHI-321",
			Action:      "create",
		},
	}
	err := h.processFeatureFlagEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("GHI-321")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "feature flag")
}

func TestProcessWikiPageEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Title:   "JKL-654 Documentation",
			Content: "Documentation for JKL-654 issue",
			URL:     "https://gitlab.example.com/wiki/JKL-654",
			Action:  "create",
			Name:    "Jane Smith",
		},
	}
	err := h.processWikiPageEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("JKL-654")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "wiki page")
}

func TestProcessPipelineEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Ref:      "feature/MNO-987",
			Title:    "Pipeline for MNO-987",
			URL:      "https://gitlab.example.com/pipeline/MNO-987",
			Status:   "success",
			Sha:      "def456ghi789",
			Duration: 120,
			Name:     "Bob Wilson",
		},
	}
	err := h.processPipelineEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("MNO-987")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "pipeline")
}

func TestProcessBuildEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Name:     "PQR-654-build",
			Stage:    "test",
			URL:      "https://gitlab.example.com/build/PQR-654",
			Status:   "success",
			Ref:      "feature/PQR-654",
			Sha:      "ghi789jkl012",
			Duration: 60,
		},
	}
	err := h.processBuildEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("PQR-654")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "build")
}

func TestProcessNoteEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Note:   "Please check STU-321 ASAP!",
			URL:    "https://gitlab.example.com/note/STU-321",
			Action: "comment",
			Title:  "STU-321",
			Name:   "John Doe",
		},
	}
	err := h.processNoteEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("STU-321")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "comment")
}

func TestProcessIssueEvent_AddsComment(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		ObjectAttributes: &ObjectAttributes{
			Title:       "VWX-987: Issue title",
			Description: "This is a description for VWX-987",
			URL:         "https://gitlab.example.com/issue/VWX-987",
			Action:      "open",
			Name:        "Jane Smith",
		},
	}
	err := h.processIssueEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("VWX-987")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "issue")
}

// Tests for checking no comments are added when there is no issueID
// without issueID
// Check that no comments were added
// Test for handling events without ObjectAttributes
// Test for handling multiple issueIDs in one event
// Check that comments were added for both issues

func TestProcessMergeRequestEvent_WithParticipants(t *testing.T) {
	jira := newMockJiraClient()
	h := &Handler{
		config: &config.Config{},
		logger: slog.Default(),
		jira:   jira,
		parser: NewParser(),
	}
	event := &Event{
		EventName: "merge_request",
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "Implement feature ABC-123",
			Description:  "This MR implements ABC-123 and involves multiple participants",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/ABC-123",
			TargetBranch: "main",
			State:        "opened",
			Name:         "John Doe",
		},
		MergeRequest: &MergeRequest{
			ID:    123,
			Title: "Implement feature ABC-123",
			Author: &User{
				ID:   1,
				Name: "John Doe",
			},
			Assignee: &User{
				ID:   2,
				Name: "Jane Smith",
			},
			Participants: []User{
				{ID: 1, Name: "John Doe"},
				{ID: 2, Name: "Jane Smith"},
				{ID: 3, Name: "Bob Johnson"},
				{ID: 4, Name: "Alice Brown"},
			},
		},
	}

	err := h.processMergeRequestEvent(event)
	require.NoError(t, err)
	comments := jira.GetComments("ABC-123")
	require.NotEmpty(t, comments)
	require.Contains(t, comments[0], "merge request")

	// Check that all participants are included in the comment
	expectedParticipants := []string{"John Doe", "Jane Smith", "Bob Johnson", "Alice Brown"}
	for _, participant := range expectedParticipants {
		require.Contains(t, comments[0], participant, "Expected participant %s in comment", participant)
	}

	// Check that participants field is present
	require.Contains(t, comments[0], "participants:", "Expected participants field in comment")
}

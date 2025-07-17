package gitlab

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestProcessMergeRequestEvent_SystemHook_LinksToMultipleIssues(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &Handler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "Fix ABC-123 and implement ABC-456 feature",
			Description:  "This MR addresses issues ABC-123 and ABC-456",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/ABC-789-cool-feature",
			TargetBranch: "release/ABC-101",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// Expect comments to be added to all found issues: ABC-123, ABC-456, ABC-789, ABC-101
	mockJira.On("AddComment", "ABC-123", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-456", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-789", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-101", mock.Anything).Return(nil)

	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProcessMergeRequestEvent_SystemHook_NoIssuesFound(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &Handler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "General improvements",
			Description:  "No specific issues mentioned",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/general-improvements",
			TargetBranch: "main",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// No comments should be added since no issue keys are found
	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertNotCalled(t, "AddComment")
}

func TestProcessMergeRequestEvent_SystemHook_DuplicateIssues(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &Handler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "Fix ABC-123",
			Description:  "Addressing ABC-123 issue",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/ABC-123-branch",
			TargetBranch: "main",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// ABC-123 appears in title, description, and source branch, but should only get one comment
	mockJira.On("AddComment", "ABC-123", mock.Anything).Return(nil)

	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
	// Verify AddComment was called exactly once for ABC-123
	mockJira.AssertNumberOfCalls(t, "AddComment", 1)
}

func TestProcessMergeRequestEvent_ProjectHook_LinksToMultipleIssues(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "Fix ABC-123 and implement ABC-456 feature",
			Description:  "This MR addresses issues ABC-123 and ABC-456",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/ABC-789-cool-feature",
			TargetBranch: "release/ABC-101",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// Expect comments to be added to all found issues: ABC-123, ABC-456, ABC-789, ABC-101
	mockJira.On("AddComment", "ABC-123", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-456", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-789", mock.Anything).Return(nil)
	mockJira.On("AddComment", "ABC-101", mock.Anything).Return(nil)

	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProcessMergeRequestEvent_ProjectHook_NoIssuesFound(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "General improvements",
			Description:  "No specific issues mentioned",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/general-improvements",
			TargetBranch: "main",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// No comments should be added since no issue keys are found
	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertNotCalled(t, "AddComment")
}

func TestProcessMergeRequestEvent_ProjectHook_DuplicateIssues(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "merge_request",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:           123,
			Title:        "Fix ABC-123",
			Description:  "Addressing ABC-123 issue",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/ABC-123-branch",
			TargetBranch: "main",
			State:        "opened",
			Name:         "Test User",
		},
	}

	// ABC-123 appears in title, description, and source branch, but should only get one comment
	mockJira.On("AddComment", "ABC-123", mock.Anything).Return(nil)

	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
	// Verify AddComment was called exactly once for ABC-123
	mockJira.AssertNumberOfCalls(t, "AddComment", 1)
}

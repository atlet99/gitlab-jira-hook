package sync

import (
	"context"
	"testing"

	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// mockGitLabAPI is a mock implementation of GitLabAPI for testing
type mockGitLabAPI struct {
	createdIssues []GitLabIssueRequest
	comments      []string
	searchResults []GitLabIssue
	users         map[string]*GitLabUser
}

func newMockGitLabAPI() *mockGitLabAPI {
	return &mockGitLabAPI{
		createdIssues: make([]GitLabIssueRequest, 0),
		comments:      make([]string, 0),
		searchResults: make([]GitLabIssue, 0),
		users:         make(map[string]*GitLabUser),
	}
}

func (m *mockGitLabAPI) CreateIssue(ctx context.Context, project string, issue GitLabIssueRequest) (*GitLabIssue, error) {
	m.createdIssues = append(m.createdIssues, issue)
	return &GitLabIssue{
		ID:    len(m.createdIssues),
		IID:   len(m.createdIssues),
		Title: issue.Title,
		State: "opened",
	}, nil
}

func (m *mockGitLabAPI) UpdateIssue(ctx context.Context, project string, issueID int, issue GitLabIssueRequest) (*GitLabIssue, error) {
	return &GitLabIssue{
		ID:    issueID,
		IID:   issueID,
		Title: issue.Title,
		State: "opened",
	}, nil
}

func (m *mockGitLabAPI) GetIssue(ctx context.Context, project string, issueID int) (*GitLabIssue, error) {
	return &GitLabIssue{
		ID:    issueID,
		IID:   issueID,
		Title: "Test Issue",
		State: "opened",
	}, nil
}

func (m *mockGitLabAPI) CreateComment(ctx context.Context, project string, issueID int, body string) error {
	m.comments = append(m.comments, body)
	return nil
}

func (m *mockGitLabAPI) SearchIssuesByTitle(ctx context.Context, project, title string) ([]GitLabIssue, error) {
	return m.searchResults, nil
}

func (m *mockGitLabAPI) FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
	if user, exists := m.users[email]; exists {
		return user, nil
	}
	return nil, nil
}

func (m *mockGitLabAPI) AddLabel(ctx context.Context, project string, issueID int, label string) error {
	return nil
}

func (m *mockGitLabAPI) SetAssignee(ctx context.Context, project string, issueID int, assigneeID int) error {
	return nil
}

// mockJiraAPI is a mock implementation of JiraAPI for testing
type mockJiraAPI struct {
	updatedIssues map[string]map[string]interface{}
	issues        map[string]*jira.JiraIssue
}

func newMockJiraAPI() *mockJiraAPI {
	return &mockJiraAPI{
		updatedIssues: make(map[string]map[string]interface{}),
		issues:        make(map[string]*jira.JiraIssue),
	}
}

func (m *mockJiraAPI) UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	m.updatedIssues[issueKey] = fields
	return nil
}

func (m *mockJiraAPI) GetIssue(ctx context.Context, issueKey string) (*jira.JiraIssue, error) {
	if issue, exists := m.issues[issueKey]; exists {
		return issue, nil
	}

	// Return a default issue if not found
	return &jira.JiraIssue{
		Key: issueKey,
		Fields: &jira.JiraIssueFields{
			Summary: "Mock Issue",
			Updated: "2023-01-01T00:00:00.000+0000",
		},
	}, nil
}

func (m *mockJiraAPI) SetAssignee(ctx context.Context, issueKey string, accountID string) error {
	return nil
}

func TestSyncManager_SyncJiraToGitLab_Disabled(t *testing.T) {
	cfg := &config.Config{
		BidirectionalEnabled: false,
	}

	logger := slog.Default()
	mockGitLabAPI := newMockGitLabAPI()
	mockJiraAPI := newMockJiraAPI()
	syncManager := NewManager(cfg, mockGitLabAPI, mockJiraAPI, logger)

	event := &jira.WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &jira.JiraIssue{
			Key: "TEST-123",
			Fields: &jira.JiraIssueFields{
				Summary: "Test Issue",
				Project: &jira.JiraProject{
					Key: "TEST",
				},
			},
		},
	}

	err := syncManager.SyncJiraToGitLab(context.Background(), event)

	if err != nil {
		t.Errorf("Expected no error when bidirectional sync is disabled, got: %v", err)
	}

	if len(mockGitLabAPI.createdIssues) != 0 {
		t.Errorf("Expected no issues to be created when sync is disabled, got: %d", len(mockGitLabAPI.createdIssues))
	}
}

func TestSyncManager_SyncJiraToGitLab_IssueCreated(t *testing.T) {
	cfg := &config.Config{
		BidirectionalEnabled: true,
		GitLabNamespace:      "test",
		BidirectionalEvents:  []string{"issue_created"},
		MaxEventAge:          24,
		SkipOldEvents:        false,
	}

	logger := slog.Default()
	mockGitLabAPI := newMockGitLabAPI()
	mockJiraAPI := newMockJiraAPI()
	syncManager := NewManager(cfg, mockGitLabAPI, mockJiraAPI, logger)

	event := &jira.WebhookEvent{
		WebhookEvent: "jira:issue_created",
		Issue: &jira.JiraIssue{
			Key: "TEST-123",
			Fields: &jira.JiraIssueFields{
				Summary:     "Test Issue Summary",
				Description: "Test Description",
				Project: &jira.JiraProject{
					Key: "TEST",
				},
				IssueType: &jira.IssueType{
					Name: "Bug",
				},
				Priority: &jira.Priority{
					Name: "High",
				},
				Status: &jira.Status{
					Name: "To Do",
				},
			},
		},
	}

	err := syncManager.SyncJiraToGitLab(context.Background(), event)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockGitLabAPI.createdIssues) != 1 {
		t.Errorf("Expected 1 issue to be created, got: %d", len(mockGitLabAPI.createdIssues))
		return
	}

	createdIssue := mockGitLabAPI.createdIssues[0]
	expectedTitle := "[TEST-123] Test Issue Summary"
	if createdIssue.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, createdIssue.Title)
	}

	expectedLabels := []string{"jira-sync", "type:bug", "priority:high"}
	if len(createdIssue.Labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(createdIssue.Labels))
	}
}

func TestSyncManager_SyncJiraToGitLab_CommentCreated(t *testing.T) {
	cfg := &config.Config{
		BidirectionalEnabled:        true,
		GitLabNamespace:             "test",
		BidirectionalEvents:         []string{"comment_created"},
		CommentSyncEnabled:          true,
		CommentSyncDirection:        "jira_to_gitlab",
		CommentTemplateJiraToGitLab: "**From Jira**: {content}",
		MaxEventAge:                 24,
		SkipOldEvents:               false,
	}

	logger := slog.Default()
	mockGitLabAPI := newMockGitLabAPI()
	mockJiraAPI := newMockJiraAPI()
	// Add existing issue to search results
	mockGitLabAPI.searchResults = []GitLabIssue{
		{
			ID:    1,
			IID:   1,
			Title: "[TEST-123] Test Issue",
			State: "opened",
		},
	}

	syncManager := NewManager(cfg, mockGitLabAPI, mockJiraAPI, logger)

	event := &jira.WebhookEvent{
		WebhookEvent: "comment_created",
		Issue: &jira.JiraIssue{
			Key: "TEST-123",
			Fields: &jira.JiraIssueFields{
				Summary: "Test Issue",
				Project: &jira.JiraProject{
					Key: "TEST",
				},
			},
		},
		Comment: &jira.Comment{
			ID:   "comment-123",
			Body: "This is a test comment",
		},
	}

	err := syncManager.SyncJiraToGitLab(context.Background(), event)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockGitLabAPI.comments) != 1 {
		t.Errorf("Expected 1 comment to be created, got: %d", len(mockGitLabAPI.comments))
		return
	}

	expectedComment := "**From Jira**: This is a test comment"
	if mockGitLabAPI.comments[0] != expectedComment {
		t.Errorf("Expected comment '%s', got '%s'", expectedComment, mockGitLabAPI.comments[0])
	}
}

func TestSyncManager_determineGitLabProject(t *testing.T) {
	tests := []struct {
		name            string
		jiraIssueKey    string
		projectMappings map[string]string
		gitlabNamespace string
		expectedProject string
		expectError     bool
	}{
		{
			name:            "Project mapping exists",
			jiraIssueKey:    "PROJ-123",
			projectMappings: map[string]string{"PROJ": "custom/project"},
			gitlabNamespace: "default",
			expectedProject: "custom/project",
			expectError:     false,
		},
		{
			name:            "Use namespace fallback",
			jiraIssueKey:    "TEST-456",
			projectMappings: map[string]string{},
			gitlabNamespace: "myorg",
			expectedProject: "myorg/test",
			expectError:     false,
		},
		{
			name:            "No mapping and no namespace",
			jiraIssueKey:    "NOMAPPING-789",
			projectMappings: map[string]string{},
			gitlabNamespace: "",
			expectedProject: "",
			expectError:     true,
		},
		{
			name:            "Invalid issue key format",
			jiraIssueKey:    "INVALIDKEY",
			projectMappings: map[string]string{},
			gitlabNamespace: "test",
			expectedProject: "",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ProjectMappings: tt.projectMappings,
				GitLabNamespace: tt.gitlabNamespace,
			}

			logger := slog.Default()
			mockGitLabAPI := newMockGitLabAPI()
			mockJiraAPI := newMockJiraAPI()
			syncManager := NewManager(cfg, mockGitLabAPI, mockJiraAPI, logger)

			project, err := syncManager.determineGitLabProject(tt.jiraIssueKey)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if project != tt.expectedProject {
					t.Errorf("Expected project '%s', got '%s'", tt.expectedProject, project)
				}
			}
		})
	}
}

func TestSyncManager_shouldProcessEvent(t *testing.T) {
	tests := []struct {
		name             string
		eventType        string
		configuredEvents []string
		shouldProcess    bool
	}{
		{
			name:             "No events configured - process all",
			eventType:        "jira:issue_created",
			configuredEvents: []string{},
			shouldProcess:    true,
		},
		{
			name:             "Event in configured list",
			eventType:        "jira:issue_created",
			configuredEvents: []string{"issue_created", "comment_created"},
			shouldProcess:    true,
		},
		{
			name:             "Event not in configured list",
			eventType:        "jira:issue_deleted",
			configuredEvents: []string{"issue_created", "comment_created"},
			shouldProcess:    false,
		},
		{
			name:             "Normalized event type",
			eventType:        "issue_updated",
			configuredEvents: []string{"issue_updated"},
			shouldProcess:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				BidirectionalEvents: tt.configuredEvents,
			}

			logger := slog.Default()
			mockGitLabAPI := newMockGitLabAPI()
			mockJiraAPI := newMockJiraAPI()
			syncManager := NewManager(cfg, mockGitLabAPI, mockJiraAPI, logger)

			result := syncManager.shouldProcessEvent(tt.eventType)

			if result != tt.shouldProcess {
				t.Errorf("Expected shouldProcessEvent to return %v, got %v", tt.shouldProcess, result)
			}
		})
	}
}

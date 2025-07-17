package gitlab

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// MockJiraClient is a mock implementation of JiraClient interface
type MockJiraClient struct {
	mock.Mock
}

func (m *MockJiraClient) AddComment(issueID, comment string) error {
	args := m.Called(issueID, comment)
	return args.Error(0)
}

func (m *MockJiraClient) GetIssue(issueID string) (*Issue, error) {
	args := m.Called(issueID)
	return args.Get(0).(*Issue), args.Error(1)
}

func (m *MockJiraClient) UpdateIssue(issueID string, fields map[string]interface{}) error {
	args := m.Called(issueID, fields)
	return args.Error(0)
}

func (m *MockJiraClient) CreateIssue(fields map[string]interface{}) (*Issue, error) {
	args := m.Called(fields)
	return args.Get(0).(*Issue), args.Error(1)
}

func (m *MockJiraClient) AddAttachment(issueID, filename string, data []byte) error {
	args := m.Called(issueID, filename, data)
	return args.Error(0)
}

func (m *MockJiraClient) TestConnection() error {
	args := m.Called()
	return args.Error(0)
}

func TestProjectHookHandler_HandleProjectHook(t *testing.T) {
	cfg := &config.Config{
		GitLabSecret: "test-secret",
	}

	tests := []struct {
		name           string
		method         string
		token          string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "Invalid method",
			method:         "GET",
			token:          "test-secret",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid token",
			method:         "POST",
			token:          "wrong-secret",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "Valid push event",
			method: "POST",
			token:  "test-secret",
			body: Event{
				ObjectKind: "push",
				Project: &Project{
					Name:   "test-project",
					WebURL: "https://gitlab.com/test/project",
				},
				Commits: []Commit{
					{
						ID:        "abc123",
						Message:   "Fix PROJ-123 issue",
						URL:       "https://gitlab.com/test/project/commit/abc123",
						Timestamp: "2024-01-01T12:00:00Z",
						Author: Author{
							Name:  "Test User",
							Email: "test@example.com",
						},
					},
				},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJira := &MockJiraClient{}
			if tt.name == "Valid push event" {
				mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)
			}
			handler := &ProjectHookHandler{
				config: cfg,
				logger: slog.Default(),
				jira:   mockJira,
				parser: NewParser(),
			}
			var body []byte
			var err error
			if tt.body != nil {
				body, err = json.Marshal(tt.body)
				assert.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/webhook", bytes.NewBuffer(body))
			if tt.token != "" {
				req.Header.Set("X-Gitlab-Token", tt.token)
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.HandleProjectHook(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockJira.AssertExpectations(t)
		})
	}
}

func TestProjectHookHandler_ProcessPushEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{}, // Add empty config
	}

	event := &Event{
		ObjectKind: "push",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		Commits: []Commit{
			{
				ID:        "abc123",
				Message:   "Fix PROJ-123 issue",
				URL:       "https://gitlab.com/test/project/commit/abc123",
				Timestamp: "2024-01-01T12:00:00Z",
				Author: Author{
					Name:  "Test User",
					Email: "test@example.com",
				},
			},
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processPushEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessMergeRequestEvent(t *testing.T) {
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
			Title:        "Fix PROJ-123 issue",
			Description:  "This fixes the issue mentioned in PROJ-123",
			URL:          "https://gitlab.com/test/project/merge_requests/123",
			Action:       "open",
			SourceBranch: "feature/fix-issue",
			TargetBranch: "main",
			State:        "opened",
			Name:         "Test User",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processMergeRequestEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessIssueEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "issue",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          456,
			Title:       "Issue related to PROJ-123",
			Description: "This issue is related to PROJ-123",
			URL:         "https://gitlab.com/test/project/issues/456",
			Action:      "open",
			State:       "opened",
			IssueType:   "issue",
			Priority:    "medium",
			Name:        "Test User",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processIssueEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessNoteEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "note",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:     789,
			Title:  "Comment on issue",
			URL:    "https://gitlab.com/test/project/issues/456#note_789",
			Action: "create",
			Name:   "Test User",
			Note:   "This comment mentions PROJ-123",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processNoteEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessPipelineEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "pipeline",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:       101,
			Ref:      "PROJ-123-feature",
			URL:      "https://gitlab.com/test/project/pipelines/101",
			Action:   "create",
			Status:   "success",
			Sha:      "abc123",
			Duration: 120,
			Name:     "Test User",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processPipelineEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessBuildEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "build",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:       202,
			Name:     "PROJ-123-test",
			URL:      "https://gitlab.com/test/project/builds/202",
			Action:   "create",
			Status:   "success",
			Stage:    "test",
			Ref:      "main",
			Sha:      "abc123",
			Duration: 60,
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processBuildEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessTagPushEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "tag_push",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:     303,
			Ref:    "PROJ-123-v1.0.0",
			URL:    "https://gitlab.com/test/project/tags/PROJ-123-v1.0.0",
			Action: "create",
			Name:   "Test User",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processTagPushEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessReleaseEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "release",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          404,
			Name:        "Release for PROJ-123",
			URL:         "https://gitlab.com/test/project/releases/404",
			Action:      "create",
			Ref:         "v1.0.0",
			Description: "Release related to PROJ-123",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processReleaseEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessDeploymentEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "deployment",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          505,
			Ref:         "PROJ-123-deploy",
			URL:         "https://gitlab.com/test/project/environments/505",
			Action:      "create",
			Environment: "production",
			Status:      "success",
			Sha:         "abc123",
			Name:        "Test User",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processDeploymentEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessFeatureFlagEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "feature_flag",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:          606,
			Name:        "PROJ-123-feature",
			URL:         "https://gitlab.com/test/project/feature_flags/606",
			Action:      "create",
			Description: "Feature flag for PROJ-123",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processFeatureFlagEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessWikiPageEvent(t *testing.T) {
	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{},
	}

	event := &Event{
		ObjectKind: "wiki_page",
		Project: &Project{
			Name:   "test-project",
			WebURL: "https://gitlab.com/test/project",
		},
		ObjectAttributes: &ObjectAttributes{
			ID:      707,
			Title:   "PROJ-123 Documentation",
			URL:     "https://gitlab.com/test/project/wikis/PROJ-123-Documentation",
			Action:  "create",
			Name:    "Test User",
			Content: "Documentation for PROJ-123 feature implementation",
		},
	}

	mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)

	err := handler.processWikiPageEvent(event)
	assert.NoError(t, err)
	mockJira.AssertExpectations(t)
}

func TestProjectHookHandler_ProcessPushEvent_BranchFilter(t *testing.T) {
	tests := []struct {
		name           string
		branchFilters  []string
		eventRef       string
		shouldProcess  bool
		expectedLogMsg string
	}{
		{
			name:          "No filters - should process",
			branchFilters: []string{},
			eventRef:      "refs/heads/main",
			shouldProcess: true,
		},
		{
			name:          "Exact match - should process",
			branchFilters: []string{"main", "develop"},
			eventRef:      "refs/heads/main",
			shouldProcess: true,
		},
		{
			name:          "Wildcard match - should process",
			branchFilters: []string{"main", "release-*"},
			eventRef:      "refs/heads/release-v1.0.0",
			shouldProcess: true,
		},
		{
			name:          "Multiple wildcard patterns - should process",
			branchFilters: []string{"main", "release-*", "hotfix/*"},
			eventRef:      "refs/heads/hotfix/critical-bug",
			shouldProcess: true,
		},
		{
			name:           "No match - should not process",
			branchFilters:  []string{"main", "develop"},
			eventRef:       "refs/heads/feature/new-feature",
			shouldProcess:  false,
			expectedLogMsg: "Push event ignored by branch filter",
		},
		{
			name:           "Wildcard no match - should not process",
			branchFilters:  []string{"main", "release-*"},
			eventRef:       "refs/heads/feature/experimental",
			shouldProcess:  false,
			expectedLogMsg: "Push event ignored by branch filter",
		},
		{
			name:          "Empty ref - should process (no filter applied)",
			branchFilters: []string{"main"},
			eventRef:      "",
			shouldProcess: true,
		},
		{
			name:          "Ref without refs/heads/ prefix - should process",
			branchFilters: []string{"main"},
			eventRef:      "main",
			shouldProcess: true,
		},
		{
			name:          "Complex wildcard pattern - should process",
			branchFilters: []string{"main", "release-*", "hotfix/*", "feature/??-*"},
			eventRef:      "refs/heads/feature/ab-new-feature",
			shouldProcess: true,
		},
		{
			name:           "Complex wildcard pattern - should not process",
			branchFilters:  []string{"main", "release-*", "hotfix/*", "feature/??-*"},
			eventRef:       "refs/heads/feature/a-new-feature",
			shouldProcess:  false,
			expectedLogMsg: "Push event ignored by branch filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJira := &MockJiraClient{}
			handler := &ProjectHookHandler{
				logger: slog.Default(),
				jira:   mockJira,
				parser: NewParser(),
				config: &config.Config{
					PushBranchFilter: tt.branchFilters,
				},
			}

			event := &Event{
				ObjectKind: "push",
				Ref:        tt.eventRef,
				Project: &Project{
					Name:   "test-project",
					WebURL: "https://gitlab.com/test/project",
				},
				Commits: []Commit{
					{
						ID:        "abc123",
						Message:   "Fix PROJ-123 issue",
						URL:       "https://gitlab.com/test/project/commit/abc123",
						Timestamp: "2024-01-01T12:00:00Z",
						Author: Author{
							Name:  "Test User",
							Email: "test@example.com",
						},
					},
				},
			}

			if tt.shouldProcess {
				mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)
			}

			err := handler.processPushEvent(event)
			assert.NoError(t, err)

			if tt.shouldProcess {
				mockJira.AssertExpectations(t)
			} else {
				mockJira.AssertNotCalled(t, "AddComment")
			}
		})
	}
}

func TestProjectHookHandler_ProcessPushEvent_BranchFilter_Integration(t *testing.T) {
	// Тест интеграции с реальными паттернами веток
	realWorldPatterns := []string{
		"main",
		"master",
		"develop",
		"release-*",
		"hotfix/*",
		"feature/*",
		"bugfix/*",
		"staging",
		"production",
	}

	mockJira := &MockJiraClient{}
	handler := &ProjectHookHandler{
		logger: slog.Default(),
		jira:   mockJira,
		parser: NewParser(),
		config: &config.Config{
			PushBranchFilter: realWorldPatterns,
		},
	}

	testCases := []struct {
		branchName    string
		shouldProcess bool
		description   string
	}{
		{"main", true, "Main branch"},
		{"master", true, "Master branch"},
		{"develop", true, "Develop branch"},
		{"release-v1.0.0", true, "Release branch with version"},
		{"release-hotfix", true, "Release branch with text"},
		{"hotfix/critical-bug", true, "Hotfix branch"},
		{"hotfix/urgent-fix", true, "Another hotfix branch"},
		{"feature/new-feature", true, "Feature branch"},
		{"feature/user-authentication", true, "Feature branch with description"},
		{"bugfix/login-issue", true, "Bugfix branch"},
		{"staging", true, "Staging branch"},
		{"production", true, "Production branch"},
		{"experimental", false, "Experimental branch (not in filter)"},
		{"temp/test", false, "Temp branch (not in filter)"},
		{"random-branch", false, "Random branch (not in filter)"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			event := &Event{
				ObjectKind: "push",
				Ref:        "refs/heads/" + tc.branchName,
				Project: &Project{
					Name:   "test-project",
					WebURL: "https://gitlab.com/test/project",
				},
				Commits: []Commit{
					{
						ID:        "abc123",
						Message:   "Fix PROJ-123 issue",
						URL:       "https://gitlab.com/test/project/commit/abc123",
						Timestamp: "2024-01-01T12:00:00Z",
						Author: Author{
							Name:  "Test User",
							Email: "test@example.com",
						},
					},
				},
			}

			if tc.shouldProcess {
				mockJira.On("AddComment", "PROJ-123", mock.AnythingOfType("string")).Return(nil)
			}

			err := handler.processPushEvent(event)
			assert.NoError(t, err)

			if tc.shouldProcess {
				mockJira.AssertExpectations(t)
			} else {
				mockJira.AssertNotCalled(t, "AddComment")
			}
		})
	}
}

package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	userlink "github.com/atlet99/gitlab-jira-hook/internal/common"
)

func TestCreateCommitBranch_EmptyURL(t *testing.T) {
	// Test with empty branchURL - should not create a link
	content := createCommitBranch("main", "")

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal content: %v", err)
	}

	// Check that there's no link mark in the JSON
	jsonStr := string(jsonData)
	if contains(jsonStr, `"type":"link"`) {
		t.Error("Expected no link mark when branchURL is empty, but found one")
	}

	// Check that the branch name is present as plain text
	if !contains(jsonStr, `"text":"main"`) {
		t.Error("Expected branch name to be present as text")
	}
}

func TestCreateCommitBranch_WithURL(t *testing.T) {
	// Test with valid branchURL - should create a link
	content := createCommitBranch("main", "https://git.local.com/project/-/tree/main")

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal content: %v", err)
	}

	// Check that there's a link mark in the JSON
	jsonStr := string(jsonData)
	if !contains(jsonStr, `"type":"link"`) {
		t.Error("Expected link mark when branchURL is provided, but not found")
	}

	// Check that the URL is present
	if !contains(jsonStr, `"href":"https://git.local.com/project/-/tree/main"`) {
		t.Error("Expected href to be present in link mark")
	}
}

func TestCreateTitleLink_EmptyURL(t *testing.T) {
	// Test with empty URL - should not create a link
	content := createTitleLink("Test Title", "", "title")

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal content: %v", err)
	}

	// Check that there's no link mark in the JSON
	jsonStr := string(jsonData)
	if contains(jsonStr, `"type":"link"`) {
		t.Error("Expected no link mark when URL is empty, but found one")
	}

	// Check that the title is present as plain text
	if !contains(jsonStr, `"text":"Test Title"`) {
		t.Error("Expected title to be present as text")
	}
}

func TestCreateProjectLink_EmptyURL(t *testing.T) {
	// Test with empty projectURL - should not create a link
	content := createProjectLink("Test Project", "")

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal content: %v", err)
	}

	// Check that there's no link mark in the JSON
	jsonStr := string(jsonData)
	if contains(jsonStr, `"type":"link"`) {
		t.Error("Expected no link mark when projectURL is empty, but found one")
	}

	// Check that the project name is present as plain text
	if !contains(jsonStr, `"text":"Test Project"`) {
		t.Error("Expected project name to be present as text")
	}
}

func TestGenerateMergeRequestADFComment_NewFormat(t *testing.T) {
	comment := GenerateMergeRequestADFComment(
		"Test MR for ABC-123",
		"https://gitlab.com/test/project/merge_requests/123",
		"test-project",
		"https://gitlab.com/test/project",
		"update",
		"feature/ABC-456-branch",
		"main",
		"opened",
		"Test User",
		"This MR addresses ABC-123 and ABC-456",
		"Asia/Almaty",
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
	)

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(comment)
	if err != nil {
		t.Fatalf("Failed to marshal comment: %v", err)
	}

	jsonStr := string(jsonData)

	// Print the actual structure for debugging
	t.Logf("Generated JSON: %s", jsonStr)

	// Check that all key fields are present (order is not important)
	fields := []string{
		`"text":"author: "`,
		`"text":"merge request: "`,
		`"text":"branches: "`,
		`"text":"commit: "`,
		`"text":"project: "`,
		`"text":"action: "`,
		`"text":"status: "`,
		`"text":"date: "`,
	}
	for _, field := range fields {
		if !contains(jsonStr, field) {
			t.Errorf("Expected field %s in MR comment", field)
		}
	}

	// Should not have the old timestamp format
	if contains(jsonStr, `"text":"**Timestamp:**"`) {
		t.Error("Should not have old timestamp format")
	}
}

func TestGenerateMergeRequestADFComment_WithParticipants(t *testing.T) {
	participants := []userlink.UserWithLink{
		{Name: "john_doe", URL: "https://gitlab.com/john_doe"},
		{Name: "jane_smith", URL: "https://gitlab.com/jane_smith"},
	}

	comment := GenerateMergeRequestADFComment(
		"Test MR",
		"https://gitlab.com/test/project/merge_requests/123",
		"test-project",
		"https://gitlab.com/test/project",
		"open",
		"feature/test",
		"main",
		"opened",
		"Test User",
		"Test description",
		"UTC",
		participants,
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
	)

	// Convert to JSON for easier testing
	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)
	jsonStr := string(jsonData)

	// Check that participants field is present
	assert.Contains(t, jsonStr, "participants", "Expected participants field in MR comment")

	// Check that all participants are included with their names
	for _, participant := range participants {
		assert.Contains(t, jsonStr, participant.Name, "Expected participant %s in MR comment", participant.Name)
	}

	// Check that participants have clickable links
	assert.Contains(t, jsonStr, `"type":"link"`, "Expected participants to have clickable links")
	assert.Contains(t, jsonStr, `"href":"https://gitlab.com/john_doe"`, "Expected first participant link")
	assert.Contains(t, jsonStr, `"href":"https://gitlab.com/jane_smith"`, "Expected second participant link")
}

func TestGenerateMergeRequestADFComment_WithApprovers(t *testing.T) {
	approvedBy := []userlink.UserWithLink{
		{Name: "reviewer1", URL: "https://gitlab.com/reviewer1"},
		{Name: "reviewer2", URL: "https://gitlab.com/reviewer2"},
	}

	comment := GenerateMergeRequestADFComment(
		"Test MR",
		"https://gitlab.com/test/project/merge_requests/123",
		"test-project",
		"https://gitlab.com/test/project",
		"open",
		"feature/test",
		"main",
		"opened",
		"Test User",
		"Test description",
		"UTC",
		[]userlink.UserWithLink{},
		approvedBy,
		[]userlink.UserWithLink{},
		[]userlink.UserWithLink{},
	)

	// Convert to JSON for easier testing
	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)
	jsonStr := string(jsonData)

	// Check that approved by field is present
	assert.Contains(t, jsonStr, "approved by", "Expected approved by field in MR comment")

	// Check that all approvers are included with their names
	for _, approver := range approvedBy {
		assert.Contains(t, jsonStr, approver.Name, "Expected approver %s in MR comment", approver.Name)
	}

	// Check that approvers have clickable links
	assert.Contains(t, jsonStr, `"type":"link"`, "Expected approvers to have clickable links")
	assert.Contains(t, jsonStr, `"href":"https://gitlab.com/reviewer1"`, "Expected first approver link")
	assert.Contains(t, jsonStr, `"href":"https://gitlab.com/reviewer2"`, "Expected second approver link")
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateCommitADFComment(t *testing.T) {
	comment := GenerateCommitADFComment(
		"abc123", "https://gitlab.com/test/project/commit/abc123",
		"Test User", "test@example.com", "https://gitlab.com/test/user",
		"Fix ABC-123 issue", "2024-01-01T12:00:00Z", "main", "https://gitlab.com/test/project/-/tree/main",
		"https://gitlab.com/test/project", "+05:00",
		[]string{"file1.txt"}, []string{"file2.txt"}, []string{},
	)

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"username: "`)
	assert.Contains(t, jsonStr, `"text":"Test User"`)
	assert.Contains(t, jsonStr, `"text":"commit: "`)
	assert.Contains(t, jsonStr, `"text":"Fix ABC-123 issue"`)
}

func TestCreateCommitHeader(t *testing.T) {
	header := createCommitHeader("abc123", "https://gitlab.com/test/project/commit/abc123")

	jsonData, err := json.Marshal(header)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"type":"paragraph"`)
	assert.Contains(t, jsonStr, `"text":"commit: "`)
	assert.Contains(t, jsonStr, `"type":"strong"`)
}

func TestCreateCommitAuthor(t *testing.T) {
	authorField := createCommitAuthor("Test User", "https://gitlab.com/test/user")

	jsonData, err := json.Marshal(authorField)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"username: "`)
	assert.Contains(t, jsonStr, `"text":"Test User"`)
	assert.Contains(t, jsonStr, `"type":"strong"`)
}

func TestCreateCommitMessage(t *testing.T) {
	message := "Fix ABC-123 issue with proper error handling"

	commitContent, _ := createCommitMessage(message, "https://gitlab.com/test/project")

	jsonData, err := json.Marshal(commitContent)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"commit: "`)
	assert.Contains(t, jsonStr, `"text":"Fix ABC-123 issue with proper error handling"`)
	assert.Contains(t, jsonStr, `"type":"strong"`)
}

func TestExtractMergeRequestLink(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with merge request link",
			message:  "Fix ABC-123\n\nSee merge request test/project!123",
			expected: "https://gitlab.com/test/project/-/merge_requests/123",
		},
		{
			name:     "without merge request link",
			message:  "Fix ABC-123",
			expected: "",
		},
		{
			name:     "with multiple merge request links",
			message:  "Fix ABC-123\n\nSee merge request test/project!123",
			expected: "https://gitlab.com/test/project/-/merge_requests/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMergeRequestLink(tt.message, "https://gitlab.com/test/project")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMRID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid MR ID",
			input:    "test/project!123",
			expected: "123",
		},
		{
			name:     "invalid MR ID",
			input:    "abc",
			expected: "abc",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMRID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateCommitDate(t *testing.T) {
	dateField := createCommitDate("2024-01-01T12:00:00Z", "+05:00")

	jsonData, err := json.Marshal(dateField)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"date: "`)
	assert.Contains(t, jsonStr, `"type":"strong"`)
}

func TestFormatDateGOST(t *testing.T) {
	// This function should format date according to GOST 7.64-90
	// We'll test that it returns a properly formatted string
	formatted := formatDateGOST("2024-01-01T12:00:00Z")

	// Should contain date in format DD.MM.YYYY HH:MM (without seconds)
	assert.Regexp(t, `^\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}$`, formatted)
}

func TestHasFileChanges(t *testing.T) {
	tests := []struct {
		name     string
		added    []string
		modified []string
		removed  []string
		expected bool
	}{
		{
			name:     "with changes",
			added:    []string{"file1.txt"},
			modified: []string{},
			removed:  []string{},
			expected: true,
		},
		{
			name:     "without changes",
			added:    []string{},
			modified: []string{},
			removed:  []string{},
			expected: false,
		},
		{
			name:     "with modified files",
			added:    []string{},
			modified: []string{"file2.txt"},
			removed:  []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFileChanges(tt.added, tt.modified, tt.removed)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateCompactFileChangesSection(t *testing.T) {
	added := []string{"file1.txt"}
	modified := []string{"file2.txt"}
	removed := []string{"file3.txt"}

	section := createCompactFileChangesSection(added, modified, removed)

	jsonData, err := json.Marshal(section)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"files: "`)
	assert.Contains(t, jsonStr, `"text":"+1 ~1 -1"`)
}

func TestCreateDescriptionField(t *testing.T) {
	description := "This is a test description"

	field := createDescriptionField(description)

	jsonData, err := json.Marshal(field)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"description: "`)
	assert.Contains(t, jsonStr, `"text":"This is a test description"`)
	assert.Contains(t, jsonStr, `"type":"strong"`)
}

func TestCreateSimpleADF(t *testing.T) {
	content := CreateSimpleADF("Test message")

	jsonData, err := json.Marshal(content)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"type":"doc"`)
	assert.Contains(t, jsonStr, `"text":"Test message"`)
}

func TestCreateSimpleParagraph(t *testing.T) {
	paragraph := createSimpleParagraph("Test paragraph")

	jsonData, err := json.Marshal(paragraph)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"type":"paragraph"`)
	assert.Contains(t, jsonStr, `"text":"Test paragraph"`)
}

func TestGenerateIssueADFComment(t *testing.T) {
	comment := GenerateIssueADFComment(
		"Test Issue", "https://gitlab.com/test/project/issues/123",
		"test-project", "https://gitlab.com/test/project",
		"open", "opened", "issue", "medium",
		"Test User", "Test description", "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"issue: "`)
	assert.Contains(t, jsonStr, `"text":"Test Issue"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGeneratePipelineADFComment(t *testing.T) {
	comment := GeneratePipelineADFComment(
		"main", "https://gitlab.com/test/project/pipelines/456",
		"test-project", "https://gitlab.com/test/project",
		"create", "success", "abc123", "Test User", 120, "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"pipeline: "`)
	assert.Contains(t, jsonStr, `"text":"success"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateBuildADFComment(t *testing.T) {
	comment := GenerateBuildADFComment(
		"test-build", "https://gitlab.com/test/project/builds/789",
		"test-project", "https://gitlab.com/test/project",
		"create", "success", "test-stage", "main", "abc123", "Test User", 60, "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"build: "`)
	assert.Contains(t, jsonStr, `"text":"success"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateSimpleADFComment(t *testing.T) {
	comment := generateSimpleADFComment(
		"Test Title", "https://gitlab.com/test/project",
		"test-project", "https://gitlab.com/test/project",
		"create", "Test User", "Test content", "", "Test ADF Title",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"type":"doc"`)
	assert.Contains(t, jsonStr, `"text":"Test content"`)
}

func TestGenerateNoteADFComment(t *testing.T) {
	comment := GenerateNoteADFComment(
		"Test Note", "https://gitlab.com/test/project/notes/101",
		"test-project", "https://gitlab.com/test/project",
		"create", "Test User", "Test note content", "",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"comment: "`)
	assert.Contains(t, jsonStr, `"text":"Test note content"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateFeatureFlagADFComment(t *testing.T) {
	comment := GenerateFeatureFlagADFComment(
		"test-feature", "https://gitlab.com/test/project/feature_flags/202",
		"test-project", "https://gitlab.com/test/project",
		"create", "Test feature flag description", "Test User", "",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"feature flag: "`)
	assert.Contains(t, jsonStr, `"text":"test-feature"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateWikiPageADFComment(t *testing.T) {
	comment := GenerateWikiPageADFComment(
		"Test Wiki Page", "https://gitlab.com/test/project/wikis/303",
		"test-project", "https://gitlab.com/test/project",
		"create", "Test User", "Test content", "",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"wiki page: "`)
	assert.Contains(t, jsonStr, `"text":"Test Wiki Page"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateTagPushADFComment(t *testing.T) {
	comment := GenerateTagPushADFComment(
		"v1.0.0", "https://gitlab.com/test/project/tags/v1.0.0",
		"test-project", "https://gitlab.com/test/project",
		"create", "Test User", "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"tag: "`)
	assert.Contains(t, jsonStr, `"text":"v1.0.0"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateReleaseADFComment(t *testing.T) {
	comment := GenerateReleaseADFComment(
		"Release v1.0.0", "https://gitlab.com/test/project/releases/404",
		"test-project", "https://gitlab.com/test/project",
		"create", "v1.0.0", "Test release", "Test User", "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"release: "`)
	assert.Contains(t, jsonStr, `"text":"Release v1.0.0"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

func TestGenerateDeploymentADFComment(t *testing.T) {
	comment := GenerateDeploymentADFComment(
		"main", "https://gitlab.com/test/project/deployments/505",
		"test-project", "https://gitlab.com/test/project",
		"create", "production", "success", "abc123", "Test User", "+05:00",
	)

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"text":"deployment: "`)
	assert.Contains(t, jsonStr, `"text":"production"`)
	assert.Contains(t, jsonStr, `"text":"success"`)
	assert.Contains(t, jsonStr, `"text":"test-project"`)
}

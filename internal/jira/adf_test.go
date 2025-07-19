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

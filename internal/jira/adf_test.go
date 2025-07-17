package jira

import (
	"encoding/json"
	"testing"
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
		"Update",
		"feature/ABC-456-branch",
		"main",
		"opened",
		"Test User",
		"This MR addresses ABC-123 and ABC-456",
		[]string{}, // participants
		[]string{}, // approvedBy
		[]string{}, // reviewers
		[]string{}, // approvers
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
	participants := []string{"John Doe", "Jane Smith", "Bob Johnson"}
	comment := GenerateMergeRequestADFComment(
		"Test MR with participants",
		"https://gitlab.com/test/project/merge_requests/456",
		"test-project",
		"https://gitlab.com/test/project",
		"Open",
		"feature/test",
		"main",
		"opened",
		"Test User",
		"This MR has multiple participants",
		participants,
		[]string{}, // approvedBy
		[]string{}, // reviewers
		[]string{}, // approvers
	)

	// Marshal to JSON to check the structure
	jsonData, err := json.Marshal(comment)
	if err != nil {
		t.Fatalf("Failed to marshal comment: %v", err)
	}

	jsonStr := string(jsonData)

	// Check that participants field is present
	if !contains(jsonStr, `"text":"participants: "`) {
		t.Error("Expected participants field in MR comment")
	}

	// Check that all participants are included
	for _, participant := range participants {
		if !contains(jsonStr, participant) {
			t.Errorf("Expected participant %s in MR comment", participant)
		}
	}

	// Check that participants are joined with commas
	if !contains(jsonStr, "John Doe, Jane Smith, Bob Johnson") {
		t.Error("Expected participants to be joined with commas")
	}
}

func TestGenerateMergeRequestADFComment_WithApprovers(t *testing.T) {
	participants := []string{"John Doe"}
	approvedBy := []string{"Alice Brown", "Bob Johnson"}
	reviewers := []string{"Jane Smith"}
	approvers := []string{"Eve Adams"}
	comment := GenerateMergeRequestADFComment(
		"Test MR with approvers",
		"https://gitlab.com/test/project/merge_requests/789",
		"test-project",
		"https://gitlab.com/test/project",
		"Open",
		"feature/test",
		"main",
		"opened",
		"Test User",
		"This MR has approvers and reviewers",
		participants,
		approvedBy,
		reviewers,
		approvers,
	)

	jsonData, err := json.Marshal(comment)
	if err != nil {
		t.Fatalf("Failed to marshal comment: %v", err)
	}
	jsonStr := string(jsonData)

	if !contains(jsonStr, `"text":"approved by: "`) {
		t.Error("Expected approved by field in MR comment")
	}
	if !contains(jsonStr, `"text":"reviewers: "`) {
		t.Error("Expected reviewers field in MR comment")
	}
	if !contains(jsonStr, `"text":"approvers: "`) {
		t.Error("Expected approvers field in MR comment")
	}
	if !contains(jsonStr, "Alice Brown, Bob Johnson") {
		t.Error("Expected approved by users to be joined with commas")
	}
	if !contains(jsonStr, "Jane Smith") {
		t.Error("Expected reviewer in MR comment")
	}
	if !contains(jsonStr, "Eve Adams") {
		t.Error("Expected approver in MR comment")
	}
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

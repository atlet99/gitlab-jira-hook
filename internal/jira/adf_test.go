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

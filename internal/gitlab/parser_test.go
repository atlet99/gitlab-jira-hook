package gitlab

import (
	"testing"
)

func TestParser_ExtractIssueIDs(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "Single issue ID",
			text:     "Fix login issue ABC-123",
			expected: []string{"ABC-123"},
		},
		{
			name:     "Multiple issue IDs",
			text:     "Implements feature XYZ-456 and resolves ABC-789",
			expected: []string{"XYZ-456", "ABC-789"},
		},
		{
			name:     "Duplicate issue IDs",
			text:     "Fix ABC-123 and also ABC-123",
			expected: []string{"ABC-123"},
		},
		{
			name:     "No issue IDs",
			text:     "Just a regular commit message",
			expected: nil,
		},
		{
			name:     "Empty text",
			text:     "",
			expected: nil,
		},
		{
			name:     "Mixed case issue IDs",
			text:     "Fix abc-123 and XYZ-456",
			expected: []string{"ABC-123", "XYZ-456"},
		},
		{
			name:     "Issue ID with special characters",
			text:     "Fix issue ABC-123! and XYZ-456?",
			expected: []string{"ABC-123", "XYZ-456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractIssueIDs(tt.text)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d issue IDs, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) {
					t.Errorf("Expected issue ID %s at position %d, but result is too short", expected, i)
					continue
				}
				if result[i] != expected {
					t.Errorf("Expected issue ID %s at position %d, got %s", expected, i, result[i])
				}
			}
		})
	}
}

func TestParser_HasIssueID(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Has issue ID",
			text:     "Fix login issue ABC-123",
			expected: true,
		},
		{
			name:     "No issue ID",
			text:     "Just a regular commit message",
			expected: false,
		},
		{
			name:     "Empty text",
			text:     "",
			expected: false,
		},
		{
			name:     "Multiple issue IDs",
			text:     "Fix ABC-123 and XYZ-456",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.HasIssueID(tt.text)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for text: %s", tt.expected, result, tt.text)
			}
		})
	}
}

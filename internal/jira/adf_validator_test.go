package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateADF(t *testing.T) {
	tests := []struct {
		name        string
		content     CommentPayload
		expectError bool
	}{
		{
			name: "valid ADF content",
			content: CommentPayload{
				Body: CommentBody{
					Type:    "doc",
					Version: 1,
					Content: []Content{
						{
							Type: "paragraph",
							Content: []TextContent{
								{Type: "text", Text: "Test content"},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid ADF type",
			content: CommentPayload{
				Body: CommentBody{
					Type:    "invalid",
					Version: 1,
					Content: []Content{
						{
							Type: "paragraph",
							Content: []TextContent{
								{Type: "text", Text: "Test content"},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid ADF version",
			content: CommentPayload{
				Body: CommentBody{
					Type:    "doc",
					Version: 2,
					Content: []Content{
						{
							Type: "paragraph",
							Content: []TextContent{
								{Type: "text", Text: "Test content"},
							},
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty content array",
			content: CommentPayload{
				Body: CommentBody{
					Type:    "doc",
					Version: 1,
					Content: []Content{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Debug output to see what's in the content
			jsonData, _ := json.Marshal(tt.content)
			t.Logf("Content JSON: %s", string(jsonData))

			_, err := validateADF(tt.content)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFallbackToPlainText(t *testing.T) {
	content := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "First paragraph"},
					},
				},
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Second paragraph"},
					},
				},
			},
		},
	}

	expected := "First paragraph\nSecond paragraph\n"
	result := fallbackToPlainText(content)
	assert.Equal(t, expected, result)
}

func TestValidateAndFallback(t *testing.T) {
	// Test with valid ADF content
	validContent := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Valid content"},
					},
				},
			},
		},
	}

	validatedContent := validateAndFallback(validContent)
	// validateAndFallback always returns a valid payload, no error to check
	assert.Equal(t, validContent, validatedContent)

	// Test with invalid ADF content (invalid type)
	invalidContent := CommentPayload{
		Body: CommentBody{
			Type:    "invalid",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Invalid content"},
					},
				},
			},
		},
	}

	fallbackContent := validateAndFallback(invalidContent)
	// validateAndFallback always returns a valid payload, no error to check
	assert.Equal(t, "doc", fallbackContent.Body.Type)
	assert.Equal(t, 1, fallbackContent.Body.Version)
}

func TestValidateCommentPayload(t *testing.T) {
	validContent := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Valid content"},
					},
				},
			},
		},
	}

	validatedContent, err := ValidateCommentPayload(validContent)
	assert.NoError(t, err)
	assert.Equal(t, validContent, validatedContent)

	invalidContent := CommentPayload{
		Body: CommentBody{
			Type:    "invalid",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Invalid content"},
					},
				},
			},
		},
	}

	_, err = ValidateCommentPayload(invalidContent)
	assert.Error(t, err)
}

func TestIsADFValid(t *testing.T) {
	validContent := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Valid content"},
					},
				},
			},
		},
	}

	assert.True(t, IsADFValid(validContent))

	invalidContent := CommentPayload{
		Body: CommentBody{
			Type:    "invalid",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Invalid content"},
					},
				},
			},
		},
	}

	assert.False(t, IsADFValid(invalidContent))
}

func TestSanitizeADF(t *testing.T) {
	content := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Valid content"},
						{Type: "", Text: "Invalid content"}, // Invalid type
					},
				},
				{
					Type: "invalidType", // Invalid content type
					Content: []TextContent{
						{Type: "text", Text: "Some content"},
					},
				},
			},
		},
	}

	sanitized := SanitizeADF(content)

	// Should have only valid content blocks
	assert.Len(t, sanitized.Body.Content, 1)
	assert.Equal(t, "paragraph", sanitized.Body.Content[0].Type)
	assert.Len(t, sanitized.Body.Content[0].Content, 1)
	assert.Equal(t, "Valid content", sanitized.Body.Content[0].Content[0].Text)
}

func TestExtractTextFromADF(t *testing.T) {
	content := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "First paragraph"},
					},
				},
				{
					Type: "paragraph",
					Content: []TextContent{
						{Type: "text", Text: "Second paragraph"},
					},
				},
			},
		},
	}

	expected := "First paragraph\nSecond paragraph"
	result := ExtractTextFromADF(content)
	assert.Equal(t, expected, result)
}

package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertMarkdownToPlainText(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected string
	}{
		{
			name:     "Simple text",
			markdown: "Hello World",
			expected: "Hello World",
		},
		{
			name:     "Markdown link",
			markdown: "Commit [1ea90eb3](https://git.test.com/...) by **John Doe**",
			expected: "Commit 1ea90eb3 (https://git.test.com/...) by John Doe",
		},
		{
			name:     "Bold text",
			markdown: "**Bold text** and normal text",
			expected: "Bold text and normal text",
		},
		{
			name:     "Code blocks",
			markdown: "Branch `main` and file `README.md`",
			expected: "Branch main and file README.md",
		},
		{
			name:     "Mixed formatting",
			markdown: "Commit [`1ea90eb3`](https://git.test.com/...) by **John Doe** on branch `main`",
			expected: "Commit 1ea90eb3 (https://git.test.com/...) by John Doe on branch main",
		},
		{
			name:     "Multiple links",
			markdown: "Project: [MyProject](https://gitlab.com/project) and [Issue](https://jira.com/issue)",
			expected: "Project: MyProject (https://gitlab.com/project) and Issue (https://jira.com/issue)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMarkdownToPlainText(tt.markdown)
			assert.Equal(t, tt.expected, result)
		})
	}
}

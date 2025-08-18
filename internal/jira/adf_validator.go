package jira

import (
	"fmt"
	"log"
	"strings"
)

// TextContentType is the constant for "text" content type in ADF
const TextContentType = "text"

// validateADF validates ADF content against the ADF schema
// Returns the validated ADF content or an error if validation fails
func validateADF(content CommentPayload) (CommentPayload, error) {
	// Check required fields
	if content.Body.Type != "doc" {
		return content, fmt.Errorf("ADF must have type 'doc'")
	}

	if content.Body.Version != 1 {
		return content, fmt.Errorf("ADF must have version 1")
	}

	if len(content.Body.Content) == 0 {
		return content, fmt.Errorf("ADF must have non-empty content array")
	}

	// If validation passes, return the original content
	return content, nil
}

// fallbackToPlainText converts ADF content to plain text
// This is used when ADF validation fails
func fallbackToPlainText(content CommentPayload) string {
	var result strings.Builder

	// Extract text content from ADF
	for i, item := range content.Body.Content {
		for _, textContent := range item.Content {
			if textContent.Type == TextContentType {
				result.WriteString(textContent.Text)
			}
		}
		// Add a newline after each paragraph
		if i < len(content.Body.Content)-1 {
			result.WriteString("\n")
		}
	}

	// Add a trailing newline for consistency with expected test output
	result.WriteString("\n")

	return result.String()
}

// ExtractTextFromADF extracts plain text from ADF content
// This is used to convert ADF content to readable text for GitLab
func ExtractTextFromADF(content CommentPayload) string {
	var result strings.Builder

	// Extract text content from ADF
	for i, item := range content.Body.Content {
		for _, textContent := range item.Content {
			if textContent.Type == TextContentType {
				result.WriteString(textContent.Text)
			}
		}
		// Add a newline after each paragraph
		if i < len(content.Body.Content)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// validateAndFallback validates ADF content and falls back to plain text if validation fails
// Returns the validated ADF content or a plain text fallback
func validateAndFallback(content CommentPayload) CommentPayload {
	// Try to validate the ADF content
	//nolint:errcheck // Error is handled in the if statement below
	validatedContent, err := validateADF(content)
	if err != nil {
		// If validation fails, fall back to plain text
		plainText := fallbackToPlainText(content)
		log.Printf("ADF validation failed, using plain text fallback: %v", err)
		return CreateSimpleADF(plainText)
	}

	// If validation passes, return the validated content
	return validatedContent
}

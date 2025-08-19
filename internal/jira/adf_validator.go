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
		return content, fmt.Errorf("ADF must have type 'doc', got: %s", content.Body.Type)
	}

	if content.Body.Version != 1 {
		return content, fmt.Errorf("ADF must have version 1, got: %d", content.Body.Version)
	}

	if len(content.Body.Content) == 0 {
		return content, fmt.Errorf("ADF must have non-empty content array")
	}

	// Validate each content block
	for i, contentBlock := range content.Body.Content {
		if err := validateContentBlock(contentBlock, i); err != nil {
			return content, fmt.Errorf("content block %d validation failed: %w", i, err)
		}
	}

	// If validation passes, return the original content
	return content, nil
}

// validateContentBlock validates a single content block in ADF
func validateContentBlock(block Content, index int) error {
	// Validate content type
	if block.Type == "" {
		return fmt.Errorf("content block %d: type is required", index)
	}

	// Validate content type is allowed
	allowedTypes := map[string]bool{
		"paragraph":   true,
		"heading":     true,
		"bulletList":  true,
		"orderedList": true,
		"codeBlock":   true,
		"blockquote":  true,
		"panel":       true,
		"table":       true,
		"layout":      true,
		"embed":       true,
		"extension":   true,
	}

	if !allowedTypes[block.Type] {
		return fmt.Errorf("content block %d: invalid content type '%s'", index, block.Type)
	}

	// Validate content array
	if len(block.Content) == 0 {
		return fmt.Errorf("content block %d: content array cannot be empty", index)
	}

	// Validate each text content
	for i, textContent := range block.Content {
		if err := validateTextContent(textContent, index, i); err != nil {
			return fmt.Errorf("content block %d, text content %d: %w", index, i, err)
		}
	}

	return nil
}

// validateTextContent validates a single text content in ADF
func validateTextContent(textContent TextContent, blockIndex, textIndex int) error {
	// Validate text content type
	if textContent.Type == "" {
		return fmt.Errorf("text content %d in block %d: type is required", textIndex, blockIndex)
	}

	// Validate text content type is allowed
	allowedTextTypes := map[string]bool{
		"text":      true,
		"hardBreak": true,
		"mention":   true,
	}

	if !allowedTextTypes[textContent.Type] {
		return fmt.Errorf("text content %d in block %d: invalid text type '%s'", textIndex, blockIndex, textContent.Type)
	}

	// Validate text content has required fields
	if textContent.Type == "text" && textContent.Text == "" {
		return fmt.Errorf("text content %d in block %d: text content cannot be empty", textIndex, blockIndex)
	}

	// Validate marks if present
	if len(textContent.Marks) > 0 {
		for i, mark := range textContent.Marks {
			if err := validateMark(mark, blockIndex, textIndex, i); err != nil {
				return fmt.Errorf("text content %d in block %d, mark %d: %w", textIndex, blockIndex, i, err)
			}
		}
	}

	return nil
}

// validateMark validates a single mark in ADF
func validateMark(mark Mark, blockIndex, textIndex, markIndex int) error {
	// Validate mark type
	if mark.Type == "" {
		return fmt.Errorf("mark %d in text content %d of block %d: type is required", markIndex, textIndex, blockIndex)
	}

	// Validate mark type is allowed
	allowedMarkTypes := map[string]bool{
		"bold":        true,
		"italic":      true,
		"underline":   true,
		"strike":      true,
		"code":        true,
		"link":        true,
		"subscript":   true,
		"superscript": true,
		"color":       true,
	}

	if !allowedMarkTypes[mark.Type] {
		return fmt.Errorf("mark %d in text content %d of block %d: invalid mark type '%s'", markIndex, textIndex, blockIndex, mark.Type)
	}

	// Validate required attributes for specific mark types
	if mark.Type == "link" {
		if mark.Attrs == nil {
			return fmt.Errorf("mark %d in text content %d of block %d: link mark requires 'attrs'", markIndex, textIndex, blockIndex)
		}
		if mark.Attrs["href"] == nil {
			return fmt.Errorf("mark %d in text content %d of block %d: link mark requires 'href' attribute", markIndex, textIndex, blockIndex)
		}
	}

	return nil
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

// ValidateCommentPayload validates a comment payload and returns a validated version
// This is the main entry point for ADF validation
func ValidateCommentPayload(content CommentPayload) (CommentPayload, error) {
	return validateADF(content)
}

// IsADFValid checks if ADF content is valid without fallback
func IsADFValid(content CommentPayload) bool {
	_, err := validateADF(content)
	return err == nil
}

// SanitizeADF sanitizes ADF content by removing invalid elements
func SanitizeADF(content CommentPayload) CommentPayload {
	// Create a deep copy of the content
	sanitized := content

	// Filter out invalid content blocks and sanitize valid ones
	validContentBlocks := make([]Content, 0, len(content.Body.Content))
	for _, block := range content.Body.Content {
		if isValidContentBlock(block) {
			validContentBlocks = append(validContentBlocks, sanitizeContentBlock(block))
		}
	}

	sanitized.Body.Content = validContentBlocks
	return sanitized
}

// isValidContentBlock checks if a content block is valid
func isValidContentBlock(block Content) bool {
	if block.Type == "" {
		return false
	}

	// Check if content type is allowed
	allowedTypes := map[string]bool{
		"paragraph":   true,
		"heading":     true,
		"bulletList":  true,
		"orderedList": true,
		"codeBlock":   true,
		"blockquote":  true,
		"panel":       true,
		"table":       true,
		"layout":      true,
		"embed":       true,
		"extension":   true,
	}

	return allowedTypes[block.Type]
}

// sanitizeContentBlock sanitizes a single content block
func sanitizeContentBlock(block Content) Content {
	// Create a copy of the block
	sanitized := block

	// Filter out invalid text contents
	validContents := make([]TextContent, 0, len(block.Content))
	for _, textContent := range block.Content {
		if isValidTextContent(textContent) {
			validContents = append(validContents, sanitizeTextContent(textContent))
		}
	}

	sanitized.Content = validContents
	return sanitized
}

// sanitizeTextContent sanitizes a single text content
func sanitizeTextContent(textContent TextContent) TextContent {
	// Create a copy of the text content
	sanitized := textContent

	// Filter out invalid marks
	validMarks := make([]Mark, 0, len(textContent.Marks))
	for _, mark := range textContent.Marks {
		if isValidMark(mark) {
			validMarks = append(validMarks, sanitizeMark(mark))
		}
	}

	sanitized.Marks = validMarks
	return sanitized
}

// sanitizeMark sanitizes a single mark
func sanitizeMark(mark Mark) Mark {
	// Create a copy of the mark
	sanitized := mark

	// Sanitize attributes if present
	if sanitized.Attrs != nil {
		if sanitized.Type == "link" && sanitized.Attrs["href"] == nil {
			// Remove invalid link marks
			return Mark{}
		}
	}

	return sanitized
}

// isValidTextContent checks if text content is valid
func isValidTextContent(textContent TextContent) bool {
	if textContent.Type == "" {
		return false
	}

	if textContent.Type == "text" && textContent.Text == "" {
		return false
	}

	return true
}

// isValidMark checks if a mark is valid
func isValidMark(mark Mark) bool {
	if mark.Type == "" {
		return false
	}

	if mark.Type == "link" && mark.Attrs == nil {
		return false
	}

	if mark.Type == "link" && mark.Attrs["href"] == nil {
		return false
	}

	return true
}

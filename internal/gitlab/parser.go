package gitlab

import (
	"regexp"
	"strings"
)

// Parser extracts Jira issue IDs from text
type Parser struct {
	issueIDRegex *regexp.Regexp
}

// NewParser creates a new parser
func NewParser() *Parser {
	// Regex pattern for Jira issue IDs: 2+ letters followed by dash and numbers (case insensitive)
	pattern := `([A-Za-z]{2,}-\d+)`

	return &Parser{
		issueIDRegex: regexp.MustCompile(pattern),
	}
}

// ExtractIssueIDs extracts all Jira issue IDs from the given text
func (p *Parser) ExtractIssueIDs(text string) []string {
	if text == "" {
		return nil
	}

	matches := p.issueIDRegex.FindAllString(text, -1)

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueMatches []string

	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			uniqueMatches = append(uniqueMatches, strings.ToUpper(match))
		}
	}

	return uniqueMatches
}

// HasIssueID checks if the text contains any Jira issue IDs
func (p *Parser) HasIssueID(text string) bool {
	return p.issueIDRegex.MatchString(text)
}

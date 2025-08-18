package jira

import (
	"fmt"
	"strings"
	"time"

	userlink "github.com/atlet99/gitlab-jira-hook/internal/common"
	"github.com/atlet99/gitlab-jira-hook/internal/timeutil"
)

// DateFormatGOST is the date format according to GOST 7.64-90 standard (DD.MM.YYYY HH:MM)
const DateFormatGOST = "02.01.2006 15:04"

// actionUpdated is a common action value used across Jira ADF builders
const actionUpdated = "updated"

// String constants for action types
const (
	ActionOpen   = "open"
	ActionOpened = "opened"
	ActionClose  = "close"
	ActionClosed = "closed"
)

// GenerateCommitADFComment generates an ADF comment for a commit event
func GenerateCommitADFComment(
	commitID, commitURL, authorName, _, authorURL, message, date, branch, branchURL, projectWebURL, timezone string,
	added, modified, removed []string,
) CommentPayload {
	content := []Content{createCommitAuthor(authorName, authorURL)}
	content = append(content, createCommitHeader(commitID, commitURL)...)
	content = append(content, createCommitBranch(branch, branchURL))

	// Get commit message and MR link (if any)
	commitContent, mrContent := createCommitMessage(message, projectWebURL)
	content = append(content, commitContent)

	// Add MR link if it exists
	if len(mrContent.Content) > 0 {
		content = append(content, mrContent)
	}

	content = append(content, createCommitDate(date, timezone))

	if hasFileChanges(added, modified, removed) {
		content = append(content, createCompactFileChangesSection(added, modified, removed)...)
	}

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

func createCommitHeader(commitID, commitURL string) []Content {
	commitLink := TextContent{
		Type: "text",
		Text: commitID,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": commitURL},
		}},
	}
	return []Content{
		{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "commit: ", Marks: []Mark{{Type: "strong"}}},
				commitLink,
			},
		},
	}
}

func createCommitAuthor(authorName, authorURL string) Content {
	var authorContent TextContent
	if authorURL != "" {
		authorContent = TextContent{
			Type: "text",
			Text: authorName,
			Marks: []Mark{
				{Type: "strong"},
				{Type: "link", Attrs: map[string]interface{}{"href": authorURL}},
			},
		}
	} else {
		authorContent = TextContent{
			Type:  "text",
			Text:  authorName,
			Marks: []Mark{{Type: "strong"}},
		}
	}

	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "username: ", Marks: []Mark{{Type: "strong"}}},
			authorContent,
		},
	}
}

func createCommitBranch(branch, branchURL string) Content {
	// Debug logging to see what values we're getting
	if branch == "" {
		// Log when branch is empty - this is likely the issue
		return Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "branch: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: "unknown", Marks: []Mark{{Type: "code"}}},
			},
		}
	}

	// Extract branch name from refs/heads/branch format
	branchName := branch
	if strings.HasPrefix(branch, "refs/heads/") {
		branchName = strings.TrimPrefix(branch, "refs/heads/")
	}

	// If branchURL is empty, just show branch name without link
	if branchURL == "" {
		return Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "branch: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: branchName, Marks: []Mark{{Type: "code"}}},
			},
		}
	}

	// Create clickable link if branchURL is available
	branchLink := TextContent{
		Type: "text",
		Text: branchName,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": branchURL},
		}},
	}
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "branch: ", Marks: []Mark{{Type: "strong"}}},
			branchLink,
		},
	}
}

func createCommitMessage(message, projectWebURL string) (commitContent, mrContent Content) {
	// Check if message contains "See merge request" and extract MR link
	mrLink := extractMergeRequestLink(message, projectWebURL)
	if mrLink != "" {
		// Split message into commit message and MR reference
		parts := strings.Split(message, "See merge request")
		commitMsg := strings.TrimSpace(parts[0])

		// Create separate paragraphs for commit message and MR link
		commitContent = Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "commit: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: commitMsg},
			},
		}

		mrContent = Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "See merge request ", Marks: []Mark{{Type: "strong"}}},
				{
					Type:  "text",
					Text:  extractMRID(mrLink),
					Marks: []Mark{{Type: "link", Attrs: map[string]interface{}{"href": mrLink}}},
				},
			},
		}

		return commitContent, mrContent
	}

	// Regular commit message without MR reference
	commitContent = Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "commit: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: message},
		},
	}

	// Return empty content for MR link
	mrContent = Content{
		Type:    "paragraph",
		Content: []TextContent{},
	}

	return commitContent, mrContent
}

// extractMergeRequestLink extracts MR URL from commit message
// This function will be called with project web_url context from the handler
func extractMergeRequestLink(message, projectWebURL string) string {
	// Look for "See merge request" pattern
	if strings.Contains(message, "See merge request") {
		// Extract the MR reference (e.g., "devops/test-jira-webhook!4")
		parts := strings.Split(message, "See merge request")
		if len(parts) > 1 {
			mrRef := strings.TrimSpace(parts[1])
			// Convert MR reference to URL format
			// Format: project!number -> project_web_url/-/merge_requests/number
			if strings.Contains(mrRef, "!") {
				projectAndNumber := strings.Split(mrRef, "!")
				const expectedParts = 2
				if len(projectAndNumber) == expectedParts {
					number := projectAndNumber[1]
					if projectWebURL != "" {
						return fmt.Sprintf("%s/-/merge_requests/%s", projectWebURL, number)
					}
				}
			}
		}
	}
	return ""
}

// extractMRID extracts MR ID from MR reference
func extractMRID(mrRef string) string {
	if strings.Contains(mrRef, "!") {
		parts := strings.Split(mrRef, "!")
		const expectedParts = 2
		if len(parts) == expectedParts {
			return parts[1]
		}
	}
	return mrRef
}

func createCommitDate(date, timezone string) Content {
	// Format the date to GOST 7.64-90 (DD.MM.YYYY HH:MM)
	formattedDate := timeutil.FormatDateGOST(date, timezone)

	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "date: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: formattedDate},
		},
	}
}

// formatDateGOST parses a date string in RFC3339 and returns it in GOST 7.64-90 format: DD.MM.YYYY HH:MM
func formatDateGOST(dateStr string) string {
	parsedTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return parsedTime.Format(DateFormatGOST)
}

func hasFileChanges(added, modified, removed []string) bool {
	return len(added)+len(modified)+len(removed) > 0
}

func createCompactFileChangesSection(added, modified, removed []string) []Content {
	var content []Content

	// Create a single line with file changes summary
	var changes []string

	if len(added) > 0 {
		changes = append(changes, fmt.Sprintf("+%d", len(added)))
	}
	if len(modified) > 0 {
		changes = append(changes, fmt.Sprintf("~%d", len(modified)))
	}
	if len(removed) > 0 {
		changes = append(changes, fmt.Sprintf("-%d", len(removed)))
	}

	if len(changes) > 0 {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "files: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: strings.Join(changes, " "), Marks: []Mark{{Type: "code"}}},
			},
		})
	}

	return content
}

// Helper functions for creating common ADF elements
func createTitleLink(title, url, label string) Content {
	// If URL is empty, just show title without link
	if url == "" {
		return Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: strings.ToLower(label) + ": ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: title},
			},
		}
	}

	// Create clickable link if URL is available
	titleLink := TextContent{
		Type: "text",
		Text: title,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: strings.ToLower(label) + ": ", Marks: []Mark{{Type: "strong"}}},
			titleLink,
		},
	}
}

func createProjectLink(projectName, projectURL string) Content {
	// If projectURL is empty, just show project name without link
	if projectURL == "" {
		return Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "project: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: projectName},
			},
		}
	}

	// Create clickable link if projectURL is available
	projectLink := TextContent{
		Type: "text",
		Text: projectName,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": projectURL},
		}},
	}
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "project: ", Marks: []Mark{{Type: "strong"}}},
			projectLink,
		},
	}
}

func createField(label, value string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: strings.ToLower(label) + ": ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: value},
		},
	}
}

func createAuthorField(author string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "author: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: author, Marks: []Mark{{Type: "strong"}}},
		},
	}
}

// createAuthorFieldWithLink creates an author field with clickable link
func createAuthorFieldWithLink(author, authorURL string) Content {
	var authorContent []TextContent

	authorContent = append(authorContent, TextContent{
		Type:  "text",
		Text:  "author: ",
		Marks: []Mark{{Type: "strong"}},
	})

	if authorURL != "" && author != "" {
		// Create clickable author link
		authorContent = append(authorContent, TextContent{
			Type: "text",
			Text: author,
			Marks: []Mark{
				{Type: "link", Attrs: map[string]interface{}{"href": authorURL}},
				{Type: "strong"},
			},
		})
	} else {
		// Fallback to non-clickable author
		authorContent = append(authorContent, TextContent{
			Type:  "text",
			Text:  author,
			Marks: []Mark{{Type: "strong"}},
		})
	}

	return Content{
		Type:    "paragraph",
		Content: authorContent,
	}
}

func createDescriptionField(description string) []Content {
	return []Content{
		{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "description: ", Marks: []Mark{{Type: "strong"}}},
			},
		},
		{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: description},
			},
		},
	}
}

// CreateSimpleADF wraps a string in an ADF document (one paragraph)
func CreateSimpleADF(text string) CommentPayload {
	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{createSimpleParagraph(text)},
		},
	}
}

func createSimpleParagraph(text string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}

// GenerateMergeRequestADFComment generates an ADF comment for a merge request event
func GenerateMergeRequestADFComment(
	title, url, projectName, projectURL, action, sourceBranch, targetBranch, status, author, description, timezone string,
	participants, approvedBy, reviewers, approvers []userlink.UserWithLink,
) CommentPayload {
	return GenerateMergeRequestADFCommentWithBranchURLs(
		title, url, projectName, projectURL, action,
		sourceBranch, "", targetBranch, "", status, author, description, "", timezone,
		participants, approvedBy, reviewers, approvers,
	)
}

// GenerateMergeRequestADFCommentSimple generates a simple ADF comment for a merge request event with author URL
func GenerateMergeRequestADFCommentSimple(
	mrID int, mrURL, title, description, authorName, authorURL, action,
	sourceBranch, targetBranch, timezone string,
) CommentPayload {
	var content []Content

	// Start with author with clickable link
	if authorName != "" {
		content = append(content, createAuthorFieldWithLink(authorName, authorURL))
	}

	// Add MR title as header with link (if present)
	content = append(content, buildMRTitleContent(mrID, mrURL, title)...)

	// Add action with emoji (if present)
	content = append(content, buildActionContent(action, authorName)...)

	// Add branches if available
	if sourceBranch != "" && targetBranch != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "branches: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: fmt.Sprintf("%s → %s", sourceBranch, targetBranch), Marks: []Mark{{Type: "code"}}},
			},
		})
	}

	// Add description if available
	content = append(content, buildMRDescriptionContent(description)...)

	// Add current date in GOST format
	formattedTime := timeutil.FormatCurrentTimeGOST(timezone)
	content = append(content, Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "date: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: formattedTime, Marks: []Mark{{Type: "code"}}},
		},
	})

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// GenerateMergeRequestADFCommentWithBranchURLs generates an ADF comment for a merge request event with branch URLs
func GenerateMergeRequestADFCommentWithBranchURLs(
	title, url, projectName, projectURL, action, sourceBranch, sourceBranchURL,
	targetBranch, targetBranchURL, status, author, description, eventTime, timezone string,
	participants, approvedBy, reviewers, approvers []userlink.UserWithLink,
) CommentPayload {
	var content []Content

	// Start with author (like commit format)
	if author != "" {
		content = append(content, createAuthorField(author))
	}

	// Add MR title as header (like commit ID)
	content = append(content, createMergeRequestHeader(title, url)...)

	// Add branches with clickable links if URLs are available
	if sourceBranch != "" && targetBranch != "" {
		if sourceBranchURL != "" || targetBranchURL != "" {
			content = append(content, createBranchesFieldWithURLs(sourceBranch, sourceBranchURL, targetBranch, targetBranchURL))
		} else {
			content = append(content, createBranchesField(sourceBranch, targetBranch))
		}
	}

	// Add description (like commit message)
	if description != "" {
		content = append(content, createMergeRequestDescription(description))
	}

	// Add participants if available
	if len(participants) > 0 {
		content = append(content, createParticipantsFieldWithLinks(participants))
	}
	// Add approved by if available
	if len(approvedBy) > 0 {
		content = append(content, createApproversFieldWithLinks("approved by", approvedBy))
	}
	// Add reviewers if available
	if len(reviewers) > 0 {
		content = append(content, createApproversFieldWithLinks("reviewers", reviewers))
	}
	// Add approvers if available
	if len(approvers) > 0 {
		content = append(content, createApproversFieldWithLinks("approvers", approvers))
	}

	// Add project info if available
	if projectName != "" {
		content = append(content, createProjectLink(projectName, projectURL))
	}

	// Add action and status
	if action != "" {
		content = append(content, createField("action", action))
	}

	if status != "" {
		content = append(content, createField("status", status))
	}

	// Add event date in GOST format, or current date if event time is not available
	if eventTime != "" {
		content = append(content, createEventDateField(eventTime))
	} else {
		content = append(content, createCurrentDateField(timezone))
	}

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

func createMergeRequestHeader(title, url string) []Content {
	titleLink := TextContent{
		Type: "text",
		Text: title,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	return []Content{
		{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "merge request: ", Marks: []Mark{{Type: "strong"}}},
				titleLink,
			},
		},
	}
}

func createMergeRequestDescription(description string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "commit: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: description},
		},
	}
}

// buildMRTitleContent builds the title content for MR if title is present
func buildMRTitleContent(mrID int, mrURL, title string) []Content {
	if title == "" {
		return nil
	}

	titleContent := []TextContent{
		{Type: "text", Text: "merge request: ", Marks: []Mark{{Type: "strong"}}},
	}

	if mrURL != "" {
		titleContent = append(titleContent, TextContent{
			Type: "text",
			Text: fmt.Sprintf("!%d - %s", mrID, title),
			Marks: []Mark{
				{Type: "link", Attrs: map[string]interface{}{"href": mrURL}},
				{Type: "strong"},
			},
		})
	} else {
		titleContent = append(titleContent, TextContent{
			Type:  "text",
			Text:  fmt.Sprintf("!%d - %s", mrID, title),
			Marks: []Mark{{Type: "strong"}},
		})
	}

	return []Content{{
		Type:    "paragraph",
		Content: titleContent,
	}}
}

// buildActionContent builds the action paragraph with emoji if action is present
func buildActionContent(action, authorName string) []Content {
	if action == "" {
		return nil
	}

	actionEmoji, actionText := getActionEmojiAndText(action, authorName)
	return []Content{{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "action: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: fmt.Sprintf("%s %s", actionEmoji, actionText), Marks: []Mark{{Type: "code"}}},
		},
	}}
}

// buildMRDescriptionContent builds a trimmed description paragraph if present
func buildMRDescriptionContent(description string) []Content {
	if description == "" {
		return nil
	}
	const maxDescLength = 200
	if len(description) > maxDescLength {
		description = description[:maxDescLength] + "..."
	}

	return []Content{{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "description: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: description},
		},
	}}
}

// getActionEmojiAndText maps action to emoji and formatted text
func getActionEmojiAndText(action, authorName string) (emoji, text string) {
	switch action {
	case ActionOpen, ActionOpened:
		return "🔄", action
	case ActionClose, ActionClosed:
		return "❌", action
	case "merge", "merged":
		return "✅", action
	case "approved", "approve":
		if authorName != "" {
			return "👍", fmt.Sprintf("%s (approved by %s)", action, authorName)
		}
		return "👍", action
	case "unapproved", "unapprove":
		if authorName != "" {
			return "👎", fmt.Sprintf("%s (by %s)", action, authorName)
		}
		return "👎", action
	case "update", actionUpdated:
		return "📝", action
	default:
		return "🔄", action
	}
}

func createCurrentDateField(timezone string) Content {
	// Format current time to GOST 7.64-90 format with timezone
	formattedDate := timeutil.FormatCurrentTimeGOST(timezone)

	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "date: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: formattedDate},
		},
	}
}

// createEventDateField creates a date field using the event timestamp
func createEventDateField(eventTime string) Content {
	// Format the event time to GOST 7.64-90 format
	formattedDate := formatDateGOST(eventTime)

	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "date: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: formattedDate},
		},
	}
}

func createBranchesField(sourceBranch, targetBranch string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "branches: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: sourceBranch, Marks: []Mark{{Type: "code"}}},
			{Type: "text", Text: " → "},
			{Type: "text", Text: targetBranch, Marks: []Mark{{Type: "code"}}},
		},
	}
}

// createBranchesFieldWithURLs creates a branches field with clickable links
func createBranchesFieldWithURLs(sourceBranch, sourceBranchURL, targetBranch, targetBranchURL string) Content {
	var sourceContent, targetContent TextContent

	// Create source branch content
	if sourceBranchURL != "" {
		sourceContent = TextContent{
			Type: "text",
			Text: sourceBranch,
			Marks: []Mark{
				{Type: "code"},
				{Type: "link", Attrs: map[string]interface{}{"href": sourceBranchURL}},
			},
		}
	} else {
		sourceContent = TextContent{
			Type:  "text",
			Text:  sourceBranch,
			Marks: []Mark{{Type: "code"}},
		}
	}

	// Create target branch content
	if targetBranchURL != "" {
		targetContent = TextContent{
			Type: "text",
			Text: targetBranch,
			Marks: []Mark{
				{Type: "code"},
				{Type: "link", Attrs: map[string]interface{}{"href": targetBranchURL}},
			},
		}
	} else {
		targetContent = TextContent{
			Type:  "text",
			Text:  targetBranch,
			Marks: []Mark{{Type: "code"}},
		}
	}

	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "branches: ", Marks: []Mark{{Type: "strong"}}},
			sourceContent,
			{Type: "text", Text: " → "},
			targetContent,
		},
	}
}

func createParticipantsFieldWithLinks(users []userlink.UserWithLink) Content {
	var userContents []TextContent
	for i, u := range users {
		userContent := TextContent{
			Type:  "text",
			Text:  u.Name,
			Marks: []Mark{{Type: "link", Attrs: map[string]interface{}{"href": u.URL}}, {Type: "code"}},
		}
		userContents = append(userContents, userContent)
		if i < len(users)-1 {
			userContents = append(userContents, TextContent{Type: "text", Text: ", "})
		}
	}
	return Content{
		Type: "paragraph",
		Content: append(
			[]TextContent{{Type: "text", Text: "participants: ", Marks: []Mark{{Type: "strong"}}}},
			userContents...,
		),
	}
}

// createApproversFieldWithLinks creates a field for approvers/reviewers with clickable links
func createApproversFieldWithLinks(label string, users []userlink.UserWithLink) Content {
	var userContents []TextContent
	for i, u := range users {
		userContent := TextContent{
			Type:  "text",
			Text:  u.Name,
			Marks: []Mark{{Type: "link", Attrs: map[string]interface{}{"href": u.URL}}, {Type: "code"}},
		}
		userContents = append(userContents, userContent)
		if i < len(users)-1 {
			userContents = append(userContents, TextContent{Type: "text", Text: ", "})
		}
	}
	return Content{
		Type:    "paragraph",
		Content: append([]TextContent{{Type: "text", Text: label + ": ", Marks: []Mark{{Type: "strong"}}}}, userContents...),
	}
}

// GenerateIssueADFComment generates ADF comment for Issue
func GenerateIssueADFComment(
	title, url, projectName, projectURL, action, status, issueType, priority, author, description, timezone string,
) CommentPayload {
	var content []Content

	content = append(content, createTitleLink(title, url, "Issue"))

	if projectName != "" {
		content = append(content, createProjectLink(projectName, projectURL))
	}

	if action != "" {
		content = append(content, createField("Action", action))
	}

	if status != "" {
		content = append(content, createField("Status", status))
	}

	if issueType != "" {
		content = append(content, createField("Type", issueType))
	}

	if priority != "" {
		content = append(content, createField("Priority", priority))
	}

	if author != "" {
		content = append(content, createAuthorField(author))
	}

	if description != "" {
		content = append(content, createDescriptionField(description)...)
	}

	content = append(content, createCurrentDateField(timezone))

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// GeneratePipelineADFComment generates an ADF comment for a Pipeline event
func GeneratePipelineADFComment(
	ref, url, projectName, projectURL, action, status, sha, author string, duration int, timezone string,
) CommentPayload {
	content := createPipelineHeader(ref, url)
	content = append(content, createPipelineProject(projectName, projectURL)...)           // may be empty
	content = append(content, createPipelineFields(action, status, ref, sha, duration)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = append(content, createCurrentDateField(timezone))
	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// createPipelineHeader returns the header for a pipeline event
func createPipelineHeader(ref, url string) []Content {
	refLink := TextContent{
		Type: "text",
		Text: ref,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	return []Content{{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "pipeline: ", Marks: []Mark{{Type: "strong"}}},
			refLink,
		},
	}}
}

// createPipelineProject returns the project field for a pipeline event
func createPipelineProject(projectName, projectURL string) []Content {
	if projectName == "" {
		return nil
	}
	return []Content{createProjectLink(projectName, projectURL)}
}

// createPipelineFields returns all pipeline fields except author and project
func createPipelineFields(action, status, ref, sha string, duration int) []Content {
	var fields []Content
	if action != "" {
		fields = append(fields, createField("Action", action))
	}
	if status != "" {
		fields = append(fields, createField("Status", status))
	}
	if ref != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "ref: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: ref, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if sha != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "sha: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: sha, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if duration > 0 {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "duration: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: fmt.Sprintf("%ds", duration)},
			},
		})
	}
	return fields
}

// GenerateBuildADFComment generates an ADF comment for a Build/Job event
func GenerateBuildADFComment(
	name, url, projectName, projectURL, action, status, stage, ref, sha, author string, duration int, timezone string,
) CommentPayload {
	content := createBuildHeader(name, url)
	content = append(content, createBuildProject(projectName, projectURL)...)                  // may be empty
	content = append(content, createBuildFields(action, status, stage, ref, sha, duration)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = append(content, createCurrentDateField(timezone))
	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// createBuildHeader returns the header for a build/job event
func createBuildHeader(name, url string) []Content {
	nameLink := TextContent{
		Type: "text",
		Text: name,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	return []Content{{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "build: ", Marks: []Mark{{Type: "strong"}}},
			nameLink,
		},
	}}
}

// createBuildProject returns the project field for a build/job event
func createBuildProject(projectName, projectURL string) []Content {
	if projectName == "" {
		return nil
	}
	return []Content{createProjectLink(projectName, projectURL)}
}

// createBuildFields returns all build/job fields except author and project
func createBuildFields(action, status, stage, ref, sha string, duration int) []Content {
	var fields []Content
	if action != "" {
		fields = append(fields, createField("Action", action))
	}
	if status != "" {
		fields = append(fields, createField("Status", status))
	}
	if stage != "" {
		fields = append(fields, createField("Stage", stage))
	}
	if ref != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "ref: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: ref, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if sha != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "sha: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: sha, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if duration > 0 {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "duration: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: fmt.Sprintf("%ds", duration)},
			},
		})
	}
	return fields
}

// generateSimpleADFComment is a generic ADF comment generator for simple event types
func generateSimpleADFComment(
	title, url, projectName, projectURL, action, author, content, _, adfTitle string,
) CommentPayload {
	var adfContent []Content

	// Title
	titleLink := TextContent{
		Type: "text",
		Text: title,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	adfContent = append(adfContent, Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: strings.ToLower(adfTitle) + ": ", Marks: []Mark{{Type: "strong"}}},
			titleLink,
		},
	})

	// Project
	if projectName != "" {
		projectLink := TextContent{
			Type: "text",
			Text: projectName,
			Marks: []Mark{{
				Type:  "link",
				Attrs: map[string]interface{}{"href": projectURL},
			}},
		}
		adfContent = append(adfContent, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "project: ", Marks: []Mark{{Type: "strong"}}},
				projectLink,
			},
		})
	}

	// Action
	if action != "" {
		adfContent = append(adfContent, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "action: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: action},
			},
		})
	}

	// Author
	if author != "" {
		adfContent = append(adfContent, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "author: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: author, Marks: []Mark{{Type: "strong"}}},
			},
		})
	}

	// Content
	if content != "" {
		adfContent = append(adfContent,
			Content{
				Type:    "paragraph",
				Content: []TextContent{{Type: "text", Text: "content: ", Marks: []Mark{{Type: "strong"}}}},
			},
			Content{
				Type:    "paragraph",
				Content: []TextContent{{Type: "text", Text: content}},
			},
		)
	}

	adfContent = append(adfContent, createCurrentDateField(""))

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: adfContent,
		},
	}
}

// GenerateNoteADFComment generates ADF comment for Note/Comment
func GenerateNoteADFComment(title, url, projectName, projectURL, action, author, content, _ string) CommentPayload {
	return generateSimpleADFComment(title, url, projectName, projectURL, action, author, content, "note", "Comment")
}

// GenerateFeatureFlagADFComment generates ADF comment for Feature Flag
func GenerateFeatureFlagADFComment(
	name, url, projectName, projectURL, action, description, author, _ string,
) CommentPayload {
	return generateSimpleADFComment(
		name, url, projectName, projectURL, action, author, description, "feature_flag", "Feature Flag",
	)
}

// GenerateWikiPageADFComment generates ADF comment for Wiki Page
func GenerateWikiPageADFComment(title, url, projectName, projectURL, action, author, content, _ string) CommentPayload {
	return generateSimpleADFComment(title, url, projectName, projectURL, action, author, content, "wiki_page", "Wiki Page")
}

// GenerateTagPushADFComment generates ADF comment for Tag Push
func GenerateTagPushADFComment(
	ref, url, projectName, projectURL, action, author, timezone string,
) CommentPayload {
	var content []Content

	// Title: Tag Push
	refLink := TextContent{
		Type: "text",
		Text: ref,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	content = append(content, Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "tag push: ", Marks: []Mark{{Type: "strong"}}},
			refLink,
		},
	})

	// Project
	if projectName != "" {
		projectLink := TextContent{
			Type: "text",
			Text: projectName,
			Marks: []Mark{{
				Type:  "link",
				Attrs: map[string]interface{}{"href": projectURL},
			}},
		}
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "project: ", Marks: []Mark{{Type: "strong"}}},
				projectLink,
			},
		})
	}

	// Action
	if action != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "action: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: action},
			},
		})
	}

	// Tag
	if ref != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "tag: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: ref, Marks: []Mark{{Type: "code"}}},
			},
		})
	}

	// Author
	if author != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "author: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: author, Marks: []Mark{{Type: "strong"}}},
			},
		})
	}

	content = append(content, createCurrentDateField(timezone))

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// GenerateReleaseADFComment generates ADF comment for Release
func GenerateReleaseADFComment(
	name, url, projectName, projectURL, action, tag, description, author, timezone string,
) CommentPayload {
	var content []Content

	// Title: Release
	nameLink := TextContent{
		Type: "text",
		Text: name,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	content = append(content, Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "release: ", Marks: []Mark{{Type: "strong"}}},
			nameLink,
		},
	})

	// Project
	if projectName != "" {
		projectLink := TextContent{
			Type: "text",
			Text: projectName,
			Marks: []Mark{{
				Type:  "link",
				Attrs: map[string]interface{}{"href": projectURL},
			}},
		}
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "project: ", Marks: []Mark{{Type: "strong"}}},
				projectLink,
			},
		})
	}

	// Action
	if action != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "action: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: action},
			},
		})
	}

	// Tag
	if tag != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "tag: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: tag, Marks: []Mark{{Type: "code"}}},
			},
		})
	}

	// Author
	if author != "" {
		content = append(content, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "author: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: author, Marks: []Mark{{Type: "strong"}}},
			},
		})
	}

	// Description
	if description != "" {
		content = append(content,
			Content{
				Type:    "paragraph",
				Content: []TextContent{{Type: "text", Text: "description: ", Marks: []Mark{{Type: "strong"}}}},
			},
			Content{
				Type:    "paragraph",
				Content: []TextContent{{Type: "text", Text: description}},
			},
		)
	}

	content = append(content, createCurrentDateField(timezone))

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// GenerateDeploymentADFComment generates an ADF comment for a Deployment event
func GenerateDeploymentADFComment(
	ref, url, projectName, projectURL, action, environment, status, sha, author, timezone string,
) CommentPayload {
	content := createDeploymentHeader(ref, url)
	content = append(content, createDeploymentProject(projectName, projectURL)...)              // may be empty
	content = append(content, createDeploymentFields(action, environment, status, ref, sha)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = append(content, createCurrentDateField(timezone))
	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
		},
	}
}

// createDeploymentHeader returns the header for a deployment event
func createDeploymentHeader(ref, url string) []Content {
	refLink := TextContent{
		Type: "text",
		Text: ref,
		Marks: []Mark{{
			Type:  "link",
			Attrs: map[string]interface{}{"href": url},
		}},
	}
	return []Content{{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "deployment: ", Marks: []Mark{{Type: "strong"}}},
			refLink,
		},
	}}
}

// createDeploymentProject returns the project field for a deployment event
func createDeploymentProject(projectName, projectURL string) []Content {
	if projectName == "" {
		return nil
	}
	return []Content{createProjectLink(projectName, projectURL)}
}

// createDeploymentFields returns all deployment fields except author and project
func createDeploymentFields(action, environment, status, ref, sha string) []Content {
	var fields []Content
	if action != "" {
		fields = append(fields, createField("Action", action))
	}
	if environment != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "environment: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: environment, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if status != "" {
		fields = append(fields, createField("Status", status))
	}
	if ref != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "ref: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: ref, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	if sha != "" {
		fields = append(fields, Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "sha: ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: sha, Marks: []Mark{{Type: "code"}}},
			},
		})
	}
	return fields
}

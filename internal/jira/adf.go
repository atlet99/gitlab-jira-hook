package jira

import (
	"fmt"
	"strings"
	"time"
)

// DateFormatGOST is the date format according to GOST 7.64-90 standard (DD.MM.YYYY HH:MM)
const DateFormatGOST = "02.01.2006 15:04"

// GenerateCommitADFComment generates an ADF comment for a commit event
func GenerateCommitADFComment(
	commitID, commitURL, authorName, _, authorURL, message, date, branch, branchURL string,
	added, modified, removed []string,
) CommentPayload {
	content := []Content{createCommitAuthor(authorName, authorURL)}
	content = append(content, createCommitHeader(commitID, commitURL)...)
	content = append(content,
		createCommitBranch(branch, branchURL),
		createCommitMessage(message),
		createCommitDate(date),
	)

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
	if branch == "" {
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

func createCommitMessage(message string) Content {
	return Content{
		Type: "paragraph",
		Content: []TextContent{
			{Type: "text", Text: "commit: ", Marks: []Mark{{Type: "strong"}}},
			{Type: "text", Text: message},
		},
	}
}

func createCommitDate(date string) Content {
	// Format the date to GOST 7.64-90 (DD.MM.YYYY HH:MM)
	formattedDate := formatDateGOST(date)

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

// addTimestamp appends a separator and a timestamp paragraph to the end of the ADF comment
func addTimestamp(content []Content) []Content {
	timestamp := time.Now().Format("2006-01-02 15:04:05 UTC")
	return append(content,
		Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "---"},
			},
		},
		Content{
			Type: "paragraph",
			Content: []TextContent{
				{Type: "text", Text: "**Timestamp:** ", Marks: []Mark{{Type: "strong"}}},
				{Type: "text", Text: timestamp, Marks: []Mark{{Type: "code"}}},
			},
		},
	)
}

// GenerateMergeRequestADFComment generates ADF comment for Merge Request
func GenerateMergeRequestADFComment(
	title, url, projectName, projectURL, action, sourceBranch, targetBranch, status, author, description string,
) CommentPayload {
	var content []Content

	content = append(content, createTitleLink(title, url, "Merge Request"))

	if projectName != "" {
		content = append(content, createProjectLink(projectName, projectURL))
	}

	if action != "" {
		content = append(content, createField("Action", action))
	}

	if sourceBranch != "" && targetBranch != "" {
		content = append(content, createBranchesField(sourceBranch, targetBranch))
	}

	if status != "" {
		content = append(content, createField("Status", status))
	}

	if author != "" {
		content = append(content, createAuthorField(author))
	}

	if description != "" {
		content = append(content, createDescriptionField(description)...)
	}

	content = addTimestamp(content)

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: content,
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

// GenerateIssueADFComment generates ADF comment for Issue
func GenerateIssueADFComment(
	title, url, projectName, projectURL, action, status, issueType, priority, author, description string,
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

	content = addTimestamp(content)

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
	ref, url, projectName, projectURL, action, status, sha, author string, duration int,
) CommentPayload {
	content := createPipelineHeader(ref, url)
	content = append(content, createPipelineProject(projectName, projectURL)...)           // may be empty
	content = append(content, createPipelineFields(action, status, ref, sha, duration)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = addTimestamp(content)
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
	name, url, projectName, projectURL, action, status, stage, ref, sha, author string, duration int,
) CommentPayload {
	content := createBuildHeader(name, url)
	content = append(content, createBuildProject(projectName, projectURL)...)                  // may be empty
	content = append(content, createBuildFields(action, status, stage, ref, sha, duration)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = addTimestamp(content)
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

	// Add timestamp
	adfContent = addTimestamp(adfContent)

	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: adfContent,
		},
	}
}

// GenerateNoteADFComment generates ADF comment for Note/Comment
func GenerateNoteADFComment(title, url, projectName, projectURL, action, author, content string) CommentPayload {
	return generateSimpleADFComment(title, url, projectName, projectURL, action, author, content, "note", "Comment")
}

// GenerateFeatureFlagADFComment generates ADF comment for Feature Flag
func GenerateFeatureFlagADFComment(
	name, url, projectName, projectURL, action, description, author string,
) CommentPayload {
	return generateSimpleADFComment(
		name, url, projectName, projectURL, action, author, description, "feature_flag", "Feature Flag",
	)
}

// GenerateWikiPageADFComment generates ADF comment for Wiki Page
func GenerateWikiPageADFComment(title, url, projectName, projectURL, action, author, content string) CommentPayload {
	return generateSimpleADFComment(title, url, projectName, projectURL, action, author, content, "wiki_page", "Wiki Page")
}

// GenerateTagPushADFComment generates ADF comment for Tag Push
func GenerateTagPushADFComment(
	ref, url, projectName, projectURL, action, author string,
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

	// Add timestamp
	content = addTimestamp(content)

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
	name, url, projectName, projectURL, action, tag, description, author string,
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

	// Add timestamp
	content = addTimestamp(content)

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
	ref, url, projectName, projectURL, action, environment, status, sha, author string,
) CommentPayload {
	content := createDeploymentHeader(ref, url)
	content = append(content, createDeploymentProject(projectName, projectURL)...)              // may be empty
	content = append(content, createDeploymentFields(action, environment, status, ref, sha)...) // may be empty
	if author != "" {
		content = append(content, createAuthorField(author))
	}
	content = addTimestamp(content)
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

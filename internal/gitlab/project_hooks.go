// Package gitlab provides GitLab webhook handling functionality.
// This file contains Project Hook specific handlers and types.

package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// ProjectHookHandler handles GitLab Project Hook requests
type ProjectHookHandler struct {
	config *config.Config
	logger *slog.Logger
	jira   JiraClient // Use interface instead of concrete type
	parser *Parser
}

// NewProjectHookHandler creates a new GitLab Project Hook handler
func NewProjectHookHandler(cfg *config.Config, logger *slog.Logger) *ProjectHookHandler {
	return &ProjectHookHandler{
		config: cfg,
		logger: logger,
		jira:   jira.NewClient(cfg),
		parser: NewParser(),
	}
}

// HandleProjectHook handles incoming GitLab Project Hook requests
func (h *ProjectHookHandler) HandleProjectHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate GitLab secret token
	if !h.validateProjectToken(r) {
		h.logger.Warn("Invalid GitLab project secret token", "remoteAddr", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		h.logger.Warn("Failed to close request body", "error", err)
	}

	// Parse webhook event
	event, err := h.parseProjectEvent(body)
	if err != nil {
		h.logger.Error("Failed to parse project webhook event", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Event filtering by project/group
	if !h.isAllowedEvent(event) {
		h.logger.Info("Event filtered by project/group", "eventType", event.Type)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Process the event
	if err := h.processProjectEvent(event); err != nil {
		h.logger.Error("Failed to process project event", "error", err, "eventType", event.Type)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
}

// validateProjectToken validates the GitLab project secret token
func (h *ProjectHookHandler) validateProjectToken(r *http.Request) bool {
	token := r.Header.Get("X-Gitlab-Token")
	return token == h.config.GitLabSecret
}

// parseProjectEvent parses the project webhook event from JSON
func (h *ProjectHookHandler) parseProjectEvent(body []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project event: %w", err)
	}

	// Set event type based on object_kind or event_name
	if event.ObjectKind != "" {
		event.Type = event.ObjectKind
	} else if event.EventName != "" {
		event.Type = event.EventName
	}

	return &event, nil
}

// processProjectEvent processes the project webhook event
func (h *ProjectHookHandler) processProjectEvent(event *Event) error {
	h.logger.Info("Processing project webhook event", "type", event.Type)

	switch event.Type {
	case "push":
		return h.processPushEvent(event)
	case "merge_request":
		return h.processMergeRequestEvent(event)
	case "issue":
		return h.processIssueEvent(event)
	case "note":
		return h.processNoteEvent(event)
	case "wiki_page":
		return h.processWikiPageEvent(event)
	case "pipeline":
		return h.processPipelineEvent(event)
	case "build":
		return h.processBuildEvent(event)
	case "tag_push":
		return h.processTagPushEvent(event)
	case "release":
		return h.processReleaseEvent(event)
	case "deployment":
		return h.processDeploymentEvent(event)
	case "feature_flag":
		return h.processFeatureFlagEvent(event)
	default:
		h.logger.Debug("Unsupported project event type", "type", event.Type)
		return nil
	}
}

// processPushEvent processes push events
func (h *ProjectHookHandler) processPushEvent(event *Event) error {
	if len(event.Commits) == 0 {
		return nil
	}

	// Filter by branch (supports mask)
	if len(h.config.PushBranchFilter) > 0 && event.Ref != "" {
		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		matched := false
		for _, pattern := range h.config.PushBranchFilter {
			if ok, _ := filepath.Match(pattern, branch); ok {
				matched = true
				break
			}
		}
		if !matched {
			h.logger.Info("Push event ignored by branch filter", "branch", branch, "filters", h.config.PushBranchFilter)
			return nil
		}
	}

	// Extract project information
	projectName := "Unknown Project"
	if event.Project != nil {
		projectName = event.Project.Name
	}

	for _, commit := range event.Commits {
		issueIDs := h.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			// Construct branch URL if we have project information
			branchURL := ""
			if event.Project != nil {
				branchURL = fmt.Sprintf("%s/-/tree/%s", event.Project.WebURL, event.Ref)
			}

			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				commit.Author.Name,
				commit.Author.Email,
				commit.Message,
				commit.Timestamp,
				event.Ref,
				branchURL,
				commit.Added,
				commit.Modified,
				commit.Removed,
			)
			if err := h.jira.AddComment(issueID, comment); err != nil {
				h.logger.Error("Failed to add commit comment to Jira",
					"error", err,
					"issueID", issueID,
					"commitID", commit.ID,
					"projectName", projectName)
			} else {
				h.logger.Info("Added commit comment to Jira issue",
					"issueID", issueID,
					"commitID", commit.ID,
					"projectName", projectName)
			}
		}
	}

	return nil
}

// processMergeRequestEvent processes merge request events
func (h *ProjectHookHandler) processMergeRequestEvent(event *Event) error {
	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	if event.Project != nil {
		projectName = event.Project.Name
	}

	// Extract Jira issue IDs from title and description
	text := attrs.Title
	if attrs.Description != "" {
		text += " " + attrs.Description
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		comment := h.createMergeRequestComment(event)
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add merge request comment to Jira",
				"error", err,
				"issueID", issueID,
				"mrID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added merge request comment to Jira issue",
				"issueID", issueID,
				"mrID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

// createMergeRequestComment creates a comment for a merge request
func (h *ProjectHookHandler) createMergeRequestComment(event *Event) string {
	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	return fmt.Sprintf(
		"Merge Request [%s](%s)\nProject: [%s](%s)\nAction: %s\nSource: `%s` → Target: `%s`\nStatus: %s\nAuthor: %s\nDescription: %s",
		attrs.Title,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.SourceBranch,
		attrs.TargetBranch,
		attrs.State,
		attrs.Name,
		attrs.Description,
	)
}

func (h *ProjectHookHandler) processTagPushEvent(event *Event) error {
	h.logger.Info("Tag push event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Tag %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nRef: `%s`\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Ref,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Ref,
		attrs.Name)

	// Extract Jira issue IDs from tag name
	issueIDs := h.parser.ExtractIssueIDs(attrs.Ref)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add tag comment to Jira",
				"error", err,
				"issueID", issueID,
				"tagName", attrs.Ref,
				"projectName", projectName)
		} else {
			h.logger.Info("Added tag comment to Jira issue",
				"issueID", issueID,
				"tagName", attrs.Ref,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processReleaseEvent(event *Event) error {
	h.logger.Info("Release event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Release %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nTag: `%s`\nDescription: %s\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Name,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Ref,
		attrs.Description,
		attrs.Name)

	// Extract Jira issue IDs from release name and description
	text := attrs.Name
	if attrs.Description != "" {
		text += " " + attrs.Description
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add release comment to Jira",
				"error", err,
				"issueID", issueID,
				"releaseName", attrs.Name,
				"projectName", projectName)
		} else {
			h.logger.Info("Added release comment to Jira issue",
				"issueID", issueID,
				"releaseName", attrs.Name,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processDeploymentEvent(event *Event) error {
	h.logger.Info("Deployment event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Deployment %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nEnvironment: `%s`\nStatus: %s\nRef: `%s`\nSHA: `%s`\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Ref,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Environment,
		attrs.Status,
		attrs.Ref,
		attrs.Sha,
		attrs.Name)

	// Extract Jira issue IDs from deployment ref and environment
	text := attrs.Ref + " " + attrs.Environment

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add deployment comment to Jira",
				"error", err,
				"issueID", issueID,
				"deploymentID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added deployment comment to Jira issue",
				"issueID", issueID,
				"deploymentID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processFeatureFlagEvent(event *Event) error {
	h.logger.Info("Feature flag event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Feature Flag %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nDescription: %s\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Name,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Description,
		attrs.Name)

	// Extract Jira issue IDs from feature flag name and description
	text := attrs.Name
	if attrs.Description != "" {
		text += " " + attrs.Description
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add feature flag comment to Jira",
				"error", err,
				"issueID", issueID,
				"featureFlagName", attrs.Name,
				"projectName", projectName)
		} else {
			h.logger.Info("Added feature flag comment to Jira issue",
				"issueID", issueID,
				"featureFlagName", attrs.Name,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processWikiPageEvent(event *Event) error {
	h.logger.Info("Wiki page event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Wiki Page %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nAuthor: %s\nContent: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Title,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Name,
		attrs.Content[:min(len(attrs.Content), 100)]+"...")

	// Extract Jira issue IDs from wiki page title and content
	text := attrs.Title
	if attrs.Content != "" {
		text += " " + attrs.Content
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add wiki comment to Jira",
				"error", err,
				"issueID", issueID,
				"wikiPageID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added wiki comment to Jira issue",
				"issueID", issueID,
				"wikiPageID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processPipelineEvent(event *Event) error {
	h.logger.Info("Pipeline event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Pipeline %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nStatus: %s\nRef: `%s`\nSHA: `%s`\nDuration: %ds\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Ref,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Status,
		attrs.Ref,
		attrs.Sha,
		attrs.Duration,
		attrs.Name)

	// Extract Jira issue IDs from pipeline ref and title
	text := attrs.Ref
	if attrs.Title != "" {
		text += " " + attrs.Title
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add pipeline comment to Jira",
				"error", err,
				"issueID", issueID,
				"pipelineID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added pipeline comment to Jira issue",
				"issueID", issueID,
				"pipelineID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processBuildEvent(event *Event) error {
	h.logger.Info("Build event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Build %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nStatus: %s\nStage: %s\nRef: `%s`\nSHA: `%s`\nDuration: %ds\nAuthor: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Name,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Status,
		attrs.Stage,
		attrs.Ref,
		attrs.Sha,
		attrs.Duration,
		attrs.Name)

	// Extract Jira issue IDs from build name and stage
	text := attrs.Name
	if attrs.Stage != "" {
		text += " " + attrs.Stage
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add build comment to Jira",
				"error", err,
				"issueID", issueID,
				"buildID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added build comment to Jira issue",
				"issueID", issueID,
				"buildID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processNoteEvent(event *Event) error {
	h.logger.Info("Note event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Comment %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nAuthor: %s\nContent: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Title,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.Name,
		attrs.Note)

	// Extract Jira issue IDs from comment content
	issueIDs := h.parser.ExtractIssueIDs(attrs.Note)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add note comment to Jira",
				"error", err,
				"issueID", issueID,
				"noteID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added note comment to Jira issue",
				"issueID", issueID,
				"noteID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processIssueEvent(event *Event) error {
	h.logger.Info("Issue event (project hook)", "type", event.Type)

	if event.ObjectAttributes == nil {
		return nil
	}

	attrs := event.ObjectAttributes
	projectName := "Unknown Project"
	projectURL := ""
	if event.Project != nil {
		projectName = event.Project.Name
		projectURL = event.Project.WebURL
	}

	comment := fmt.Sprintf("Issue %s: [%s](%s)\nProject: [%s](%s)\nAction: %s\nStatus: %s\nType: %s\nPriority: %s\nAuthor: %s\nDescription: %s",
		cases.Title(language.English).String(attrs.Action),
		attrs.Title,
		attrs.URL,
		projectName,
		projectURL,
		cases.Title(language.English).String(attrs.Action),
		attrs.State,
		attrs.IssueType,
		attrs.Priority,
		attrs.Name,
		attrs.Description)

	// Extract Jira issue IDs from issue title and description
	text := attrs.Title
	if attrs.Description != "" {
		text += " " + attrs.Description
	}

	issueIDs := h.parser.ExtractIssueIDs(text)
	for _, issueID := range issueIDs {
		if err := h.jira.AddComment(issueID, jira.CreateSimpleADF(comment)); err != nil {
			h.logger.Error("Failed to add issue comment to Jira",
				"error", err,
				"issueID", issueID,
				"gitlabIssueID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added issue comment to Jira issue",
				"issueID", issueID,
				"gitlabIssueID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

// isAllowedEvent checks if the event is allowed by project/group filter
func (h *ProjectHookHandler) isAllowedEvent(event *Event) bool {
	if len(h.config.AllowedProjects) == 0 && len(h.config.AllowedGroups) == 0 {
		return true
	}
	if event.Project != nil && len(h.config.AllowedProjects) > 0 {
		for _, p := range h.config.AllowedProjects {
			// Check exact match
			if event.Project.PathWithNamespace == p || event.Project.Name == p {
				return true
			}
			// Check if project path starts with allowed group (for group-based filtering)
			if strings.HasPrefix(event.Project.PathWithNamespace, p+"/") {
				return true
			}
		}
	}
	if event.Group != nil && len(h.config.AllowedGroups) > 0 {
		for _, g := range h.config.AllowedGroups {
			if event.Group.FullPath == g || event.Group.Name == g {
				return true
			}
		}
	}
	return false
}

// Package gitlab provides GitLab webhook handling functionality.
// This file contains Project Hook specific handlers and types.

package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	userlink "github.com/atlet99/gitlab-jira-hook/internal/common"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// ProjectHookHandler handles GitLab Project Hook requests
type ProjectHookHandler struct {
	config     *config.Config
	logger     *slog.Logger
	jira       JiraClient // Use interface instead of concrete type
	parser     *Parser
	monitor    *monitoring.WebhookMonitor
	workerPool webhook.WorkerPoolInterface
}

// NewProjectHookHandler creates a new GitLab Project Hook handler
func NewProjectHookHandler(cfg *config.Config, logger *slog.Logger) *ProjectHookHandler {
	return &ProjectHookHandler{
		config:  cfg,
		logger:  logger,
		jira:    jira.NewClient(cfg),
		parser:  NewParser(),
		monitor: nil, // Will be set by server
	}
}

// SetMonitor sets the webhook monitor for metrics recording
func (h *ProjectHookHandler) SetMonitor(monitor *monitoring.WebhookMonitor) {
	h.monitor = monitor
}

// SetWorkerPool sets the worker pool for async processing
func (h *ProjectHookHandler) SetWorkerPool(workerPool webhook.WorkerPoolInterface) {
	h.workerPool = workerPool
}

// HandleProjectHook handles incoming GitLab Project Hook requests
func (h *ProjectHookHandler) HandleProjectHook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := false

	defer func() {
		// Record metrics if monitor is available
		if h.monitor != nil {
			h.monitor.RecordRequest("/gitlab-project-hook", success, time.Since(start))
		}
	}()

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
	if err := h.processProjectEvent(r.Context(), event); err != nil {
		h.logger.Error("Failed to process project event", "error", err, "eventType", event.Type)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
		return
	}

	success = true
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
func (h *ProjectHookHandler) processProjectEvent(ctx context.Context, event *Event) error {
	h.logger.Info("Processing project webhook event", "type", event.Type)

	switch event.Type {
	case "push":
		return h.processPushEvent(ctx, event)
	case "merge_request":
		return h.processMergeRequestEvent(ctx, event)
	case "issue":
		return h.processIssueEvent(ctx, event)
	case "note":
		return h.processNoteEvent(ctx, event)
	case "wiki_page":
		return h.processWikiPageEvent(ctx, event)
	case "pipeline":
		return h.processPipelineEvent(ctx, event)
	case "build":
		return h.processBuildEvent(ctx, event)
	case "tag_push":
		return h.processTagPushEvent(ctx, event)
	case "release":
		return h.processReleaseEvent(ctx, event)
	case "deployment":
		return h.processDeploymentEvent(ctx, event)
	case "feature_flag":
		return h.processFeatureFlagEvent(ctx, event)
	default:
		h.logger.Debug("Unsupported project event type", "type", event.Type)
		return nil
	}
}

// processPushEvent processes push events
func (h *ProjectHookHandler) processPushEvent(ctx context.Context, event *Event) error {
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

	// Create URL builder for generating proper URLs
	urlBuilder := NewURLBuilder(h.config)

	for _, commit := range event.Commits {
		issueIDs := h.parser.ExtractIssueIDs(commit.Message)
		for _, issueID := range issueIDs {
			// Extract branch name from refs/heads/branch format
			branchName := event.Ref
			if strings.HasPrefix(event.Ref, "refs/heads/") {
				branchName = strings.TrimPrefix(event.Ref, "refs/heads/")
			} else if strings.HasPrefix(event.Ref, "refs/tags/") {
				branchName = strings.TrimPrefix(event.Ref, "refs/tags/")
			}

			// Use URLBuilder for proper URL construction
			branchURL := urlBuilder.ConstructBranchURL(event, event.Ref)
			authorURL := urlBuilder.ConstructAuthorURLFromEmail(commit.Author.Email)

			// Get project web URL for MR links
			projectWebURL := ""
			if event.Project != nil {
				projectWebURL = event.Project.WebURL
			}

			comment := jira.GenerateCommitADFComment(
				commit.ID,
				commit.URL,
				commit.Author.Name,
				commit.Author.Email,
				authorURL,
				commit.Message,
				commit.Timestamp,
				branchName, // Use extracted branch name instead of event.Ref
				branchURL,
				projectWebURL,
				h.config.Timezone,
				commit.Added,
				commit.Modified,
				commit.Removed,
			)
			if err := h.jira.AddComment(ctx, issueID, comment); err != nil {
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
func (h *ProjectHookHandler) processMergeRequestEvent(ctx context.Context, event *Event) error {
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

	// Get issue-keys from all relevant fields
	var allTexts []string
	allTexts = append(allTexts, attrs.Title)
	allTexts = append(allTexts, attrs.Description)
	allTexts = append(allTexts, attrs.SourceBranch)
	allTexts = append(allTexts, attrs.TargetBranch)
	// TODO: if there are comments, add them here

	issueKeySet := make(map[string]struct{})
	for _, text := range allTexts {
		for _, key := range h.parser.ExtractIssueIDs(text) {
			if key != "" {
				issueKeySet[key] = struct{}{}
			}
		}
	}

	// If no issue keys are found, do nothing
	if len(issueKeySet) == 0 {
		return nil
	}

	// Extract participants and approvers from MR
	var participants, approvedBy, reviewers, approvers []userlink.UserWithLink
	if event.MergeRequest != nil {
		if event.MergeRequest.Author != nil {
			name := event.MergeRequest.Author.Username
			if name == "" {
				name = event.MergeRequest.Author.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: ""})
		}
		if event.MergeRequest.Assignee != nil {
			name := event.MergeRequest.Assignee.Username
			if name == "" {
				name = event.MergeRequest.Assignee.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: ""})
		}
		for _, participant := range event.MergeRequest.Participants {
			name := participant.Username
			if name == "" {
				name = participant.Name
			}
			participants = append(participants, userlink.UserWithLink{Name: name, URL: ""})
		}
		for _, user := range event.MergeRequest.ApprovedBy {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			approvedBy = append(approvedBy, userlink.UserWithLink{Name: name, URL: ""})
		}
		for _, user := range event.MergeRequest.Reviewers {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			reviewers = append(reviewers, userlink.UserWithLink{Name: name, URL: ""})
		}
		for _, user := range event.MergeRequest.Approvers {
			name := user.Username
			if name == "" {
				name = user.Name
			}
			approvers = append(approvers, userlink.UserWithLink{Name: name, URL: ""})
		}
	}

	// Generate ADF comment for MR
	comment := jira.GenerateMergeRequestADFComment(
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
		h.config.Timezone,
		participants,
		approvedBy,
		reviewers,
		approvers,
	)

	// Add comment to each issue
	for issueID := range issueKeySet {
		if err := h.jira.AddComment(ctx, issueID, comment); err != nil {
			h.logger.Error("Failed to add MR comment to Jira",
				"error", err,
				"issueID", issueID,
				"mrID", attrs.ID,
				"projectName", projectName)
		} else {
			h.logger.Info("Added MR comment to Jira issue",
				"issueID", issueID,
				"mrID", attrs.ID,
				"projectName", projectName)
		}
	}

	return nil
}

func (h *ProjectHookHandler) processTagPushEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processReleaseEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processDeploymentEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processFeatureFlagEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processWikiPageEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processPipelineEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processBuildEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processNoteEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

func (h *ProjectHookHandler) processIssueEvent(ctx context.Context, event *Event) error {
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
		if err := h.jira.AddComment(ctx, issueID, jira.CreateSimpleADF(comment)); err != nil {
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

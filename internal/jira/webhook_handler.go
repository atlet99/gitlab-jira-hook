// Package jira provides Jira webhook handling and GitLab synchronization.
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// Context key types for safe context usage
type contextKey string

const (
	jiraUserIDKey    contextKey = "jira_user_id"
	jiraClientKeyKey contextKey = "jira_client_key"
)

// GitLabAPIClient interface for GitLab API operations
type GitLabAPIClient interface {
	CreateIssue(ctx context.Context, projectID string, request *GitLabIssueCreateRequest) (*GitLabIssue, error)
	UpdateIssue(ctx context.Context, projectID string, issueIID int,
		request *GitLabIssueUpdateRequest) (*GitLabIssue, error)
	GetIssue(ctx context.Context, projectID string, issueIID int) (*GitLabIssue, error)
	CreateComment(ctx context.Context, projectID string, issueIID int,
		request *GitLabCommentCreateRequest) (*GitLabComment, error)
	SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*GitLabIssue, error)
	FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error)
	TestConnection(ctx context.Context) error
}

// Manager interface for bidirectional synchronization
type Manager interface {
	SyncJiraToGitLab(ctx context.Context, event *WebhookEvent) error
}

// GitLabIssueCreateRequest represents a request to create a GitLab issue
type GitLabIssueCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  int      `json:"assignee_id,omitempty"`
	MilestoneID int      `json:"milestone_id,omitempty"`
}

// GitLabIssueUpdateRequest represents a request to update a GitLab issue
type GitLabIssueUpdateRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
	StateEvent  string   `json:"state_event,omitempty"`
}

// GitLabIssue represents a GitLab issue response
type GitLabIssue struct {
	ID          int         `json:"id"`
	IID         int         `json:"iid"`
	ProjectID   int         `json:"project_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	State       string      `json:"state"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ClosedAt    *time.Time  `json:"closed_at"`
	Labels      []string    `json:"labels"`
	Author      *GitLabUser `json:"author"`
	Assignee    *GitLabUser `json:"assignee"`
	WebURL      string      `json:"web_url"`
}

// GitLabUser represents a GitLab user
type GitLabUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// GitLabCommentCreateRequest represents a request to create a comment
type GitLabCommentCreateRequest struct {
	Body string `json:"body"`
}

// GitLabComment represents a GitLab comment/note
type GitLabComment struct {
	ID        int         `json:"id"`
	Body      string      `json:"body"`
	Author    *GitLabUser `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	System    bool        `json:"system"`
	WebURL    string      `json:"web_url"`
}

// WebhookHandler handles incoming Jira webhook requests
type WebhookHandler struct {
	config        *config.Config
	logger        *slog.Logger
	gitlabClient  GitLabAPIClient
	monitor       *monitoring.WebhookMonitor
	workerPool    webhook.WorkerPoolInterface
	eventCache    map[string]*WebhookEvent // Cache for passing events to async processing
	authValidator *AuthValidator
	syncManager   Manager // Bidirectional sync manager
}

// NewWebhookHandler creates a new Jira webhook handler
func NewWebhookHandler(cfg *config.Config, logger *slog.Logger) *WebhookHandler {
	// Initialize auth validator with HMAC secret and development mode
	authValidator := NewAuthValidator(logger, cfg.JiraWebhookSecret, cfg.DebugMode)

	return &WebhookHandler{
		config:        cfg,
		logger:        logger,
		eventCache:    make(map[string]*WebhookEvent),
		authValidator: authValidator,
	}
}

// SetGitLabClient sets the GitLab API client
func (h *WebhookHandler) SetGitLabClient(client GitLabAPIClient) {
	h.gitlabClient = client
}

// SetMonitor sets the webhook monitor for metrics recording
func (h *WebhookHandler) SetMonitor(monitor *monitoring.WebhookMonitor) {
	h.monitor = monitor
}

// SetManager sets the bidirectional sync manager
func (h *WebhookHandler) SetManager(syncManager Manager) {
	h.syncManager = syncManager
}

// SetWorkerPool sets the worker pool for async processing
func (h *WebhookHandler) SetWorkerPool(workerPool webhook.WorkerPoolInterface) {
	h.workerPool = workerPool
}

// ConfigureJWTValidation configures JWT validation for Connect apps
func (h *WebhookHandler) ConfigureJWTValidation(expectedAudience string, allowedIssuers []string) {
	if h.authValidator != nil {
		h.authValidator = h.authValidator.WithJWTValidator(expectedAudience, allowedIssuers)
	}
}

// HandleWebhook handles incoming Jira webhook requests
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	success := false

	defer func() {
		// Record metrics if monitor is available
		if h.monitor != nil {
			h.monitor.RecordRequest("/jira-webhook", success, time.Since(start))
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if closeErr := r.Body.Close(); closeErr != nil {
		h.logger.Warn("Failed to close request body", "error", closeErr)
	}

	// Validate authentication (HMAC signature or JWT)
	authResult := h.authValidator.ValidateRequest(r.Context(), r, body)
	h.authValidator.LogAuthenticationAttempt(r, authResult)

	if !authResult.Valid {
		authError := h.authValidator.CreateAuthError(authResult, "webhook processing")
		h.logger.Error("Authentication failed",
			"error", authError.Error(),
			"auth_type", authResult.AuthType,
			"remote_addr", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Add authentication context for processing
	authContext := h.authValidator.GetUserContext(authResult)
	if authResult.UserID != "" {
		r = r.WithContext(context.WithValue(r.Context(), jiraUserIDKey, authResult.UserID))
	}
	if authResult.IssuerClientKey != "" {
		r = r.WithContext(context.WithValue(r.Context(), jiraClientKeyKey, authResult.IssuerClientKey))
	}

	h.logger.Info("Authentication successful",
		"auth_type", authResult.AuthType,
		"auth_context", authContext)

	// Parse webhook event
	event, err := h.parseWebhookEvent(body)
	if err != nil {
		h.logger.Error("Failed to parse Jira webhook event", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Log webhook event for debugging
	if h.config.DebugMode {
		h.logWebhookEvent(r, event)
	}

	// Convert to interface event for async processing
	interfaceEvent := h.convertToInterfaceEvent(event)

	// Submit job for async processing
	if h.workerPool == nil {
		h.logger.Error("Worker pool not initialized")
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := h.workerPool.SubmitJob(interfaceEvent, h); err != nil {
		h.logger.Error("Failed to submit job to worker pool", "error", err, "eventType", event.WebhookEvent)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Jira webhook job submitted for async processing", "eventType", event.WebhookEvent)

	// Return success immediately - processing will happen asynchronously
	w.WriteHeader(http.StatusAccepted)
	if _, err := w.Write([]byte(`{"status":"accepted","message":"jira webhook queued for processing"}`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
		return
	}

	success = true
}

// parseWebhookEvent parses the Jira webhook event from JSON
func (h *WebhookHandler) parseWebhookEvent(body []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Jira webhook event: %w", err)
	}

	return &event, nil
}

// logWebhookEvent logs webhook event for debugging
func (h *WebhookHandler) logWebhookEvent(r *http.Request, event *WebhookEvent) {
	h.logger.Debug("Received Jira webhook",
		"eventType", event.WebhookEvent,
		"timestamp", event.Timestamp,
		"userAgent", r.Header.Get("User-Agent"),
		"remoteAddr", r.RemoteAddr,
		"issueKey", getIssueKey(event),
		"projectKey", getProjectKey(event))
}

// convertToInterfaceEvent converts Jira webhook event to interface event
func (h *WebhookHandler) convertToInterfaceEvent(event *WebhookEvent) *webhook.Event {
	// Create a unique key for this event
	eventKey := fmt.Sprintf("%s-%d", event.WebhookEvent, event.Timestamp)

	// Store event in cache
	h.eventCache[eventKey] = event

	return &webhook.Event{
		Type:      event.WebhookEvent,
		EventName: event.WebhookEvent,
		// Use User field to pass the event key
		User: &webhook.User{
			ID:       int(event.Timestamp),
			Username: eventKey,
			Name:     event.WebhookEvent,
			Email:    "",
		},
	}
}

// ProcessEventAsync processes the Jira webhook event asynchronously
//
//nolint:gocyclo // This function coordinates multiple processing steps
func (h *WebhookHandler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing
	}

	// Validate event data
	if err := h.validateEventData(event); err != nil {
		return err
	}

	// Retrieve Jira event from cache
	jiraEvent, eventKey, err := h.retrieveFromCache(event.User.Username)
	if err != nil {
		return err
	}
	defer delete(h.eventCache, eventKey)

	h.logger.Info("Processing Jira webhook event",
		"eventType", jiraEvent.WebhookEvent,
		"issueKey", getIssueKey(jiraEvent),
		"projectKey", getProjectKey(jiraEvent))

	// Process the event by type
	if err := h.processEventByType(ctx, jiraEvent); err != nil {
		return err
	}

	// Perform bidirectional sync
	h.performBidirectionalSync(ctx, jiraEvent)

	return nil
}

// validateEventData validates the incoming event data
func (h *WebhookHandler) validateEventData(event *webhook.Event) error {
	if event == nil || event.User == nil {
		return fmt.Errorf("invalid event data")
	}
	return nil
}

// retrieveFromCache retrieves the Jira event from cache
func (h *WebhookHandler) retrieveFromCache(eventKey string) (*WebhookEvent, string, error) {
	jiraEvent, exists := h.eventCache[eventKey]
	if !exists {
		return nil, "", fmt.Errorf("jira event not found in cache: %s", eventKey)
	}
	return jiraEvent, eventKey, nil
}

// processEventByType processes the event based on its type
func (h *WebhookHandler) processEventByType(ctx context.Context, jiraEvent *WebhookEvent) error {
	switch jiraEvent.WebhookEvent {
	case "jira:issue_created":
		return h.processIssueCreated(ctx, jiraEvent)
	case "jira:issue_updated":
		return h.processIssueUpdated(ctx, jiraEvent)
	case "jira:issue_deleted":
		return h.processIssueDeleted(ctx, jiraEvent)
	case "comment_created":
		return h.processCommentCreated(ctx, jiraEvent)
	case "comment_updated":
		return h.processCommentUpdated(ctx, jiraEvent)
	case "comment_deleted":
		return h.processCommentDeleted(ctx, jiraEvent)
	case "worklog_created":
		return h.processWorklogCreated(ctx, jiraEvent)
	case "worklog_updated":
		return h.processWorklogUpdated(ctx, jiraEvent)
	case "worklog_deleted":
		return h.processWorklogDeleted(ctx, jiraEvent)
	default:
		h.logger.Debug("Unsupported Jira webhook event type", "type", jiraEvent.WebhookEvent)
		return nil
	}
}

// performBidirectionalSync performs bidirectional sync if available
func (h *WebhookHandler) performBidirectionalSync(ctx context.Context, jiraEvent *WebhookEvent) {
	if h.syncManager == nil {
		return
	}

	if syncErr := h.syncManager.SyncJiraToGitLab(ctx, jiraEvent); syncErr != nil {
		// Log sync error but don't fail the main processing
		h.logger.Warn("Failed to sync Jira event to GitLab",
			"event_type", jiraEvent.WebhookEvent,
			"issue_key", getIssueKey(jiraEvent),
			"error", syncErr)
	}
}

// processIssueCreated handles Jira issue creation events
func (h *WebhookHandler) processIssueCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue created",
		"issueKey", issue.Key,
		"summary", issue.Fields.Summary,
		"projectKey", issue.Fields.Project.Key)

	// Find the corresponding GitLab project
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return nil
	}

	// Create GitLab issue
	createRequest := &GitLabIssueCreateRequest{
		Title:       fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary),
		Description: h.buildIssueDescription(issue),
		Labels:      h.buildLabels(issue),
	}

	// Set assignee if available
	if assigneeID := h.findGitLabUser(ctx, issue.Fields.Assignee); assigneeID > 0 {
		createRequest.AssigneeID = assigneeID
	}

	gitlabIssue, err := h.gitlabClient.CreateIssue(ctx, gitlabProjectID, createRequest)
	if err != nil {
		return fmt.Errorf("failed to create GitLab issue: %w", err)
	}

	h.logger.Info("Created GitLab issue",
		"jiraIssueKey", issue.Key,
		"gitlabIssueIID", gitlabIssue.IID,
		"gitlabProjectID", gitlabProjectID,
		"url", gitlabIssue.WebURL)

	return nil
}

// processIssueUpdated handles Jira issue update events
func (h *WebhookHandler) processIssueUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue updated",
		"issueKey", issue.Key,
		"summary", issue.Fields.Summary)

	// Find the corresponding GitLab project and issue
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return nil
	}

	// Search for existing GitLab issue by title pattern
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, existingIssue := range existingIssues {
		if strings.Contains(existingIssue.Title, fmt.Sprintf("[%s]", issue.Key)) {
			gitlabIssue = existingIssue
			break
		}
	}

	if gitlabIssue == nil {
		h.logger.Debug("No corresponding GitLab issue found", "jiraIssueKey", issue.Key)
		return nil
	}

	// Update GitLab issue
	updateRequest := &GitLabIssueUpdateRequest{
		Title:  fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary),
		Labels: h.buildLabels(issue),
	}

	// Update state based on Jira status
	if issue.Fields.Status != nil {
		if h.isClosedStatus(issue.Fields.Status.Name) {
			updateRequest.StateEvent = "close"
		} else if gitlabIssue.State == "closed" {
			updateRequest.StateEvent = "reopen"
		}
	}

	// Set assignee if available
	if assigneeID := h.findGitLabUser(ctx, issue.Fields.Assignee); assigneeID > 0 {
		updateRequest.AssigneeID = &assigneeID
	}

	updatedIssue, err := h.gitlabClient.UpdateIssue(ctx, gitlabProjectID, gitlabIssue.IID, updateRequest)
	if err != nil {
		return fmt.Errorf("failed to update GitLab issue: %w", err)
	}

	h.logger.Info("Updated GitLab issue",
		"jiraIssueKey", issue.Key,
		"gitlabIssueIID", updatedIssue.IID,
		"url", updatedIssue.WebURL)

	return nil
}

// processIssueDeleted handles Jira issue deletion events
func (h *WebhookHandler) processIssueDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil {
		return fmt.Errorf("issue data is nil")
	}

	issue := event.Issue
	h.logger.Info("Processing Jira issue deleted", "issueKey", issue.Key)

	// Find the corresponding GitLab project and issue
	gitlabProjectID := h.findGitLabProject(issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		h.logger.Debug("No GitLab project mapping found", "jiraProjectKey", issue.Fields.Project.Key)
		return nil
	}

	// Search for existing GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	for _, existingIssue := range existingIssues {
		if !strings.Contains(existingIssue.Title, fmt.Sprintf("[%s]", issue.Key)) {
			continue
		}

		// Add a comment indicating the Jira issue was deleted
		deletionMessage := fmt.Sprintf("🗑️ **Jira Issue Deleted**\n\n"+
			"The corresponding Jira issue [%s] has been deleted.\n\n"+
			"*This GitLab issue remains for reference.*", issue.Key)
		comment := &GitLabCommentCreateRequest{
			Body: deletionMessage,
		}

		_, err := h.gitlabClient.CreateComment(ctx, gitlabProjectID, existingIssue.IID, comment)
		if err != nil {
			h.logger.Error("Failed to add deletion comment", "error", err)
		}

		h.logger.Info("Added deletion comment to GitLab issue",
			"jiraIssueKey", issue.Key,
			"gitlabIssueIID", existingIssue.IID)
		break
	}

	return nil
}

// processCommentCreated handles Jira comment creation events
func (h *WebhookHandler) processCommentCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment created",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID,
		"author", event.Comment.Author.DisplayName)

	return h.syncCommentToGitLab(ctx, event, "created")
}

// processCommentUpdated handles Jira comment update events
func (h *WebhookHandler) processCommentUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment updated",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID,
		"author", event.Comment.Author.DisplayName)

	return h.syncCommentToGitLab(ctx, event, "updated")
}

// processCommentDeleted handles Jira comment deletion events
func (h *WebhookHandler) processCommentDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Comment == nil {
		return fmt.Errorf("issue or comment data is nil")
	}

	h.logger.Info("Processing Jira comment deleted",
		"issueKey", event.Issue.Key,
		"commentID", event.Comment.ID)

	return h.syncCommentToGitLab(ctx, event, "deleted")
}

// processWorklogCreated handles Jira worklog creation events
func (h *WebhookHandler) processWorklogCreated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog created",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID,
		"timeSpent", event.Worklog.TimeSpent)

	return h.syncWorklogToGitLab(ctx, event, "created")
}

// processWorklogUpdated handles Jira worklog update events
func (h *WebhookHandler) processWorklogUpdated(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog updated",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID,
		"timeSpent", event.Worklog.TimeSpent)

	return h.syncWorklogToGitLab(ctx, event, "updated")
}

// processWorklogDeleted handles Jira worklog deletion events
func (h *WebhookHandler) processWorklogDeleted(ctx context.Context, event *WebhookEvent) error {
	if event.Issue == nil || event.Worklog == nil {
		return fmt.Errorf("issue or worklog data is nil")
	}

	h.logger.Info("Processing Jira worklog deleted",
		"issueKey", event.Issue.Key,
		"worklogID", event.Worklog.ID)

	return h.syncWorklogToGitLab(ctx, event, "deleted")
}

// Helper functions

// findGitLabProject finds the GitLab project ID for a Jira project key
func (h *WebhookHandler) findGitLabProject(jiraProjectKey string) string {
	// Check for explicit mapping first
	if gitlabProject, exists := h.config.ProjectMappings[jiraProjectKey]; exists {
		h.logger.Debug("Using explicit project mapping",
			"jiraProject", jiraProjectKey,
			"gitlabProject", gitlabProject)
		return gitlabProject
	}

	// Fallback to namespace-based convention
	defaultProject := fmt.Sprintf("%s/%s", h.config.GitLabNamespace, strings.ToLower(jiraProjectKey))

	h.logger.Debug("Using default project mapping",
		"jiraProject", jiraProjectKey,
		"gitlabProject", defaultProject,
		"namespace", h.config.GitLabNamespace)

	return defaultProject
}

// findGitLabUser finds the GitLab user ID for a Jira user
func (h *WebhookHandler) findGitLabUser(ctx context.Context, jiraUser *JiraUser) int {
	if jiraUser == nil || jiraUser.EmailAddress == "" {
		return 0
	}

	// Try to find GitLab user by email
	gitlabUser, err := h.gitlabClient.FindUserByEmail(ctx, jiraUser.EmailAddress)
	if err != nil {
		h.logger.Warn("Failed to find GitLab user by email",
			"email", jiraUser.EmailAddress,
			"error", err)
		return 0
	}

	if gitlabUser == nil {
		h.logger.Debug("No GitLab user found for email", "email", jiraUser.EmailAddress)
		return 0
	}

	h.logger.Info("Found GitLab user mapping",
		"jiraEmail", jiraUser.EmailAddress,
		"gitlabUser", gitlabUser.Username,
		"gitlabID", gitlabUser.ID)

	return gitlabUser.ID
}

// buildIssueDescription builds GitLab issue description from Jira issue
func (h *WebhookHandler) buildIssueDescription(issue *JiraIssue) string {
	var description strings.Builder

	description.WriteString(fmt.Sprintf("**Jira Issue**: [%s](%s)\n\n", issue.Key, issue.Self))

	if issue.Fields.Description != nil {
		// Handle both string and ADF descriptions
		switch desc := issue.Fields.Description.(type) {
		case string:
			if desc != "" {
				description.WriteString("**Description**:\n")
				description.WriteString(desc)
				description.WriteString("\n\n")
			}
		default:
			// For ADF content, we'd need to convert it to markdown
			// For now, just indicate it's rich content
			description.WriteString("**Description**: *Rich content from Jira*\n\n")
		}
	}

	if issue.Fields.Priority != nil {
		description.WriteString(fmt.Sprintf("**Priority**: %s\n", issue.Fields.Priority.Name))
	}

	if issue.Fields.IssueType != nil {
		description.WriteString(fmt.Sprintf("**Issue Type**: %s\n", issue.Fields.IssueType.Name))
	}

	if issue.Fields.Reporter != nil {
		description.WriteString(fmt.Sprintf("**Reporter**: %s\n", issue.Fields.Reporter.DisplayName))
	}

	description.WriteString("\n---\n*Synchronized from Jira*")

	return description.String()
}

// buildLabels builds GitLab labels from Jira issue
func (h *WebhookHandler) buildLabels(issue *JiraIssue) []string {
	var labels []string

	// Add Jira sync label
	labels = append(labels, "jira-sync")

	// Add issue type
	if issue.Fields.IssueType != nil {
		labels = append(labels, fmt.Sprintf("jira-type:%s", strings.ToLower(issue.Fields.IssueType.Name)))
	}

	// Add priority
	if issue.Fields.Priority != nil {
		labels = append(labels, fmt.Sprintf("jira-priority:%s", strings.ToLower(issue.Fields.Priority.Name)))
	}

	// Add status
	if issue.Fields.Status != nil {
		labels = append(labels, fmt.Sprintf("jira-status:%s", strings.ToLower(issue.Fields.Status.Name)))
	}

	// Add existing labels
	labels = append(labels, issue.Fields.Labels...)

	return labels
}

// isClosedStatus checks if a Jira status indicates a closed state
func (h *WebhookHandler) isClosedStatus(statusName string) bool {
	closedStatuses := []string{"closed", "done", "resolved", "fixed", "completed", "canceled"}
	statusLower := strings.ToLower(statusName)

	for _, closed := range closedStatuses {
		if statusLower == closed {
			return true
		}
	}

	return false
}

// syncCommentToGitLab syncs a Jira comment to GitLab
func (h *WebhookHandler) syncCommentToGitLab(ctx context.Context, event *WebhookEvent, action string) error {
	// Find the corresponding GitLab issue
	gitlabProjectID := h.findGitLabProject(event.Issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		return nil
	}

	// Search for GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", event.Issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, issue := range existingIssues {
		if strings.Contains(issue.Title, fmt.Sprintf("[%s]", event.Issue.Key)) {
			gitlabIssue = issue
			break
		}
	}

	if gitlabIssue == nil {
		return nil
	}

	// Build comment content
	var commentBody string
	switch action {
	case "created":
		commentBody = fmt.Sprintf("💬 **Jira Comment Added** by %s\n\n%s\n\n---\n*From Jira at %s*",
			event.Comment.Author.DisplayName,
			h.extractCommentText(event.Comment.Body),
			event.Comment.Created)
	case "updated":
		commentBody = fmt.Sprintf("✏️ **Jira Comment Updated** by %s\n\n%s\n\n---\n*Updated in Jira at %s*",
			event.Comment.UpdateAuthor.DisplayName,
			h.extractCommentText(event.Comment.Body),
			event.Comment.Updated)
	case "deleted":
		commentBody = fmt.Sprintf("🗑️ **Jira Comment Deleted**\n\n"+
			"A comment by %s was deleted from Jira.\n\n---\n*Deleted from Jira*",
			event.Comment.Author.DisplayName)
	}

	// Create comment in GitLab
	comment := &GitLabCommentCreateRequest{Body: commentBody}
	_, err = h.gitlabClient.CreateComment(ctx, gitlabProjectID, gitlabIssue.IID, comment)
	if err != nil {
		return fmt.Errorf("failed to create GitLab comment: %w", err)
	}

	h.logger.Info("Synced Jira comment to GitLab",
		"action", action,
		"jiraIssueKey", event.Issue.Key,
		"gitlabIssueIID", gitlabIssue.IID)

	return nil
}

// syncWorklogToGitLab syncs a Jira worklog to GitLab
func (h *WebhookHandler) syncWorklogToGitLab(ctx context.Context, event *WebhookEvent, action string) error {
	// Find the corresponding GitLab issue
	gitlabProjectID := h.findGitLabProject(event.Issue.Fields.Project.Key)
	if gitlabProjectID == "" {
		return nil
	}

	// Search for GitLab issue
	existingIssues, err := h.gitlabClient.SearchIssuesByTitle(ctx, gitlabProjectID, fmt.Sprintf("[%s]", event.Issue.Key))
	if err != nil {
		return fmt.Errorf("failed to search GitLab issues: %w", err)
	}

	var gitlabIssue *GitLabIssue
	for _, issue := range existingIssues {
		if strings.Contains(issue.Title, fmt.Sprintf("[%s]", event.Issue.Key)) {
			gitlabIssue = issue
			break
		}
	}

	if gitlabIssue == nil {
		return nil
	}

	// Build worklog comment content
	var commentBody string
	switch action {
	case "created":
		commentBody = fmt.Sprintf("⏱️ **Time Logged** by %s\n\n"+
			"**Time Spent**: %s\n**Date**: %s\n\n%s\n\n---\n*Logged in Jira*",
			event.Worklog.Author.DisplayName,
			event.Worklog.TimeSpent,
			event.Worklog.Started,
			h.extractCommentText(event.Worklog.Comment))
	case "updated":
		commentBody = fmt.Sprintf("⏱️ **Time Log Updated** by %s\n\n"+
			"**Time Spent**: %s\n**Date**: %s\n\n%s\n\n---\n*Updated in Jira*",
			event.Worklog.UpdateAuthor.DisplayName,
			event.Worklog.TimeSpent,
			event.Worklog.Started,
			h.extractCommentText(event.Worklog.Comment))
	case "deleted":
		commentBody = fmt.Sprintf("🗑️ **Time Log Deleted**\n\n"+
			"A time log entry by %s (%s) was deleted from Jira.\n\n---\n*Deleted from Jira*",
			event.Worklog.Author.DisplayName,
			event.Worklog.TimeSpent)
	}

	// Create comment in GitLab
	comment := &GitLabCommentCreateRequest{Body: commentBody}
	_, err = h.gitlabClient.CreateComment(ctx, gitlabProjectID, gitlabIssue.IID, comment)
	if err != nil {
		return fmt.Errorf("failed to create GitLab comment: %w", err)
	}

	h.logger.Info("Synced Jira worklog to GitLab",
		"action", action,
		"jiraIssueKey", event.Issue.Key,
		"gitlabIssueIID", gitlabIssue.IID,
		"timeSpent", event.Worklog.TimeSpent)

	return nil
}

// extractCommentText extracts text from comment body (handles both string and ADF)
func (h *WebhookHandler) extractCommentText(body interface{}) string {
	if body == nil {
		return ""
	}

	switch content := body.(type) {
	case string:
		return content
	default:
		// For ADF content, we'd need to convert it to markdown
		// For now, just indicate it's rich content
		return "*Rich content from Jira*"
	}
}

// Utility functions for extracting data from events

func getIssueKey(event *WebhookEvent) string {
	if event.Issue != nil {
		return event.Issue.Key
	}
	return ""
}

func getProjectKey(event *WebhookEvent) string {
	if event.Issue != nil && event.Issue.Fields != nil && event.Issue.Fields.Project != nil {
		return event.Issue.Fields.Project.Key
	}
	return ""
}

// Package sync provides bidirectional synchronization between Jira and GitLab
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

const (
	// defaultMaxAuditEvents is the default maximum number of audit events to keep in memory
	defaultMaxAuditEvents = 10000
)

// GitLabAPI defines the interface for GitLab API operations needed for sync
type GitLabAPI interface {
	CreateIssue(ctx context.Context, project string, issue GitLabIssueRequest) (*GitLabIssue, error)
	UpdateIssue(ctx context.Context, project string, issueID int, issue GitLabIssueRequest) (*GitLabIssue, error)
	GetIssue(ctx context.Context, project string, issueID int) (*GitLabIssue, error)
	CreateComment(ctx context.Context, project string, issueID int, body string) error
	SearchIssuesByTitle(ctx context.Context, project, title string) ([]GitLabIssue, error)
	FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error)
	AddLabel(ctx context.Context, project string, issueID int, label string) error
	SetAssignee(ctx context.Context, project string, issueID int, assigneeID int) error
}

// JiraAPI defines the interface for Jira API operations needed for sync
type JiraAPI interface {
	UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error
	GetIssue(ctx context.Context, issueKey string) (*jira.JiraIssue, error)
	SetAssignee(ctx context.Context, issueKey string, accountID string) error
}

// GitLabIssueRequest represents a request to create/update a GitLab issue
type GitLabIssueRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
}

// GitLabIssue represents a GitLab issue (re-export from gitlab package)
type GitLabIssue = gitlab.GitLabIssue

// GitLabUser represents a GitLab user
type GitLabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// Manager manages bidirectional synchronization between Jira and GitLab
type Manager struct {
	config           *config.Config
	gitlabAPI        GitLabAPI
	jiraAPI          JiraAPI
	conflictDetector *ConflictDetector
	conflictResolver *ConflictResolver
	auditTrail       *AuditTrail
	logger           *slog.Logger
}

// NewManager creates a new sync manager with conflict resolution and audit trail
func NewManager(cfg *config.Config, gitlabAPI GitLabAPI, jiraAPI JiraAPI, logger *slog.Logger) *Manager {
	// Create audit trail (in-memory for Phase 1)
	auditTrail := NewAuditTrail(logger, defaultMaxAuditEvents)

	// Create conflict detector
	conflictDetector := NewConflictDetector(logger)

	// Create conflict resolver
	conflictResolver := NewConflictResolver(cfg, gitlabAPI, jiraAPI, auditTrail, logger)

	return &Manager{
		config:           cfg,
		gitlabAPI:        gitlabAPI,
		jiraAPI:          jiraAPI,
		conflictDetector: conflictDetector,
		conflictResolver: conflictResolver,
		auditTrail:       auditTrail,
		logger:           logger,
	}
}

// GetAuditTrail returns the audit trail for external access
func (sm *Manager) GetAuditTrail() *AuditTrail {
	return sm.auditTrail
}

// GetConflictResolver returns the conflict resolver for external access
func (sm *Manager) GetConflictResolver() *ConflictResolver {
	return sm.conflictResolver
}

// SyncJiraToGitLab processes a Jira webhook event and syncs it to GitLab
func (sm *Manager) SyncJiraToGitLab(ctx context.Context, event *jira.WebhookEvent) error {
	// Check if bidirectional sync is enabled
	if !sm.config.BidirectionalEnabled {
		sm.logger.Debug("Bidirectional sync is disabled, skipping")
		return nil
	}

	// Check if we should process this event type
	if !sm.shouldProcessEvent(event.WebhookEvent) {
		sm.logger.Debug("Event type not configured for sync",
			"event_type", event.WebhookEvent)
		return nil
	}

	// Check event age if configured
	if sm.config.SkipOldEvents && sm.isEventTooOld(event) {
		sm.logger.Debug("Event is too old, skipping",
			"event_type", event.WebhookEvent,
			"max_age_hours", sm.config.MaxEventAge)
		return nil
	}

	sm.logger.Info("Processing Jira event for GitLab sync",
		"event_type", event.WebhookEvent,
		"jira_issue_key", event.Issue.Key)

	switch event.WebhookEvent {
	case "jira:issue_created":
		return sm.handleIssueCreated(ctx, event)
	case "jira:issue_updated":
		return sm.handleIssueUpdated(ctx, event)
	case "comment_created":
		return sm.handleCommentCreated(ctx, event)
	case "comment_updated":
		return sm.handleCommentUpdated(ctx, event)
	default:
		sm.logger.Debug("Unsupported event type for sync",
			"event_type", event.WebhookEvent)
		return nil
	}
}

// shouldProcessEvent checks if the event type should be processed
func (sm *Manager) shouldProcessEvent(eventType string) bool {
	if len(sm.config.BidirectionalEvents) == 0 {
		// If no events configured, process all by default
		return true
	}

	// Normalize event type (remove jira: prefix if present)
	normalizedEvent := strings.TrimPrefix(eventType, "jira:")

	for _, configuredEvent := range sm.config.BidirectionalEvents {
		if configuredEvent == normalizedEvent {
			return true
		}
	}
	return false
}

// isEventTooOld checks if the event is older than the configured maximum age
func (sm *Manager) isEventTooOld(event *jira.WebhookEvent) bool {
	if event.Issue.Fields.Created == "" {
		return false // Can't determine age, process it
	}

	created, err := time.Parse(time.RFC3339, event.Issue.Fields.Created)
	if err != nil {
		sm.logger.Warn("Failed to parse event creation time", "error", err)
		return false // Can't determine age, process it
	}

	maxAge := time.Duration(sm.config.MaxEventAge) * time.Hour
	return time.Since(created) > maxAge
}

// handleIssueCreated creates a new GitLab issue from a Jira issue
func (sm *Manager) handleIssueCreated(ctx context.Context, event *jira.WebhookEvent) error {
	project, err := sm.determineGitLabProject(event.Issue.Key)
	if err != nil {
		return fmt.Errorf("failed to determine GitLab project: %w", err)
	}

	// Check if issue already exists in GitLab
	issueTitle := sm.formatGitLabIssueTitle(event.Issue)
	existingIssues, err := sm.gitlabAPI.SearchIssuesByTitle(ctx, project, issueTitle)
	if err != nil {
		sm.logger.Warn("Failed to search for existing GitLab issues", "error", err)
	} else if len(existingIssues) > 0 {
		sm.logger.Info("GitLab issue already exists, skipping creation",
			"gitlab_issue_id", existingIssues[0].ID,
			"jira_issue_key", event.Issue.Key)
		return nil
	}

	// Find GitLab assignee if configured
	var assigneeID *int
	if sm.config.AssigneeSyncEnabled && event.Issue.Fields.Assignee != nil {
		if gitlabUser, userErr := sm.findGitLabUser(ctx, event.Issue.Fields.Assignee.EmailAddress); userErr == nil {
			assigneeID = &gitlabUser.ID
		}
	}

	// Prepare labels
	labels := sm.prepareLabels(event.Issue)

	// Create GitLab issue
	issueReq := GitLabIssueRequest{
		Title:       issueTitle,
		Description: sm.formatGitLabIssueDescription(event.Issue),
		Labels:      labels,
		AssigneeID:  assigneeID,
	}

	gitlabIssue, err := sm.gitlabAPI.CreateIssue(ctx, project, issueReq)
	if err != nil {
		return fmt.Errorf("failed to create GitLab issue: %w", err)
	}

	sm.logger.Info("Successfully created GitLab issue from Jira",
		"jira_issue_key", event.Issue.Key,
		"gitlab_issue_id", gitlabIssue.ID,
		"gitlab_project", project)

	return nil
}

// handleIssueUpdated updates an existing GitLab issue from a Jira issue update
func (sm *Manager) handleIssueUpdated(ctx context.Context, event *jira.WebhookEvent) error {
	startTime := time.Now()

	// Setup audit event and project
	auditEvent, project, err := sm.setupIssueUpdateAudit(event, startTime)
	if err != nil {
		sm.recordFailedAudit(auditEvent, err, startTime)
		return err
	}

	// Find existing GitLab issue
	gitlabIssue, err := sm.findExistingGitLabIssue(ctx, auditEvent, project, event, startTime)
	if err != nil {
		return err // Error handling done in function
	}

	// Handle conflict resolution
	conflict, err := sm.handleConflictResolution(ctx, auditEvent, event, gitlabIssue, project, startTime)
	if err != nil {
		return err // Error handling done in function
	}

	// Perform issue updates
	updateErrors := sm.performIssueUpdates(ctx, project, gitlabIssue, event)

	// Finalize audit event
	sm.finalizeIssueUpdateAudit(ctx, auditEvent, project, gitlabIssue, updateErrors, conflict, event, startTime)

	return nil
}

// setupIssueUpdateAudit creates audit event and determines GitLab project
func (sm *Manager) setupIssueUpdateAudit(event *jira.WebhookEvent, startTime time.Time) (*AuditEvent, string, error) {
	auditEvent := &AuditEvent{
		EventType:    AuditEventSync,
		Timestamp:    startTime,
		JiraIssueKey: event.Issue.Key,
		SourceSystem: "jira",
		TargetSystem: "gitlab",
		Operation:    "update",
		Details: map[string]interface{}{
			"webhook_event": event.WebhookEvent,
			"issue_key":     event.Issue.Key,
		},
	}

	project, err := sm.determineGitLabProject(event.Issue.Key)
	if err != nil {
		return auditEvent, "", fmt.Errorf("failed to determine GitLab project: %w", err)
	}

	auditEvent.GitLabProjectID = project
	return auditEvent, project, nil
}

// recordFailedAudit records a failed audit event
func (sm *Manager) recordFailedAudit(auditEvent *AuditEvent, err error, startTime time.Time) {
	auditEvent.Success = false
	auditEvent.Error = err.Error()
	auditEvent.Duration = time.Since(startTime)
	sm.auditTrail.RecordEvent(auditEvent)
}

// findExistingGitLabIssue finds corresponding GitLab issue or creates new one
func (sm *Manager) findExistingGitLabIssue(
	ctx context.Context,
	auditEvent *AuditEvent,
	project string,
	event *jira.WebhookEvent,
	startTime time.Time,
) (*GitLabIssue, error) {
	issueTitle := sm.formatGitLabIssueTitle(event.Issue)
	existingIssues, err := sm.gitlabAPI.SearchIssuesByTitle(ctx, project, issueTitle)
	if err != nil {
		sm.recordFailedAudit(auditEvent, fmt.Errorf("failed to search for GitLab issues: %w", err), startTime)
		return nil, fmt.Errorf("failed to search for GitLab issues: %w", err)
	}

	if len(existingIssues) == 0 {
		sm.logger.Info("No corresponding GitLab issue found, creating new one",
			"jira_issue_key", event.Issue.Key)
		auditEvent.Operation = "create"
		auditEvent.Duration = time.Since(startTime)
		sm.auditTrail.RecordEvent(auditEvent)
		return nil, sm.handleIssueCreated(ctx, event)
	}

	gitlabIssue := existingIssues[0]
	auditEvent.GitLabIssueID = &gitlabIssue.ID

	// Record before state
	auditEvent.BeforeState = map[string]interface{}{
		"gitlab_title":       gitlabIssue.Title,
		"gitlab_description": gitlabIssue.Description,
		"gitlab_state":       gitlabIssue.State,
	}

	return &gitlabIssue, nil
}

// handleConflictResolution detects and resolves conflicts
func (sm *Manager) handleConflictResolution(
	ctx context.Context,
	auditEvent *AuditEvent,
	event *jira.WebhookEvent,
	gitlabIssue *GitLabIssue,
	project string,
	startTime time.Time,
) (*Conflict, error) {
	conflict, err := sm.conflictDetector.DetectConflicts(ctx, event.Issue, gitlabIssue)
	if err != nil {
		sm.logger.Warn("Failed to detect conflicts", "error", err)
		return nil, nil
	}

	if conflict == nil {
		return nil, nil
	}

	// Record conflict detection
	conflictAuditEvent := &AuditEvent{
		EventType:       AuditEventConflictDetection,
		Timestamp:       time.Now(),
		JiraIssueKey:    event.Issue.Key,
		GitLabProjectID: project,
		GitLabIssueID:   &gitlabIssue.ID,
		Success:         true,
		ConflictID:      conflict.ID,
		Details: map[string]interface{}{
			"conflict_type":      conflict.Type,
			"conflicting_fields": len(conflict.ConflictingFields),
		},
	}
	sm.auditTrail.RecordEvent(conflictAuditEvent)

	// Resolve conflict using configured strategy
	if resolveErr := sm.conflictResolver.ResolveConflict(ctx, conflict); resolveErr != nil {
		sm.logger.Error("Failed to resolve conflict",
			"conflict_id", conflict.ID,
			"error", resolveErr)

		auditEvent.Success = false
		auditEvent.Error = fmt.Sprintf("conflict resolution failed: %v", resolveErr)
		auditEvent.ConflictID = conflict.ID
		auditEvent.Duration = time.Since(startTime)
		sm.auditTrail.RecordEvent(auditEvent)
		return nil, resolveErr
	}

	// If conflict was resolved, continue with sync
	auditEvent.ConflictID = conflict.ID
	auditEvent.Details["conflict_resolved"] = true
	return conflict, nil
}

// performIssueUpdates performs status and assignee updates
func (sm *Manager) performIssueUpdates(
	ctx context.Context,
	project string,
	gitlabIssue *GitLabIssue,
	event *jira.WebhookEvent,
) []string {
	var updateErrors []string

	// Update status label if configured
	if sm.config.StatusMappingEnabled {
		if err := sm.updateStatusLabel(ctx, project, gitlabIssue.ID, event.Issue.Fields.Status.Name); err != nil {
			sm.logger.Warn("Failed to update status label", "error", err)
			updateErrors = append(updateErrors, fmt.Sprintf("status update: %v", err))
		}
	}

	// Update assignee if configured
	if sm.config.AssigneeSyncEnabled {
		if err := sm.updateAssignee(ctx, project, gitlabIssue.ID, event.Issue.Fields.Assignee); err != nil {
			sm.logger.Warn("Failed to update assignee", "error", err)
			updateErrors = append(updateErrors, fmt.Sprintf("assignee update: %v", err))
		}
	}

	return updateErrors
}

// finalizeIssueUpdateAudit completes the audit event and logs the result
func (sm *Manager) finalizeIssueUpdateAudit(
	ctx context.Context,
	auditEvent *AuditEvent,
	project string,
	gitlabIssue *GitLabIssue,
	updateErrors []string,
	conflict *Conflict,
	event *jira.WebhookEvent,
	startTime time.Time,
) {
	// Record after state
	updatedIssue, err := sm.gitlabAPI.GetIssue(ctx, project, gitlabIssue.ID)
	if err != nil {
		sm.logger.Warn("Failed to get updated GitLab issue for audit trail",
			"project", project,
			"issue_id", gitlabIssue.ID,
			"error", err)
	} else if updatedIssue != nil {
		auditEvent.AfterState = map[string]interface{}{
			"gitlab_title":       updatedIssue.Title,
			"gitlab_description": updatedIssue.Description,
			"gitlab_state":       updatedIssue.State,
		}
	}

	// Complete audit event
	auditEvent.Duration = time.Since(startTime)
	if len(updateErrors) > 0 {
		auditEvent.Success = false
		auditEvent.Error = fmt.Sprintf("partial update failures: %v", updateErrors)
	} else {
		auditEvent.Success = true
	}

	sm.auditTrail.RecordEvent(auditEvent)

	sm.logger.Info("Successfully updated GitLab issue from Jira",
		"jira_issue_key", event.Issue.Key,
		"gitlab_issue_id", gitlabIssue.ID,
		"gitlab_project", project,
		"had_conflict", conflict != nil,
		"update_errors", len(updateErrors))
}

// handleCommentCreated creates a GitLab comment from a Jira comment
func (sm *Manager) handleCommentCreated(ctx context.Context, event *jira.WebhookEvent) error {
	if !sm.config.CommentSyncEnabled {
		return nil
	}

	if sm.config.CommentSyncDirection != "jira_to_gitlab" && sm.config.CommentSyncDirection != "bidirectional" {
		return nil
	}

	project, err := sm.determineGitLabProject(event.Issue.Key)
	if err != nil {
		return fmt.Errorf("failed to determine GitLab project: %w", err)
	}

	// Find corresponding GitLab issue
	issueTitle := sm.formatGitLabIssueTitle(event.Issue)
	existingIssues, err := sm.gitlabAPI.SearchIssuesByTitle(ctx, project, issueTitle)
	if err != nil {
		return fmt.Errorf("failed to search for GitLab issues: %w", err)
	}

	if len(existingIssues) == 0 {
		sm.logger.Debug("No corresponding GitLab issue found for comment sync",
			"jira_issue_key", event.Issue.Key)
		return nil
	}

	gitlabIssue := existingIssues[0]

	// Format comment using template
	commentBodyStr, ok := event.Comment.Body.(string)
	if !ok {
		return fmt.Errorf("comment body is not a string")
	}
	commentBody := sm.formatCommentForGitLab(commentBodyStr)

	if err := sm.gitlabAPI.CreateComment(ctx, project, gitlabIssue.ID, commentBody); err != nil {
		return fmt.Errorf("failed to create GitLab comment: %w", err)
	}

	sm.logger.Info("Successfully created GitLab comment from Jira",
		"jira_issue_key", event.Issue.Key,
		"gitlab_issue_id", gitlabIssue.ID,
		"comment_id", event.Comment.ID)

	return nil
}

// handleCommentUpdated handles Jira comment updates
func (sm *Manager) handleCommentUpdated(ctx context.Context, event *jira.WebhookEvent) error {
	// For simplicity, we treat comment updates as new comments
	// In a more sophisticated implementation, you might want to track comment IDs
	return sm.handleCommentCreated(ctx, event)
}

// Helper methods

// determineGitLabProject determines the GitLab project for a Jira issue
func (sm *Manager) determineGitLabProject(jiraIssueKey string) (string, error) {
	// Extract project key from issue key (e.g., "PROJ-123" -> "PROJ")
	parts := strings.Split(jiraIssueKey, "-")
	const keyValueSeparatorParts = 2
	if len(parts) < keyValueSeparatorParts {
		return "", fmt.Errorf("invalid Jira issue key format: %s", jiraIssueKey)
	}

	jiraProjectKey := parts[0]

	// Check project mappings first
	if gitlabProject, exists := sm.config.ProjectMappings[jiraProjectKey]; exists {
		return gitlabProject, nil
	}

	// Fall back to namespace + project key
	if sm.config.GitLabNamespace != "" {
		return fmt.Sprintf("%s/%s", sm.config.GitLabNamespace, strings.ToLower(jiraProjectKey)), nil
	}

	return "", fmt.Errorf("no GitLab project mapping found for Jira project: %s", jiraProjectKey)
}

// formatGitLabIssueTitle formats a GitLab issue title from a Jira issue
func (sm *Manager) formatGitLabIssueTitle(issue *jira.JiraIssue) string {
	return fmt.Sprintf("[%s] %s", issue.Key, issue.Fields.Summary)
}

// formatGitLabIssueDescription formats a GitLab issue description from a Jira issue
func (sm *Manager) formatGitLabIssueDescription(issue *jira.JiraIssue) string {
	desc := fmt.Sprintf("**Jira Issue:** [%s](%s)\n\n", issue.Key, issue.Self)

	if issue.Fields.Description != "" {
		desc += fmt.Sprintf("**Description:**\n%s\n\n", issue.Fields.Description)
	}

	desc += fmt.Sprintf("**Issue Type:** %s\n", issue.Fields.IssueType.Name)
	desc += fmt.Sprintf("**Priority:** %s\n", issue.Fields.Priority.Name)
	desc += fmt.Sprintf("**Status:** %s\n", issue.Fields.Status.Name)

	if issue.Fields.Reporter != nil {
		desc += fmt.Sprintf("**Reporter:** %s\n", issue.Fields.Reporter.DisplayName)
	}

	if issue.Fields.Assignee != nil {
		desc += fmt.Sprintf("**Assignee:** %s\n", issue.Fields.Assignee.DisplayName)
	}

	desc += "\n---\n*This issue was automatically created from Jira*"

	return desc
}

// prepareLabels prepares GitLab labels for a Jira issue
func (sm *Manager) prepareLabels(issue *jira.JiraIssue) []string {
	labels := []string{"jira-sync"}

	// Add issue type as label
	if issue.Fields.IssueType != nil {
		labels = append(labels, fmt.Sprintf("type:%s", strings.ToLower(issue.Fields.IssueType.Name)))
	}

	// Add priority as label
	if issue.Fields.Priority != nil {
		labels = append(labels, fmt.Sprintf("priority:%s", strings.ToLower(issue.Fields.Priority.Name)))
	}

	// Add status label if mapping is enabled
	if sm.config.StatusMappingEnabled && issue.Fields.Status != nil {
		if statusLabel, exists := sm.config.StatusMapping[issue.Fields.Status.Name]; exists {
			labels = append(labels, statusLabel)
		} else if sm.config.DefaultGitLabLabel != "" {
			labels = append(labels, sm.config.DefaultGitLabLabel)
		}
	}

	return labels
}

// findGitLabUser finds a GitLab user by email
func (sm *Manager) findGitLabUser(ctx context.Context, email string) (*GitLabUser, error) {
	// Check user mapping first
	if sm.config.UserMapping != nil {
		if gitlabUsername, exists := sm.config.UserMapping[email]; exists {
			// For simplicity, we'll assume the user exists
			// In a real implementation, you'd want to verify this
			return &GitLabUser{
				ID:       0, // Would need to fetch actual ID
				Username: gitlabUsername,
				Email:    email,
			}, nil
		}
	}

	// Try to find user by email using GitLab API
	return sm.gitlabAPI.FindUserByEmail(ctx, email)
}

// updateStatusLabel updates the status label on a GitLab issue
func (sm *Manager) updateStatusLabel(ctx context.Context, project string, issueID int, jiraStatus string) error {
	if statusLabel, exists := sm.config.StatusMapping[jiraStatus]; exists {
		return sm.gitlabAPI.AddLabel(ctx, project, issueID, statusLabel)
	} else if sm.config.DefaultGitLabLabel != "" {
		return sm.gitlabAPI.AddLabel(ctx, project, issueID, sm.config.DefaultGitLabLabel)
	}
	return nil
}

// updateAssignee updates the assignee on a GitLab issue
func (sm *Manager) updateAssignee(ctx context.Context, project string, issueID int,
	jiraAssignee *jira.JiraUser) error {
	if jiraAssignee == nil {
		return nil
	}

	gitlabUser, err := sm.findGitLabUser(ctx, jiraAssignee.EmailAddress)
	if err != nil {
		return err
	}

	return sm.gitlabAPI.SetAssignee(ctx, project, issueID, gitlabUser.ID)
}

// formatCommentForGitLab formats a Jira comment for GitLab using the configured template
func (sm *Manager) formatCommentForGitLab(jiraCommentBody string) string {
	template := sm.config.CommentTemplateJiraToGitLab
	return strings.ReplaceAll(template, "{content}", jiraCommentBody)
}

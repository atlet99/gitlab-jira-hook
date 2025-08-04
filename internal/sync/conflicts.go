// Package sync provides conflict detection and resolution for bidirectional sync
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// GitLabIssue is defined in bidirectional.go

// ConflictType represents the type of conflict detected
type ConflictType string

const (
	// ConflictTypeTitle indicates title field conflict
	ConflictTypeTitle ConflictType = "title"
	// ConflictTypeDescription indicates description field conflict
	ConflictTypeDescription ConflictType = "description"
	// ConflictTypeStatus indicates status field conflict
	ConflictTypeStatus ConflictType = "status"
	// ConflictTypeAssignee indicates assignee field conflict
	ConflictTypeAssignee ConflictType = "assignee"
	// ConflictTypeLabels indicates labels field conflict
	ConflictTypeLabels ConflictType = "labels"
	// ConflictTypeConcurrent indicates concurrent modification
	ConflictTypeConcurrent ConflictType = "concurrent"
)

// ConflictResolutionStrategy represents available conflict resolution strategies
type ConflictResolutionStrategy string

const (
	// StrategyLastWriteWins resolves conflicts by accepting the most recent change
	StrategyLastWriteWins ConflictResolutionStrategy = "last_write_wins"
	// StrategyMerge attempts to merge non-conflicting changes
	StrategyMerge ConflictResolutionStrategy = "merge"
	// StrategyManual queues conflicts for manual resolution
	StrategyManual ConflictResolutionStrategy = "manual"
)

// Conflict represents a detected synchronization conflict
type Conflict struct {
	ID                 string                     `json:"id"`
	Type               ConflictType               `json:"type"`
	JiraIssueKey       string                     `json:"jira_issue_key"`
	GitLabProjectID    string                     `json:"gitlab_project_id"`
	GitLabIssueID      int                        `json:"gitlab_issue_id"`
	DetectedAt         time.Time                  `json:"detected_at"`
	JiraLastModified   time.Time                  `json:"jira_last_modified"`
	GitLabLastModified time.Time                  `json:"gitlab_last_modified"`
	ConflictingFields  []ConflictingField         `json:"conflicting_fields"`
	Strategy           ConflictResolutionStrategy `json:"strategy"`
	Status             ConflictStatus             `json:"status"`
	ResolvedAt         *time.Time                 `json:"resolved_at,omitempty"`
	ResolvedBy         string                     `json:"resolved_by,omitempty"`
	Resolution         *ConflictResolution        `json:"resolution,omitempty"`
}

// ConflictingField represents a specific field conflict
type ConflictingField struct {
	Field       string      `json:"field"`
	JiraValue   interface{} `json:"jira_value"`
	GitLabValue interface{} `json:"gitlab_value"`
}

// ConflictStatus represents the status of conflict resolution
type ConflictStatus string

const (
	// ConflictStatusPending indicates conflict is waiting for resolution
	ConflictStatusPending ConflictStatus = "pending"
	// ConflictStatusResolved indicates conflict has been resolved
	ConflictStatusResolved ConflictStatus = "resolved"
	// ConflictStatusFailed indicates conflict resolution failed
	ConflictStatusFailed ConflictStatus = "failed"
	// ConflictStatusSkipped indicates conflict was skipped
	ConflictStatusSkipped ConflictStatus = "skipped"
)

// ConflictResolution represents the resolution applied to a conflict
type ConflictResolution struct {
	Strategy     ConflictResolutionStrategy `json:"strategy"`
	AppliedBy    string                     `json:"applied_by"`
	AppliedAt    time.Time                  `json:"applied_at"`
	ChosenSource string                     `json:"chosen_source"` // "jira" or "gitlab"
	MergedFields map[string]interface{}     `json:"merged_fields,omitempty"`
	Notes        string                     `json:"notes,omitempty"`
}

// ConflictDetector handles conflict detection between Jira and GitLab
type ConflictDetector struct {
	logger *slog.Logger
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector(logger *slog.Logger) *ConflictDetector {
	return &ConflictDetector{
		logger: logger,
	}
}

// DetectConflicts analyzes Jira and GitLab states to detect conflicts
func (cd *ConflictDetector) DetectConflicts(
	_ context.Context,
	jiraIssue *jira.JiraIssue,
	gitlabIssue *GitLabIssue,
) (*Conflict, error) {
	if jiraIssue == nil || gitlabIssue == nil {
		return nil, nil // No conflict if one side doesn't exist
	}

	// Parse timestamps for conflict detection
	jiraModified, err := cd.parseJiraTimestamp(jiraIssue.Fields.Updated)
	if err != nil {
		cd.logger.Warn("Failed to parse Jira timestamp", "error", err)
		return nil, nil // Can't detect conflicts without timestamps
	}

	gitlabModified := gitlabIssue.UpdatedAt

	// Check if modifications are concurrent (within conflict window)
	const conflictWindowMinutes = 5
	timeDiff := jiraModified.Sub(gitlabModified)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if timeDiff > conflictWindowMinutes*time.Minute {
		return nil, nil // No concurrent modification
	}

	// Detect field-level conflicts
	conflictingFields := cd.detectFieldConflicts(jiraIssue, gitlabIssue)
	if len(conflictingFields) == 0 {
		return nil, nil // No field conflicts detected
	}

	conflict := &Conflict{
		ID:                 fmt.Sprintf("conflict_%s_%d_%d", jiraIssue.Key, gitlabIssue.ID, time.Now().Unix()),
		Type:               ConflictTypeConcurrent,
		JiraIssueKey:       jiraIssue.Key,
		GitLabProjectID:    fmt.Sprintf("%d", gitlabIssue.ProjectID),
		GitLabIssueID:      gitlabIssue.ID,
		DetectedAt:         time.Now(),
		JiraLastModified:   jiraModified,
		GitLabLastModified: gitlabModified,
		ConflictingFields:  conflictingFields,
		Status:             ConflictStatusPending,
	}

	cd.logger.Info("Conflict detected",
		"conflict_id", conflict.ID,
		"jira_issue", jiraIssue.Key,
		"gitlab_issue", gitlabIssue.ID,
		"fields", len(conflictingFields))

	return conflict, nil
}

// detectFieldConflicts compares specific fields between Jira and GitLab issues
func (cd *ConflictDetector) detectFieldConflicts(
	jiraIssue *jira.JiraIssue,
	gitlabIssue *GitLabIssue,
) []ConflictingField {
	var conflicts []ConflictingField

	// Check title conflicts
	jiraTitle := cd.extractJiraTitle(jiraIssue)
	if jiraTitle != gitlabIssue.Title {
		conflicts = append(conflicts, ConflictingField{
			Field:       "title",
			JiraValue:   jiraTitle,
			GitLabValue: gitlabIssue.Title,
		})
	}

	// Check description conflicts
	jiraDescription := cd.extractJiraDescription(jiraIssue)
	if jiraDescription != gitlabIssue.Description {
		conflicts = append(conflicts, ConflictingField{
			Field:       "description",
			JiraValue:   jiraDescription,
			GitLabValue: gitlabIssue.Description,
		})
	}

	// Check status conflicts (if status mapping is available)
	if jiraIssue.Fields.Status != nil {
		// Note: This would require status mapping logic
		// For now, we'll log the difference but not treat as conflict
		cd.logger.Debug("Status difference detected",
			"jira_status", jiraIssue.Fields.Status.Name,
			"gitlab_state", gitlabIssue.State)
	}

	return conflicts
}

// extractJiraTitle extracts a comparable title from Jira issue
func (cd *ConflictDetector) extractJiraTitle(jiraIssue *jira.JiraIssue) string {
	if jiraIssue.Fields.Summary != "" {
		return jiraIssue.Fields.Summary
	}
	return jiraIssue.Key
}

// extractJiraDescription extracts a comparable description from Jira issue
func (cd *ConflictDetector) extractJiraDescription(jiraIssue *jira.JiraIssue) string {
	// Description can be string or ADF object, handle both
	if desc, ok := jiraIssue.Fields.Description.(string); ok {
		return desc
	}
	return ""
}

// parseJiraTimestamp parses Jira timestamp format
func (cd *ConflictDetector) parseJiraTimestamp(timestamp string) (time.Time, error) {
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Jira typically uses ISO 8601 format
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000+0000",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timestamp); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestamp)
}

// Package sync provides conflict resolution strategies for bidirectional sync
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

const (
	// Source system constants
	sourceJira   = "jira"
	sourceGitLab = "gitlab"

	// Field constants
	fieldTitle       = "title"
	fieldDescription = "description"
)

// ConflictResolver handles resolution of synchronization conflicts
type ConflictResolver struct {
	config      *config.Config
	gitlabAPI   GitLabAPI
	jiraAPI     JiraAPI
	auditTrail  *AuditTrail
	logger      *slog.Logger
	manualQueue *ManualResolutionQueue
}

// JiraAPI is defined in bidirectional.go

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(
	cfg *config.Config,
	gitlabAPI GitLabAPI,
	jiraAPI JiraAPI,
	auditTrail *AuditTrail,
	logger *slog.Logger,
) *ConflictResolver {
	return &ConflictResolver{
		config:      cfg,
		gitlabAPI:   gitlabAPI,
		jiraAPI:     jiraAPI,
		auditTrail:  auditTrail,
		logger:      logger,
		manualQueue: NewManualResolutionQueue(logger),
	}
}

// ResolveConflict resolves a conflict using the configured strategy
func (cr *ConflictResolver) ResolveConflict(ctx context.Context, conflict *Conflict) error {
	cr.logger.Info("Resolving conflict",
		"conflict_id", conflict.ID,
		"strategy", cr.config.BidirectionalConflictStrategy)

	// Record conflict resolution attempt
	auditEvent := &AuditEvent{
		EventType:     AuditEventConflictResolution,
		JiraIssueKey:  conflict.JiraIssueKey,
		GitLabIssueID: &conflict.GitLabIssueID,
		Timestamp:     time.Now(),
		Details: map[string]interface{}{
			"conflict_id": conflict.ID,
			"strategy":    cr.config.BidirectionalConflictStrategy,
			"fields":      conflict.ConflictingFields,
		},
	}

	var err error
	strategy := ConflictResolutionStrategy(cr.config.BidirectionalConflictStrategy)

	switch strategy {
	case StrategyLastWriteWins:
		err = cr.resolveLastWriteWins(ctx, conflict)
	case StrategyMerge:
		err = cr.resolveMerge(ctx, conflict)
	case StrategyManual:
		err = cr.resolveManual(ctx, conflict)
	default:
		err = fmt.Errorf("unknown conflict resolution strategy: %s", strategy)
	}

	// Update conflict status and audit trail
	if err != nil {
		conflict.Status = ConflictStatusFailed
		auditEvent.Success = false
		auditEvent.Error = err.Error()
		cr.logger.Error("Conflict resolution failed",
			"conflict_id", conflict.ID,
			"error", err)
	} else {
		conflict.Status = ConflictStatusResolved
		conflict.ResolvedAt = &auditEvent.Timestamp
		conflict.ResolvedBy = "system"
		auditEvent.Success = true
		cr.logger.Info("Conflict resolved successfully",
			"conflict_id", conflict.ID,
			"strategy", strategy)
	}

	// Record audit event
	cr.auditTrail.RecordEvent(auditEvent)

	return err
}

// resolveLastWriteWins implements last-write-wins strategy
func (cr *ConflictResolver) resolveLastWriteWins(ctx context.Context, conflict *Conflict) error {
	// Determine which system has the most recent change
	var chosenSource string
	if conflict.JiraLastModified.After(conflict.GitLabLastModified) {
		chosenSource = sourceJira
	} else {
		chosenSource = sourceGitLab
	}

	resolution := &ConflictResolution{
		Strategy:     StrategyLastWriteWins,
		AppliedBy:    "system",
		AppliedAt:    time.Now(),
		ChosenSource: chosenSource,
		Notes:        fmt.Sprintf("Applied last-write-wins strategy, chose %s changes", chosenSource),
	}

	conflict.Resolution = resolution

	if chosenSource == "jira" {
		return cr.applyJiraChangesToGitLab(ctx, conflict)
	}
	return cr.applyGitLabChangesToJira(ctx, conflict)
}

// resolveMerge implements merge strategy for non-conflicting changes
func (cr *ConflictResolver) resolveMerge(ctx context.Context, conflict *Conflict) error {
	mergedFields := make(map[string]interface{})
	var nonMergeable []string

	// Analyze each conflicting field to determine if it can be merged
	for _, field := range conflict.ConflictingFields {
		switch field.Field {
		case fieldTitle:
			// Titles typically can't be merged, apply newest
			if conflict.JiraLastModified.After(conflict.GitLabLastModified) {
				mergedFields[field.Field] = field.JiraValue
			} else {
				mergedFields[field.Field] = field.GitLabValue
			}
		case fieldDescription:
			// For descriptions, we could implement intelligent merging
			// For now, treat as non-mergeable
			nonMergeable = append(nonMergeable, field.Field)
		default:
			// Other fields default to last-write-wins within merge
			if conflict.JiraLastModified.After(conflict.GitLabLastModified) {
				mergedFields[field.Field] = field.JiraValue
			} else {
				mergedFields[field.Field] = field.GitLabValue
			}
		}
	}

	// If any fields are non-mergeable, fall back to manual resolution
	if len(nonMergeable) > 0 {
		cr.logger.Info("Some fields cannot be merged, falling back to manual resolution",
			"conflict_id", conflict.ID,
			"non_mergeable", nonMergeable)
		return cr.resolveManual(ctx, conflict)
	}

	resolution := &ConflictResolution{
		Strategy:     StrategyMerge,
		AppliedBy:    "system",
		AppliedAt:    time.Now(),
		MergedFields: mergedFields,
		Notes:        "Successfully merged non-conflicting changes",
	}

	conflict.Resolution = resolution

	// Apply merged changes to both systems
	if err := cr.applyMergedChanges(ctx, conflict, mergedFields); err != nil {
		return fmt.Errorf("failed to apply merged changes: %w", err)
	}

	return nil
}

// resolveManual queues conflict for manual resolution
func (cr *ConflictResolver) resolveManual(_ context.Context, conflict *Conflict) error {
	conflict.Status = ConflictStatusPending

	resolution := &ConflictResolution{
		Strategy:  StrategyManual,
		AppliedBy: "queued",
		AppliedAt: time.Now(),
		Notes:     "Queued for manual resolution",
	}

	conflict.Resolution = resolution

	// Add to manual resolution queue
	if err := cr.manualQueue.AddConflict(conflict); err != nil {
		return fmt.Errorf("failed to queue conflict for manual resolution: %w", err)
	}

	cr.logger.Info("Conflict queued for manual resolution",
		"conflict_id", conflict.ID,
		"queue_size", cr.manualQueue.Size())

	return nil
}

// applyJiraChangesToGitLab applies Jira changes to GitLab issue
func (cr *ConflictResolver) applyJiraChangesToGitLab(ctx context.Context, conflict *Conflict) error {
	// Prepare GitLab update request
	updateReq := GitLabIssueRequest{}

	// Apply conflicting field values from Jira
	for _, field := range conflict.ConflictingFields {
		switch field.Field {
		case fieldTitle:
			if title, ok := field.JiraValue.(string); ok {
				updateReq.Title = title
			}
		case fieldDescription:
			if desc, ok := field.JiraValue.(string); ok {
				updateReq.Description = desc
			}
		}
	}

	// Update GitLab issue
	_, err := cr.gitlabAPI.UpdateIssue(ctx, conflict.GitLabProjectID, conflict.GitLabIssueID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update GitLab issue: %w", err)
	}

	cr.logger.Info("Applied Jira changes to GitLab",
		"conflict_id", conflict.ID,
		"jira_issue", conflict.JiraIssueKey,
		"gitlab_issue", conflict.GitLabIssueID)

	return nil
}

// applyGitLabChangesToJira applies GitLab changes to Jira issue
func (cr *ConflictResolver) applyGitLabChangesToJira(ctx context.Context, conflict *Conflict) error {
	// Prepare Jira update fields
	updateFields := make(map[string]interface{})

	// Apply conflicting field values from GitLab
	for _, field := range conflict.ConflictingFields {
		switch field.Field {
		case fieldTitle:
			updateFields["summary"] = field.GitLabValue
		case fieldDescription:
			updateFields["description"] = field.GitLabValue
		}
	}

	// Update Jira issue
	err := cr.jiraAPI.UpdateIssue(ctx, conflict.JiraIssueKey, updateFields)
	if err != nil {
		return fmt.Errorf("failed to update Jira issue: %w", err)
	}

	cr.logger.Info("Applied GitLab changes to Jira",
		"conflict_id", conflict.ID,
		"jira_issue", conflict.JiraIssueKey,
		"gitlab_issue", conflict.GitLabIssueID)

	return nil
}

// applyMergedChanges applies merged field values to both systems
func (cr *ConflictResolver) applyMergedChanges(
	ctx context.Context,
	conflict *Conflict,
	mergedFields map[string]interface{},
) error {
	// Apply to GitLab
	gitlabUpdate := GitLabIssueRequest{}
	if title, ok := mergedFields[fieldTitle].(string); ok {
		gitlabUpdate.Title = title
	}
	if desc, ok := mergedFields[fieldDescription].(string); ok {
		gitlabUpdate.Description = desc
	}

	_, err := cr.gitlabAPI.UpdateIssue(ctx, conflict.GitLabProjectID, conflict.GitLabIssueID, gitlabUpdate)
	if err != nil {
		return fmt.Errorf("failed to update GitLab with merged changes: %w", err)
	}

	// Apply to Jira
	jiraUpdate := make(map[string]interface{})
	if title, ok := mergedFields[fieldTitle]; ok {
		jiraUpdate["summary"] = title
	}
	if desc, ok := mergedFields[fieldDescription]; ok {
		jiraUpdate["description"] = desc
	}

	err = cr.jiraAPI.UpdateIssue(ctx, conflict.JiraIssueKey, jiraUpdate)
	if err != nil {
		return fmt.Errorf("failed to update Jira with merged changes: %w", err)
	}

	cr.logger.Info("Applied merged changes to both systems",
		"conflict_id", conflict.ID,
		"merged_fields", len(mergedFields))

	return nil
}

// GetManualResolutionQueue returns the manual resolution queue
func (cr *ConflictResolver) GetManualResolutionQueue() *ManualResolutionQueue {
	return cr.manualQueue
}

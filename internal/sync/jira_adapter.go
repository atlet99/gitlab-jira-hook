// Package sync provides Jira API adapter for sync operations
package sync

import (
	"context"
	"fmt"

	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// JiraSyncAdapter adapts jira.Client to sync.JiraAPI interface
type JiraSyncAdapter struct {
	client *jira.Client
}

// NewJiraSyncAdapter creates a new Jira sync adapter
func NewJiraSyncAdapter(client *jira.Client) *JiraSyncAdapter {
	return &JiraSyncAdapter{
		client: client,
	}
}

// UpdateIssue updates a Jira issue with the given fields
func (a *JiraSyncAdapter) UpdateIssue(_ context.Context, _ string, _ map[string]interface{}) error {
	// Note: This is a simplified implementation
	// In a real implementation, you'd need to implement jira.Client.UpdateIssue method
	// For now, we'll return a not implemented error
	return fmt.Errorf("jira issue update not yet implemented in jira.Client")
}

// GetIssue retrieves a Jira issue by key
func (a *JiraSyncAdapter) GetIssue(_ context.Context, _ string) (*jira.JiraIssue, error) {
	// Note: This is a simplified implementation
	// In a real implementation, you'd need to implement jira.Client.GetIssue method
	// For now, we'll return a not implemented error
	return nil, fmt.Errorf("jira issue retrieval not yet implemented in jira.Client")
}

// SetAssignee updates the assignee for a Jira issue using accountId
func (a *JiraSyncAdapter) SetAssignee(ctx context.Context, issueKey string, accountId string) error {
	return a.client.SetAssignee(ctx, issueKey, accountId)
}

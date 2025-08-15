// Package sync provides bidirectional synchronization interfaces
package sync

import (
	"context"

	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// BiDirectionalSyncer defines the interface for bidirectional synchronization
type BiDirectionalSyncer interface {
	// SyncJiraToGitLab processes a Jira webhook event and syncs it to GitLab
	SyncJiraToGitLab(ctx context.Context, event *jira.WebhookEvent) error
}

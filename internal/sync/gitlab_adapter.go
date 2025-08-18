// Package sync provides GitLab API adapter for sync operations
package sync

import (
	"context"
	"fmt"

	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
)

// GitLabSyncAdapter adapts the gitlab.APIClient to the sync.GitLabAPI interface
type GitLabSyncAdapter struct {
	client *gitlab.APIClient
}

// NewGitLabSyncAdapter creates a new GitLab sync adapter
func NewGitLabSyncAdapter(client *gitlab.APIClient) *GitLabSyncAdapter {
	return &GitLabSyncAdapter{
		client: client,
	}
}

// CreateIssue creates a new GitLab issue
func (g *GitLabSyncAdapter) CreateIssue(_ context.Context, _ string, _ GitLabIssueRequest) (*GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab issue creation not yet implemented")
}

// UpdateIssue updates a GitLab issue
func (g *GitLabSyncAdapter) UpdateIssue(_ context.Context, _ string, _ int, _ GitLabIssueRequest) (*GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab issue update not yet implemented")
}

// GetIssue retrieves a GitLab issue
func (g *GitLabSyncAdapter) GetIssue(_ context.Context, _ string, _ int) (*GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab get issue not yet implemented")
}

// CreateComment creates a comment on a GitLab issue
func (g *GitLabSyncAdapter) CreateComment(_ context.Context, _ string, _ int, _ string) error {
	return fmt.Errorf("gitlab create comment not yet implemented")
}

// SearchIssuesByTitle searches for GitLab issues by title
func (g *GitLabSyncAdapter) SearchIssuesByTitle(_ context.Context, _, _ string) ([]GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab search issues not yet implemented")
}

// FindUserByEmail finds a GitLab user by email
func (g *GitLabSyncAdapter) FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
	gitlabUser, err := g.client.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if gitlabUser == nil {
		return nil, nil
	}

	// Convert gitlab.GitLabUser to sync.GitLabUser
	return &GitLabUser{
		ID:       gitlabUser.ID,
		Username: gitlabUser.Username,
		Email:    gitlabUser.Email,
		Name:     gitlabUser.Name,
	}, nil
}

// AddLabel adds a label to a GitLab issue
func (g *GitLabSyncAdapter) AddLabel(_ context.Context, _ string, _ int, _ string) error {
	return fmt.Errorf("gitlab add label not yet implemented")
}

// SetAssignee sets the assignee for a GitLab issue
func (g *GitLabSyncAdapter) SetAssignee(_ context.Context, _ string, _ int, _ int) error {
	return fmt.Errorf("gitlab set assignee not yet implemented")
}

// TestConnection tests the GitLab API connection
func (g *GitLabSyncAdapter) TestConnection(ctx context.Context) error {
	// Simple test by trying to get a user (which will validate the token)
	_, err := g.client.FindUserByEmail(ctx, "test@example.com")
	// We expect this to fail for non-existent email, but if it's an auth error, we'll catch it
	if err != nil && err.Error() != "user not found for email: test@example.com" {
		return err
	}
	return nil
}

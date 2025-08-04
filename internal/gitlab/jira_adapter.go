// Package gitlab provides an adapter to make APIClient compatible with jira.GitLabAPIClient interface.
package gitlab

import (
	"context"
	"fmt"

	"github.com/atlet99/gitlab-jira-hook/internal/jira"
)

// JiraAPIAdapter adapts gitlab.APIClient to implement jira.GitLabAPIClient interface
type JiraAPIAdapter struct {
	client *APIClient
}

// NewJiraAPIAdapter creates a new adapter for GitLab API client
func NewJiraAPIAdapter(client *APIClient) *JiraAPIAdapter {
	return &JiraAPIAdapter{
		client: client,
	}
}

// CreateIssue creates a new issue in GitLab
func (a *JiraAPIAdapter) CreateIssue(_ context.Context, _ string, _ *jira.GitLabIssueCreateRequest) (*jira.GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab issue creation not yet implemented")
}

// UpdateIssue updates an existing issue in GitLab
func (a *JiraAPIAdapter) UpdateIssue(_ context.Context, _ string, _ int, _ *jira.GitLabIssueUpdateRequest) (*jira.GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab issue update not yet implemented")
}

// GetIssue retrieves an issue from GitLab
func (a *JiraAPIAdapter) GetIssue(_ context.Context, _ string, _ int) (*jira.GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab get issue not yet implemented")
}

// CreateComment creates a comment on a GitLab issue
func (a *JiraAPIAdapter) CreateComment(_ context.Context, _ string, _ int, _ *jira.GitLabCommentCreateRequest) (*jira.GitLabComment, error) {
	return nil, fmt.Errorf("gitlab create comment not yet implemented")
}

// SearchIssuesByTitle searches for issues by title in a project
func (a *JiraAPIAdapter) SearchIssuesByTitle(_ context.Context, _, _ string) ([]*jira.GitLabIssue, error) {
	return nil, fmt.Errorf("gitlab search issues not yet implemented")
}

// FindUserByEmail finds a GitLab user by email
func (a *JiraAPIAdapter) FindUserByEmail(ctx context.Context, email string) (*jira.GitLabUser, error) {
	gitlabUser, err := a.client.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if gitlabUser == nil {
		return nil, nil
	}

	// Convert gitlab types to jira types
	return convertGitLabUser(gitlabUser), nil
}

// TestConnection tests the connection to GitLab API
func (a *JiraAPIAdapter) TestConnection(ctx context.Context) error {
	// Simple test by trying to get a user (which will validate the token)
	_, err := a.client.GetUserByEmail(ctx, "test@example.com")
	// We expect this to fail for non-existent email, but if it's an auth error, we'll catch it
	if err != nil && err.Error() != "user not found for email: test@example.com" {
		return err
	}
	return nil
}

// Helper function to convert GitLab user types
func convertGitLabUser(gitlabUser *GitLabUser) *jira.GitLabUser {
	if gitlabUser == nil {
		return nil
	}

	return &jira.GitLabUser{
		ID:        gitlabUser.ID,
		Name:      gitlabUser.Name,
		Username:  gitlabUser.Username,
		Email:     gitlabUser.Email,
		AvatarURL: gitlabUser.AvatarURL,
		WebURL:    gitlabUser.WebURL,
	}
}

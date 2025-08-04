// Package gitlab provides an adapter to make APIClient compatible with jira.GitLabAPIClient interface.
package gitlab

import (
	"context"

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
func (a *JiraAPIAdapter) CreateIssue(ctx context.Context, projectID string, request *jira.GitLabIssueCreateRequest) (*jira.GitLabIssue, error) {
	// Convert jira types to gitlab types
	gitlabRequest := &IssueCreateRequest{
		Title:       request.Title,
		Description: request.Description,
		Labels:      request.Labels,
		AssigneeID:  request.AssigneeID,
		MilestoneID: request.MilestoneID,
	}

	gitlabIssue, err := a.client.CreateIssue(ctx, projectID, gitlabRequest)
	if err != nil {
		return nil, err
	}

	// Convert gitlab types back to jira types
	return &jira.GitLabIssue{
		ID:          gitlabIssue.ID,
		IID:         gitlabIssue.IID,
		ProjectID:   gitlabIssue.ProjectID,
		Title:       gitlabIssue.Title,
		Description: gitlabIssue.Description,
		State:       gitlabIssue.State,
		CreatedAt:   gitlabIssue.CreatedAt,
		UpdatedAt:   gitlabIssue.UpdatedAt,
		ClosedAt:    gitlabIssue.ClosedAt,
		Labels:      gitlabIssue.Labels,
		Author:      convertGitLabUser(gitlabIssue.Author),
		Assignee:    convertGitLabUser(gitlabIssue.Assignee),
		WebURL:      gitlabIssue.WebURL,
	}, nil
}

// UpdateIssue updates an existing issue in GitLab
func (a *JiraAPIAdapter) UpdateIssue(ctx context.Context, projectID string, issueIID int, request *jira.GitLabIssueUpdateRequest) (*jira.GitLabIssue, error) {
	// Convert jira types to gitlab types
	gitlabRequest := &IssueUpdateRequest{
		Title:       request.Title,
		Description: request.Description,
		Labels:      request.Labels,
		AssigneeID:  request.AssigneeID,
		StateEvent:  request.StateEvent,
	}

	gitlabIssue, err := a.client.UpdateIssue(ctx, projectID, issueIID, gitlabRequest)
	if err != nil {
		return nil, err
	}

	// Convert gitlab types back to jira types
	return &jira.GitLabIssue{
		ID:          gitlabIssue.ID,
		IID:         gitlabIssue.IID,
		ProjectID:   gitlabIssue.ProjectID,
		Title:       gitlabIssue.Title,
		Description: gitlabIssue.Description,
		State:       gitlabIssue.State,
		CreatedAt:   gitlabIssue.CreatedAt,
		UpdatedAt:   gitlabIssue.UpdatedAt,
		ClosedAt:    gitlabIssue.ClosedAt,
		Labels:      gitlabIssue.Labels,
		Author:      convertGitLabUser(gitlabIssue.Author),
		Assignee:    convertGitLabUser(gitlabIssue.Assignee),
		WebURL:      gitlabIssue.WebURL,
	}, nil
}

// GetIssue retrieves an issue from GitLab
func (a *JiraAPIAdapter) GetIssue(ctx context.Context, projectID string, issueIID int) (*jira.GitLabIssue, error) {
	gitlabIssue, err := a.client.GetIssue(ctx, projectID, issueIID)
	if err != nil {
		return nil, err
	}

	if gitlabIssue == nil {
		return nil, nil
	}

	// Convert gitlab types to jira types
	return &jira.GitLabIssue{
		ID:          gitlabIssue.ID,
		IID:         gitlabIssue.IID,
		ProjectID:   gitlabIssue.ProjectID,
		Title:       gitlabIssue.Title,
		Description: gitlabIssue.Description,
		State:       gitlabIssue.State,
		CreatedAt:   gitlabIssue.CreatedAt,
		UpdatedAt:   gitlabIssue.UpdatedAt,
		ClosedAt:    gitlabIssue.ClosedAt,
		Labels:      gitlabIssue.Labels,
		Author:      convertGitLabUser(gitlabIssue.Author),
		Assignee:    convertGitLabUser(gitlabIssue.Assignee),
		WebURL:      gitlabIssue.WebURL,
	}, nil
}

// CreateComment creates a comment on a GitLab issue
func (a *JiraAPIAdapter) CreateComment(ctx context.Context, projectID string, issueIID int, request *jira.GitLabCommentCreateRequest) (*jira.GitLabComment, error) {
	// Convert jira types to gitlab types
	gitlabRequest := &CommentCreateRequest{
		Body: request.Body,
	}

	gitlabComment, err := a.client.CreateComment(ctx, projectID, issueIID, gitlabRequest)
	if err != nil {
		return nil, err
	}

	// Convert gitlab types back to jira types
	return &jira.GitLabComment{
		ID:        gitlabComment.ID,
		Body:      gitlabComment.Body,
		Author:    convertGitLabUser(gitlabComment.Author),
		CreatedAt: gitlabComment.CreatedAt,
		UpdatedAt: gitlabComment.UpdatedAt,
		System:    gitlabComment.System,
		WebURL:    gitlabComment.WebURL,
	}, nil
}

// SearchIssuesByTitle searches for issues by title in a project
func (a *JiraAPIAdapter) SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*jira.GitLabIssue, error) {
	gitlabIssues, err := a.client.SearchIssuesByTitle(ctx, projectID, title)
	if err != nil {
		return nil, err
	}

	// Convert gitlab types to jira types
	jiraIssues := make([]*jira.GitLabIssue, len(gitlabIssues))
	for i, gitlabIssue := range gitlabIssues {
		jiraIssues[i] = &jira.GitLabIssue{
			ID:          gitlabIssue.ID,
			IID:         gitlabIssue.IID,
			ProjectID:   gitlabIssue.ProjectID,
			Title:       gitlabIssue.Title,
			Description: gitlabIssue.Description,
			State:       gitlabIssue.State,
			CreatedAt:   gitlabIssue.CreatedAt,
			UpdatedAt:   gitlabIssue.UpdatedAt,
			ClosedAt:    gitlabIssue.ClosedAt,
			Labels:      gitlabIssue.Labels,
			Author:      convertGitLabUser(gitlabIssue.Author),
			Assignee:    convertGitLabUser(gitlabIssue.Assignee),
			WebURL:      gitlabIssue.WebURL,
		}
	}

	return jiraIssues, nil
}

// FindUserByEmail finds a GitLab user by email
func (a *JiraAPIAdapter) FindUserByEmail(ctx context.Context, email string) (*jira.GitLabUser, error) {
	gitlabUser, err := a.client.FindUserByEmail(ctx, email)
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
	return a.client.TestConnection(ctx)
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

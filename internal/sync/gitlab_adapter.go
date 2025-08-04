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
func (g *GitLabSyncAdapter) CreateIssue(ctx context.Context, project string, issue GitLabIssueRequest) (*GitLabIssue, error) {
	var assigneeID int
	if issue.AssigneeID != nil {
		assigneeID = *issue.AssigneeID
	}

	gitlabIssue := &gitlab.IssueCreateRequest{
		Title:       issue.Title,
		Description: issue.Description,
		Labels:      issue.Labels,
		AssigneeID:  assigneeID,
	}

	createdIssue, err := g.client.CreateIssue(ctx, project, gitlabIssue)
	if err != nil {
		return nil, err
	}

	return &GitLabIssue{
		ID:          createdIssue.ID,
		IID:         createdIssue.IID,
		Title:       createdIssue.Title,
		Description: createdIssue.Description,
		State:       createdIssue.State,
		WebURL:      createdIssue.WebURL,
	}, nil
}

// UpdateIssue updates an existing GitLab issue
func (g *GitLabSyncAdapter) UpdateIssue(ctx context.Context, project string, issueID int, issue GitLabIssueRequest) (*GitLabIssue, error) {
	var assigneeID *int
	if issue.AssigneeID != nil {
		assigneeID = issue.AssigneeID
	}

	gitlabIssue := &gitlab.IssueUpdateRequest{
		Title:       issue.Title,
		Description: issue.Description,
		Labels:      issue.Labels,
		AssigneeID:  assigneeID,
	}

	updatedIssue, err := g.client.UpdateIssue(ctx, project, issueID, gitlabIssue)
	if err != nil {
		return nil, err
	}

	return &GitLabIssue{
		ID:          updatedIssue.ID,
		IID:         updatedIssue.IID,
		Title:       updatedIssue.Title,
		Description: updatedIssue.Description,
		State:       updatedIssue.State,
		WebURL:      updatedIssue.WebURL,
	}, nil
}

// GetIssue retrieves a GitLab issue by ID
func (g *GitLabSyncAdapter) GetIssue(ctx context.Context, project string, issueID int) (*GitLabIssue, error) {
	issue, err := g.client.GetIssue(ctx, project, issueID)
	if err != nil {
		return nil, err
	}

	return &GitLabIssue{
		ID:          issue.ID,
		IID:         issue.IID,
		Title:       issue.Title,
		Description: issue.Description,
		State:       issue.State,
		WebURL:      issue.WebURL,
	}, nil
}

// CreateComment creates a comment on a GitLab issue
func (g *GitLabSyncAdapter) CreateComment(ctx context.Context, project string, issueID int, body string) error {
	request := &gitlab.CommentCreateRequest{
		Body: body,
	}

	_, err := g.client.CreateComment(ctx, project, issueID, request)
	return err
}

// SearchIssuesByTitle searches for GitLab issues by title
func (g *GitLabSyncAdapter) SearchIssuesByTitle(ctx context.Context, project, title string) ([]GitLabIssue, error) {
	issues, err := g.client.SearchIssuesByTitle(ctx, project, title)
	if err != nil {
		return nil, err
	}

	var result []GitLabIssue
	for _, issue := range issues {
		result = append(result, GitLabIssue{
			ID:          issue.ID,
			IID:         issue.IID,
			Title:       issue.Title,
			Description: issue.Description,
			State:       issue.State,
			WebURL:      issue.WebURL,
		})
	}

	return result, nil
}

// FindUserByEmail finds a GitLab user by email
func (g *GitLabSyncAdapter) FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
	user, err := g.client.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return &GitLabUser{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Name:     user.Name,
	}, nil
}

// AddLabel adds a label to a GitLab issue
func (g *GitLabSyncAdapter) AddLabel(ctx context.Context, project string, issueID int, label string) error {
	// This is a placeholder implementation
	// In a real scenario, you would need to implement label management in gitlab.APIClient
	return fmt.Errorf("AddLabel not implemented in gitlab.APIClient")
}

// SetAssignee sets the assignee of a GitLab issue
func (g *GitLabSyncAdapter) SetAssignee(ctx context.Context, project string, issueID int, assigneeID int) error {
	// This is a placeholder implementation
	// In a real scenario, you would need to implement assignee management in gitlab.APIClient
	return fmt.Errorf("SetAssignee not implemented in gitlab.APIClient")
}

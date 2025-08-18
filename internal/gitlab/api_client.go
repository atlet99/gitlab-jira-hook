package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// APIClient handles GitLab API calls
type APIClient struct {
	baseURL string
	token   string
	client  *http.Client
	logger  *slog.Logger
}

// NewAPIClient creates a new GitLab API client
func NewAPIClient(cfg *config.Config, logger *slog.Logger) *APIClient {
	return &APIClient{
		baseURL: strings.TrimSuffix(cfg.GitLabBaseURL, "/"),
		token:   cfg.GitLabAPIToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GitLabUser represents a GitLab user from API
type GitLabUser struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// GetUserByEmail fetches user info by email from GitLab API
func (c *APIClient) GetUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
	if c.token == "" {
		c.logger.Debug("GitLab token not configured, skipping API call")
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// URL encode the email parameter
	encodedEmail := url.QueryEscape(email)
	apiURL := fmt.Sprintf("%s/api/v4/users?search=%s", c.baseURL, encodedEmail)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Making GitLab API request",
		"url", apiURL,
		"email", email)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("GitLab API request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"email", email)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var users []GitLabUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find user with exact email match
	for _, user := range users {
		if strings.EqualFold(user.Email, email) {
			c.logger.Debug("Found GitLab user by email",
				"email", email,
				"username", user.Username,
				"name", user.Name)
			return &user, nil
		}
	}

	c.logger.Debug("No GitLab user found for email", "email", email)
	return nil, fmt.Errorf("user not found for email: %s", email)
}

// GetProjectInfo fetches project information from GitLab API
func (c *APIClient) GetProjectInfo(ctx context.Context, projectID int) (*GitLabProject, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d", c.baseURL, projectID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Getting project info from GitLab API",
		"project_id", projectID,
		"url", apiURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("GitLab API project request failed",
			"status_code", resp.StatusCode,
			"project_id", projectID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var project GitLabProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project response: %w", err)
	}

	c.logger.Debug("Retrieved project info",
		"project_id", projectID,
		"default_branch", project.DefaultBranch,
		"name", project.Name)

	return &project, nil
}

// GitLabProject represents project info from GitLab API
type GitLabProject struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	WebURL        string `json:"web_url"`
}

// GitLabIssue represents a GitLab issue from API
type GitLabIssue struct {
	ID          int         `json:"id"`
	IID         int         `json:"iid"`
	ProjectID   int         `json:"project_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	State       string      `json:"state"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
	ClosedAt    *string     `json:"closed_at"`
	Labels      []string    `json:"labels"`
	Author      *GitLabUser `json:"author"`
	Assignee    *GitLabUser `json:"assignee"`
	WebURL      string      `json:"web_url"`
}

// GitLabMergeRequest represents a GitLab merge request from API
type GitLabMergeRequest struct {
	ID           int         `json:"id"`
	IID          int         `json:"iid"`
	ProjectID    int         `json:"project_id"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	State        string      `json:"state"`
	SourceBranch string      `json:"source_branch"`
	TargetBranch string      `json:"target_branch"`
	Author       *GitLabUser `json:"author"`
	WebURL       string      `json:"web_url"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
}

// GetMergeRequest fetches MR details from GitLab API
func (c *APIClient) GetMergeRequest(ctx context.Context, projectID, mrIID int) (*GitLabMergeRequest, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d",
		strings.TrimSuffix(c.baseURL, "/"), projectID, mrIID)

	c.logger.Debug("Getting MR info from GitLab API",
		"project_id", projectID,
		"mr_iid", mrIID,
		"url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("User-Agent", "GitLab-Jira-Hook/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var mr GitLabMergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Retrieved MR info",
		"project_id", projectID,
		"mr_iid", mrIID,
		"source_branch", mr.SourceBranch,
		"target_branch", mr.TargetBranch,
		"title", mr.Title)

	return &mr, nil
}

// CreateIssueRequest represents the request body for creating a GitLab issue
type CreateIssueRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	Milestone   string   `json:"milestone_id,omitempty"`
	AssigneeID  int      `json:"assignee_id,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
}

// CreateIssue creates a new issue in GitLab
func (c *APIClient) CreateIssue(ctx context.Context, projectID int, request CreateIssueRequest) (*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues", c.baseURL, projectID)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Creating GitLab issue",
		"project_id", projectID,
		"title", request.Title,
		"labels", request.Labels,
		"milestone", request.Milestone)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		c.logger.Warn("GitLab API create issue request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var issue GitLabIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Created GitLab issue",
		"project_id", projectID,
		"issue_id", issue.ID,
		"issue_iid", issue.IID,
		"title", issue.Title)

	return &issue, nil
}

// UpdateIssueRequest represents the request body for updating a GitLab issue
type UpdateIssueRequest struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Labels      *[]string `json:"labels,omitempty"`
	Milestone   *string   `json:"milestone_id,omitempty"`
	AssigneeID  *int      `json:"assignee_id,omitempty"`
	StateEvent  string    `json:"state_event,omitempty"`
	DueDate     *string   `json:"due_date,omitempty"`
}

// UpdateIssue updates an existing GitLab issue
func (c *APIClient) UpdateIssue(ctx context.Context, projectID, issueIID int, request UpdateIssueRequest) (*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues/%d", c.baseURL, projectID, issueIID)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Updating GitLab issue",
		"project_id", projectID,
		"issue_iid", issueIID,
		"request", request)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("GitLab API update issue request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID,
			"issue_iid", issueIID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var issue GitLabIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Updated GitLab issue",
		"project_id", projectID,
		"issue_id", issue.ID,
		"issue_iid", issue.IID,
		"title", issue.Title)

	return &issue, nil
}

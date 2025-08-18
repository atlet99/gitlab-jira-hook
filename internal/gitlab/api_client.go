package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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

// FindUserByEmail fetches user info by email from GitLab API
func (c *APIClient) FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
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
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ClosedAt    *time.Time  `json:"closed_at"`
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

// GitLabIssueCreateRequest represents the request body for creating a GitLab issue
type GitLabIssueCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  int      `json:"assignee_id,omitempty"`
	MilestoneID int      `json:"milestone_id,omitempty"`
}

// CreateIssue creates a new issue in GitLab
func (c *APIClient) CreateIssue(ctx context.Context, projectID string, request *GitLabIssueCreateRequest) (*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues", c.baseURL, projectIDInt)

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
		"milestone_id", request.MilestoneID)

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

// GitLabIssueUpdateRequest represents the request body for updating a GitLab issue
type GitLabIssueUpdateRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
	MilestoneID *int     `json:"milestone_id,omitempty"`
	StateEvent  string   `json:"state_event,omitempty"`
}

// UpdateIssue updates an existing GitLab issue
func (c *APIClient) UpdateIssue(ctx context.Context, projectID string, issueIID int, request *GitLabIssueUpdateRequest) (*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues/%d", c.baseURL, projectIDInt, issueIID)

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

// GetIssue fetches an existing GitLab issue
func (c *APIClient) GetIssue(ctx context.Context, projectID string, issueIID int) (*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues/%d", c.baseURL, projectIDInt, issueIID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Getting GitLab issue",
		"project_id", projectID,
		"issue_iid", issueIID)

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
		c.logger.Warn("GitLab API get issue request failed",
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

	c.logger.Debug("Retrieved GitLab issue",
		"project_id", projectID,
		"issue_id", issue.ID,
		"issue_iid", issue.IID,
		"title", issue.Title)

	return &issue, nil
}

// SearchIssuesByTitle searches GitLab issues by title
func (c *APIClient) SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*GitLabIssue, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	// URL encode the title parameter
	encodedTitle := url.QueryEscape(title)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues?search=%s", c.baseURL, projectIDInt, encodedTitle)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Searching GitLab issues by title",
		"project_id", projectID,
		"title", title)

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
		c.logger.Warn("GitLab API search issues request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID,
			"title", title)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var issues []GitLabIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to slice of pointers
	result := make([]*GitLabIssue, len(issues))
	for i, issue := range issues {
		result[i] = &issue
	}

	c.logger.Debug("Found GitLab issues",
		"project_id", projectID,
		"title", title,
		"count", len(result))

	return result, nil
}

// GitLabCommentCreateRequest represents the request body for creating a GitLab comment
type GitLabCommentCreateRequest struct {
	Body string `json:"body"`
}

// GitLabComment represents a GitLab comment/note
type GitLabComment struct {
	ID        int         `json:"id"`
	Body      string      `json:"body"`
	Author    *GitLabUser `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	System    bool        `json:"system"`
	WebURL    string      `json:"web_url"`
}

// CreateComment creates a comment on a GitLab issue
func (c *APIClient) CreateComment(ctx context.Context, projectID string, issueIID int, request *GitLabCommentCreateRequest) (*GitLabComment, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/issues/%d/notes", c.baseURL, projectIDInt, issueIID)

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

	c.logger.Debug("Creating GitLab comment",
		"project_id", projectID,
		"issue_iid", issueIID,
		"body", request.Body)

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
		c.logger.Warn("GitLab API create comment request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID,
			"issue_iid", issueIID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var comment GitLabComment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Created GitLab comment",
		"project_id", projectID,
		"issue_iid", issueIID,
		"comment_id", comment.ID)

	return &comment, nil
}

// GitLabMilestone represents a GitLab milestone
type GitLabMilestone struct {
	ID          int        `json:"id"`
	IID         int        `json:"iid"`
	ProjectID   int        `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	WebURL      string     `json:"web_url"`
}

// GitLabMilestoneCreateRequest represents the request body for creating a GitLab milestone
type GitLabMilestoneCreateRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
}

// GitLabMilestoneUpdateRequest represents the request body for updating a GitLab milestone
type GitLabMilestoneUpdateRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	StartDate   *time.Time `json:"start_date"`
	StateEvent  string     `json:"state_event"`
}

// GetMilestones fetches milestones from GitLab
func (c *APIClient) GetMilestones(ctx context.Context, projectID string) ([]*GitLabMilestone, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/milestones", c.baseURL, projectIDInt)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Getting GitLab milestones",
		"project_id", projectID)

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
		c.logger.Warn("GitLab API get milestones request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var milestones []GitLabMilestone
	if err := json.NewDecoder(resp.Body).Decode(&milestones); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to slice of pointers
	result := make([]*GitLabMilestone, len(milestones))
	for i, milestone := range milestones {
		result[i] = &milestone
	}

	c.logger.Debug("Retrieved GitLab milestones",
		"project_id", projectID,
		"count", len(result))

	return result, nil
}

// CreateMilestone creates a new milestone in GitLab
func (c *APIClient) CreateMilestone(ctx context.Context, projectID string, request *GitLabMilestoneCreateRequest) (*GitLabMilestone, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/milestones", c.baseURL, projectIDInt)

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

	c.logger.Debug("Creating GitLab milestone",
		"project_id", projectID,
		"title", request.Title)

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
		c.logger.Warn("GitLab API create milestone request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var milestone GitLabMilestone
	if err := json.NewDecoder(resp.Body).Decode(&milestone); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Created GitLab milestone",
		"project_id", projectID,
		"milestone_id", milestone.ID,
		"title", milestone.Title)

	return &milestone, nil
}

// UpdateMilestone updates an existing GitLab milestone
func (c *APIClient) UpdateMilestone(ctx context.Context, projectID string, milestoneID int, request *GitLabMilestoneUpdateRequest) (*GitLabMilestone, error) {
	if c.token == "" {
		return nil, fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/milestones/%d", c.baseURL, projectIDInt, milestoneID)

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

	c.logger.Debug("Updating GitLab milestone",
		"project_id", projectID,
		"milestone_id", milestoneID,
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
		c.logger.Warn("GitLab API update milestone request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID,
			"milestone_id", milestoneID)
		return nil, fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	var milestone GitLabMilestone
	if err := json.NewDecoder(resp.Body).Decode(&milestone); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Updated GitLab milestone",
		"project_id", projectID,
		"milestone_id", milestone.ID,
		"title", milestone.Title)

	return &milestone, nil
}

// DeleteMilestone deletes a GitLab milestone
func (c *APIClient) DeleteMilestone(ctx context.Context, projectID string, milestoneID int) error {
	if c.token == "" {
		return fmt.Errorf("gitlab token not configured")
	}

	// Convert string projectID to int
	projectIDInt, err := c.parseProjectID(projectID)
	if err != nil {
		return fmt.Errorf("invalid project ID: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/milestones/%d", c.baseURL, projectIDInt, milestoneID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Deleting GitLab milestone",
		"project_id", projectID,
		"milestone_id", milestoneID)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		c.logger.Warn("GitLab API delete milestone request failed",
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"project_id", projectID,
			"milestone_id", milestoneID)
		return fmt.Errorf("gitlab API returned status %d", resp.StatusCode)
	}

	c.logger.Debug("Deleted GitLab milestone",
		"project_id", projectID,
		"milestone_id", milestoneID)

	return nil
}

// TestConnection tests the GitLab API connection
func (c *APIClient) TestConnection(ctx context.Context) error {
	if c.token == "" {
		return fmt.Errorf("gitlab token not configured")
	}

	// Use a simple API call to test connection
	apiURL := fmt.Sprintf("%s/api/v4/user", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Testing GitLab API connection")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("GitLab API connection test failed",
			"status_code", resp.StatusCode,
			"status", resp.Status)
		return fmt.Errorf("gitlab API connection test failed with status %d", resp.StatusCode)
	}

	c.logger.Debug("GitLab API connection test successful")
	return nil
}

// parseProjectID parses a project ID that can be either numeric or string-based
func (c *APIClient) parseProjectID(projectID string) (int, error) {
	// Try to parse as integer first
	if id, err := strconv.Atoi(projectID); err == nil {
		return id, nil
	}

	// If it's not a numeric ID, try to get it by namespace/project path
	// This is a simplified implementation - in production you might need more complex logic
	return 0, fmt.Errorf("project ID must be numeric")
}

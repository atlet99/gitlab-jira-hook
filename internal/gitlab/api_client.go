// Package gitlab provides GitLab API client for issue management.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// APIClient represents a GitLab API client
type APIClient struct {
	config     *config.Config
	httpClient *http.Client
	baseURL    string
	token      string
}

// IssueCreateRequest represents a request to create a GitLab issue
type IssueCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  int      `json:"assignee_id,omitempty"`
	MilestoneID int      `json:"milestone_id,omitempty"`
}

// IssueUpdateRequest represents a request to update a GitLab issue
type IssueUpdateRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
	StateEvent  string   `json:"state_event,omitempty"` // "close" or "reopen"
}

// GitLabIssue represents a GitLab issue response
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

// GitLabUser represents a GitLab user
type GitLabUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// CommentCreateRequest represents a request to create a comment
type CommentCreateRequest struct {
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

// NewAPIClient creates a new GitLab API client
func NewAPIClient(cfg *config.Config) *APIClient {
	if cfg == nil {
		return nil
	}

	// Create HTTP client with timeout
	const clientTimeout = 30 * time.Second
	httpClient := &http.Client{
		Timeout: clientTimeout,
	}

	return &APIClient{
		config:     cfg,
		httpClient: httpClient,
		baseURL:    cfg.GitLabBaseURL,
		token:      cfg.GitLabAPIToken,
	}
}

// CreateIssue creates a new issue in GitLab
func (c *APIClient) CreateIssue(ctx context.Context, projectID string, request *IssueCreateRequest) (*GitLabIssue, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues", c.baseURL, url.PathEscape(projectID))

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create issue request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var issue GitLabIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue response: %w", err)
	}

	return &issue, nil
}

// UpdateIssue updates an existing issue in GitLab
func (c *APIClient) UpdateIssue(ctx context.Context, projectID string, issueIID int, request *IssueUpdateRequest) (*GitLabIssue, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%d", c.baseURL, url.PathEscape(projectID), issueIID)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update issue request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var issue GitLabIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue response: %w", err)
	}

	return &issue, nil
}

// GetIssue retrieves an issue from GitLab
func (c *APIClient) GetIssue(ctx context.Context, projectID string, issueIID int) (*GitLabIssue, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%d", c.baseURL, url.PathEscape(projectID), issueIID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, nil // Issue not found
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var issue GitLabIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue response: %w", err)
	}

	return &issue, nil
}

// CreateComment creates a comment on a GitLab issue
func (c *APIClient) CreateComment(ctx context.Context, projectID string, issueIID int, request *CommentCreateRequest) (*GitLabComment, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%d/notes", c.baseURL, url.PathEscape(projectID), issueIID)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create comment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var comment GitLabComment
	if err := json.Unmarshal(body, &comment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comment response: %w", err)
	}

	return &comment, nil
}

// SearchIssuesByTitle searches for issues by title in a project
func (c *APIClient) SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*GitLabIssue, error) {
	urlStr := fmt.Sprintf("%s/api/v4/projects/%s/issues", c.baseURL, url.PathEscape(projectID))

	// Add search parameter
	params := url.Values{}
	params.Add("search", title)
	params.Add("scope", "title")
	params.Add("state", "all")

	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var issues []*GitLabIssue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issues response: %w", err)
	}

	return issues, nil
}

// FindUserByEmail finds a GitLab user by email
func (c *APIClient) FindUserByEmail(ctx context.Context, email string) (*GitLabUser, error) {
	urlStr := fmt.Sprintf("%s/api/v4/users", c.baseURL)

	params := url.Values{}
	params.Add("search", email)
	urlStr += "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error: %s - %s", resp.Status, string(body))
	}

	var users []*GitLabUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users response: %w", err)
	}

	// Find exact email match
	for _, user := range users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}

	return nil, nil // User not found
}

// TestConnection tests the connection to GitLab API
func (c *APIClient) TestConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v4/user", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override existing error
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitLab API connection failed: %s", resp.Status)
	}

	return nil
}

// setHeaders sets common headers for GitLab API requests
func (c *APIClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// ParseProjectID extracts project ID from various formats (ID, namespace/project, URL)
func ParseProjectID(projectStr string) string {
	// If it's already numeric, return as is
	if _, err := strconv.Atoi(projectStr); err == nil {
		return projectStr
	}

	// If it contains a slash, it's likely namespace/project format
	if strings.Contains(projectStr, "/") {
		// Remove any URL prefixes
		if strings.Contains(projectStr, "://") {
			parts := strings.Split(projectStr, "/")
			if len(parts) >= 2 {
				return parts[len(parts)-2] + "/" + parts[len(parts)-1]
			}
		}
		return projectStr
	}

	// Return as is for other formats
	return projectStr
}

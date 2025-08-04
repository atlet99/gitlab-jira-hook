package gitlab

import (
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

// Package jira provides a client for interacting with Jira Cloud REST API.
// It handles authentication, comment posting, and connection testing.
package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Client represents a Jira API client
type Client struct {
	config     *config.Config
	httpClient *http.Client
	baseURL    string
	authHeader string
}

// NewClient creates a new Jira API client
func NewClient(cfg *config.Config) *Client {
	// Create HTTP client with timeout
	const clientTimeout = 30 * time.Second
	httpClient := &http.Client{
		Timeout: clientTimeout,
	}

	// Create Basic Auth header
	auth := cfg.JiraEmail + ":" + cfg.JiraToken
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	return &Client{
		config:     cfg,
		httpClient: httpClient,
		baseURL:    cfg.JiraBaseURL,
		authHeader: authHeader,
	}
}

// AddComment adds a comment to a Jira issue
func (c *Client) AddComment(issueID, comment string) error {
	// Create comment payload
	payload := CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{
							Type: "text",
							Text: comment,
						},
					},
				},
			},
		},
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.baseURL, issueID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // explicitly ignore the error
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// TestConnection tests the connection to Jira API
func (c *Client) TestConnection() error {
	// Create request to get server info
	url := fmt.Sprintf("%s/rest/api/3/serverInfo", c.baseURL)
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // explicitly ignore the error
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jira API connection failed: %s", resp.Status)
	}

	return nil
}

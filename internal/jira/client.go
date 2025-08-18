// Package jira provides a client for interacting with Jira Cloud REST API.
// It handles authentication, comment posting, and connection testing.
package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Client represents a Jira API client
type Client struct {
	config       *config.Config
	httpClient   *http.Client
	baseURL      string
	authHeader   string        // Used for Basic Auth
	oauth2Client *OAuth2Client // Used for OAuth 2.0
	rateLimiter  *RateLimiter
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     int
	capacity   int
	rate       int // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int) *RateLimiter {
	return &RateLimiter{
		tokens:     rate,
		capacity:   rate,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))

	if tokensToAdd > 0 {
		rl.tokens = intMin(rl.capacity, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	// If no tokens available, wait
	if rl.tokens <= 0 {
		waitTime := time.Duration(float64(time.Second) / float64(rl.rate))
		rl.mu.Unlock()
		time.Sleep(waitTime)
		rl.mu.Lock()
		rl.tokens = 1
		rl.lastRefill = time.Now()
	}

	rl.tokens--
}

// NewClient creates a new Jira API client
func NewClient(cfg *config.Config) *Client {
	if cfg == nil {
		return nil
	}

	// Create HTTP client with timeout
	const clientTimeout = 30 * time.Second
	httpClient := &http.Client{
		Timeout: clientTimeout,
	}

	// Create rate limiter (default 10 requests per second)
	rateLimit := 10
	if cfg.JiraRateLimit > 0 {
		rateLimit = cfg.JiraRateLimit
	}

	client := &Client{
		config:      cfg,
		httpClient:  httpClient,
		baseURL:     cfg.JiraBaseURL,
		rateLimiter: NewRateLimiter(rateLimit),
	}

	// Initialize authentication based on method
	if cfg.JiraAuthMethod == config.JiraAuthMethodOAuth2 {
		client.oauth2Client = NewOAuth2Client(cfg, slog.Default())
	} else {
		// Default to Basic Auth for backward compatibility
		auth := cfg.JiraEmail + ":" + cfg.JiraToken
		client.authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	}

	return client
}

// getAuthorizationHeader returns the appropriate Authorization header based on auth method
func (c *Client) getAuthorizationHeader(ctx context.Context) (string, error) {
	if c.config.JiraAuthMethod == config.JiraAuthMethodOAuth2 && c.oauth2Client != nil {
		return c.oauth2Client.CreateAuthorizationHeader(ctx)
	}

	// Default to Basic Auth
	if c.authHeader == "" {
		return "", fmt.Errorf("no authentication configured")
	}
	return c.authHeader, nil
}

// AddComment adds a comment to a Jira issue
//
//nolint:gocyclo // Complex retry logic is necessary for reliability
func (c *Client) AddComment(ctx context.Context, issueID string, payload CommentPayload) error {
	// Validate ADF content and fallback to plain text if validation fails
	validatedPayload, err := validateAndFallback(payload)
	if err != nil {
		// Log the validation error but continue with the fallback content
		fmt.Printf("ADF validation failed for issue %s: %v\n", issueID, err)
	}

	maxAttempts := c.config.JiraRetryMaxAttempts
	baseDelay := time.Duration(c.config.JiraRetryBaseDelayMs) * time.Millisecond
	var lastErr error

	// Log attempt
	fmt.Printf("Adding comment to Jira issue %s (attempts: %d, base delay: %v)\n", issueID, maxAttempts, baseDelay)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Wait for rate limiter
		c.rateLimiter.Wait()

		// Build and send request
		req, err := c.buildCommentRequest(ctx, issueID, validatedPayload)
		if err != nil {
			return err
		}

		// Send request and handle response
		success, retryable, respErr := c.executeCommentRequest(req, attempt, maxAttempts)
		if success {
			fmt.Printf("Successfully added comment to Jira issue %s\n", issueID)
			return nil
		}

		lastErr = respErr
		if !retryable || attempt >= maxAttempts {
			return lastErr
		}

		// Wait before retry with exponential backoff
		time.Sleep(baseDelay * (1 << (attempt - 1)))
	}

	return lastErr
}

// GetTransitions retrieves available transitions for a Jira issue
func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var transitionsResp TransitionsResponse
	if err := json.Unmarshal(body, &transitionsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transitions response: %w", err)
	}

	return transitionsResp.Transitions, nil
}

// ExecuteTransition executes a transition for a Jira issue
func (c *Client) ExecuteTransition(ctx context.Context, issueKey, transitionID string) error {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create payload
	payload := TransitionPayload{
		Transition: TransitionID{
			ID: transitionID,
		},
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal transition payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
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

// FindTransition finds a transition by target status name
func (c *Client) FindTransition(ctx context.Context, issueKey, targetStatus string) (*Transition, error) {
	transitions, err := c.GetTransitions(ctx, issueKey)
	if err != nil {
		return nil, err
	}

	for _, t := range transitions {
		if t.To.Name == targetStatus {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("transition to status '%s' not available for issue %s", targetStatus, issueKey)
}

// TransitionToStatus transitions an issue to a specific status if possible
func (c *Client) TransitionToStatus(ctx context.Context, issueKey, targetStatus string) error {
	// First check if we're already in the target status
	currentIssue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("failed to get current issue status: %w", err)
	}

	if currentIssue.Fields.Status.Name == targetStatus {
		// Already in target status, nothing to do
		return nil
	}

	// Find the appropriate transition
	transition, err := c.FindTransition(ctx, issueKey, targetStatus)
	if err != nil {
		return err
	}

	// Execute the transition
	return c.ExecuteTransition(ctx, issueKey, transition.ID)
}

// GetIssue retrieves a Jira issue by key
func (c *Client) GetIssue(ctx context.Context, issueKey string) (*JiraIssue, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var issue JiraIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue response: %w", err)
	}

	return &issue, nil
}

// buildCommentRequest builds HTTP request for adding comment to Jira issue
func (c *Client) buildCommentRequest(
	ctx context.Context, issueID string, payload CommentPayload,
) (*http.Request, error) {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.baseURL, issueID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// executeCommentRequest executes the comment request and handles the response
// Returns (success, retryable, error)
func (c *Client) executeCommentRequest(
	req *http.Request, attempt, maxAttempts int,
) (success, retryable bool, err error) {
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("Request failed (attempt %d/%d): %s\n", attempt, maxAttempts, err)
		return false, true, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // explicitly ignore the error
		}
		fmt.Printf("Failed to read response (attempt %d/%d): %s\n", attempt, maxAttempts, err)
		return false, true, fmt.Errorf("failed to read response body: %w", err)
	}

	// Close response body
	if closeErr := resp.Body.Close(); closeErr != nil {
		_ = closeErr // explicitly ignore the error
	}

	// Check response status
	const serverErrorMin = 500
	const serverErrorMax = 600
	const successMin = 200
	const successMax = 300

	if resp.StatusCode >= serverErrorMin && resp.StatusCode < serverErrorMax {
		err := fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
		fmt.Printf("Jira API 5xx error (attempt %d/%d): %s\n", attempt, maxAttempts, err)
		return false, true, err
	}

	if resp.StatusCode < successMin || resp.StatusCode >= successMax {
		err := fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
		fmt.Printf("Jira API error (attempt %d/%d): %s\n", attempt, maxAttempts, err)
		return false, false, err
	}

	return true, false, nil
}

// TestConnection tests the connection to Jira API
func (c *Client) TestConnection(ctx context.Context) error {
	maxAttempts := c.config.JiraRetryMaxAttempts
	baseDelay := time.Duration(c.config.JiraRetryBaseDelayMs) * time.Millisecond
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Wait for rate limiter
		c.rateLimiter.Wait()

		// Create request to get server info
		url := fmt.Sprintf("%s/rest/api/3/serverInfo", c.baseURL)
		req, err := http.NewRequest("GET", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		authHeader, err := c.getAuthorizationHeader(ctx)
		if err != nil {
			return fmt.Errorf("failed to get authorization header: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Accept", "application/json")

		// Send request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(baseDelay * (1 << (attempt - 1)))
				continue
			}
			return fmt.Errorf("failed to send request: %w", err)
		}

		// Close response body
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // explicitly ignore the error
		}

		// Check response status
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf("jira API connection failed: %s", resp.Status)
			if attempt < maxAttempts {
				time.Sleep(baseDelay * (1 << (attempt - 1)))
				continue
			}
			return lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("jira API connection failed: %s", resp.Status)
		}

		return nil
	}

	return lastErr
}

// intMin returns the minimum of two integers
func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetAssignee updates the assignee for a Jira issue using accountId
func (c *Client) SetAssignee(ctx context.Context, issueKey, accountId string) error {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create payload
	payload := map[string]interface{}{
		"accountId": accountId,
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal assignee payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.baseURL, issueKey)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
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

// SearchIssues executes a JQL query and returns matching issues
func (c *Client) SearchIssues(ctx context.Context, jql string) ([]JiraIssue, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/search", c.baseURL)
	
	// Create payload
	payload := map[string]interface{}{
		"jql": jql,
	}
	
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JQL payload: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var searchResult struct {
		Issues []JiraIssue `json:"issues"`
	}
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	return searchResult.Issues, nil
}

// UpdateIssue updates a Jira issue with the given fields
func (c *Client) UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create payload
	payload := map[string]interface{}{
		"fields": fields,
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal update payload: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
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

// SearchUsers searches for users by email address
func (c *Client) SearchUsers(ctx context.Context, email string) ([]JiraUser, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/user/search?query=%s", c.baseURL, email)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	authHeader, err := c.getAuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorization header: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	// Add context
	req = req.WithContext(ctx)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// explicitly ignore the error
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var users []JiraUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users response: %w", err)
	}

	return users, nil
}

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
	"strings"
	"sync"
	"time"

	configpkg "github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Client represents a Jira API client
type Client struct {
	config       *configpkg.Config
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
func NewClient(cfg *configpkg.Config) *Client {
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
	if cfg.JiraAuthMethod == configpkg.JiraAuthMethodOAuth2 {
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
	if c.config.JiraAuthMethod == configpkg.JiraAuthMethodOAuth2 && c.oauth2Client != nil {
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
	validatedPayload := validateAndFallback(payload)
	// Note: validateAndFallback always returns a valid payload, no error handling needed

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
	req, err := http.NewRequest("GET", url, http.NoBody)
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
			slog.Error("Failed to close response body", "error", closeErr)
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
			slog.Error("Failed to close response body", "error", closeErr)
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

	for i := 0; i < len(transitions); i++ {
		if transitions[i].To.Name == targetStatus {
			return &transitions[i], nil
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

// ValidateTransition validates if a transition to the target status is available and valid
func (c *Client) ValidateTransition(ctx context.Context, issueKey, targetStatus string) error {
	// Get current issue to verify current status
	currentIssue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("failed to get current issue status: %w", err)
	}

	// Check if we're already in the target status (idempotency check)
	if currentIssue.Fields.Status != nil && currentIssue.Fields.Status.Name == targetStatus {
		return nil // Already in target status, transition is valid but not needed
	}

	// Get available transitions
	transitions, err := c.GetTransitions(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("failed to get available transitions: %w", err)
	}

	// Validate that transition to target status exists
	for i := 0; i < len(transitions); i++ {
		if transitions[i].To.Name == targetStatus {
			return nil // Transition is valid
		}
	}

	return fmt.Errorf("no valid transition found to status '%s' for issue %s", targetStatus, issueKey)
}

// ExecuteTransitionWithValidation executes a transition with full validation and idempotency checks
func (c *Client) ExecuteTransitionWithValidation(ctx context.Context, issueKey, targetStatus string) error {
	// Validate transition first
	if err := c.ValidateTransition(ctx, issueKey, targetStatus); err != nil {
		return fmt.Errorf("transition validation failed: %w", err)
	}

	// Find the transition ID
	transition, err := c.FindTransition(ctx, issueKey, targetStatus)
	if err != nil {
		return fmt.Errorf("failed to find transition: %w", err)
	}

	// Execute the transition
	return c.ExecuteTransition(ctx, issueKey, transition.ID)
}

// GetTransitionDetails gets detailed information about available transitions for an issue
func (c *Client) GetTransitionDetails(ctx context.Context, issueKey string) ([]Transition, error) {
	return c.GetTransitions(ctx, issueKey)
}

// GetIssue retrieves a Jira issue by key
func (c *Client) GetIssue(ctx context.Context, issueKey string) (*JiraIssue, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)
	req, err := http.NewRequest("GET", url, http.NoBody)
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
			slog.Error("Failed to close response body", "error", closeErr)
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
			slog.Error("Failed to close response body", "error", closeErr)
		}
		fmt.Printf("Failed to read response (attempt %d/%d): %s\n", attempt, maxAttempts, err)
		return false, true, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := resp.Body.Close(); err != nil {
		slog.Error("Failed to close response body", "error", err)
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

		if err := resp.Body.Close(); err != nil {
			slog.Error("Failed to close response body", "error", err)
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

// executePutRequest executes a PUT request to Jira API with the given payload
func (c *Client) executePutRequest(ctx context.Context, url string, payload map[string]interface{}) error {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
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
			slog.Error("Failed to close response body", "error", closeErr)
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

// SetAssignee updates the assignee for a Jira issue using accountId
func (c *Client) SetAssignee(ctx context.Context, issueKey, accountID string) error {
	// Handle special assignee cases
	if c.isSpecialAssigneeValue(accountID) {
		return c.handleSpecialAssignee(ctx, issueKey, accountID)
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.baseURL, issueKey)
	payload := map[string]interface{}{
		"accountId": accountID,
	}
	return c.executePutRequest(ctx, url, payload)
}

// SetAssigneeWithValidation sets assignee with full validation and special value handling
func (c *Client) SetAssigneeWithValidation(ctx context.Context, issueKey, accountID string) error {
	// Handle special assignee cases
	if c.isSpecialAssigneeValue(accountID) {
		return c.handleSpecialAssignee(ctx, issueKey, accountID)
	}

	// Validate that the user exists if not a special value
	if accountID != "" && accountID != SpecialAssigneeNull {
		users, err := c.SearchUsers(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to validate assignee: %w", err)
		}

		// Check if accountId exists among found users
		userExists := false
		for _, user := range users {
			if user.AccountID == accountID {
				userExists = true
				break
			}
		}

		if !userExists {
			return fmt.Errorf("user with accountId '%s' not found", accountID)
		}
	}

	return c.SetAssignee(ctx, issueKey, accountID)
}

const (
	// SpecialAssigneeNull represents the null assignee value in Jira
	SpecialAssigneeNull = "null"
	// SpecialAssigneeNil represents the nil assignee value in Jira
	SpecialAssigneeNil = "nil"
	// SpecialAssigneeUnassigned represents the unassigned value in Jira
	SpecialAssigneeUnassigned = "unassigned"
	// SpecialAssigneeDefault represents the default assignee value in Jira
	SpecialAssigneeDefault = "default"
	// SpecialAssigneeMinusOne represents the -1 assignee value in Jira
	SpecialAssigneeMinusOne = "-1"
	// SystemAccount represents the system account in Jira
	SystemAccount = "system"
	// AutomationAccount represents the automation account in Jira
	AutomationAccount = "automation"
	// BotAccount represents the bot account in Jira
	BotAccount = "bot"
	// UnassignedAccount represents the unassigned account in Jira
	UnassignedAccount = "unassigned"
)

// isSpecialAssigneeValue checks if the accountId represents a special assignee value
func (c *Client) isSpecialAssigneeValue(accountID string) bool {
	if accountID == "" || accountID == SpecialAssigneeNull || accountID == SpecialAssigneeNil {
		return true // Unassigned
	}

	// Check for default assignee
	if accountID == SpecialAssigneeMinusOne || accountID == SpecialAssigneeDefault {
		return true // Default assignee
	}

	// Check for system accounts
	systemAccounts := []string{
		SpecialAssigneeMinusOne, SpecialAssigneeNull, SpecialAssigneeNil,
		SpecialAssigneeUnassigned, SystemAccount, AutomationAccount,
		BotAccount, UnassignedAccount,
	}
	for _, systemAccount := range systemAccounts {
		if strings.EqualFold(accountID, systemAccount) {
			return true
		}
	}

	return false
}

// handleSpecialAssignee handles special assignee cases (Unassigned, Default)
func (c *Client) handleSpecialAssignee(ctx context.Context, issueKey, accountID string) error {
	switch strings.ToLower(accountID) {
	case "", SpecialAssigneeNull, SpecialAssigneeNil, SpecialAssigneeUnassigned:
		// Set to unassigned
		url := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.baseURL, issueKey)
		payload := map[string]interface{}{
			"accountId": "", // Empty accountId means unassigned
		}
		return c.executePutRequest(ctx, url, payload)
	case SpecialAssigneeMinusOne, SpecialAssigneeDefault:
		// Set to default assignee (remove specific assignee)
		url := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.baseURL, issueKey)
		payload := map[string]interface{}{
			"accountId": "null", // null means default assignee
		}
		return c.executePutRequest(ctx, url, payload)
	default:
		// Handle other system accounts
		url := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.baseURL, issueKey)
		payload := map[string]interface{}{
			"accountId": accountID,
		}
		return c.executePutRequest(ctx, url, payload)
	}
}

// GetAssigneeDetails gets detailed information about the current assignee
func (c *Client) GetAssigneeDetails(ctx context.Context, issueKey string) (*AssigneeDetails, error) {
	issue, err := c.GetIssue(ctx, issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	details := &AssigneeDetails{
		IssueKey:    issueKey,
		HasAssignee: issue.Fields.Assignee != nil,
	}

	if issue.Fields.Assignee != nil {
		details.Assignee = *issue.Fields.Assignee
		details.IsSpecial = c.isSpecialAssigneeValue(issue.Fields.Assignee.AccountID)
		details.SpecialType = c.getSpecialAssigneeType(issue.Fields.Assignee.AccountID)
	} else {
		details.IsSpecial = true
		details.SpecialType = "unassigned"
	}

	return details, nil
}

// AssigneeDetails represents detailed information about an issue's assignee
type AssigneeDetails struct {
	IssueKey    string   `json:"issue_key"`
	HasAssignee bool     `json:"has_assignee"`
	Assignee    JiraUser `json:"assignee,omitempty"`
	IsSpecial   bool     `json:"is_special"`
	SpecialType string   `json:"special_type,omitempty"`
}

// getSpecialAssigneeType determines the type of special assignee
func (c *Client) getSpecialAssigneeType(accountID string) string {
	switch strings.ToLower(accountID) {
	case "", SpecialAssigneeNull, SpecialAssigneeNil, SpecialAssigneeUnassigned:
		return "unassigned"
	case SpecialAssigneeMinusOne, SpecialAssigneeDefault:
		return "default"
	default:
		return "system"
	}
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
			slog.Error("Failed to close response body", "error", closeErr)
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
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)
	payload := map[string]interface{}{
		"fields": fields,
	}
	return c.executePutRequest(ctx, url, payload)
}

// SearchUsers searches for users by email address
func (c *Client) SearchUsers(ctx context.Context, email string) ([]JiraUser, error) {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Create request
	url := fmt.Sprintf("%s/rest/api/3/user/search?query=%s", c.baseURL, email)
	req, err := http.NewRequest("GET", url, http.NoBody)
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
			slog.Error("Failed to close response body", "error", closeErr)
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

// do executes a generic HTTP request and handles the response
func (c *Client) do(req *http.Request, response interface{}) error {
	// Wait for rate limiter
	c.rateLimiter.Wait()

	// Set authorization header if not already set
	if req.Header.Get("Authorization") == "" {
		authHeader, err := c.getAuthorizationHeader(req.Context())
		if err != nil {
			return fmt.Errorf("failed to get authorization header: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
	}

	// Set default headers if not already set
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	// Add context
	req = req.WithContext(req.Context())

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
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

	// Parse response if response interface is provided
	if response != nil {
		if err := json.Unmarshal(body, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

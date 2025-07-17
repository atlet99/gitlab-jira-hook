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
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// Client represents a Jira API client
type Client struct {
	config      *config.Config
	httpClient  *http.Client
	baseURL     string
	authHeader  string
	rateLimiter *RateLimiter
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
	// Create HTTP client with timeout
	const clientTimeout = 30 * time.Second
	httpClient := &http.Client{
		Timeout: clientTimeout,
	}

	// Create Basic Auth header
	auth := cfg.JiraEmail + ":" + cfg.JiraToken
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	// Create rate limiter (default 10 requests per second)
	rateLimit := 10
	if cfg.JiraRateLimit > 0 {
		rateLimit = cfg.JiraRateLimit
	}

	return &Client{
		config:      cfg,
		httpClient:  httpClient,
		baseURL:     cfg.JiraBaseURL,
		authHeader:  authHeader,
		rateLimiter: NewRateLimiter(rateLimit),
	}
}

// AddComment adds a comment to a Jira issue
func (c *Client) AddComment(issueID string, payload CommentPayload) error {
	maxAttempts := c.config.JiraRetryMaxAttempts
	baseDelay := time.Duration(c.config.JiraRetryBaseDelayMs) * time.Millisecond
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Wait for rate limiter
		c.rateLimiter.Wait()

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
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(baseDelay * (1 << (attempt - 1)))
				continue
			}
			return fmt.Errorf("failed to send request: %w", err)
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				_ = closeErr // explicitly ignore the error
			}
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(baseDelay * (1 << (attempt - 1)))
				continue
			}
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Close response body
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr // explicitly ignore the error
		}

		// Check response status
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
			if attempt < maxAttempts {
				time.Sleep(baseDelay * (1 << (attempt - 1)))
				continue
			}
			return lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("jira API error: %s - %s", resp.Status, string(body))
		}

		return nil
	}

	return lastErr
}

// TestConnection tests the connection to Jira API
func (c *Client) TestConnection() error {
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
		req.Header.Set("Authorization", c.authHeader)
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

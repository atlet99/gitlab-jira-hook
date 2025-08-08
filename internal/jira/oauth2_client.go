// Package jira provides OAuth 2.0 client for Jira Cloud API.
package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

// OAuth2Client handles OAuth 2.0 authentication for Jira
type OAuth2Client struct {
	config     *config.Config
	httpClient *http.Client
	logger     *slog.Logger
}

// OAuth2TokenResponse represents the response from OAuth 2.0 token endpoint
type OAuth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuth2ErrorResponse represents an error response from OAuth 2.0 endpoints
type OAuth2ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// NewOAuth2Client creates a new OAuth 2.0 client for Jira
func NewOAuth2Client(cfg *config.Config, logger *slog.Logger) *OAuth2Client {
	if cfg == nil {
		return nil
	}

	const clientTimeout = 30 * time.Second
	httpClient := &http.Client{
		Timeout: clientTimeout,
	}

	return &OAuth2Client{
		config:     cfg,
		httpClient: httpClient,
		logger:     logger,
	}
}

// GetAuthorizationURL generates the OAuth 2.0 authorization URL
func (c *OAuth2Client) GetAuthorizationURL(state string) string {
	params := url.Values{}
	params.Add("audience", "api.atlassian.com")
	params.Add("client_id", c.config.JiraOAuth2ClientID)
	params.Add("scope", c.config.JiraOAuth2Scope)
	params.Add("redirect_uri", c.config.JiraOAuth2RedirectURL)
	params.Add("state", state)
	params.Add("response_type", "code")
	params.Add("prompt", "consent")

	return c.config.JiraOAuth2AuthURL + "?" + params.Encode()
}

// ExchangeCodeForTokens exchanges an authorization code for OAuth 2.0 tokens
func (c *OAuth2Client) ExchangeCodeForTokens(ctx context.Context, code string) (*OAuth2TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", c.config.JiraOAuth2ClientID)
	data.Set("client_secret", c.config.JiraOAuth2ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", c.config.JiraOAuth2RedirectURL)

	return c.makeTokenRequest(ctx, data)
}

// RefreshToken refreshes the OAuth 2.0 access token using the refresh token
func (c *OAuth2Client) RefreshToken(ctx context.Context, refreshToken string) (*OAuth2TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.config.JiraOAuth2ClientID)
	data.Set("client_secret", c.config.JiraOAuth2ClientSecret)
	data.Set("refresh_token", refreshToken)

	return c.makeTokenRequest(ctx, data)
}

// IsTokenExpired checks if the current access token is expired
func (c *OAuth2Client) IsTokenExpired() bool {
	if c.config.JiraTokenExpiry == 0 {
		return true
	}

	// Add 5-minute buffer before actual expiry
	const bufferSeconds = 300
	return time.Now().Unix() >= (c.config.JiraTokenExpiry - bufferSeconds)
}

// GetValidAccessToken returns a valid access token, refreshing if necessary
func (c *OAuth2Client) GetValidAccessToken(ctx context.Context) (string, error) {
	// If we don't have an access token, return error
	if c.config.JiraAccessToken == "" {
		return "", fmt.Errorf("no access token available - authorization required")
	}

	// If token is not expired, return current token
	if !c.IsTokenExpired() {
		return c.config.JiraAccessToken, nil
	}

	// If we don't have a refresh token, return error
	if c.config.JiraRefreshToken == "" {
		return "", fmt.Errorf("access token expired and no refresh token available - re-authorization required")
	}

	// Refresh the token
	tokenResponse, err := c.RefreshToken(ctx, c.config.JiraRefreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update configuration with new tokens
	c.updateTokensInConfig(tokenResponse)

	return tokenResponse.AccessToken, nil
}

// makeTokenRequest makes a request to the OAuth 2.0 token endpoint
func (c *OAuth2Client) makeTokenRequest(ctx context.Context, data url.Values) (*OAuth2TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.JiraOAuth2TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body",
				"error", closeErr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errorResp OAuth2ErrorResponse
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			return nil, fmt.Errorf("OAuth 2.0 error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuth2TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// updateTokensInConfig updates the configuration with new OAuth 2.0 tokens
func (c *OAuth2Client) updateTokensInConfig(tokenResp *OAuth2TokenResponse) {
	c.config.JiraAccessToken = tokenResp.AccessToken

	if tokenResp.RefreshToken != "" {
		c.config.JiraRefreshToken = tokenResp.RefreshToken
	}

	if tokenResp.ExpiresIn > 0 {
		c.config.JiraTokenExpiry = time.Now().Unix() + int64(tokenResp.ExpiresIn)
	}
}

// CreateAuthorizationHeader creates the Authorization header for OAuth 2.0 requests
func (c *OAuth2Client) CreateAuthorizationHeader(ctx context.Context) (string, error) {
	accessToken, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return "", err
	}

	return "Bearer " + accessToken, nil
}

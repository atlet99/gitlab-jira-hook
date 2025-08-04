// Package jira provides OAuth 2.0 HTTP handlers for Jira authorization flow.
package jira

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

const (
	// OAuth2StateTimeout defines how long OAuth state is valid
	OAuth2StateTimeout = 10 * time.Minute
	// OAuth2TokenTimeout defines timeout for token exchange requests
	OAuth2TokenTimeout = 30 * time.Second
)

// OAuth2Handlers provides HTTP handlers for OAuth 2.0 authorization flow
type OAuth2Handlers struct {
	config       *config.Config
	oauth2Client *OAuth2Client
	logger       *slog.Logger
}

// NewOAuth2Handlers creates a new OAuth 2.0 handlers instance
func NewOAuth2Handlers(cfg *config.Config, logger *slog.Logger) *OAuth2Handlers {
	return &OAuth2Handlers{
		config:       cfg,
		oauth2Client: NewOAuth2Client(cfg),
		logger:       logger,
	}
}

// HandleAuthorize handles the OAuth 2.0 authorization initiation
func (h *OAuth2Handlers) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate a random state parameter for CSRF protection
	state, err := h.generateState()
	if err != nil {
		h.logger.Error("Failed to generate OAuth state", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store state in session/cookie for validation later
	// In production, you might want to store this in Redis or database
	cookie := &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(OAuth2StateTimeout),
	}
	http.SetCookie(w, cookie)

	// Get authorization URL
	authURL := h.oauth2Client.GetAuthorizationURL(state)

	h.logger.Info("Redirecting to Jira authorization", "url", authURL)

	// Redirect to Jira authorization page
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback handles the OAuth 2.0 authorization callback
func (h *OAuth2Handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate state parameter
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		h.logger.Error("Missing OAuth state cookie", "error", err)
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam == "" || stateParam != stateCookie.Value {
		h.logger.Error("Invalid OAuth state parameter",
			"expected", stateCookie.Value,
			"received", stateParam)
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// Clear the state cookie
	clearCookie := &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(w, clearCookie)

	// Check for authorization error
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		h.logger.Error("OAuth authorization error",
			"error", errorParam,
			"description", errorDesc)
		http.Error(w, fmt.Sprintf("Authorization failed: %s", errorDesc), http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		h.logger.Error("Missing authorization code")
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	ctx, cancel := context.WithTimeout(r.Context(), OAuth2TokenTimeout)
	defer cancel()

	tokenResp, err := h.oauth2Client.ExchangeCodeForTokens(ctx, code)
	if err != nil {
		h.logger.Error("Failed to exchange code for tokens", "error", err)
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}

	// TODO: In production, you should persist tokens securely
	// For now, we'll store them in the config (this is not production-ready)
	h.updateConfigWithTokens(tokenResp)

	h.logger.Info("OAuth 2.0 authorization successful",
		"access_token_expires_in", tokenResp.ExpiresIn,
		"scope", tokenResp.Scope)

	// Return success response
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	successHTML := `
<!DOCTYPE html>
<html>
<head>
    <title>Authorization Successful</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
        .success { color: #28a745; }
        .info { color: #17a2b8; margin-top: 20px; }
    </style>
</head>
<body>
    <h1 class="success">✅ Authorization Successful!</h1>
    <p>Your GitLab-Jira Hook application has been successfully authorized to access Jira.</p>
    <p class="info">You can now close this window and return to your application.</p>
    <p class="info">The application will now use OAuth 2.0 for Jira API access.</p>
</body>
</html>
`
	if _, err := fmt.Fprint(w, successHTML); err != nil {
		h.logger.Error("Failed to write success HTML response", "error", err)
	}
}

// HandleStatus provides OAuth 2.0 status information
func (h *OAuth2Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	status := map[string]interface{}{
		"auth_method":       h.config.JiraAuthMethod,
		"oauth2_enabled":    h.config.JiraAuthMethod == "oauth2",
		"has_access_token":  h.config.JiraAccessToken != "",
		"has_refresh_token": h.config.JiraRefreshToken != "",
		"token_expired":     h.oauth2Client.IsTokenExpired(),
	}

	if h.config.JiraTokenExpiry > 0 {
		status["token_expires_at"] = time.Unix(h.config.JiraTokenExpiry, 0).Format(time.RFC3339)
	}

	if err := writeJSON(w, status); err != nil {
		h.logger.Error("Failed to write status response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// generateState generates a cryptographically secure random state parameter
func (h *OAuth2Handlers) generateState() (string, error) {
	const stateLength = 32
	bytes := make([]byte, stateLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// updateConfigWithTokens updates the configuration with new OAuth 2.0 tokens
// TODO: In production, implement secure token storage (database, encrypted file, etc.)
func (h *OAuth2Handlers) updateConfigWithTokens(tokenResp *OAuth2TokenResponse) {
	h.config.JiraAccessToken = tokenResp.AccessToken

	if tokenResp.RefreshToken != "" {
		h.config.JiraRefreshToken = tokenResp.RefreshToken
	}

	if tokenResp.ExpiresIn > 0 {
		h.config.JiraTokenExpiry = time.Now().Unix() + int64(tokenResp.ExpiresIn)
	}

	h.logger.Info("Updated OAuth 2.0 tokens in configuration",
		"expires_in", tokenResp.ExpiresIn,
		"has_refresh_token", tokenResp.RefreshToken != "")
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")

	// Simple JSON marshaling - in production you might want to use encoding/json
	switch v := data.(type) {
	case map[string]interface{}:
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{`)
		first := true
		for key, value := range v {
			if !first {
				fmt.Fprintf(w, `,`)
			}
			first = false

			switch val := value.(type) {
			case string:
				fmt.Fprintf(w, `"%s":"%s"`, key, val)
			case bool:
				fmt.Fprintf(w, `"%s":%t`, key, val)
			case int, int64:
				fmt.Fprintf(w, `"%s":%v`, key, val)
			default:
				fmt.Fprintf(w, `"%s":"%v"`, key, val)
			}
		}
		fmt.Fprintf(w, `}`)
		return nil
	default:
		return fmt.Errorf("unsupported data type for JSON response")
	}
}

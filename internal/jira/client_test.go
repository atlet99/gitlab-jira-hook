package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "create client with valid config",
			config: &config.Config{
				JiraEmail:   "test@example.com",
				JiraToken:   "test-token",
				JiraBaseURL: "https://jira.example.com",
			},
			expectError: false,
		},
		{
			name: "create client with custom rate limit",
			config: &config.Config{
				JiraEmail:            "test@example.com",
				JiraToken:            "test-token",
				JiraBaseURL:          "https://jira.example.com",
				JiraRateLimit:        20,
				JiraRetryMaxAttempts: 5,
				JiraRetryBaseDelayMs: 100,
			},
			expectError: false,
		},
		{
			name:        "create client with nil config",
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)

			if tt.expectError {
				assert.Nil(t, client)
				return
			}

			require.NotNil(t, client)
			assert.Equal(t, tt.config.JiraBaseURL, client.baseURL)
			assert.NotEmpty(t, client.authHeader)
			assert.True(t, strings.HasPrefix(client.authHeader, "Basic "))
			assert.NotNil(t, client.rateLimiter)
			assert.NotNil(t, client.httpClient)

			// Verify auth header format
			expectedAuth := base64Encode(tt.config.JiraEmail + ":" + tt.config.JiraToken)
			assert.Equal(t, expectedAuth, client.authHeader)
		})
	}
}

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name     string
		rate     int
		expected *RateLimiter
	}{
		{
			name: "create rate limiter with default rate",
			rate: 10,
			expected: &RateLimiter{
				tokens:   10,
				capacity: 10,
				rate:     10,
			},
		},
		{
			name: "create rate limiter with custom rate",
			rate: 5,
			expected: &RateLimiter{
				tokens:   5,
				capacity: 5,
				rate:     5,
			},
		},
		{
			name: "create rate limiter with zero rate",
			rate: 0,
			expected: &RateLimiter{
				tokens:   0,
				capacity: 0,
				rate:     0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.rate)

			require.NotNil(t, rl)
			assert.Equal(t, tt.expected.tokens, rl.tokens)
			assert.Equal(t, tt.expected.capacity, rl.capacity)
			assert.Equal(t, tt.expected.rate, rl.rate)
			assert.NotZero(t, rl.lastRefill)
		})
	}
}

func TestRateLimiterWait(t *testing.T) {
	t.Run("wait with available tokens", func(t *testing.T) {
		rl := NewRateLimiter(10)
		start := time.Now()

		// Should not block when tokens are available
		rl.Wait()

		duration := time.Since(start)
		assert.Less(t, duration, 10*time.Millisecond)
		assert.Equal(t, 9, rl.tokens)
	})

	t.Run("wait with no tokens", func(t *testing.T) {
		rl := NewRateLimiter(1)

		// Consume the only token
		rl.Wait()
		assert.Equal(t, 0, rl.tokens)

		// Next wait should block
		start := time.Now()
		rl.Wait()
		duration := time.Since(start)

		// Should have waited approximately 1 second (1 token per second)
		assert.GreaterOrEqual(t, duration, 900*time.Millisecond)
		assert.LessOrEqual(t, duration, 1100*time.Millisecond)
	})

	t.Run("wait with token refill", func(t *testing.T) {
		rl := NewRateLimiter(2)

		// Consume all tokens
		rl.Wait()
		rl.Wait()
		assert.Equal(t, 0, rl.tokens)

		// Simulate time passing (1 second)
		rl.lastRefill = time.Now().Add(-1 * time.Second)

		// Should refill tokens
		start := time.Now()
		rl.Wait()
		duration := time.Since(start)

		// Should not block since tokens were refilled
		assert.Less(t, duration, 10*time.Millisecond)
		assert.Equal(t, 1, rl.tokens)
	})
}

func TestClientAddComment(t *testing.T) {
	t.Run("successful comment addition", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/comment", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			// Verify request body
			var payload CommentPayload
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "doc", payload.Body.Type)
			assert.Equal(t, 1, payload.Body.Version)
			assert.Len(t, payload.Body.Content, 1)
			assert.Equal(t, "paragraph", payload.Body.Content[0].Type)
			assert.Len(t, payload.Body.Content[0].Content, 1)
			assert.Equal(t, "text", payload.Body.Content[0].Content[0].Type)
			assert.Equal(t, "Test comment", payload.Body.Content[0].Content[0].Text)

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "12345"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.NoError(t, err)
	})

	t.Run("comment addition with retry on 5xx error", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "Internal server error"}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "12345"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10, // Short delay for testing
		}

		client := NewClient(cfg)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("comment addition fails after max retries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal server error"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 2,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jira API error")
	})

	t.Run("comment addition fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Server received request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "Bad request"}`))
		}))
		defer server.Close()

		t.Logf("Test server URL: %s", server.URL)

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		t.Logf("Client base URL: %s", client.baseURL)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		t.Logf("AddComment returned error: %v", err)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "jira API error")
			assert.Contains(t, err.Error(), "400")
		}
	})

	t.Run("comment addition fails on network error", func(t *testing.T) {
		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          "http://invalid-url-that-does-not-exist.com",
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.Error(t, err)
		if err != nil {
			// Network error can be either "failed to send request" or HTTP error
			assert.True(t, strings.Contains(err.Error(), "failed to send request") ||
				strings.Contains(err.Error(), "jira API error"),
				"Expected network error or HTTP error, got: %s", err.Error())
		}
	})
}

func TestClientTestConnection(t *testing.T) {
	t.Run("successful connection test", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/api/3/serverInfo", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"baseUrl": "https://jira.example.com"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.TestConnection(context.Background())
		assert.NoError(t, err)
	})

	t.Run("connection test with retry on 5xx error", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"baseUrl": "https://jira.example.com"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 2,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		err := client.TestConnection(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("connection test fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          server.URL,
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		err := client.TestConnection(context.Background())
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "jira API connection failed")
			assert.Contains(t, err.Error(), "401")
		}
	})

	t.Run("connection test fails on network error", func(t *testing.T) {
		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          "http://invalid-url-that-does-not-exist.com",
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		err := client.TestConnection(context.Background())
		assert.Error(t, err)
		if err != nil {
			// Network error can be either "failed to send request" or HTTP error
			assert.True(t, strings.Contains(err.Error(), "failed to send request") ||
				strings.Contains(err.Error(), "jira API connection failed"),
				"Expected network error or HTTP error, got: %s", err.Error())
		}
	})
}

func TestIntMin(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"positive numbers", 5, 10, 5},
		{"negative numbers", -5, -10, -10},
		{"mixed numbers", 5, -10, -10},
		{"equal numbers", 5, 5, 5},
		{"zero and positive", 0, 5, 0},
		{"zero and negative", 0, -5, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intMin(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClientEdgeCases(t *testing.T) {
	t.Run("client with empty base URL", func(t *testing.T) {
		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          "",
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)
		payload := createTestCommentPayload("Test comment")

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.Error(t, err)
		if err != nil {
			// Empty URL can cause various errors
			assert.True(t, strings.Contains(err.Error(), "failed to create request") ||
				strings.Contains(err.Error(), "failed to send request") ||
				strings.Contains(err.Error(), "unsupported protocol scheme"),
				"Expected URL-related error, got: %s", err.Error())
		}
	})

	t.Run("client with invalid JSON payload", func(t *testing.T) {
		cfg := &config.Config{
			JiraEmail:            "test@example.com",
			JiraToken:            "test-token",
			JiraBaseURL:          "https://jira.example.com",
			JiraRetryMaxAttempts: 3,
			JiraRetryBaseDelayMs: 10,
		}

		client := NewClient(cfg)

		// Create a payload that can't be marshaled to JSON
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
								Text: string([]byte{0xFF, 0xFE, 0xFD}), // Invalid UTF-8
							},
						},
					},
				},
			},
		}

		err := client.AddComment(context.Background(), "ABC-123", payload)
		assert.Error(t, err)
		if err != nil {
			// Invalid payload can cause marshaling error or network error
			assert.True(t, strings.Contains(err.Error(), "failed to marshal comment payload") ||
				strings.Contains(err.Error(), "failed to send request") ||
				strings.Contains(err.Error(), "jira API error"),
				"Expected marshaling or network error, got: %s", err.Error())
		}
	})
}

// Helper function to create test comment payload
func createTestCommentPayload(text string) CommentPayload {
	return CommentPayload{
		Body: CommentBody{
			Type:    "doc",
			Version: 1,
			Content: []Content{
				{
					Type: "paragraph",
					Content: []TextContent{
						{
							Type: "text",
							Text: text,
						},
					},
				},
			},
		},
	}
}

// Helper function to encode base64
func base64Encode(s string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(s))
}

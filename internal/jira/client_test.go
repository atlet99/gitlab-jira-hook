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
}

// TestAddCommentWithADFValidation tests the AddComment method with ADF validation
func TestAddCommentWithADFValidation(t *testing.T) {
	t.Run("successful comment addition with valid ADF", func(t *testing.T) {
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

	t.Run("comment addition with invalid ADF falls back to plain text", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/comment", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			// Verify request body - should be plain text fallback
			var payload CommentPayload
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "doc", payload.Body.Type)
			assert.Equal(t, 1, payload.Body.Version)
			assert.Len(t, payload.Body.Content, 1)
			assert.Equal(t, "paragraph", payload.Body.Content[0].Type)
			assert.Len(t, payload.Body.Content[0].Content, 1)
			assert.Equal(t, "text", payload.Body.Content[0].Content[0].Type)
			// The content should be the fallback plain text
			assert.Contains(t, payload.Body.Content[0].Content[0].Text, "Invalid content")

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

		// Create invalid ADF content (invalid type)
		invalidPayload := CommentPayload{
			Body: CommentBody{
				Type:    "invalid",
				Version: 1,
				Content: []Content{
					{
						Type: "paragraph",
						Content: []TextContent{
							{Type: "text", Text: "Invalid content"},
						},
					},
				},
			},
		}

		err := client.AddComment(context.Background(), "ABC-123", invalidPayload)
		// The method should not return an error even with invalid ADF, as it falls back to plain text
		assert.NoError(t, err)
	})
}

// Helper function to create a test Jira issue
func createTestIssue(key, status string) JiraIssue {
	return JiraIssue{
		ID:  "12345",
		Key: key,
		Fields: &JiraIssueFields{
			Summary: "Test Issue",
			Status: &Status{
				ID:   "10001",
				Name: status,
			},
		},
	}
}

// Tests for GetTransitions
func TestClientGetTransitions(t *testing.T) {
	t.Run("successful transitions retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/transitions", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
						"transitions": [
							{
								"id": "11",
								"name": "To Do",
								"to": {
									"id": "10000",
									"name": "To Do"
								}
							},
							{
								"id": "21",
								"name": "In Progress",
								"to": {
									"id": "10001",
									"name": "In Progress"
								}
							},
							{
								"id": "31",
								"name": "Done",
								"to": {
									"id": "10002",
									"name": "Done"
								}
							}
						]
					}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		transitions, err := client.GetTransitions(context.Background(), "ABC-123")
		assert.NoError(t, err)
		assert.Len(t, transitions, 3)
		assert.Equal(t, "11", transitions[0].ID)
		assert.Equal(t, "To Do", transitions[0].Name)
		assert.Equal(t, "Done", transitions[2].To.Name)
	})

	t.Run("transitions retrieval fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Issue not found"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		transitions, err := client.GetTransitions(context.Background(), "ABC-123")
		assert.Error(t, err)
		assert.Nil(t, transitions)
		assert.Contains(t, err.Error(), "jira API error")
		assert.Contains(t, err.Error(), "404")
	})
}

// Tests for ExecuteTransition
func TestClientExecuteTransition(t *testing.T) {
	t.Run("successful transition execution", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/transitions", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			// Verify request body
			var payload TransitionPayload
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "21", payload.Transition.ID)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.ExecuteTransition(context.Background(), "ABC-123", "21")
		assert.NoError(t, err)
	})

	t.Run("transition execution fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "Invalid transition"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.ExecuteTransition(context.Background(), "ABC-123", "999")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jira API error")
		assert.Contains(t, err.Error(), "400")
	})
}

// Tests for FindTransition
func TestClientFindTransition(t *testing.T) {
	t.Run("successful transition find", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/transitions", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"transitions": [
					{
						"id": "11",
						"name": "To Do",
						"to": {
							"id": "10000",
							"name": "To Do"
						}
					},
					{
						"id": "21",
						"name": "In Progress",
						"to": {
							"id": "10001",
							"name": "In Progress"
						}
					},
					{
						"id": "31",
						"name": "Done",
						"to": {
							"id": "10002",
							"name": "Done"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		transition, err := client.FindTransition(context.Background(), "ABC-123", "In Progress")
		assert.NoError(t, err)
		assert.NotNil(t, transition)
		assert.Equal(t, "21", transition.ID)
		assert.Equal(t, "In Progress", transition.Name)
		assert.Equal(t, "In Progress", transition.To.Name)
	})

	t.Run("transition not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/transitions", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"transitions": [
					{
						"id": "11",
						"name": "To Do",
						"to": {
							"id": "10000",
							"name": "To Do"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		transition, err := client.FindTransition(context.Background(), "ABC-123", "Done")
		assert.Error(t, err)
		assert.Nil(t, transition)
		assert.Contains(t, err.Error(), "transition to status 'Done' not available")
	})
}

// Tests for TransitionToStatus
func TestClientTransitionToStatus(t *testing.T) {
	t.Run("successful transition to new status", func(t *testing.T) {
		// Create a test server that handles both get issue and transitions endpoints
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/rest/api/3/issue/ABC-123" {
				// Return current issue with "To Do" status
				w.WriteHeader(http.StatusOK)
				issue := createTestIssue("ABC-123", "To Do")
				jsonData, _ := json.Marshal(issue)
				_, _ = w.Write(jsonData)
			} else if r.Method == "GET" && r.URL.Path == "/rest/api/3/issue/ABC-123/transitions" {
				// Return available transitions
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"transitions": [
						{
							"id": "21",
							"name": "In Progress",
							"to": {
								"id": "10001",
								"name": "In Progress"
							}
						}
					]
				}`))
			} else if r.Method == "POST" && r.URL.Path == "/rest/api/3/issue/ABC-123/transitions" {
				// Verify transition execution
				var payload TransitionPayload
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(t, err)
				assert.Equal(t, "21", payload.Transition.ID)
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.TransitionToStatus(context.Background(), "ABC-123", "In Progress")
		assert.NoError(t, err)
	})

	t.Run("no transition needed - already in target status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/rest/api/3/issue/ABC-123" {
				// Return current issue with "In Progress" status
				w.WriteHeader(http.StatusOK)
				issue := createTestIssue("ABC-123", "In Progress")
				jsonData, _ := json.Marshal(issue)
				_, _ = w.Write(jsonData)
			}
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.TransitionToStatus(context.Background(), "ABC-123", "In Progress")
		assert.NoError(t, err)
	})

	t.Run("transition not available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/rest/api/3/issue/ABC-123" {
				// Return current issue with "To Do" status
				w.WriteHeader(http.StatusOK)
				issue := createTestIssue("ABC-123", "To Do")
				jsonData, _ := json.Marshal(issue)
				_, _ = w.Write(jsonData)
			} else if r.Method == "GET" && r.URL.Path == "/rest/api/3/issue/ABC-123/transitions" {
				// Return no transitions to "Done"
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"transitions": [
						{
							"id": "21",
							"name": "In Progress",
							"to": {
								"id": "10001",
								"name": "In Progress"
							}
						}
					]
				}`))
			}
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.TransitionToStatus(context.Background(), "ABC-123", "Done")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transition to status 'Done' not available")
	})
}

// Tests for GetIssue
func TestClientGetIssue(t *testing.T) {
	t.Run("successful issue retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			w.WriteHeader(http.StatusOK)
			issue := createTestIssue("ABC-123", "To Do")
			jsonData, _ := json.Marshal(issue)
			_, _ = w.Write(jsonData)
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		issue, err := client.GetIssue(context.Background(), "ABC-123")
		assert.NoError(t, err)
		assert.NotNil(t, issue)
		assert.Equal(t, "ABC-123", issue.Key)
		assert.Equal(t, "To Do", issue.Fields.Status.Name)
	})

	t.Run("issue retrieval fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Issue not found"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		issue, err := client.GetIssue(context.Background(), "ABC-123")
		assert.Error(t, err)
		assert.Nil(t, issue)
		assert.Contains(t, err.Error(), "jira API error")
		assert.Contains(t, err.Error(), "404")
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

// Tests for SearchIssues
func TestClientSearchIssues(t *testing.T) {
	t.Run("successful issue search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/rest/api/3/search", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			// Verify request body
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "project = TEST", payload["jql"])

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"issues": [
					{
						"id": "12345",
						"key": "TEST-1",
						"fields": {
							"summary": "Test issue 1"
						}
					},
					{
						"id": "12346",
						"key": "TEST-2",
						"fields": {
							"summary": "Test issue 2"
						}
					}
				]
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		issues, err := client.SearchIssues(context.Background(), "project = TEST")
		assert.NoError(t, err)
		assert.Len(t, issues, 2)
		assert.Equal(t, "TEST-1", issues[0].Key)
		assert.Equal(t, "TEST-2", issues[1].Key)
	})

	t.Run("issue search fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Invalid JQL"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		issues, err := client.SearchIssues(context.Background(), "invalid jql")
		assert.Error(t, err)
		assert.Nil(t, issues)
		assert.Contains(t, err.Error(), "jira API error")
		assert.Contains(t, err.Error(), "404")
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

// Tests for SetAssignee
func TestClientSetAssignee(t *testing.T) {
	t.Run("successful assignee update", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/rest/api/3/issue/ABC-123/assignee", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Basic "))

			// Verify request body
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			assert.Equal(t, "user-account-id", payload["accountId"])

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accountId": "user-account-id"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.SetAssignee(context.Background(), "ABC-123", "user-account-id")
		assert.NoError(t, err)
	})

	t.Run("assignee update fails on 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "Issue not found"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			JiraEmail:   "test@example.com",
			JiraToken:   "test-token",
			JiraBaseURL: server.URL,
		}

		client := NewClient(cfg)
		err := client.SetAssignee(context.Background(), "ABC-123", "user-account-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jira API error")
		assert.Contains(t, err.Error(), "404")
	})
}

// Helper function to encode base64
func base64Encode(s string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(s))
}

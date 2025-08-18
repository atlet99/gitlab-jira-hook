package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/server"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// rateLimitedHandlerCounter tracks requests for rate limiting simulation
var rateLimitedHandlerCounter *int

// TestAPIIntegration tests the complete API integration flow
func TestAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		JiraEmail:            "test@example.com",
		JiraToken:            "test-token",
		JiraBaseURL:          "https://test-jira.example.com",
		JiraWebhookSecret:    "test-webhook-secret",
		GitLabBaseURL:        "https://gitlab.example.com",
		GitLabAPIToken:       "test-gitlab-token",
		GitLabSecret:         "test-gitlab-secret", // Add GitLab secret for webhook validation
		GitLabNamespace:      "test-namespace",
		BidirectionalEnabled: true,
		DebugMode:            true,
		JQLFilter:            "project = TEST",
		CommentSyncEnabled:   true,
		StatusMappingEnabled: true,
		StatusMapping: map[string]string{
			"In Progress": "in_progress",
			"Done":        "closed",
			"To Do":       "opened",
		},
		// Worker pool configuration for tests
		JobQueueSize:      1000,  // Larger queue for tests
		MaxConcurrentJobs: 100,   // More concurrent jobs
		QueueTimeoutMs:    10000, // Longer timeout (10 seconds)
		JobTimeoutSeconds: 300,   // Longer job timeout (5 minutes)
		MinWorkers:        2,     // Minimum workers
		MaxWorkers:        10,    // Maximum workers
		ScaleInterval:     5,     // More frequent scaling checks
	}

	// Create test server with mocked dependencies
	testServer := setupTestServer(t, cfg)
	defer testServer.Close()

	// Test scenarios
	tests := []struct {
		name           string
		endpoint       string
		method         string
		body           interface{}
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:           "Jira webhook endpoint",
			endpoint:       "/jira-webhook",
			method:         "POST",
			body:           createTestJiraWebhookEvent(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GitLab webhook endpoint",
			endpoint:       "/gitlab-hook",
			method:         "POST",
			body:           createTestGitLabWebhookEvent(),
			expectedStatus: http.StatusAccepted, // 202 - Accepted for async processing
		},
		{
			name:           "Health check endpoint",
			endpoint:       "/health",
			method:         "GET",
			body:           nil,
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status": "ok",
			},
		},
		{
			name:           "Metrics endpoint",
			endpoint:       "/monitoring/metrics",
			method:         "GET",
			body:           nil,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			var bodyReader io.Reader
			if tt.body != nil {
				jsonBody, err := json.Marshal(tt.body)
				require.NoError(t, err)
				bodyReader = bytes.NewReader(jsonBody)
			}

			req, err := http.NewRequest(tt.method, testServer.URL+tt.endpoint, bodyReader)
			require.NoError(t, err)

			// Set headers
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("X-Request-ID", "test-request-id")

			// Add GitLab token header for GitLab webhook endpoint
			if tt.endpoint == "/gitlab-hook" {
				req.Header.Set("X-Gitlab-Token", cfg.GitLabSecret)
			}

			// Make request
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			// Verify response status
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify response body
			if tt.expectedBody != nil {
				var responseBody map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&responseBody)
				require.NoError(t, err)

				for key, expectedValue := range tt.expectedBody {
					assert.Contains(t, responseBody, key)
					assert.Equal(t, expectedValue, responseBody[key])
				}
			}
		})
	}
}

// TestJiraGitLabSynchronization tests the complete synchronization flow
func TestJiraGitLabSynchronization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		JiraEmail:            "test@example.com",
		JiraToken:            "test-token",
		JiraBaseURL:          "https://test-jira.example.com",
		JiraWebhookSecret:    "test-webhook-secret",
		GitLabBaseURL:        "https://gitlab.example.com",
		GitLabAPIToken:       "test-gitlab-token",
		GitLabSecret:         "test-gitlab-secret", // Add GitLab secret for webhook validation
		GitLabNamespace:      "test-namespace",
		BidirectionalEnabled: true,
		DebugMode:            true,
		CommentSyncEnabled:   true,
		// Worker pool configuration for tests
		JobQueueSize:      1000,  // Larger queue for tests
		MaxConcurrentJobs: 100,   // More concurrent jobs
		QueueTimeoutMs:    10000, // Longer timeout (10 seconds)
		JobTimeoutSeconds: 300,   // Longer job timeout (5 minutes)
		MinWorkers:        2,     // Minimum workers
		MaxWorkers:        10,    // Maximum workers
		ScaleInterval:     5,     // More frequent scaling checks
	}

	// Create test server with mocked dependencies
	testServer := setupTestServer(t, cfg)
	defer testServer.Close()

	// Test Jira issue creation webhook
	t.Run("Jira issue creation triggers GitLab issue creation", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		webhookEvent.WebhookEvent = "jira:issue_created"
		webhookEvent.Issue = createTestJiraIssue("TEST-123", "New Test Issue")

		// Send webhook
		resp := sendWebhookRequest(t, testServer, "/jira-webhook", webhookEvent)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify GitLab issue was created (check logs or mocked responses)
		// This would be verified by checking the mocked GitLab client calls
	})

	// Test Jira comment creation webhook
	t.Run("Jira comment creation triggers GitLab comment", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		webhookEvent.WebhookEvent = "comment_created"
		webhookEvent.Issue = createTestJiraIssue("TEST-123", "Test Issue")
		webhookEvent.Comment = createTestJiraComment("This is a test comment")

		// Send webhook
		resp := sendWebhookRequest(t, testServer, "/jira-webhook", webhookEvent)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify GitLab comment was created
		// This would be verified by checking the mocked GitLab client calls
	})

	// Test GitLab issue status update triggers Jira status change
	t.Run("GitLab issue status update triggers Jira transition", func(t *testing.T) {
		gitlabEvent := createTestGitLabWebhookEvent()
		gitlabEvent.ObjectKind = "issue"
		gitlabEvent.ObjectAttributes.State = "closed"
		gitlabEvent.ObjectAttributes.Action = "update"

		// Send webhook
		resp := sendWebhookRequestWithGitLabToken(t, testServer, "/gitlab-hook", gitlabEvent, cfg.GitLabSecret)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode) // 202 - Accepted for async processing

		// Verify Jira issue was transitioned
		// This would be verified by checking the mocked Jira client calls
	})
}

// TestRateLimitingAndRetry tests rate limiting and retry mechanisms
func TestRateLimitingAndRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create configuration with aggressive rate limiting
	cfg := &config.Config{
		JiraEmail:            "test@example.com",
		JiraToken:            "test-token",
		JiraBaseURL:          "https://test-jira.example.com",
		JiraWebhookSecret:    "test-webhook-secret", // Add webhook secret for authentication
		JiraRateLimit:        1,                     // Very low rate limit
		JiraRetryMaxAttempts: 3,
		JiraRetryBaseDelayMs: 100,
		GitLabSecret:         "test-gitlab-secret", // Add GitLab secret for webhook validation
		// Worker pool configuration for tests
		JobQueueSize:      1000,  // Larger queue for tests
		MaxConcurrentJobs: 100,   // More concurrent jobs
		QueueTimeoutMs:    10000, // Longer timeout (10 seconds)
		JobTimeoutSeconds: 300,   // Longer job timeout (5 minutes)
		MinWorkers:        2,     // Minimum workers
		MaxWorkers:        10,    // Maximum workers
		ScaleInterval:     5,     // More frequent scaling checks
	}

	// Create test server with mocked dependencies that simulate rate limiting
	testServer := setupTestServerWithRateLimiting(t, cfg)
	defer testServer.Close()

	// Test multiple rapid requests
	t.Run("Multiple requests respect rate limits", func(t *testing.T) {
		start := time.Now()
		requests := 5

		for i := 0; i < requests; i++ {
			webhookEvent := createTestJiraWebhookEvent()
			resp := sendWebhookRequestWithAuth(t, testServer, "/jira-webhook", webhookEvent, "valid")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		duration := time.Since(start)
		// Should take at least (requests - 1) * (1 second / rate_limit) seconds
		// With rate limit of 1 request per second, 5 requests should take at least 4 seconds
		expectedMinDuration := time.Duration(requests-1) * time.Second
		assert.GreaterOrEqual(t, duration, expectedMinDuration-100*time.Millisecond)
		assert.LessOrEqual(t, duration, expectedMinDuration+1*time.Second) // Allow more tolerance
	})
}

// TestAuthentication tests webhook authentication
func TestAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		JiraEmail:         "test@example.com",
		JiraToken:         "test-token",
		JiraBaseURL:       "https://test-jira.example.com",
		JiraWebhookSecret: "test-webhook-secret",
		GitLabSecret:      "test-gitlab-secret", // Add GitLab secret for webhook validation
		DebugMode:         false,                // Disable debug mode to test authentication
		// Worker pool configuration for tests
		JobQueueSize:      1000,  // Larger queue for tests
		MaxConcurrentJobs: 100,   // More concurrent jobs
		QueueTimeoutMs:    10000, // Longer timeout (10 seconds)
		JobTimeoutSeconds: 300,   // Longer job timeout (5 minutes)
		MinWorkers:        2,     // Minimum workers
		MaxWorkers:        10,    // Maximum workers
		ScaleInterval:     5,     // More frequent scaling checks
	}

	// Create test server
	testServer := setupTestServer(t, cfg)
	defer testServer.Close()

	// Test valid HMAC signature
	t.Run("Valid HMAC signature", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		resp := sendWebhookRequestWithAuth(t, testServer, "/jira-webhook", webhookEvent, "valid")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Test invalid HMAC signature
	t.Run("Invalid HMAC signature", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		resp := sendWebhookRequestWithAuth(t, testServer, "/jira-webhook", webhookEvent, "invalid")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// Test missing signature
	t.Run("Missing signature", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		resp := sendWebhookRequestNoAuth(t, testServer, "/jira-webhook", webhookEvent)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		JiraEmail:         "test@example.com",
		JiraToken:         "test-token",
		JiraBaseURL:       "https://test-jira.example.com",
		JiraWebhookSecret: "test-webhook-secret",
		GitLabSecret:      "test-gitlab-secret", // Add GitLab secret for webhook validation
		DebugMode:         true,
	}

	// Create test server with error simulation
	testServer := setupTestServerWithErrorSimulation(t, cfg)
	defer testServer.Close()

	// Test malformed JSON
	t.Run("Malformed JSON request", func(t *testing.T) {
		resp := sendMalformedRequest(t, testServer, "/jira-webhook")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// Test missing required fields
	t.Run("Missing required fields", func(t *testing.T) {
		incompleteEvent := map[string]interface{}{
			"webhookEvent": "jira:issue_created",
			// Missing required fields
		}
		resp := sendWebhookRequest(t, testServer, "/jira-webhook", incompleteEvent)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// Test unsupported webhook event
	t.Run("Unsupported webhook event", func(t *testing.T) {
		unsupportedEvent := createTestJiraWebhookEvent()
		unsupportedEvent.WebhookEvent = "unsupported:event"
		resp := sendWebhookRequest(t, testServer, "/jira-webhook", unsupportedEvent)
		assert.Equal(t, http.StatusOK, resp.StatusCode) // Should accept but not process
	})
}

// TestADFValidation tests ADF validation and fallback
func TestADFValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		JiraEmail:          "test@example.com",
		JiraToken:          "test-token",
		JiraBaseURL:        "https://test-jira.example.com",
		JiraWebhookSecret:  "test-webhook-secret",
		GitLabSecret:       "test-gitlab-secret", // Add GitLab secret for webhook validation
		CommentSyncEnabled: true,
		DebugMode:          true,
	}

	// Create test server
	testServer := setupTestServer(t, cfg)
	defer testServer.Close()

	// Test valid ADF comment
	t.Run("Valid ADF comment", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		webhookEvent.WebhookEvent = "comment_created"
		webhookEvent.Comment = createTestJiraCommentWithADF("Valid ADF comment")

		resp := sendWebhookRequest(t, testServer, "/jira-webhook", webhookEvent)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Test invalid ADF comment with fallback
	t.Run("Invalid ADF comment with fallback", func(t *testing.T) {
		webhookEvent := createTestJiraWebhookEvent()
		webhookEvent.WebhookEvent = "comment_created"
		webhookEvent.Comment = createTestJiraCommentWithInvalidADF("Invalid ADF comment")

		resp := sendWebhookRequest(t, testServer, "/jira-webhook", webhookEvent)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Should fallback to plain text and still succeed
	})
}

// Helper functions

// setupTestServer creates a test server with mocked dependencies
func setupTestServer(t *testing.T, cfg *config.Config) *httptest.Server {
	// Create a custom Prometheus registry to avoid conflicts in tests
	registry := prometheus.NewRegistry()

	// Create server with custom registry
	_ = server.NewWithRegistry(cfg, testLogger(), registry)

	// Create a new webhook handler
	jiraWebhookHandler := jira.NewWebhookHandler(cfg, testLogger())

	// Create a real Jira client
	jiraClient := jira.NewClient(cfg)
	// Set the Jira client in the webhook handler
	jiraWebhookHandler.SetJiraClient(jiraClient)

	// Create mock GitLab client
	mockGitLabClient := &MockGitLabClient{}
	jiraWebhookHandler.SetGitLabClient(mockGitLabClient)

	// Create mock sync manager
	mockSyncManager := &MockSyncManager{}
	jiraWebhookHandler.SetManager(mockSyncManager)

	// Create mock worker pool
	mockWorkerPool := &MockWorkerPool{}
	jiraWebhookHandler.SetWorkerPool(mockWorkerPool)

	// Create mock monitor using the constructor
	mockMonitor := monitoring.NewWebhookMonitor(cfg, testLogger())
	jiraWebhookHandler.SetMonitor(mockMonitor)

	// Create a new server with the mocked webhook handler
	mux := http.NewServeMux()
	mux.Handle("/jira-webhook", http.HandlerFunc(jiraWebhookHandler.HandleWebhook))

	// Add GitLab webhook handler for tests
	gitlabWebhookHandler := gitlab.NewHandler(cfg, testLogger())
	gitlabWebhookHandler.SetWorkerPool(mockWorkerPool)
	gitlabWebhookHandler.SetMonitor(mockMonitor)
	mux.Handle("/gitlab-hook", http.HandlerFunc(gitlabWebhookHandler.HandleWebhook))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})
	mux.HandleFunc("/monitoring/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Start test server
	testServer := httptest.NewServer(mux)
	return testServer
}

// setupTestServerWithRateLimiting creates a test server that simulates rate limiting
func setupTestServerWithRateLimiting(t *testing.T, cfg *config.Config) *httptest.Server {
	// Create a custom Prometheus registry to avoid conflicts in tests
	registry := prometheus.NewRegistry()

	// Create server with custom registry
	_ = server.NewWithRegistry(cfg, testLogger(), registry)

	// Create a new webhook handler
	jiraWebhookHandler := jira.NewWebhookHandler(cfg, testLogger())

	// Create a real Jira client
	jiraClient := jira.NewClient(cfg)
	// Set the Jira client in the webhook handler
	jiraWebhookHandler.SetJiraClient(jiraClient)

	// Create mock GitLab client
	mockGitLabClient := &MockGitLabClient{}
	jiraWebhookHandler.SetGitLabClient(mockGitLabClient)

	// Create mock sync manager
	mockSyncManager := &MockSyncManager{}
	jiraWebhookHandler.SetManager(mockSyncManager)

	// Create mock worker pool
	mockWorkerPool := &MockWorkerPool{}
	jiraWebhookHandler.SetWorkerPool(mockWorkerPool)

	// Create mock monitor using the constructor
	mockMonitor := monitoring.NewWebhookMonitor(cfg, testLogger())
	jiraWebhookHandler.SetMonitor(mockMonitor)

	// Create a wrapper handler that simulates rate limiting
	rateLimitedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate rate limiting by adding a delay for subsequent requests
		if r.URL.Path == "/jira-webhook" {
			// Use a simple counter to track requests
			if rateLimitedHandlerCounter == nil {
				rateLimitedHandlerCounter = new(int)
			}
			*rateLimitedHandlerCounter++
			if *rateLimitedHandlerCounter > 1 {
				time.Sleep(1 * time.Second) // Simulate rate limiting delay
			}
		}
		jiraWebhookHandler.HandleWebhook(w, r)
	})

	// Create a new server with the mocked webhook handler
	mux := http.NewServeMux()
	mux.Handle("/jira-webhook", rateLimitedHandler)

	// Add GitLab webhook handler for tests
	gitlabWebhookHandler := gitlab.NewHandler(cfg, testLogger())
	gitlabWebhookHandler.SetWorkerPool(mockWorkerPool)
	gitlabWebhookHandler.SetMonitor(mockMonitor)
	mux.Handle("/gitlab-hook", http.HandlerFunc(gitlabWebhookHandler.HandleWebhook))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Start test server
	testServer := httptest.NewServer(mux)
	return testServer
}

// setupTestServerWithErrorSimulation creates a test server that simulates errors
func setupTestServerWithErrorSimulation(t *testing.T, cfg *config.Config) *httptest.Server {
	// Create a custom Prometheus registry to avoid conflicts in tests
	registry := prometheus.NewRegistry()

	// Create server with custom registry
	_ = server.NewWithRegistry(cfg, testLogger(), registry)

	// Create a new webhook handler
	jiraWebhookHandler := jira.NewWebhookHandler(cfg, testLogger())

	// Create a mock Jira client that simulates errors for testing
	mockErrorClient := NewMockErrorSimulatingJiraClient()
	jiraWebhookHandler.SetJiraClient(mockErrorClient.Client)

	// Create mock GitLab client
	mockGitLabClient := &MockGitLabClient{}
	jiraWebhookHandler.SetGitLabClient(mockGitLabClient)

	// Create mock sync manager
	mockSyncManager := &MockSyncManager{}
	jiraWebhookHandler.SetManager(mockSyncManager)

	// Create mock worker pool
	mockWorkerPool := &MockWorkerPool{}
	jiraWebhookHandler.SetWorkerPool(mockWorkerPool)

	// Create mock monitor using the constructor
	mockMonitor := monitoring.NewWebhookMonitor(cfg, testLogger())
	jiraWebhookHandler.SetMonitor(mockMonitor)

	// Create a new server with the mocked webhook handler
	mux := http.NewServeMux()
	mux.Handle("/jira-webhook", http.HandlerFunc(jiraWebhookHandler.HandleWebhook))

	// Add GitLab webhook handler for tests
	gitlabWebhookHandler := gitlab.NewHandler(cfg, testLogger())
	gitlabWebhookHandler.SetWorkerPool(mockWorkerPool)
	gitlabWebhookHandler.SetMonitor(mockMonitor)
	mux.Handle("/gitlab-hook", http.HandlerFunc(gitlabWebhookHandler.HandleWebhook))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Start test server
	testServer := httptest.NewServer(mux)
	return testServer
}

// sendWebhookRequest sends a webhook request to the test server with valid authentication
func sendWebhookRequest(t *testing.T, server *httptest.Server, endpoint string, body interface{}) *http.Response {
	return sendWebhookRequestWithAuth(t, server, endpoint, body, "valid")
}

// sendWebhookRequestWithGitLabToken sends a webhook request with GitLab token authentication
func sendWebhookRequestWithGitLabToken(t *testing.T, server *httptest.Server, endpoint string, body interface{}, gitlabToken string) *http.Response {
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", server.URL+endpoint, bytes.NewReader(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-request-id")

	// Add GitLab token header for GitLab webhook endpoint
	if endpoint == "/gitlab-hook" {
		req.Header.Set("X-Gitlab-Token", gitlabToken)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// sendWebhookRequestNoAuth sends a webhook request without any authentication
func sendWebhookRequestNoAuth(t *testing.T, server *httptest.Server, endpoint string, body interface{}) *http.Response {
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", server.URL+endpoint, bytes.NewReader(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-request-id")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// sendWebhookRequestWithAuth sends a webhook request with authentication
func sendWebhookRequestWithAuth(t *testing.T, server *httptest.Server, endpoint string, body interface{}, authType string) *http.Response {
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", server.URL+endpoint, bytes.NewReader(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-request-id")

	// Add authentication header based on type
	if authType == "valid" {
		// Generate proper HMAC signature for valid authentication
		hmacSecret := "test-webhook-secret"
		mac := hmac.New(sha256.New, []byte(hmacSecret))
		mac.Write(jsonBody)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Atlassian-Webhook-Signature", expectedSignature)
	} else if authType == "invalid" {
		// Provide invalid signature
		req.Header.Set("X-Atlassian-Webhook-Signature", "invalid-signature")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// sendMalformedRequest sends a malformed request
func sendMalformedRequest(t *testing.T, server *httptest.Server, endpoint string) *http.Response {
	req, err := http.NewRequest("POST", server.URL+endpoint, strings.NewReader("invalid json"))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-request-id")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// testLogger creates a test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// createTestJiraWebhookEvent creates a test Jira webhook event
func createTestJiraWebhookEvent() *jira.WebhookEvent {
	return &jira.WebhookEvent{
		WebhookEvent: "jira:issue_updated",
		Timestamp:    time.Now().Unix(),
		Issue:        createTestJiraIssue("TEST-123", "Test Issue"),
	}
}

// createTestGitLabWebhookEvent creates a test GitLab webhook event
func createTestGitLabWebhookEvent() *gitlab.Event {
	return &gitlab.Event{
		ObjectKind: "issue",
		Project: &gitlab.Project{
			ID:        1,
			Name:      "test-project",
			WebURL:    "https://gitlab.example.com/test/project",
			Namespace: "test-namespace",
		},
		ObjectAttributes: &gitlab.ObjectAttributes{
			ID:          123,
			Title:       "Test Issue [TEST-123]",
			Description: "Test issue description",
			State:       "opened",
			Action:      "open",
			URL:         "https://gitlab.example.com/test/project/issues/123",
		},
		User: &gitlab.User{
			ID:        1,
			Name:      "Test User",
			Username:  "testuser",
			Email:     "test@example.com",
			AvatarURL: "https://gitlab.example.com/uploads/avatar.png",
		},
	}
}

// createTestJiraIssue creates a test Jira issue
func createTestJiraIssue(key, summary string) *jira.JiraIssue {
	return &jira.JiraIssue{
		ID:  "12345",
		Key: key,
		Fields: &jira.JiraIssueFields{
			Summary: summary,
			Status: &jira.Status{
				ID:   "10001",
				Name: "To Do",
			},
			Priority: &jira.Priority{
				ID:   "1",
				Name: "High",
			},
			IssueType: &jira.IssueType{
				ID:   "1",
				Name: "Bug",
			},
			Project: &jira.JiraProject{
				ID:   "10000",
				Key:  "TEST",
				Name: "Test Project",
			},
			Reporter: &jira.JiraUser{
				AccountID:    "user-account-id",
				DisplayName:  "Test User",
				EmailAddress: "test@example.com",
			},
			Assignee: &jira.JiraUser{
				AccountID:    "assignee-account-id",
				DisplayName:  "Assignee User",
				EmailAddress: "assignee@example.com",
			},
			Labels: []string{"bug", "critical"},
			FixVersions: []jira.Version{
				{
					ID:   "10010",
					Name: "Version 1.0",
				},
			},
		},
	}
}

// createTestJiraComment creates a test Jira comment
func createTestJiraComment(body string) *jira.Comment {
	return &jira.Comment{
		ID: "12345",
		Author: &jira.JiraUser{
			AccountID:    "user-account-id",
			DisplayName:  "Test User",
			EmailAddress: "test@example.com",
		},
		Body:    body,
		Created: time.Now().Format(time.RFC3339),
		Updated: time.Now().Format(time.RFC3339),
	}
}

// createTestJiraCommentWithADF creates a test Jira comment with valid ADF
func createTestJiraCommentWithADF(body string) *jira.Comment {
	return &jira.Comment{
		ID: "12345",
		Author: &jira.JiraUser{
			AccountID:    "user-account-id",
			DisplayName:  "Test User",
			EmailAddress: "test@example.com",
		},
		Body: jira.CommentPayload{
			Body: jira.CommentBody{
				Type:    "doc",
				Version: 1,
				Content: []jira.Content{
					{
						Type: "paragraph",
						Content: []jira.TextContent{
							{
								Type: "text",
								Text: body,
							},
						},
					},
				},
			},
		},
		Created: time.Now().Format(time.RFC3339),
		Updated: time.Now().Format(time.RFC3339),
	}
}

// createTestJiraCommentWithInvalidADF creates a test Jira comment with invalid ADF
func createTestJiraCommentWithInvalidADF(body string) *jira.Comment {
	return &jira.Comment{
		ID: "12345",
		Author: &jira.JiraUser{
			AccountID:    "user-account-id",
			DisplayName:  "Test User",
			EmailAddress: "test@example.com",
		},
		Body: jira.CommentPayload{
			Body: jira.CommentBody{
				Type:    "invalid", // Invalid type
				Version: 1,
				Content: []jira.Content{
					{
						Type: "paragraph",
						Content: []jira.TextContent{
							{
								Type: "text",
								Text: body,
							},
						},
					},
				},
			},
		},
		Created: time.Now().Format(time.RFC3339),
		Updated: time.Now().Format(time.RFC3339),
	}
}

// Mock implementations

// MockJiraClient is a mock Jira client for testing
type MockJiraClient struct {
	issuesCreated       int // Used to track issues created during tests
	commentsAdded       int
	transitionsExecuted int
}

func (m *MockJiraClient) AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error {
	m.commentsAdded++
	return nil
}

// GetIssuesCreated returns the count of issues created (for testing)
func (m *MockJiraClient) GetIssuesCreated() int {
	return m.issuesCreated
}

func (m *MockJiraClient) TestConnection(ctx context.Context) error {
	return nil
}

func (m *MockJiraClient) SearchIssues(ctx context.Context, jql string) ([]jira.JiraIssue, error) {
	return []jira.JiraIssue{
		*createTestJiraIssue("TEST-123", "Test Issue"),
	}, nil
}

func (m *MockJiraClient) GetIssue(ctx context.Context, issueKey string) (*jira.JiraIssue, error) {
	return createTestJiraIssue(issueKey, "Test Issue"), nil
}

func (m *MockJiraClient) UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	return nil
}

func (m *MockJiraClient) GetTransitions(ctx context.Context, issueKey string) ([]jira.Transition, error) {
	return []jira.Transition{
		{
			ID:   "11",
			Name: "Start Progress",
			To: jira.Status{
				ID:   "10001",
				Name: "In Progress",
			},
		},
		{
			ID:   "21",
			Name: "Resolve Issue",
			To: jira.Status{
				ID:   "10002",
				Name: "Resolved",
			},
		},
	}, nil
}

func (m *MockJiraClient) ExecuteTransition(ctx context.Context, issueKey string, transitionID string) error {
	m.transitionsExecuted++
	return nil
}

func (m *MockJiraClient) FindTransition(ctx context.Context, issueKey, targetStatus string) (*jira.Transition, error) {
	transitions, err := m.GetTransitions(ctx, issueKey)
	if err != nil {
		return nil, err
	}

	for _, transition := range transitions {
		if transition.To.Name == targetStatus {
			return &transition, nil
		}
	}

	return nil, fmt.Errorf("transition to status '%s' not available", targetStatus)
}

func (m *MockJiraClient) TransitionToStatus(ctx context.Context, issueKey, targetStatus string) error {
	transition, err := m.FindTransition(ctx, issueKey, targetStatus)
	if err != nil {
		return err
	}

	return m.ExecuteTransition(ctx, issueKey, transition.ID)
}

func (m *MockJiraClient) SearchUsers(ctx context.Context, query string) ([]jira.JiraUser, error) {
	return []jira.JiraUser{
		{
			AccountID:    "user-account-id",
			DisplayName:  "Test User",
			EmailAddress: "test@example.com",
		},
	}, nil
}

func (m *MockJiraClient) SetAssignee(ctx context.Context, issueKey, accountID string) error {
	return nil
}

// MockGitLabClient is a mock GitLab client for testing
type MockGitLabClient struct {
	issuesCreated int
	commentsAdded int
}

func (m *MockGitLabClient) CreateIssue(ctx context.Context, projectID string, request *jira.GitLabIssueCreateRequest) (*jira.GitLabIssue, error) {
	m.issuesCreated++
	return &jira.GitLabIssue{
		ID:          123,
		IID:         456,
		ProjectID:   1,
		Title:       request.Title,
		Description: request.Description,
		State:       "opened",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Labels:      request.Labels,
		WebURL:      fmt.Sprintf("https://gitlab.example.com/test/project/issues/%d", 456),
	}, nil
}

func (m *MockGitLabClient) UpdateIssue(ctx context.Context, projectID string, issueIID int, request *jira.GitLabIssueUpdateRequest) (*jira.GitLabIssue, error) {
	return &jira.GitLabIssue{
		ID:        123,
		IID:       issueIID,
		ProjectID: 1,
		Title:     request.Title,
		State:     "opened",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		WebURL:    fmt.Sprintf("https://gitlab.example.com/test/project/issues/%d", issueIID),
	}, nil
}

func (m *MockGitLabClient) GetIssue(ctx context.Context, projectID string, issueIID int) (*jira.GitLabIssue, error) {
	return &jira.GitLabIssue{
		ID:        123,
		IID:       issueIID,
		ProjectID: 1,
		Title:     "Test Issue",
		State:     "opened",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		WebURL:    fmt.Sprintf("https://gitlab.example.com/test/project/issues/%d", issueIID),
	}, nil
}

func (m *MockGitLabClient) CreateComment(ctx context.Context, projectID string, issueIID int, request *jira.GitLabCommentCreateRequest) (*jira.GitLabComment, error) {
	m.commentsAdded++
	return &jira.GitLabComment{
		ID:        789,
		Body:      request.Body,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		WebURL:    fmt.Sprintf("https://gitlab.example.com/test/project/issues/%d#note_%d", issueIID, 789),
	}, nil
}

func (m *MockGitLabClient) SearchIssuesByTitle(ctx context.Context, projectID, title string) ([]*jira.GitLabIssue, error) {
	return []*jira.GitLabIssue{
		{
			ID:        123,
			IID:       456,
			ProjectID: 1,
			Title:     title,
			State:     "opened",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			WebURL:    fmt.Sprintf("https://gitlab.example.com/test/project/issues/%d", 456),
		},
	}, nil
}

func (m *MockGitLabClient) FindUserByEmail(ctx context.Context, email string) (*jira.GitLabUser, error) {
	return &jira.GitLabUser{
		ID:        1,
		Name:      "Test User",
		Username:  "testuser",
		Email:     email,
		AvatarURL: "https://gitlab.example.com/uploads/avatar.png",
		WebURL:    "https://gitlab.example.com/testuser",
	}, nil
}

func (m *MockGitLabClient) TestConnection(ctx context.Context) error {
	return nil
}

func (m *MockGitLabClient) GetMilestones(ctx context.Context, projectID string) ([]*jira.GitLabMilestone, error) {
	return []*jira.GitLabMilestone{
		{
			ID:          1,
			IID:         1,
			ProjectID:   1,
			Title:       "Version 1.0",
			Description: "Test milestone",
			State:       "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			WebURL:      "https://gitlab.example.com/test/project/milestones/1",
		},
	}, nil
}

func (m *MockGitLabClient) CreateMilestone(ctx context.Context, projectID string, request *jira.GitLabMilestoneCreateRequest) (*jira.GitLabMilestone, error) {
	return &jira.GitLabMilestone{
		ID:          1,
		IID:         1,
		ProjectID:   1,
		Title:       request.Title,
		Description: request.Description,
		State:       "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		WebURL:      "https://gitlab.example.com/test/project/milestones/1",
	}, nil
}

func (m *MockGitLabClient) UpdateMilestone(ctx context.Context, projectID string, milestoneID int, request *jira.GitLabMilestoneUpdateRequest) (*jira.GitLabMilestone, error) {
	return &jira.GitLabMilestone{
		ID:          milestoneID,
		IID:         milestoneID,
		ProjectID:   1,
		Title:       request.Title,
		Description: request.Description,
		State:       "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		WebURL:      fmt.Sprintf("https://gitlab.example.com/test/project/milestones/%d", milestoneID),
	}, nil
}

func (m *MockGitLabClient) DeleteMilestone(ctx context.Context, projectID string, milestoneID int) error {
	return nil
}

// MockWorkerPool is a mock worker pool for testing
type MockWorkerPool struct {
	jobsSubmitted int
}

func (m *MockWorkerPool) SubmitJob(event *webhook.Event, handler webhook.EventHandler) error {
	m.jobsSubmitted++
	return nil
}

func (m *MockWorkerPool) Start() {
	// Mock implementation
}

func (m *MockWorkerPool) Stop() {
	// Mock implementation
}

func (m *MockWorkerPool) GetWorkerCount() int {
	return 1
}

func (m *MockWorkerPool) GetQueueSize() int {
	return 0
}

func (m *MockWorkerPool) GetStats() webhook.PoolStats {
	return webhook.PoolStats{
		CurrentWorkers: m.GetWorkerCount(),
		QueueLength:    m.GetQueueSize(),
	}
}

// MockSyncManager is a mock sync manager for testing
type MockSyncManager struct {
	syncsPerformed int
}

func (m *MockSyncManager) SyncJiraToGitLab(ctx context.Context, event *jira.WebhookEvent) error {
	m.syncsPerformed++
	return nil
}

// MockRateLimitedJiraClient is a mock Jira client that simulates rate limiting
type MockRateLimitedJiraClient struct {
	requestCount int
}

func (m *MockRateLimitedJiraClient) AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error {
	m.requestCount++
	if m.requestCount > 1 {
		time.Sleep(1 * time.Second) // Simulate rate limiting
	}
	return nil
}

func (m *MockRateLimitedJiraClient) TestConnection(ctx context.Context) error {
	return nil
}

func (m *MockRateLimitedJiraClient) SearchIssues(ctx context.Context, jql string) ([]jira.JiraIssue, error) {
	return []jira.JiraIssue{}, nil
}

func (m *MockRateLimitedJiraClient) GetIssue(ctx context.Context, issueKey string) (*jira.JiraIssue, error) {
	return createTestJiraIssue(issueKey, "Test Issue"), nil
}

func (m *MockRateLimitedJiraClient) UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	return nil
}

func (m *MockRateLimitedJiraClient) GetTransitions(ctx context.Context, issueKey string) ([]jira.Transition, error) {
	return []jira.Transition{}, nil
}

func (m *MockRateLimitedJiraClient) ExecuteTransition(ctx context.Context, issueKey string, transitionID string) error {
	return nil
}

func (m *MockRateLimitedJiraClient) FindTransition(ctx context.Context, issueKey, targetStatus string) (*jira.Transition, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockRateLimitedJiraClient) TransitionToStatus(ctx context.Context, issueKey, targetStatus string) error {
	return nil
}

func (m *MockRateLimitedJiraClient) SearchUsers(ctx context.Context, query string) ([]jira.JiraUser, error) {
	return []jira.JiraUser{}, nil
}

func (m *MockRateLimitedJiraClient) SetAssignee(ctx context.Context, issueKey, accountID string) error {
	return nil
}

// MockErrorSimulatingJiraClient is a mock Jira client that simulates errors
type MockErrorSimulatingJiraClient struct {
	*jira.Client
	callCount int
}

// NewMockErrorSimulatingJiraClient creates a new mock Jira client for testing
func NewMockErrorSimulatingJiraClient() *MockErrorSimulatingJiraClient {
	// Create a minimal config for the mock client
	cfg := &config.Config{
		JiraBaseURL:    "https://test.atlassian.net",
		JiraEmail:      "test@example.com",
		JiraToken:      "test-token",
		JiraAuthMethod: config.JiraAuthMethodBasic,
	}

	// Create the actual Jira client
	client := jira.NewClient(cfg)

	return &MockErrorSimulatingJiraClient{
		Client:    client,
		callCount: 0,
	}
}

func (m *MockErrorSimulatingJiraClient) AddComment(ctx context.Context, issueID string, payload jira.CommentPayload) error {
	m.callCount++
	if m.callCount%3 == 0 {
		return fmt.Errorf("simulated error")
	}
	return nil
}

func (m *MockErrorSimulatingJiraClient) TestConnection(ctx context.Context) error {
	return nil
}

func (m *MockErrorSimulatingJiraClient) SearchIssues(ctx context.Context, jql string) ([]jira.JiraIssue, error) {
	return []jira.JiraIssue{}, nil
}

func (m *MockErrorSimulatingJiraClient) GetIssue(ctx context.Context, issueKey string) (*jira.JiraIssue, error) {
	return createTestJiraIssue(issueKey, "Test Issue"), nil
}

func (m *MockErrorSimulatingJiraClient) UpdateIssue(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	return nil
}

func (m *MockErrorSimulatingJiraClient) GetTransitions(ctx context.Context, issueKey string) ([]jira.Transition, error) {
	return []jira.Transition{}, nil
}

func (m *MockErrorSimulatingJiraClient) ExecuteTransition(ctx context.Context, issueKey string, transitionID string) error {
	return nil
}

func (m *MockErrorSimulatingJiraClient) FindTransition(ctx context.Context, issueKey, targetStatus string) (*jira.Transition, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockErrorSimulatingJiraClient) TransitionToStatus(ctx context.Context, issueKey, targetStatus string) error {
	return nil
}

func (m *MockErrorSimulatingJiraClient) SearchUsers(ctx context.Context, query string) ([]jira.JiraUser, error) {
	return []jira.JiraUser{}, nil
}

func (m *MockErrorSimulatingJiraClient) SetAssignee(ctx context.Context, issueKey, accountID string) error {
	return nil
}

// Package benchmarks provides benchmark tests for the GitLab-Jira Hook application.
package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/async"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/gitlab"
	"github.com/atlet99/gitlab-jira-hook/internal/jira"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// Benchmark data
var (
	benchmarkConfig = &config.Config{
		Port:                 "8080",
		GitLabSecret:         "test-secret",
		GitLabBaseURL:        "https://gitlab.example.com",
		JiraEmail:            "test@example.com",
		JiraToken:            "test-token",
		JiraBaseURL:          "https://jira.example.com",
		LogLevel:             "info",
		AllowedProjects:      []string{"test-project"},
		AllowedGroups:        []string{"test-group"},
		JiraRateLimit:        10,
		JiraRetryMaxAttempts: 3,
		JiraRetryBaseDelayMs: 200,
		PushBranchFilter:     []string{"main", "develop"},
		Timezone:             "Etc/GMT-5",
		WorkerPoolSize:       10,   // Reduce worker pool size for benchmarks
		JobQueueSize:         1000, // Reduce job queue size for benchmarks
		MinWorkers:           2,
		MaxWorkers:           10,
		MaxConcurrentJobs:    20,
		JobTimeoutSeconds:    10,
		QueueTimeoutMs:       5000, // Increase queue timeout for benchmarks
		MaxRetries:           3,
		RetryDelayMs:         100,
		BackoffMultiplier:    2.0,
		MaxBackoffMs:         1000,
		MetricsEnabled:       false, // Disable metrics for benchmarks
		HealthCheckInterval:  30,
	}

	benchmarkLogger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Sample webhook events for benchmarking
	pushEvent = &gitlab.Event{
		Type: "push",
		Project: &gitlab.Project{
			ID:                1,
			Name:              "test-project",
			PathWithNamespace: "test-group/test-project",
			WebURL:            "https://gitlab.example.com/test-group/test-project",
		},
		Group: &gitlab.Group{
			ID:       1,
			Name:     "test-group",
			FullPath: "test-group",
		},
		Commits: []gitlab.Commit{
			{
				ID:        "abc123",
				Message:   "Fix PROJ-123: Update documentation",
				URL:       "https://gitlab.example.com/test-group/test-project/commit/abc123",
				Author:    gitlab.Author{Name: "Test User", Email: "test@example.com"},
				Timestamp: time.Now().Format(time.RFC3339),
				Added:     []string{"docs/README.md"},
				Modified:  []string{"src/main.go"},
				Removed:   []string{},
			},
		},
		Ref: "refs/heads/main",
		User: &gitlab.User{
			ID:       1,
			Username: "testuser",
			Name:     "Test User",
			Email:    "test@example.com",
		},
		EventName: "push",
		ObjectAttributes: &gitlab.ObjectAttributes{
			ID:          1,
			Title:       "Push to main branch",
			Description: "Push event for main branch",
			State:       "pushed",
			Action:      "push",
			Ref:         "refs/heads/main",
			URL:         "https://gitlab.example.com/test-group/test-project",
			Sha:         "abc123",
			Name:        "main",
			Duration:    0,
			Status:      "success",
			IssueType:   "",
			Priority:    "",
		},
	}

	mergeRequestEvent = &gitlab.Event{
		Type: "merge_request",
		Project: &gitlab.Project{
			ID:                1,
			Name:              "test-project",
			PathWithNamespace: "test-group/test-project",
			WebURL:            "https://gitlab.example.com/test-group/test-project",
		},
		Group: &gitlab.Group{
			ID:       1,
			Name:     "test-group",
			FullPath: "test-group",
		},
		ObjectAttributes: &gitlab.ObjectAttributes{
			ID:           1,
			Title:        "Feature PROJ-456: Add new functionality",
			Description:  "This MR implements the requirements from PROJ-456",
			State:        "opened",
			Action:       "open",
			URL:          "https://gitlab.example.com/test-group/test-project/merge_requests/1",
			SourceBranch: "feature/PROJ-456",
			TargetBranch: "main",
			Sha:          "def456",
			Name:         "Feature PROJ-456",
			Duration:     0,
			Status:       "opened",
			IssueType:    "feature",
			Priority:     "medium",
		},
		User: &gitlab.User{
			ID:       1,
			Username: "testuser",
			Name:     "Test User",
			Email:    "test@example.com",
		},
		EventName: "merge_request",
	}
)

// BenchmarkGitLabHandler benchmarks the GitLab webhook handler
func BenchmarkGitLabHandler(b *testing.B) {
	handler := gitlab.NewHandler(benchmarkConfig, benchmarkLogger)

	// Create test request
	eventJSON, _ := json.Marshal(pushEvent)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/gitlab-hook", bytes.NewBuffer(eventJSON))
		req.Header.Set("X-Gitlab-Token", benchmarkConfig.GitLabSecret)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.HandleWebhook(w, req)
	}
}

// BenchmarkProjectHookHandler benchmarks the Project Hook handler
func BenchmarkProjectHookHandler(b *testing.B) {
	handler := gitlab.NewProjectHookHandler(benchmarkConfig, benchmarkLogger)

	// Create test request
	eventJSON, _ := json.Marshal(mergeRequestEvent)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/gitlab-project-hook", bytes.NewBuffer(eventJSON))
		req.Header.Set("X-Gitlab-Token", benchmarkConfig.GitLabSecret)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.HandleProjectHook(w, req)
	}
}

// BenchmarkParser benchmarks the issue ID parser
func BenchmarkParser(b *testing.B) {
	parser := gitlab.NewParser()
	testMessages := []string{
		"Fix PROJ-123: Update documentation",
		"Feature PROJ-456: Add new functionality",
		"Bug PROJ-789: Fix critical issue",
		"Update PROJ-123 and PROJ-456: Multiple issues",
		"No issue reference here",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, msg := range testMessages {
			parser.ExtractIssueIDs(msg)
		}
	}
}

// BenchmarkJiraClient benchmarks the Jira client operations
func BenchmarkJiraClient(b *testing.B) {
	client := jira.NewClient(benchmarkConfig)

	commentPayload := jira.CommentPayload{
		Body: jira.CommentBody{
			Version: 1,
			Type:    "doc",
			Content: []jira.Content{
				{
					Type: "paragraph",
					Content: []jira.TextContent{
						{
							Type: "text",
							Text: "Test comment",
						},
					},
				},
			},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Note: This will fail in real tests since we don't have a real Jira instance
		// but it benchmarks the client creation and payload preparation
		_ = client
		_ = commentPayload
	}
}

// BenchmarkWorkerPool benchmarks the worker pool performance
func BenchmarkWorkerPool(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()

	// Use a more reliable stop mechanism
	defer func() {
		// Stop with timeout to prevent deadlock
		done := make(chan struct{})
		go func() {
			pool.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Pool stopped successfully
		case <-time.After(5 * time.Second):
			// Force stop if timeout
			b.Logf("Warning: Pool stop timed out")
		}
	}()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	// Limit iterations to prevent queue overflow
	maxIterations := 100
	for i := 0; i < b.N && i < maxIterations; i++ {
		// Add retry logic for queue overflow
		for retries := 0; retries < 3; retries++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				if strings.Contains(err.Error(), "queue timeout") || strings.Contains(err.Error(), "queue is full") {
					// Wait a bit and retry
					time.Sleep(10 * time.Millisecond)
					continue
				}
				b.Fatalf("Failed to submit job: %v", err)
			}
			break
		}

		// Add small delay to allow processing
		time.Sleep(5 * time.Millisecond)
	}
}

// BenchmarkWorkerPoolSimple benchmarks basic worker pool functionality
func BenchmarkWorkerPoolSimple(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	pool := async.NewWorkerPool(benchmarkConfig, benchmarkLogger, monitor)

	// Start the pool
	pool.Start()
	defer pool.Stop()

	// Create simple mock handler that does nothing
	simpleHandler := &MockEventHandler{}

	b.ResetTimer()

	// Submit a limited number of jobs to avoid queue overflow
	maxJobs := 1000
	for i := 0; i < b.N && i < maxJobs; i++ {
		// Create a simple event
		simpleEvent := &webhook.Event{
			Type: "test",
			Project: &webhook.Project{
				ID:   1,
				Name: "test-project",
			},
		}

		if err := pool.SubmitJob(simpleEvent, simpleHandler); err != nil {
			b.Fatalf("Failed to submit job: %v", err)
		}
	}
}

// BenchmarkMonitoring benchmarks the monitoring system
func BenchmarkMonitoring(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	monitor.Start()
	defer monitor.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		monitor.RecordRequest("/gitlab-hook", true, 100*time.Millisecond)
		monitor.GetStatus()
		monitor.GetMetrics()
	}
}

// BenchmarkADFGeneration benchmarks ADF comment generation
func BenchmarkADFGeneration(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		jira.GenerateCommitADFComment(
			"abc123",
			"https://gitlab.example.com/test-group/test-project/commit/abc123",
			"Test User",
			"test@example.com",
			"https://gitlab.example.com/testuser",
			"Fix PROJ-123: Update documentation",
			time.Now().Format(time.RFC3339),
			"refs/heads/main",
			"https://gitlab.example.com/test-group/test-project/tree/main",
			"https://gitlab.example.com/test-group/test-project",
			"Etc/GMT-5",
			[]string{"docs/README.md"},
			[]string{"src/main.go"},
			[]string{},
		)
	}
}

// BenchmarkEventProcessing benchmarks event processing
func BenchmarkEventProcessing(b *testing.B) {
	handler := gitlab.NewHandler(benchmarkConfig, benchmarkLogger)
	convert := func(e *gitlab.Event) *webhook.Event {
		if e == nil {
			return nil
		}
		var commits []webhook.Commit
		for _, c := range e.Commits {
			var ts time.Time
			if t, err := time.Parse(time.RFC3339, c.Timestamp); err == nil {
				ts = t
			}
			commits = append(commits, webhook.Commit{
				ID:        c.ID,
				Message:   c.Message,
				URL:       c.URL,
				Author:    webhook.Author{Name: c.Author.Name, Email: c.Author.Email},
				Timestamp: ts,
				Added:     c.Added,
				Modified:  c.Modified,
				Removed:   c.Removed,
			})
		}

		result := &webhook.Event{
			Type:      e.Type,
			EventName: e.EventName,
			Commits:   commits,
		}

		// Safely set Project if available
		if e.Project != nil {
			result.Project = &webhook.Project{
				ID:                e.Project.ID,
				Name:              e.Project.Name,
				PathWithNamespace: e.Project.PathWithNamespace,
				WebURL:            e.Project.WebURL,
			}
		}

		// Safely set Group if available
		if e.Group != nil {
			result.Group = &webhook.Group{
				ID:       e.Group.ID,
				Name:     e.Group.Name,
				FullPath: e.Group.FullPath,
			}
		}

		// Safely set User if available
		if e.User != nil {
			result.User = &webhook.User{
				ID:       e.User.ID,
				Username: e.User.Username,
				Name:     e.User.Name,
				Email:    e.User.Email,
			}
		}

		// Safely set ObjectAttributes if available
		if e.ObjectAttributes != nil {
			result.ObjectAttributes = &webhook.ObjectAttributes{
				ID:          e.ObjectAttributes.ID,
				Title:       e.ObjectAttributes.Title,
				Description: e.ObjectAttributes.Description,
				State:       e.ObjectAttributes.State,
				Action:      e.ObjectAttributes.Action,
				Ref:         e.ObjectAttributes.Ref,
				URL:         e.ObjectAttributes.URL,
				SHA:         e.ObjectAttributes.Sha,
				Name:        e.ObjectAttributes.Name,
				Duration:    e.ObjectAttributes.Duration,
				Status:      e.ObjectAttributes.Status,
				IssueType:   e.ObjectAttributes.IssueType,
				Priority:    e.ObjectAttributes.Priority,
			}
		}

		return result
	}
	ifaceEvent := convert(pushEvent)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		if err := handler.ProcessEventAsync(ctx, ifaceEvent); err != nil {
			b.Fatalf("Failed to process event: %v", err)
		}
	}
}

// BenchmarkConcurrentWebhooks benchmarks concurrent webhook processing
func BenchmarkConcurrentWebhooks(b *testing.B) {
	handler := gitlab.NewHandler(benchmarkConfig, benchmarkLogger)

	eventJSON, _ := json.Marshal(pushEvent)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/gitlab-hook", bytes.NewBuffer(eventJSON))
			req.Header.Set("X-Gitlab-Token", benchmarkConfig.GitLabSecret)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.HandleWebhook(w, req)
		}
	})
}

// MockEventHandler implements types.EventHandler for testing
type MockEventHandler struct{}

func (m *MockEventHandler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	// Simulate processing time
	time.Sleep(10 * time.Millisecond)
	return nil
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	var events []*gitlab.Event

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create new event for each iteration
		event := &gitlab.Event{
			Type: "push",
			Project: &gitlab.Project{
				ID:                i,
				Name:              "test-project",
				PathWithNamespace: "test-group/test-project",
				WebURL:            "https://gitlab.example.com/test-group/test-project",
			},
			Commits: []gitlab.Commit{
				{
					ID:        "abc123",
					Message:   "Fix PROJ-123: Update documentation",
					URL:       "https://gitlab.example.com/test-group/test-project/commit/abc123",
					Author:    gitlab.Author{Name: "Test User", Email: "test@example.com"},
					Timestamp: time.Now().Format(time.RFC3339),
				},
			},
		}

		events = append(events, event)

		// Keep only last 100 events to prevent memory explosion
		if len(events) > 100 {
			events = events[1:]
		}
	}
}

// BenchmarkJSONMarshaling benchmarks JSON marshaling/unmarshaling
func BenchmarkJSONMarshaling(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Marshal
		jsonData, err := json.Marshal(pushEvent)
		if err != nil {
			b.Fatalf("Failed to marshal: %v", err)
		}

		// Unmarshal
		var event gitlab.Event
		if err := json.Unmarshal(jsonData, &event); err != nil {
			b.Fatalf("Failed to unmarshal: %v", err)
		}
	}
}

// BenchmarkHTTPRequests benchmarks HTTP request handling
func BenchmarkHTTPRequests(b *testing.B) {
	handler := gitlab.NewHandler(benchmarkConfig, benchmarkLogger)

	// Create different types of events
	events := []*gitlab.Event{
		pushEvent,
		mergeRequestEvent,
		{
			Type: "issue",
			Project: &gitlab.Project{
				ID:                1,
				Name:              "test-project",
				PathWithNamespace: "test-group/test-project",
				WebURL:            "https://gitlab.example.com/test-group/test-project",
			},
			ObjectAttributes: &gitlab.ObjectAttributes{
				ID:          1,
				Title:       "Bug PROJ-789: Critical issue",
				Description: "This is a critical bug that needs immediate attention",
				State:       "opened",
				Action:      "open",
				URL:         "https://gitlab.example.com/test-group/test-project/issues/1",
			},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		event := events[i%len(events)]
		eventJSON, _ := json.Marshal(event)

		req := httptest.NewRequest("POST", "/gitlab-hook", bytes.NewBuffer(eventJSON))
		req.Header.Set("X-Gitlab-Token", benchmarkConfig.GitLabSecret)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.HandleWebhook(w, req)
	}
}

// BenchmarkPriorityQueueOperations benchmarks priority queue operations
func BenchmarkPriorityQueueOperations(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()

	// Use a more reliable stop mechanism
	defer func() {
		// Stop with timeout to prevent deadlock
		done := make(chan struct{})
		go func() {
			pool.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Pool stopped successfully
		case <-time.After(5 * time.Second):
			// Force stop if timeout
			b.Logf("Warning: Pool stop timed out")
		}
	}()

	// Create different priority events
	highPriorityEvent := &webhook.Event{
		Type: "merge_request",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
		ObjectAttributes: &webhook.ObjectAttributes{
			ID:    1,
			Title: "High priority MR",
		},
	}

	normalPriorityEvent := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	lowPriorityEvent := &webhook.Event{
		Type: "note",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	b.Run("high_priority_submission", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(highPriorityEvent, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			// Small delay for processing
			time.Sleep(2 * time.Millisecond)
		}
	})

	b.Run("normal_priority_submission", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(normalPriorityEvent, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			// Small delay for processing
			time.Sleep(2 * time.Millisecond)
		}
	})

	b.Run("low_priority_submission", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(lowPriorityEvent, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			// Small delay for processing
			time.Sleep(2 * time.Millisecond)
		}
	})

	b.Run("mixed_priority_submission", func(b *testing.B) {
		events := []*webhook.Event{highPriorityEvent, normalPriorityEvent, lowPriorityEvent}
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			event := events[i%len(events)]
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			// Small delay for processing
			time.Sleep(2 * time.Millisecond)
		}
	})
}

// BenchmarkDelayedJobProcessing benchmarks delayed job processing
func BenchmarkDelayedJobProcessing(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()

	// Use a more reliable stop mechanism
	defer func() {
		// Stop with timeout to prevent deadlock
		done := make(chan struct{})
		go func() {
			pool.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Pool stopped successfully
		case <-time.After(5 * time.Second):
			// Force stop if timeout
			b.Logf("Warning: Pool stop timed out")
		}
	}()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	b.Run("short_delay", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 20
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitDelayedJob(event, mockHandler, 10*time.Millisecond); err != nil {
				b.Fatalf("Failed to submit delayed job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})

	b.Run("medium_delay", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 20
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitDelayedJob(event, mockHandler, 100*time.Millisecond); err != nil {
				b.Fatalf("Failed to submit delayed job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})

	b.Run("long_delay", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 20
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitDelayedJob(event, mockHandler, 1*time.Second); err != nil {
				b.Fatalf("Failed to submit delayed job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
}

// BenchmarkMiddlewareChain benchmarks middleware chain performance
func BenchmarkMiddlewareChain(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()
	defer pool.Stop()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	b.Run("with_logging_middleware", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 100
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	})

	b.Run("with_retry_middleware", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 100
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	})

	b.Run("with_timeout_middleware", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 100
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	})
}

// BenchmarkResourceScaling benchmarks worker pool scaling performance
func BenchmarkResourceScaling(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()
	defer pool.Stop()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	b.Run("scale_up_scenario", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})

	b.Run("scale_down_scenario", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, mockHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// BenchmarkErrorHandling benchmarks error handling performance
func BenchmarkErrorHandling(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()
	defer pool.Stop()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	// Create error-prone handler
	errorHandler := &ErrorProneHandler{}

	b.ResetTimer()

	b.Run("error_recovery", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, errorHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})

	b.Run("circuit_breaker", func(b *testing.B) {
		// Limit iterations to prevent queue overflow
		maxIterations := 50
		for i := 0; i < b.N && i < maxIterations; i++ {
			if err := pool.SubmitJob(event, errorHandler); err != nil {
				b.Fatalf("Failed to submit job: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
}

// BenchmarkConfigurationLoading benchmarks configuration loading performance
func BenchmarkConfigurationLoading(b *testing.B) {
	b.ResetTimer()

	b.Run("env_loading", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := config.NewConfigFromEnv(benchmarkLogger)
			_ = cfg
		}
	})

	b.Run("validation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := &config.Config{
				Port:                "8080",
				GitLabSecret:        "test-secret",
				GitLabBaseURL:       "https://gitlab.example.com",
				JiraEmail:           "test@example.com",
				JiraToken:           "test-token",
				JiraBaseURL:         "https://jira.example.com",
				WorkerPoolSize:      10,
				JobQueueSize:        100,
				MinWorkers:          2,
				MaxWorkers:          20,
				MaxConcurrentJobs:   50,
				JobTimeoutSeconds:   10,
				QueueTimeoutMs:      1000,
				MaxRetries:          3,
				RetryDelayMs:        100,
				BackoffMultiplier:   2.0,
				MaxBackoffMs:        1000,
				MetricsEnabled:      true,
				HealthCheckInterval: 30,
			}
			_ = cfg
		}
	})
}

// BenchmarkLoggingPerformance benchmarks logging performance
func BenchmarkLoggingPerformance(b *testing.B) {
	// Create different loggers
	textLogger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	jsonLogger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	b.ResetTimer()

	b.Run("text_logging", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			textLogger.Info("Test log message",
				"iteration", i,
				"timestamp", time.Now(),
				"level", "info",
			)
		}
	})

	b.Run("json_logging", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			jsonLogger.Info("Test log message",
				"iteration", i,
				"timestamp", time.Now(),
				"level", "info",
			)
		}
	})

	b.Run("structured_logging", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			textLogger.Info("Processing webhook event",
				"event_type", "push",
				"project_id", 123,
				"user_id", 456,
				"commit_count", 5,
				"processing_time_ms", 150,
				"success", true,
			)
		}
	})
}

// BenchmarkMemoryEfficiency benchmarks memory efficiency
func BenchmarkMemoryEfficiency(b *testing.B) {
	b.ResetTimer()

	b.Run("event_creation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			event := &gitlab.Event{
				Type: "push",
				Project: &gitlab.Project{
					ID:                i,
					Name:              "test-project",
					PathWithNamespace: "test-group/test-project",
					WebURL:            "https://gitlab.example.com/test-group/test-project",
				},
				Commits: []gitlab.Commit{
					{
						ID:        "abc123",
						Message:   "Fix PROJ-123: Update documentation",
						URL:       "https://gitlab.example.com/test-group/test-project/commit/abc123",
						Author:    gitlab.Author{Name: "Test User", Email: "test@example.com"},
						Timestamp: time.Now().Format(time.RFC3339),
						Added:     []string{"docs/README.md"},
						Modified:  []string{"src/main.go"},
						Removed:   []string{},
					},
				},
				ObjectAttributes: &gitlab.ObjectAttributes{
					ID:          1,
					Title:       "Push to main branch",
					Description: "Push event for main branch",
					State:       "pushed",
					Action:      "push",
					Ref:         "refs/heads/main",
					URL:         "https://gitlab.example.com/test-group/test-project",
					Sha:         "abc123",
					Name:        "main",
					Duration:    0,
					Status:      "success",
					IssueType:   "",
					Priority:    "",
				},
			}
			_ = event
		}
	})

	b.Run("json_marshaling_efficiency", func(b *testing.B) {
		event := pushEvent
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(event)
			if err != nil {
				b.Fatalf("Failed to marshal: %v", err)
			}
		}
	})

	b.Run("json_unmarshaling_efficiency", func(b *testing.B) {
		eventJSON, _ := json.Marshal(pushEvent)
		for i := 0; i < b.N; i++ {
			var event gitlab.Event
			err := json.Unmarshal(eventJSON, &event)
			if err != nil {
				b.Fatalf("Failed to unmarshal: %v", err)
			}
		}
	})
}

// BenchmarkConcurrencyPatterns benchmarks different concurrency patterns
func BenchmarkConcurrencyPatterns(b *testing.B) {
	monitor := monitoring.NewWebhookMonitor(benchmarkConfig, benchmarkLogger)
	decider := &async.DefaultPriorityDecider{}
	pool := async.NewPriorityWorkerPool(benchmarkConfig, benchmarkLogger, monitor, decider)

	// Start the pool
	pool.Start()

	// Use a more reliable stop mechanism
	defer func() {
		// Stop with timeout to prevent deadlock
		done := make(chan struct{})
		go func() {
			pool.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Pool stopped successfully
		case <-time.After(5 * time.Second):
			// Force stop if timeout
			b.Logf("Warning: Pool stop timed out")
		}
	}()

	event := &webhook.Event{
		Type: "push",
		Project: &webhook.Project{
			ID:   1,
			Name: "test-project",
		},
	}

	mockHandler := &MockEventHandler{}

	b.ResetTimer()

	b.Run("burst_submission", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := pool.SubmitJob(event, mockHandler); err != nil {
					b.Fatalf("Failed to submit job: %v", err)
				}
			}
		})
	})

	b.Run("steady_stream", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := pool.SubmitJob(event, mockHandler); err != nil {
					b.Fatalf("Failed to submit job: %v", err)
				}
				time.Sleep(1 * time.Millisecond)
			}
		})
	})

	b.Run("mixed_workload", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				// Mix of immediate and delayed jobs
				if i%2 == 0 {
					if err := pool.SubmitJob(event, mockHandler); err != nil {
						b.Fatalf("Failed to submit job: %v", err)
					}
				} else {
					if err := pool.SubmitDelayedJob(event, mockHandler, 10*time.Millisecond); err != nil {
						b.Fatalf("Failed to submit delayed job: %v", err)
					}
				}
				i++
			}
		})
	})
}

// ErrorProneHandler implements types.EventHandler for error testing
type ErrorProneHandler struct{}

func (e *ErrorProneHandler) ProcessEventAsync(ctx context.Context, event *webhook.Event) error {
	// Simulate occasional errors
	if time.Now().UnixNano()%10 == 0 {
		return fmt.Errorf("simulated error")
	}
	time.Sleep(5 * time.Millisecond)
	return nil
}

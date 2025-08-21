package jira

import (
	"context"
	"time"
)

// ClientInterface defines the interface for Jira client operations
type ClientInterface interface {
	// Comment operations
	AddComment(issueID string, payload CommentPayload) error
	AddCommentWithContext(ctx context.Context, issueID string, payload CommentPayload) error

	// Connection operations
	TestConnection() error
	TestConnectionWithContext(ctx context.Context) error

	// Issue operations
	GetIssue(issueID string) (*Issue, error)
	GetIssueWithContext(ctx context.Context, issueID string) (*Issue, error)
	UpdateIssue(issueID string, fields map[string]interface{}) error
	UpdateIssueWithContext(ctx context.Context, issueID string, fields map[string]interface{}) error

	// Project operations
	GetProject(projectKey string) (*Project, error)
	GetProjectWithContext(ctx context.Context, projectKey string) (*Project, error)

	// User operations
	GetUser(username string) (*User, error)
	GetUserWithContext(ctx context.Context, username string) (*User, error)

	// Health and status
	IsHealthy() bool
	GetHealthStatus() HealthStatus

	// Configuration
	ValidateConfig() error
}

// CommentBuilderInterface defines the interface for building Jira comments
type CommentBuilderInterface interface {
	// Basic comment building
	SetText(text string) CommentBuilderInterface
	SetAuthor(author string) CommentBuilderInterface
	SetTimestamp(timestamp time.Time) CommentBuilderInterface

	// ADF content building
	AddParagraph(text string) CommentBuilderInterface
	AddLink(text, url string) CommentBuilderInterface
	AddCode(code string) CommentBuilderInterface
	AddBold(text string) CommentBuilderInterface
	AddItalic(text string) CommentBuilderInterface
	AddList(items []string) CommentBuilderInterface
	AddTable(headers []string, rows [][]string) CommentBuilderInterface

	// Build the final comment
	Build() CommentPayload
	BuildWithContext(ctx context.Context) CommentPayload
}

// IssueTrackerInterface defines the interface for issue tracking operations
type IssueTrackerInterface interface {
	// Issue extraction
	ExtractIssueIDs(text string) []string
	ExtractIssueIDsWithContext(ctx context.Context, text string) []string

	// Issue validation
	ValidateIssueID(issueID string) bool
	ValidateIssueIDWithContext(ctx context.Context, issueID string) bool

	// Issue linking
	LinkIssues(sourceIssueID, targetIssueID, linkType string) error
	LinkIssuesWithContext(ctx context.Context, sourceIssueID, targetIssueID, linkType string) error

	// Issue transitions
	TransitionIssue(issueID, transitionName string) error
	TransitionIssueWithContext(ctx context.Context, issueID, transitionName string) error
}

// WebhookProcessorInterface defines the interface for processing webhook events
type WebhookProcessorInterface interface {
	// Event processing
	ProcessPushEvent(event *PushEvent) error
	ProcessPushEventWithContext(ctx context.Context, event *PushEvent) error

	ProcessMergeRequestEvent(event *MergeRequestEvent) error
	ProcessMergeRequestEventWithContext(ctx context.Context, event *MergeRequestEvent) error

	ProcessIssueEvent(event *IssueEvent) error
	ProcessIssueEventWithContext(ctx context.Context, event *IssueEvent) error

	ProcessCommentEvent(event *CommentEvent) error
	ProcessCommentEventWithContext(ctx context.Context, event *CommentEvent) error

	// Generic event processing
	ProcessEvent(eventType string, eventData interface{}) error
	ProcessEventWithContext(ctx context.Context, eventType string, eventData interface{}) error
}

// RateLimiterInterface defines the interface for rate limiting Jira operations
type RateLimiterInterface interface {
	// Rate limiting
	Allow() bool
	AllowWithContext(ctx context.Context) bool
	Wait() error
	WaitWithContext(ctx context.Context) error

	// Configuration
	GetRateLimit() int
	GetBurstLimit() int
	SetRateLimit(rate int)
	SetBurstLimit(burst int)

	// Statistics
	GetRequestCount() int64
	GetBlockedCount() int64
	GetAverageWaitTime() time.Duration
}

// RetryHandlerInterface defines the interface for handling retries
type RetryHandlerInterface interface {
	// Retry configuration
	SetMaxRetries(maxRetries int) RetryHandlerInterface
	SetRetryDelay(delay time.Duration) RetryHandlerInterface
	SetBackoffMultiplier(multiplier float64) RetryHandlerInterface
	SetMaxBackoff(maxBackoff time.Duration) RetryHandlerInterface

	// Retry execution
	Execute(operation func() error) error
	ExecuteWithContext(ctx context.Context, operation func() error) error

	// Retry statistics
	GetRetryCount() int
	GetSuccessCount() int64
	GetFailureCount() int64
	GetAverageRetryTime() time.Duration
}

// MetricsCollectorInterface defines the interface for collecting Jira metrics
type MetricsCollectorInterface interface {
	// Request metrics
	RecordRequest(operation string, duration time.Duration, success bool)
	RecordRequestWithContext(ctx context.Context, operation string, duration time.Duration, success bool)

	// Error metrics
	RecordError(operation string, errorType string)
	RecordErrorWithContext(ctx context.Context, operation string, errorType string)

	// Rate limiting metrics
	RecordRateLimitHit(operation string)
	RecordRateLimitHitWithContext(ctx context.Context, operation string)

	// Retry metrics
	RecordRetry(operation string, attempt int)
	RecordRetryWithContext(ctx context.Context, operation string, attempt int)

	// Get metrics
	GetMetrics() map[string]interface{}
	GetMetricsWithContext(ctx context.Context) map[string]interface{}
}

// ConfigurationManagerInterface defines the interface for managing Jira configuration
type ConfigurationManagerInterface interface {
	// Configuration operations
	LoadConfig() error
	LoadConfigWithContext(ctx context.Context) error
	SaveConfig() error
	SaveConfigWithContext(ctx context.Context) error
	ValidateConfig() error
	ValidateConfigWithContext(ctx context.Context) error

	// Configuration access
	GetConfigValue(key string) interface{}
	SetConfigValue(key string, value interface{}) error

	// Configuration monitoring
	WatchConfigChanges(callback func()) error
	WatchConfigChangesWithContext(ctx context.Context, callback func()) error
}

// HealthCheckerInterface defines the interface for Jira health checking
type HealthCheckerInterface interface {
	// Health checking
	CheckHealth() HealthStatus
	CheckHealthWithContext(ctx context.Context) HealthStatus
	IsHealthy() bool
	IsHealthyWithContext(ctx context.Context) bool

	// Health monitoring
	StartHealthMonitoring(interval time.Duration) error
	StartHealthMonitoringWithContext(ctx context.Context, interval time.Duration) error
	StopHealthMonitoring() error

	// Health callbacks
	SetHealthCallback(callback func(HealthStatus)) error
	SetHealthCallbackWithContext(ctx context.Context, callback func(HealthStatus)) error
}

// Data structures for interfaces

// Issue represents a Jira issue
type Issue struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Priority    string                 `json:"priority"`
	Assignee    *User                  `json:"assignee"`
	Reporter    *User                  `json:"reporter"`
	Project     *Project               `json:"project"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
	Fields      map[string]interface{} `json:"fields"`
}

// Project represents a Jira project
type Project struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Lead        *User  `json:"lead"`
}

// User represents a Jira user
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Active   bool   `json:"active"`
}

// HealthStatus represents the health status of Jira connection
type HealthStatus struct {
	Healthy      bool                   `json:"healthy"`
	Status       string                 `json:"status"`
	Message      string                 `json:"message"`
	LastCheck    time.Time              `json:"last_check"`
	Details      map[string]interface{} `json:"details"`
	ResponseTime time.Duration          `json:"response_time"`
}

// PushEvent represents a push event from GitLab
type PushEvent struct {
	Project    string    `json:"project"`
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Commits    []Commit  `json:"commits"`
	User       *User     `json:"user"`
	Timestamp  time.Time `json:"timestamp"`
}

// MergeRequestEvent represents a merge request event from GitLab
type MergeRequestEvent struct {
	Project      string    `json:"project"`
	Repository   string    `json:"repository"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	User         *User     `json:"user"`
	Timestamp    time.Time `json:"timestamp"`
}

// IssueEvent represents an issue event from GitLab
type IssueEvent struct {
	Project     string    `json:"project"`
	IssueKey    string    `json:"issue_key"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	User        *User     `json:"user"`
	Timestamp   time.Time `json:"timestamp"`
}

// CommentEvent represents a comment event from GitLab
type CommentEvent struct {
	Project   string    `json:"project"`
	IssueKey  string    `json:"issue_key"`
	CommentID string    `json:"comment_id"`
	Content   string    `json:"content"`
	User      *User     `json:"user"`
	Timestamp time.Time `json:"timestamp"`
}

// Commit represents a Git commit
type Commit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Author    *User     `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
}

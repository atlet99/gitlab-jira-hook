// Package webhook provides common types and interfaces used across the application.
package webhook

import (
	"context"
	"time"
)

// Event represents a webhook event
type Event struct {
	Type             string            `json:"object_kind,omitempty"`
	EventName        string            `json:"event_name,omitempty"`
	Project          *Project          `json:"project,omitempty"`
	Group            *Group            `json:"group,omitempty"`
	User             *User             `json:"user,omitempty"`
	Commits          []Commit          `json:"commits,omitempty"`
	ObjectAttributes *ObjectAttributes `json:"object_attributes,omitempty"`
	// Add other fields as needed
}

// Project represents a GitLab project
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
}

// Group represents a GitLab group
type Group struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

// User represents a GitLab user
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// Commit represents a Git commit
type Commit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	Added     []string  `json:"added,omitempty"`
	Modified  []string  `json:"modified,omitempty"`
	Removed   []string  `json:"removed,omitempty"`
}

// Author represents a commit author
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ObjectAttributes represents object attributes in webhook events
type ObjectAttributes struct {
	ID          int    `json:"id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
	Action      string `json:"action,omitempty"`
	Ref         string `json:"ref,omitempty"`
	URL         string `json:"url,omitempty"`
	SHA         string `json:"sha,omitempty"`
	Name        string `json:"name,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Status      string `json:"status,omitempty"`
	IssueType   string `json:"issue_type,omitempty"`
	Priority    string `json:"priority,omitempty"`
}

// PoolStats holds statistics for the worker pool
type PoolStats struct {
	TotalJobsProcessed int64
	SuccessfulJobs     int64
	FailedJobs         int64
	AverageJobTime     time.Duration
	ActiveWorkers      int
	QueueSize          int
	QueueCapacity      int
	// QueueLength is the current length of the job queue
	QueueLength int
	// CurrentWorkers is the current number of active workers
	CurrentWorkers int
	// ScaleUpEvents is the number of times the pool was scaled up
	ScaleUpEvents int
	// ScaleDownEvents is the number of times the pool was scaled down
	ScaleDownEvents int
}

// WorkerPoolInterface defines the interface for worker pool operations
type WorkerPoolInterface interface {
	SubmitJob(event *Event, handler EventHandler) error
	GetStats() PoolStats
	Start()
	Stop()
}

// EventHandler defines the interface for event processing
type EventHandler interface {
	ProcessEventAsync(ctx context.Context, event *Event) error
}

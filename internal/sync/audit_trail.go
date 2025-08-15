// Package sync provides audit trail functionality for synchronization events
package sync

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	// percentage constants for calculations
	percentageMultiplier = 100
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// AuditEventSync represents a synchronization event
	AuditEventSync AuditEventType = "sync"
	// AuditEventConflictDetection represents conflict detection
	AuditEventConflictDetection AuditEventType = "conflict_detection"
	// AuditEventConflictResolution represents conflict resolution
	AuditEventConflictResolution AuditEventType = "conflict_resolution"
	// AuditEventManualResolution represents manual conflict resolution
	AuditEventManualResolution AuditEventType = "manual_resolution"
	// AuditEventRollback represents a rollback operation
	AuditEventRollback AuditEventType = "rollback"
)

// AuditEvent represents a single audit trail event
type AuditEvent struct {
	ID              string                 `json:"id"`
	EventType       AuditEventType         `json:"event_type"`
	Timestamp       time.Time              `json:"timestamp"`
	JiraIssueKey    string                 `json:"jira_issue_key,omitempty"`
	GitLabProjectID string                 `json:"gitlab_project_id,omitempty"`
	GitLabIssueID   *int                   `json:"gitlab_issue_id,omitempty"`
	SourceSystem    string                 `json:"source_system"` // "jira" or "gitlab"
	TargetSystem    string                 `json:"target_system"` // "jira" or "gitlab"
	Operation       string                 `json:"operation"`     // "create", "update", "comment"
	Success         bool                   `json:"success"`
	Error           string                 `json:"error,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
	BeforeState     map[string]interface{} `json:"before_state,omitempty"`
	AfterState      map[string]interface{} `json:"after_state,omitempty"`
	ConflictID      string                 `json:"conflict_id,omitempty"`
	Duration        time.Duration          `json:"duration,omitempty"`
	UserAgent       string                 `json:"user_agent,omitempty"`
	RequestID       string                 `json:"request_id,omitempty"`
}

// AuditTrail manages the audit trail for synchronization events
type AuditTrail struct {
	events     []*AuditEvent
	eventIndex map[string]*AuditEvent // ID -> Event for fast lookups
	mutex      sync.RWMutex
	logger     *slog.Logger
	maxEvents  int // Maximum number of events to keep in memory
}

// AuditQuery represents query parameters for searching audit events
type AuditQuery struct {
	EventTypes    []AuditEventType `json:"event_types,omitempty"`
	JiraIssueKey  string           `json:"jira_issue_key,omitempty"`
	GitLabIssueID *int             `json:"gitlab_issue_id,omitempty"`
	SourceSystem  string           `json:"source_system,omitempty"`
	TargetSystem  string           `json:"target_system,omitempty"`
	Success       *bool            `json:"success,omitempty"`
	ConflictID    string           `json:"conflict_id,omitempty"`
	StartTime     *time.Time       `json:"start_time,omitempty"`
	EndTime       *time.Time       `json:"end_time,omitempty"`
	Limit         int              `json:"limit,omitempty"`
	Offset        int              `json:"offset,omitempty"`
}

// AuditStats provides statistics about audit events
type AuditStats struct {
	TotalEvents        int                    `json:"total_events"`
	EventsByType       map[AuditEventType]int `json:"events_by_type"`
	EventsBySystem     map[string]int         `json:"events_by_system"`
	SuccessRate        float64                `json:"success_rate"`
	ErrorRate          float64                `json:"error_rate"`
	AverageDuration    string                 `json:"average_duration"`
	OldestEvent        *time.Time             `json:"oldest_event,omitempty"`
	NewestEvent        *time.Time             `json:"newest_event,omitempty"`
	ConflictsDetected  int                    `json:"conflicts_detected"`
	ConflictsResolved  int                    `json:"conflicts_resolved"`
	RollbacksPerformed int                    `json:"rollbacks_performed"`
}

// RollbackRequest represents a request to rollback synchronization changes
type RollbackRequest struct {
	EventID       string    `json:"event_id"`
	JiraIssueKey  string    `json:"jira_issue_key,omitempty"`
	GitLabIssueID *int      `json:"gitlab_issue_id,omitempty"`
	RollbackTo    time.Time `json:"rollback_to"`
	Reason        string    `json:"reason"`
	RequestedBy   string    `json:"requested_by"`
	DryRun        bool      `json:"dry_run,omitempty"`
}

// NewAuditTrail creates a new audit trail
func NewAuditTrail(logger *slog.Logger, maxEvents int) *AuditTrail {
	if maxEvents <= 0 {
		maxEvents = 10000 // Default maximum events
	}

	return &AuditTrail{
		events:     make([]*AuditEvent, 0),
		eventIndex: make(map[string]*AuditEvent),
		logger:     logger,
		maxEvents:  maxEvents,
	}
}

// RecordEvent records a new audit event
func (at *AuditTrail) RecordEvent(event *AuditEvent) {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	// Generate ID if not provided
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
	}

	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Add to events list
	at.events = append(at.events, event)
	at.eventIndex[event.ID] = event

	// Enforce maximum events limit
	if len(at.events) > at.maxEvents {
		// Remove oldest event
		oldestEvent := at.events[0]
		at.events = at.events[1:]
		delete(at.eventIndex, oldestEvent.ID)
	}

	at.logger.Debug("Recorded audit event",
		"event_id", event.ID,
		"event_type", event.EventType,
		"success", event.Success)
}

// GetEvent retrieves a specific audit event by ID
func (at *AuditTrail) GetEvent(eventID string) (*AuditEvent, error) {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	event, exists := at.eventIndex[eventID]
	if !exists {
		return nil, fmt.Errorf("audit event with ID %s not found", eventID)
	}

	// Return a copy to prevent external modification
	eventCopy := *event
	return &eventCopy, nil
}

// QueryEvents searches for audit events based on query parameters
func (at *AuditTrail) QueryEvents(query *AuditQuery) []*AuditEvent {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	var matches []*AuditEvent

	for _, event := range at.events {
		if at.eventMatchesQuery(event, query) {
			// Create a copy to prevent external modification
			eventCopy := *event
			matches = append(matches, &eventCopy)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Timestamp.After(matches[j].Timestamp)
	})

	// Apply offset and limit
	if query.Offset > 0 && query.Offset < len(matches) {
		matches = matches[query.Offset:]
	}

	if query.Limit > 0 && query.Limit < len(matches) {
		matches = matches[:query.Limit]
	}

	return matches
}

// eventMatchesQuery checks if an event matches the given query
func (at *AuditTrail) eventMatchesQuery(event *AuditEvent, query *AuditQuery) bool {
	return at.matchesEventTypes(event, query) &&
		at.matchesIssueKeys(event, query) &&
		at.matchesSystems(event, query) &&
		at.matchesStatus(event, query) &&
		at.matchesTimeRange(event, query)
}

// matchesEventTypes checks if event type matches query
func (at *AuditTrail) matchesEventTypes(event *AuditEvent, query *AuditQuery) bool {
	if len(query.EventTypes) == 0 {
		return true
	}

	for _, eventType := range query.EventTypes {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

// matchesIssueKeys checks if issue keys match query
func (at *AuditTrail) matchesIssueKeys(event *AuditEvent, query *AuditQuery) bool {
	// Check Jira issue key
	if query.JiraIssueKey != "" && event.JiraIssueKey != query.JiraIssueKey {
		return false
	}

	// Check GitLab issue ID
	if query.GitLabIssueID != nil &&
		(event.GitLabIssueID == nil || *event.GitLabIssueID != *query.GitLabIssueID) {
		return false
	}

	return true
}

// matchesSystems checks if source/target systems match query
func (at *AuditTrail) matchesSystems(event *AuditEvent, query *AuditQuery) bool {
	// Check source system
	if query.SourceSystem != "" && event.SourceSystem != query.SourceSystem {
		return false
	}

	// Check target system
	if query.TargetSystem != "" && event.TargetSystem != query.TargetSystem {
		return false
	}

	return true
}

// matchesStatus checks if status fields match query
func (at *AuditTrail) matchesStatus(event *AuditEvent, query *AuditQuery) bool {
	// Check success status
	if query.Success != nil && event.Success != *query.Success {
		return false
	}

	// Check conflict ID
	if query.ConflictID != "" && event.ConflictID != query.ConflictID {
		return false
	}

	return true
}

// matchesTimeRange checks if event timestamp falls within query time range
func (at *AuditTrail) matchesTimeRange(event *AuditEvent, query *AuditQuery) bool {
	// Check time range
	if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
		return false
	}

	if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
		return false
	}

	return true
}

// GetStats returns statistics about the audit trail
func (at *AuditTrail) GetStats() *AuditStats {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	stats := &AuditStats{
		TotalEvents:    len(at.events),
		EventsByType:   make(map[AuditEventType]int),
		EventsBySystem: make(map[string]int),
	}

	if stats.TotalEvents == 0 {
		return stats
	}

	// Process all events to gather statistics
	counters := at.processEventsForStats(stats)

	// Calculate final rates and durations
	at.calculateRatesAndDurations(stats, counters)

	return stats
}

// statsCounters holds intermediate counting data for statistics
type statsCounters struct {
	successCount  int
	totalDuration time.Duration
	durationCount int
}

// processEventsForStats processes all events to gather statistics
func (at *AuditTrail) processEventsForStats(stats *AuditStats) *statsCounters {
	counters := &statsCounters{}

	for _, event := range at.events {
		at.processEventStats(event, stats, counters)
	}

	return counters
}

// processEventStats processes a single event for statistics
func (at *AuditTrail) processEventStats(event *AuditEvent, stats *AuditStats, counters *statsCounters) {
	// Count by type
	stats.EventsByType[event.EventType]++

	// Count by system
	if event.SourceSystem != "" {
		stats.EventsBySystem[event.SourceSystem]++
	}

	// Count success/failure
	if event.Success {
		counters.successCount++
	}

	// Calculate duration statistics
	if event.Duration > 0 {
		counters.totalDuration += event.Duration
		counters.durationCount++
	}

	// Track specific event types
	at.trackSpecialEventTypes(event, stats)

	// Track time range
	at.updateTimeRange(event, stats)
}

// trackSpecialEventTypes tracks special event type counters
func (at *AuditTrail) trackSpecialEventTypes(event *AuditEvent, stats *AuditStats) {
	switch event.EventType {
	case AuditEventConflictDetection:
		stats.ConflictsDetected++
	case AuditEventConflictResolution, AuditEventManualResolution:
		if event.Success {
			stats.ConflictsResolved++
		}
	case AuditEventRollback:
		if event.Success {
			stats.RollbacksPerformed++
		}
	}
}

// updateTimeRange updates the oldest and newest event timestamps
func (at *AuditTrail) updateTimeRange(event *AuditEvent, stats *AuditStats) {
	if stats.OldestEvent == nil || event.Timestamp.Before(*stats.OldestEvent) {
		stats.OldestEvent = &event.Timestamp
	}
	if stats.NewestEvent == nil || event.Timestamp.After(*stats.NewestEvent) {
		stats.NewestEvent = &event.Timestamp
	}
}

// calculateRatesAndDurations calculates final rates and durations
func (at *AuditTrail) calculateRatesAndDurations(stats *AuditStats, counters *statsCounters) {
	// Calculate rates
	stats.SuccessRate = float64(counters.successCount) / float64(stats.TotalEvents) * percentageMultiplier
	stats.ErrorRate = percentageMultiplier - stats.SuccessRate

	// Calculate average duration
	if counters.durationCount > 0 {
		avgDuration := counters.totalDuration / time.Duration(counters.durationCount)
		stats.AverageDuration = avgDuration.Round(time.Millisecond).String()
	}
}

// PrepareRollback analyzes what would be affected by a rollback operation
func (at *AuditTrail) PrepareRollback(request *RollbackRequest) (*RollbackPlan, error) {
	at.mutex.RLock()
	defer at.mutex.RUnlock()

	var affectedEvents []*AuditEvent

	// Find all events that need to be rolled back
	for _, event := range at.events {
		if at.shouldRollbackEvent(event, request) {
			affectedEvents = append(affectedEvents, event)
		}
	}

	if len(affectedEvents) == 0 {
		return nil, fmt.Errorf("no events found for rollback criteria")
	}

	// Sort by timestamp (newest first for rollback order)
	sort.Slice(affectedEvents, func(i, j int) bool {
		return affectedEvents[i].Timestamp.After(affectedEvents[j].Timestamp)
	})

	plan := &RollbackPlan{
		RequestID:      fmt.Sprintf("rollback_%d", time.Now().Unix()),
		Request:        request,
		AffectedEvents: affectedEvents,
		EstimatedSteps: len(affectedEvents),
		CreatedAt:      time.Now(),
	}

	return plan, nil
}

// shouldRollbackEvent determines if an event should be included in rollback
func (at *AuditTrail) shouldRollbackEvent(event *AuditEvent, request *RollbackRequest) bool {
	// Check if event occurred after the rollback target time
	if !event.Timestamp.After(request.RollbackTo) {
		return false
	}

	// Check specific event ID
	if request.EventID != "" && event.ID != request.EventID {
		return false
	}

	// Check Jira issue key
	if request.JiraIssueKey != "" && event.JiraIssueKey != request.JiraIssueKey {
		return false
	}

	// Check GitLab issue ID
	if request.GitLabIssueID != nil &&
		(event.GitLabIssueID == nil || *event.GitLabIssueID != *request.GitLabIssueID) {
		return false
	}

	// Only include successful sync events
	return event.Success && event.EventType == AuditEventSync
}

// RollbackPlan represents a plan for rolling back synchronization changes
type RollbackPlan struct {
	RequestID      string           `json:"request_id"`
	Request        *RollbackRequest `json:"request"`
	AffectedEvents []*AuditEvent    `json:"affected_events"`
	EstimatedSteps int              `json:"estimated_steps"`
	CreatedAt      time.Time        `json:"created_at"`
	ExecutedAt     *time.Time       `json:"executed_at,omitempty"`
	Status         string           `json:"status"` // "planned", "executing", "completed", "failed"
	Results        []RollbackResult `json:"results,omitempty"`
}

// RollbackResult represents the result of rolling back a single event
type RollbackResult struct {
	EventID string `json:"event_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Action  string `json:"action"` // "reverted", "skipped", "failed"
}

// PurgeOldEvents removes events older than the specified duration
func (at *AuditTrail) PurgeOldEvents(olderThan time.Duration) int {
	at.mutex.Lock()
	defer at.mutex.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var newEvents []*AuditEvent
	purged := 0

	for _, event := range at.events {
		if event.Timestamp.After(cutoff) {
			newEvents = append(newEvents, event)
		} else {
			delete(at.eventIndex, event.ID)
			purged++
		}
	}

	at.events = newEvents

	if purged > 0 {
		at.logger.Info("Purged old audit events",
			"purged_count", purged,
			"older_than", olderThan)
	}

	return purged
}

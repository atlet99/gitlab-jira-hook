// Package sync provides manual conflict resolution queue
package sync

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// ManualResolutionQueue manages conflicts that require manual intervention
type ManualResolutionQueue struct {
	conflicts map[string]*Conflict
	mutex     sync.RWMutex
	logger    *slog.Logger
}

// QueueStats provides statistics about the manual resolution queue
type QueueStats struct {
	TotalConflicts    int                    `json:"total_conflicts"`
	PendingConflicts  int                    `json:"pending_conflicts"`
	ResolvedConflicts int                    `json:"resolved_conflicts"`
	FailedConflicts   int                    `json:"failed_conflicts"`
	OldestConflict    *time.Time             `json:"oldest_conflict,omitempty"`
	ConflictsByType   map[ConflictType]int   `json:"conflicts_by_type"`
	ConflictsByStatus map[ConflictStatus]int `json:"conflicts_by_status"`
	AverageAge        string                 `json:"average_age"`
}

// ManualResolutionRequest represents a manual resolution submitted by user
type ManualResolutionRequest struct {
	ConflictID   string                 `json:"conflict_id"`
	Resolution   string                 `json:"resolution"` // "use_jira", "use_gitlab", "custom"
	CustomValues map[string]interface{} `json:"custom_values,omitempty"`
	Notes        string                 `json:"notes,omitempty"`
	ResolvedBy   string                 `json:"resolved_by"`
}

// NewManualResolutionQueue creates a new manual resolution queue
func NewManualResolutionQueue(logger *slog.Logger) *ManualResolutionQueue {
	return &ManualResolutionQueue{
		conflicts: make(map[string]*Conflict),
		logger:    logger,
	}
}

// AddConflict adds a conflict to the manual resolution queue
func (q *ManualResolutionQueue) AddConflict(conflict *Conflict) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if conflict.ID == "" {
		return fmt.Errorf("conflict ID cannot be empty")
	}

	// Check if conflict already exists
	if _, exists := q.conflicts[conflict.ID]; exists {
		return fmt.Errorf("conflict with ID %s already exists in queue", conflict.ID)
	}

	// Mark as pending and add to queue
	conflict.Status = ConflictStatusPending
	q.conflicts[conflict.ID] = conflict

	q.logger.Info("Added conflict to manual resolution queue",
		"conflict_id", conflict.ID,
		"jira_issue", conflict.JiraIssueKey,
		"gitlab_issue", conflict.GitLabIssueID,
		"queue_size", len(q.conflicts))

	return nil
}

// GetConflict retrieves a specific conflict from the queue
func (q *ManualResolutionQueue) GetConflict(conflictID string) (*Conflict, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	conflict, exists := q.conflicts[conflictID]
	if !exists {
		return nil, fmt.Errorf("conflict with ID %s not found", conflictID)
	}

	// Return a copy to prevent external modification
	conflictCopy := *conflict
	return &conflictCopy, nil
}

// ListPendingConflicts returns all pending conflicts, sorted by detection time
func (q *ManualResolutionQueue) ListPendingConflicts() []*Conflict {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var pending []*Conflict
	for _, conflict := range q.conflicts {
		if conflict.Status == ConflictStatusPending {
			// Create a copy to prevent external modification
			conflictCopy := *conflict
			pending = append(pending, &conflictCopy)
		}
	}

	// Sort by detection time (oldest first)
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].DetectedAt.Before(pending[j].DetectedAt)
	})

	return pending
}

// ListAllConflicts returns all conflicts in the queue
func (q *ManualResolutionQueue) ListAllConflicts() []*Conflict {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var conflicts []*Conflict
	for _, conflict := range q.conflicts {
		// Create a copy to prevent external modification
		conflictCopy := *conflict
		conflicts = append(conflicts, &conflictCopy)
	}

	// Sort by detection time (newest first)
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].DetectedAt.After(conflicts[j].DetectedAt)
	})

	return conflicts
}

// ResolveConflict manually resolves a conflict based on user input
func (q *ManualResolutionQueue) ResolveConflict(request *ManualResolutionRequest) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	conflict, exists := q.conflicts[request.ConflictID]
	if !exists {
		return fmt.Errorf("conflict with ID %s not found", request.ConflictID)
	}

	if conflict.Status != ConflictStatusPending {
		return fmt.Errorf("conflict %s is not in pending status", request.ConflictID)
	}

	// Create resolution record
	now := time.Now()
	resolution := &ConflictResolution{
		Strategy:  StrategyManual,
		AppliedBy: request.ResolvedBy,
		AppliedAt: now,
		Notes:     request.Notes,
	}

	// Determine resolution based on request
	switch request.Resolution {
	case "use_jira":
		resolution.ChosenSource = "jira"
	case "use_gitlab":
		resolution.ChosenSource = "gitlab"
	case "custom":
		if len(request.CustomValues) == 0 {
			return fmt.Errorf("custom resolution requires custom values")
		}
		resolution.MergedFields = request.CustomValues
		resolution.ChosenSource = "custom"
	default:
		return fmt.Errorf("invalid resolution type: %s", request.Resolution)
	}

	// Update conflict
	conflict.Status = ConflictStatusResolved
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = request.ResolvedBy
	conflict.Resolution = resolution

	q.logger.Info("Manually resolved conflict",
		"conflict_id", request.ConflictID,
		"resolution", request.Resolution,
		"resolved_by", request.ResolvedBy)

	return nil
}

// RemoveConflict removes a conflict from the queue
func (q *ManualResolutionQueue) RemoveConflict(conflictID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if _, exists := q.conflicts[conflictID]; !exists {
		return fmt.Errorf("conflict with ID %s not found", conflictID)
	}

	delete(q.conflicts, conflictID)
	q.logger.Info("Removed conflict from queue", "conflict_id", conflictID)

	return nil
}

// Size returns the current size of the queue
func (q *ManualResolutionQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.conflicts)
}

// GetStats returns detailed statistics about the queue
func (q *ManualResolutionQueue) GetStats() *QueueStats {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	stats := &QueueStats{
		TotalConflicts:    len(q.conflicts),
		ConflictsByType:   make(map[ConflictType]int),
		ConflictsByStatus: make(map[ConflictStatus]int),
	}

	var totalAge time.Duration
	var oldestTime *time.Time

	for _, conflict := range q.conflicts {
		// Count by status
		stats.ConflictsByStatus[conflict.Status]++
		switch conflict.Status {
		case ConflictStatusPending:
			stats.PendingConflicts++
		case ConflictStatusResolved:
			stats.ResolvedConflicts++
		case ConflictStatusFailed:
			stats.FailedConflicts++
		}

		// Count by type
		stats.ConflictsByType[conflict.Type]++

		// Calculate age statistics
		age := time.Since(conflict.DetectedAt)
		totalAge += age

		if oldestTime == nil || conflict.DetectedAt.Before(*oldestTime) {
			oldestTime = &conflict.DetectedAt
		}
	}

	// Calculate average age
	if stats.TotalConflicts > 0 {
		averageAge := totalAge / time.Duration(stats.TotalConflicts)
		stats.AverageAge = averageAge.Round(time.Minute).String()
	}

	stats.OldestConflict = oldestTime

	return stats
}

// PurgeResolvedConflicts removes resolved conflicts older than the specified duration
func (q *ManualResolutionQueue) PurgeResolvedConflicts(olderThan time.Duration) int {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var purged int

	for id, conflict := range q.conflicts {
		if conflict.Status == ConflictStatusResolved &&
			conflict.ResolvedAt != nil &&
			conflict.ResolvedAt.Before(cutoff) {
			delete(q.conflicts, id)
			purged++
		}
	}

	if purged > 0 {
		q.logger.Info("Purged resolved conflicts from queue",
			"purged_count", purged,
			"older_than", olderThan)
	}

	return purged
}

// GetConflictsByJiraIssue returns all conflicts for a specific Jira issue
func (q *ManualResolutionQueue) GetConflictsByJiraIssue(jiraIssueKey string) []*Conflict {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var conflicts []*Conflict
	for _, conflict := range q.conflicts {
		if conflict.JiraIssueKey == jiraIssueKey {
			conflictCopy := *conflict
			conflicts = append(conflicts, &conflictCopy)
		}
	}

	return conflicts
}

// GetConflictsByGitLabIssue returns all conflicts for a specific GitLab issue
func (q *ManualResolutionQueue) GetConflictsByGitLabIssue(projectID string, issueID int) []*Conflict {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var conflicts []*Conflict
	for _, conflict := range q.conflicts {
		if conflict.GitLabProjectID == projectID && conflict.GitLabIssueID == issueID {
			conflictCopy := *conflict
			conflicts = append(conflicts, &conflictCopy)
		}
	}

	return conflicts
}

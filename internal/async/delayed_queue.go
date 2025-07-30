package async

import (
	"context"
	"sync"
	"time"

	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

const (
	// SchedulerInterval is the interval for the delayed queue scheduler
	SchedulerInterval = 5 * time.Millisecond
	// WaitForReadyInterval is the interval for waiting for ready jobs
	WaitForReadyInterval = 2 * time.Millisecond
)

// DelayedJob represents a job that should be executed after a delay
type DelayedJob struct {
	*Job
	ScheduledAt time.Time // When the job should be moved to the main queue
}

// DelayedQueue manages delayed jobs and schedules them for execution
type DelayedQueue struct {
	config    *config.Config
	logger    *slog.Logger
	jobs      []*DelayedJob
	mutex     sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mainQueue *PriorityQueue
	decider   PriorityDecider
}

// NewDelayedQueue creates a new delayed job queue
func NewDelayedQueue(
	cfg *config.Config,
	logger *slog.Logger,
	mainQueue *PriorityQueue,
	decider PriorityDecider,
) *DelayedQueue {
	ctx, cancel := context.WithCancel(context.Background())

	dq := &DelayedQueue{
		config:    cfg,
		logger:    logger,
		jobs:      make([]*DelayedJob, 0),
		mainQueue: mainQueue,
		decider:   decider,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start the scheduler
	dq.wg.Add(1)
	go dq.scheduler()

	return dq
}

// SubmitDelayedJob submits a job to be executed after the specified delay
func (dq *DelayedQueue) SubmitDelayedJob(
	event *webhook.Event,
	handler webhook.EventHandler,
	delay time.Duration,
	priorityOverride ...JobPriority,
) error {
	priority := PriorityNormal
	if len(priorityOverride) > 0 {
		priority = priorityOverride[0]
	} else if dq.decider != nil {
		priority = dq.decider.DecidePriority(event, handler)
	}

	job := &Job{
		ID:        generateJobID(),
		Event:     event,
		Handler:   handler,
		Priority:  priority,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		ctx:       context.Background(),
	}

	delayedJob := &DelayedJob{
		Job:         job,
		ScheduledAt: time.Now().Add(delay),
	}

	dq.mutex.Lock()
	dq.jobs = append(dq.jobs, delayedJob)
	dq.mutex.Unlock()

	dq.logger.Debug("Submitted delayed job",
		"job_id", job.ID,
		"delay", delay,
		"priority", priority,
		"scheduled_at", delayedJob.ScheduledAt,
	)

	return nil
}

// scheduler runs in a goroutine and moves ready delayed jobs to the main queue
func (dq *DelayedQueue) scheduler() {
	defer dq.wg.Done()

	// Use shorter interval for faster response (especially in tests)
	ticker := time.NewTicker(SchedulerInterval)
	defer ticker.Stop()

	dq.logger.Info("Delayed queue scheduler started", "interval_ms", SchedulerInterval.Milliseconds())

	// Process jobs immediately on startup
	dq.processReadyJobs()

	for {
		select {
		case <-dq.ctx.Done():
			dq.logger.Info("Delayed queue scheduler stopped")
			return
		case <-ticker.C:
			dq.processReadyJobs()
		}
	}
}

// processReadyJobs moves jobs that are ready to execute to the main queue
func (dq *DelayedQueue) processReadyJobs() {
	now := time.Now()
	var readyJobs []*DelayedJob
	var remainingJobs []*DelayedJob

	dq.mutex.Lock()
	totalJobs := len(dq.jobs)
	for _, job := range dq.jobs {
		if now.After(job.ScheduledAt) || now.Equal(job.ScheduledAt) {
			readyJobs = append(readyJobs, job)
		} else {
			remainingJobs = append(remainingJobs, job)
		}
	}
	dq.jobs = remainingJobs
	dq.mutex.Unlock()

	if totalJobs > 0 || len(readyJobs) > 0 {
		dq.logger.Info("processReadyJobs check",
			"total_jobs", totalJobs,
			"ready_jobs", len(readyJobs),
			"remaining_jobs", len(remainingJobs),
			"now", now,
		)
	}

	// Move ready jobs to main queue
	for _, delayedJob := range readyJobs {
		err := dq.mainQueue.SubmitJob(delayedJob.Event, delayedJob.Handler, delayedJob.Priority)
		if err != nil {
			dq.logger.Error("Failed to move delayed job to main queue",
				"job_id", delayedJob.ID,
				"error", err,
			)
			// Re-add to delayed queue with short delay
			if retryErr := dq.SubmitDelayedJob(
				delayedJob.Event,
				delayedJob.Handler,
				1*time.Second,
				delayedJob.Priority,
			); retryErr != nil {
				dq.logger.Error("Failed to re-add delayed job", "job_id", delayedJob.ID, "error", retryErr)
			}
		} else {
			dq.logger.Info("Moved delayed job to main queue",
				"job_id", delayedJob.ID,
				"priority", delayedJob.Priority,
				"delay_actual", time.Since(delayedJob.CreatedAt),
				"scheduled_at", delayedJob.ScheduledAt,
			)
		}
	}

	if len(readyJobs) > 0 {
		dq.logger.Info("Processed ready jobs", "count", len(readyJobs))
	}
}

// WaitForReadyJobs waits for jobs to be ready and moves them to main queue
// This is useful for testing and synchronization
func (dq *DelayedQueue) WaitForReadyJobs(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(dq.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(WaitForReadyInterval)
	defer ticker.Stop()

	initialJobCount := 0
	dq.mutex.RLock()
	initialJobCount = len(dq.jobs)
	dq.mutex.RUnlock()

	if initialJobCount == 0 {
		return nil // No jobs to wait for
	}

	dq.logger.Debug("WaitForReadyJobs started", "initial_jobs", initialJobCount, "timeout", timeout)

	for {
		select {
		case <-ctx.Done():
			dq.logger.Debug("WaitForReadyJobs timeout", "error", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			dq.processReadyJobs()

			// Check if all jobs are processed
			dq.mutex.RLock()
			remainingJobs := len(dq.jobs)
			dq.mutex.RUnlock()

			dq.logger.Debug("WaitForReadyJobs check", "remaining_jobs", remainingJobs, "initial_jobs", initialJobCount)

			if remainingJobs == 0 {
				dq.logger.Debug("WaitForReadyJobs completed", "processed_jobs", initialJobCount)
				return nil
			}
		}
	}
}

// GetStats returns statistics about delayed jobs
func (dq *DelayedQueue) GetStats() map[string]interface{} {
	dq.mutex.RLock()
	defer dq.mutex.RUnlock()

	now := time.Now()
	var readyCount int
	var totalDelay time.Duration

	for _, job := range dq.jobs {
		if now.After(job.ScheduledAt) || now.Equal(job.ScheduledAt) {
			readyCount++
		}
		totalDelay += job.ScheduledAt.Sub(job.CreatedAt)
	}

	avgDelay := time.Duration(0)
	if len(dq.jobs) > 0 {
		avgDelay = totalDelay / time.Duration(len(dq.jobs))
	}

	return map[string]interface{}{
		"total_delayed_jobs": len(dq.jobs),
		"ready_jobs":         readyCount,
		"average_delay":      avgDelay.String(),
	}
}

// Shutdown gracefully shuts down the delayed queue
func (dq *DelayedQueue) Shutdown() {
	dq.logger.Info("Shutting down delayed queue")
	dq.cancel()
	dq.wg.Wait()
	dq.logger.Info("Delayed queue shutdown complete")
}

// GetPendingJobs returns a copy of pending delayed jobs (for monitoring)
func (dq *DelayedQueue) GetPendingJobs() []*DelayedJob {
	dq.mutex.RLock()
	defer dq.mutex.RUnlock()

	jobs := make([]*DelayedJob, len(dq.jobs))
	copy(jobs, dq.jobs)
	return jobs
}

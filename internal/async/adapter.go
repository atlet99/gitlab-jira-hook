package async

import (
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// WorkerPoolAdapter adapts PriorityWorkerPool to webhook.WorkerPoolInterface
type WorkerPoolAdapter struct {
	pool *PriorityWorkerPool
}

// NewWorkerPoolAdapter creates a new adapter for PriorityWorkerPool
func NewWorkerPoolAdapter(pool *PriorityWorkerPool) *WorkerPoolAdapter {
	return &WorkerPoolAdapter{
		pool: pool,
	}
}

// SubmitJob submits a job to the priority worker pool
func (a *WorkerPoolAdapter) SubmitJob(event *webhook.Event, handler webhook.EventHandler) error {
	return a.pool.SubmitJob(event, handler)
}

// GetStats returns pool statistics in the format expected by webhook.WorkerPoolInterface
func (a *WorkerPoolAdapter) GetStats() webhook.PoolStats {
	stats := a.pool.GetStats()

	return webhook.PoolStats{
		TotalJobsProcessed: stats.TotalJobsProcessed,
		SuccessfulJobs:     stats.SuccessfulJobs,
		FailedJobs:         stats.FailedJobs,
		AverageJobTime:     stats.AverageJobTime,
		ActiveWorkers:      stats.ActiveWorkers,
		QueueSize:          int(stats.QueueStats.PendingJobs),
		QueueCapacity:      a.pool.config.JobQueueSize,
		QueueLength:        int(stats.QueueStats.PendingJobs),
		CurrentWorkers:     stats.TotalWorkers,
		ScaleUpEvents:      0, // Not available in PriorityPoolStats
		ScaleDownEvents:    0, // Not available in PriorityPoolStats
	}
}

// Start starts the priority worker pool
func (a *WorkerPoolAdapter) Start() {
	a.pool.Start()
}

// Stop stops the priority worker pool
func (a *WorkerPoolAdapter) Stop() {
	a.pool.Stop()
}

// SubmitDelayedJob submits a delayed job to the priority worker pool
func (a *WorkerPoolAdapter) SubmitDelayedJob(event *webhook.Event, handler webhook.EventHandler, delay time.Duration, priority ...JobPriority) error {
	return a.pool.SubmitDelayedJob(event, handler, delay, priority...)
}

// GetDelayedQueueStats returns delayed queue statistics
func (a *WorkerPoolAdapter) GetDelayedQueueStats() map[string]interface{} {
	return a.pool.GetDelayedQueueStats()
}

// GetPendingDelayedJobs returns pending delayed jobs
func (a *WorkerPoolAdapter) GetPendingDelayedJobs() []*DelayedJob {
	return a.pool.GetPendingDelayedJobs()
}

// WaitForReadyJobs waits for ready jobs with timeout
func (a *WorkerPoolAdapter) WaitForReadyJobs(timeout time.Duration) error {
	return a.pool.WaitForReadyJobs(timeout)
}

// IsHealthy checks if the worker pool is healthy
func (a *WorkerPoolAdapter) IsHealthy() bool {
	return a.pool.IsHealthy()
}

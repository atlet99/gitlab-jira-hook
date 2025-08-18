package async

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

const (
	workerSleepMs             = 100
	averageJobTimeDivisor     = 2
	errorRateThreshold        = 10.0
	queueUtilizationThreshold = 95.0
	workerScaleUpDivisor      = 2
	workerScaleDownDivisor    = 4
)

// PriorityWorkerPool represents a worker pool that uses priority-based job processing
type PriorityWorkerPool struct {
	config       *config.Config
	logger       *slog.Logger
	monitor      *monitoring.WebhookMonitor
	queue        *PriorityQueue
	delayedQueue *DelayedQueue
	workers      []*PriorityWorker
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	stats        *PriorityPoolStats
	statsMutex   sync.RWMutex
	middleware   *MiddlewareChain
	decider      PriorityDecider
}

// PriorityWorker represents a worker in the priority pool
type PriorityWorker struct {
	id       int
	pool     *PriorityWorkerPool
	ctx      context.Context
	active   bool
	mu       sync.RWMutex
	jobCount int64
}

// PriorityPoolStats represents statistics for the priority worker pool
type PriorityPoolStats struct {
	TotalWorkers       int
	ActiveWorkers      int
	IdleWorkers        int
	TotalJobsProcessed int64
	SuccessfulJobs     int64
	FailedJobs         int64
	AverageJobTime     time.Duration
	QueueStats         QueueStats
	LastJobProcessed   *time.Time
}

// NewPriorityWorkerPool creates a new priority-based worker pool
func NewPriorityWorkerPool(
	cfg *config.Config,
	logger *slog.Logger,
	monitor *monitoring.WebhookMonitor,
	decider PriorityDecider,
) *PriorityWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	// Create default job handler
	defaultHandler := func(ctx context.Context, job *Job) error {
		return job.Handler.ProcessEventAsync(ctx, job.Event)
	}

	// Create middleware chain with default handler
	middleware := NewMiddlewareChain(defaultHandler)

	// Create priority queue with decider
	queue := NewPriorityQueue(cfg, decider, logger)

	// Create delayed queue
	delayedQueue := NewDelayedQueue(cfg, logger, queue, decider)

	pool := &PriorityWorkerPool{
		config:       cfg,
		logger:       logger,
		monitor:      monitor,
		queue:        queue,
		delayedQueue: delayedQueue,
		workers:      make([]*PriorityWorker, 0, cfg.MaxWorkers),
		ctx:          ctx,
		cancel:       cancel,
		stats:        &PriorityPoolStats{},
		middleware:   middleware,
		decider:      decider,
	}

	return pool
}

// UseMiddleware adds middleware to the worker pool
func (pwp *PriorityWorkerPool) UseMiddleware(middleware JobMiddleware) *PriorityWorkerPool {
	pwp.middleware.Use(middleware)
	return pwp
}

// BuildMiddleware builds the middleware chain
func (pwp *PriorityWorkerPool) BuildMiddleware() {
	// Build the middleware chain
	pwp.middleware.Build()
}

// Start launches the priority worker pool
func (pwp *PriorityWorkerPool) Start() {
	pwp.logger.Info("Starting priority worker pool",
		"min_workers", pwp.config.MinWorkers,
		"max_workers", pwp.config.MaxWorkers,
		"queue_size", pwp.config.JobQueueSize,
		"job_timeout_seconds", pwp.config.JobTimeoutSeconds)

	// Start initial workers
	for i := 0; i < pwp.config.MinWorkers; i++ {
		worker := pwp.createWorker()
		pwp.workers = append(pwp.workers, worker)
		go worker.start()
		pwp.logger.Debug("Started priority worker", "worker_id", worker.id, "total_workers", len(pwp.workers))
	}

	// Start scaling monitor
	go pwp.scalingMonitor()

	pwp.logger.Info("Priority worker pool started successfully",
		"initial_workers", len(pwp.workers),
		"queue_capacity", pwp.config.JobQueueSize)
}

// Stop gracefully shuts down the worker pool and delayed queue
func (pwp *PriorityWorkerPool) Stop() {
	pwp.logger.Info("Stopping priority worker pool")

	// Cancel context to stop all workers
	pwp.cancel()

	// Wait for all workers to finish
	pwp.wg.Wait()

	// Stop delayed queue
	pwp.delayedQueue.Shutdown()

	// Stop main queue
	pwp.queue.Shutdown()

	// Get final statistics
	stats := pwp.GetStats()
	pwp.logger.Info("Priority worker pool stopped",
		"total_jobs_processed", stats.TotalJobsProcessed,
		"successful_jobs", stats.SuccessfulJobs,
		"failed_jobs", stats.FailedJobs,
		"average_job_time", stats.AverageJobTime)
}

// SubmitJob submits a job to the priority queue with dynamic priority determination
func (pwp *PriorityWorkerPool) SubmitJob(
	event *webhook.Event,
	handler webhook.EventHandler,
	priorityOverride ...JobPriority,
) error {
	return pwp.queue.SubmitJob(event, handler, priorityOverride...)
}

// SubmitJobWithDefaultPriority submits a job with priority determined by decider
func (pwp *PriorityWorkerPool) SubmitJobWithDefaultPriority(event *webhook.Event, handler webhook.EventHandler) error {
	return pwp.queue.SubmitJob(event, handler)
}

// SubmitDelayedJob submits a job to be executed after the specified delay
func (pwp *PriorityWorkerPool) SubmitDelayedJob(
	event *webhook.Event,
	handler webhook.EventHandler,
	delay time.Duration,
	priorityOverride ...JobPriority,
) error {
	return pwp.delayedQueue.SubmitDelayedJob(event, handler, delay, priorityOverride...)
}

// GetStats returns current pool statistics
func (pwp *PriorityWorkerPool) GetStats() PriorityPoolStats {
	pwp.statsMutex.RLock()
	defer pwp.statsMutex.RUnlock()

	stats := *pwp.stats

	// Update queue stats
	stats.QueueStats = pwp.queue.GetStats()

	// Update worker stats
	activeWorkers := 0
	for _, worker := range pwp.workers {
		worker.mu.RLock()
		if worker.active {
			activeWorkers++
		}
		worker.mu.RUnlock()
	}

	stats.TotalWorkers = len(pwp.workers)
	stats.ActiveWorkers = activeWorkers
	stats.IdleWorkers = stats.TotalWorkers - activeWorkers

	return stats
}

// GetDelayedQueueStats returns statistics about delayed jobs
func (pwp *PriorityWorkerPool) GetDelayedQueueStats() map[string]interface{} {
	return pwp.delayedQueue.GetStats()
}

// GetPendingDelayedJobs returns a copy of pending delayed jobs
func (pwp *PriorityWorkerPool) GetPendingDelayedJobs() []*DelayedJob {
	return pwp.delayedQueue.GetPendingJobs()
}

// WaitForReadyJobs waits for delayed jobs to be ready and moved to main queue
func (pwp *PriorityWorkerPool) WaitForReadyJobs(timeout time.Duration) error {
	return pwp.delayedQueue.WaitForReadyJobs(timeout)
}

// createWorker creates a new priority worker
func (pwp *PriorityWorkerPool) createWorker() *PriorityWorker {
	pwp.statsMutex.Lock()
	defer pwp.statsMutex.Unlock()

	workerID := len(pwp.workers) + 1
	worker := &PriorityWorker{
		id:     workerID,
		pool:   pwp,
		ctx:    pwp.ctx,
		mu:     sync.RWMutex{},
		active: false,
	}

	pwp.wg.Add(1)
	pwp.logger.Debug("Created priority worker", "worker_id", workerID)
	return worker
}

// scalingMonitor monitors queue and scales workers accordingly
func (pwp *PriorityWorkerPool) scalingMonitor() {
	// Ensure ScaleInterval is at least 1 second to avoid panic
	scaleInterval := pwp.config.ScaleInterval
	if scaleInterval <= 0 {
		scaleInterval = 10 // Default to 10 seconds if not set
		pwp.logger.Warn("ScaleInterval was 0 or negative, using default value", "default_interval", scaleInterval)
	}

	ticker := time.NewTicker(time.Duration(scaleInterval) * time.Second)
	defer ticker.Stop()

	pwp.logger.Debug("Started scaling monitor", "check_interval_seconds", scaleInterval)

	for {
		select {
		case <-pwp.ctx.Done():
			pwp.logger.Debug("Scaling monitor stopped")
			return
		case <-ticker.C:
			pwp.checkScaling()
		}
	}
}

// checkScaling checks if scaling is needed and performs it
func (pwp *PriorityWorkerPool) checkScaling() {
	stats := pwp.GetStats()
	activeWorkers := len(pwp.workers)

	pwp.logger.Debug("Checking scaling conditions",
		"active_workers", activeWorkers,
		"pending_jobs", stats.QueueStats.PendingJobs,
		"processing_jobs", stats.QueueStats.ProcessingJobs,
		"queue_utilization", stats.QueueStats.QueueUtilization,
		"error_rate", stats.QueueStats.ErrorRate,
		"total_jobs", stats.QueueStats.TotalJobs,
		"failed_jobs", stats.QueueStats.FailedJobs,
		"successful_jobs", stats.QueueStats.CompletedJobs)

	// Scale up if queue is getting full
	if stats.QueueStats.PendingJobs > int64(pwp.config.JobQueueSize/workerScaleUpDivisor) &&
		activeWorkers < pwp.config.MaxWorkers {
		pwp.ScaleUp()
	}

	// Scale down if queue is mostly empty and we have more than minimum workers
	if stats.QueueStats.PendingJobs < int64(pwp.config.JobQueueSize/workerScaleDownDivisor) &&
		activeWorkers > pwp.config.MinWorkers {
		pwp.ScaleDown()
	}
}

// ScaleUp adds a new worker to the pool
func (pwp *PriorityWorkerPool) ScaleUp() {
	pwp.statsMutex.Lock()
	defer pwp.statsMutex.Unlock()

	if len(pwp.workers) >= pwp.config.MaxWorkers {
		pwp.logger.Debug("Cannot scale up: maximum workers reached",
			"current_workers", len(pwp.workers),
			"max_workers", pwp.config.MaxWorkers)
		return
	}

	worker := pwp.createWorker()
	pwp.workers = append(pwp.workers, worker)
	go worker.start()

	pwp.logger.Info("Scaled up priority worker pool",
		"new_worker_id", worker.id,
		"total_workers", len(pwp.workers),
		"queue_stats", pwp.queue.GetStats())
}

// ScaleDown removes a worker from the pool
func (pwp *PriorityWorkerPool) ScaleDown() {
	pwp.statsMutex.Lock()
	defer pwp.statsMutex.Unlock()

	if len(pwp.workers) <= pwp.config.MinWorkers {
		pwp.logger.Debug("Cannot scale down: minimum workers reached",
			"current_workers", len(pwp.workers),
			"min_workers", pwp.config.MinWorkers)
		return
	}

	// Find the least busy worker
	var leastBusyWorker *PriorityWorker
	var minJobCount int64 = math.MaxInt64

	for _, worker := range pwp.workers {
		worker.mu.Lock()
		if !worker.active && worker.jobCount < minJobCount {
			minJobCount = worker.jobCount
			leastBusyWorker = worker
		}
		worker.mu.Unlock()
	}

	if leastBusyWorker != nil {
		// Remove worker from slice
		for i, worker := range pwp.workers {
			if worker == leastBusyWorker {
				pwp.workers = append(pwp.workers[:i], pwp.workers[i+1:]...)
				break
			}
		}

		pwp.logger.Info("Scaled down priority worker pool",
			"removed_worker_id", leastBusyWorker.id,
			"total_workers", len(pwp.workers),
			"worker_job_count", leastBusyWorker.jobCount)
	}
}

// GetQueueStats returns queue statistics
func (pwp *PriorityWorkerPool) GetQueueStats() QueueStats {
	return pwp.queue.GetStats()
}

// IsHealthy returns true if the pool is healthy
func (pwp *PriorityWorkerPool) IsHealthy() bool {
	stats := pwp.GetStats()
	if stats.QueueStats.ErrorRate > errorRateThreshold {
		return false
	}
	if stats.QueueStats.QueueUtilization > queueUtilizationThreshold {
		return false
	}
	if stats.ActiveWorkers == 0 && stats.QueueStats.PendingJobs > 0 {
		return false
	}
	return true
}

// start starts the worker
func (w *PriorityWorker) start() {
	defer w.pool.wg.Done()
	w.pool.logger.Debug("Priority worker started", "worker_id", w.id)

	jobCount := 0
	for {
		select {
		case <-w.ctx.Done():
			w.pool.logger.Debug("Priority worker context canceled", "worker_id", w.id, "jobs_processed", jobCount)
			return
		default:
			job, err := w.pool.queue.GetJob()
			if err != nil {
				// Check context before sleeping
				select {
				case <-w.ctx.Done():
					w.pool.logger.Debug("Priority worker context canceled during sleep", "worker_id", w.id)
					return
				case <-time.After(workerSleepMs * time.Millisecond):
					continue
				}
			}

			jobCount++
			w.pool.logger.Debug("Priority worker processing job",
				"worker_id", w.id,
				"job_id", job.ID,
				"job_priority", job.Priority,
				"event_type", job.Event.Type,
				"total_jobs_processed", jobCount)

			w.processJob(job)
		}
	}
}

// processJob processes a job
func (w *PriorityWorker) processJob(job *Job) {
	start := time.Now()

	// Mark worker as active
	w.mu.Lock()
	w.active = true
	w.mu.Unlock()

	// Process the event using middleware chain
	handler := w.pool.middleware.Build()
	err := handler(job.ctx, job)

	// Check if the error is due to context cancellation
	if err != nil && job.ctx.Err() == context.Canceled {
		w.pool.logger.Warn("Job canceled due to timeout or context cancellation",
			"worker_id", w.id,
			"job_id", job.ID,
			"event_type", job.Event.Type,
			"duration_ms", time.Since(start).Milliseconds(),
			"retry_count", job.RetryCount,
			"timeout_seconds", w.pool.config.JobTimeoutSeconds)
	} else if err != nil {
		w.pool.logger.Warn("Job failed with non-cancellation error",
			"worker_id", w.id,
			"job_id", job.ID,
			"event_type", job.Event.Type,
			"duration_ms", time.Since(start).Milliseconds(),
			"retry_count", job.RetryCount,
			"error_type", fmt.Sprintf("%T", err))
	}

	// Complete the job
	w.pool.queue.CompleteJob(job, err)

	duration := time.Since(start)

	// Mark worker as inactive
	w.mu.Lock()
	w.active = false
	w.mu.Unlock()

	// Update statistics
	w.updateStats(job, duration)

	w.pool.logger.Debug("Processing priority job",
		"worker_id", w.id,
		"job_id", job.ID,
		"priority", job.Priority,
		"event_type", job.Event.Type,
		"retry_count", job.RetryCount,
		"started_at", start)

	if err != nil {
		w.pool.logger.Error("Priority job processing failed",
			"worker_id", w.id,
			"job_id", job.ID,
			"event_type", job.Event.Type,
			"duration_ms", duration.Milliseconds(),
			"error", err,
			"retry_count", job.RetryCount)
	} else {
		w.pool.logger.Info("Priority job processed successfully",
			"worker_id", w.id,
			"job_id", job.ID,
			"event_type", job.Event.Type,
			"duration_ms", duration.Milliseconds(),
			"priority", job.Priority)
	}
}

// updateStats updates pool statistics
func (w *PriorityWorker) updateStats(job *Job, duration time.Duration) {
	w.pool.statsMutex.Lock()
	defer w.pool.statsMutex.Unlock()

	w.pool.stats.TotalJobsProcessed++
	w.jobCount++

	if w.pool.stats.AverageJobTime == 0 {
		w.pool.stats.AverageJobTime = duration
	} else {
		w.pool.stats.AverageJobTime = (w.pool.stats.AverageJobTime + duration) / averageJobTimeDivisor
	}

	switch job.Status {
	case StatusCompleted:
		w.pool.stats.SuccessfulJobs++
		w.pool.logger.Debug("Job completed successfully",
			"worker_id", w.id,
			"job_id", job.ID,
			"duration", duration,
			"total_successful", w.pool.stats.SuccessfulJobs)
	case StatusFailed:
		w.pool.stats.FailedJobs++
		w.pool.logger.Debug("Job failed",
			"worker_id", w.id,
			"job_id", job.ID,
			"duration", duration,
			"total_failed", w.pool.stats.FailedJobs)
	}

	now := time.Now()
	w.pool.stats.LastJobProcessed = &now

	// Log periodic statistics
	if w.pool.stats.TotalJobsProcessed%10 == 0 {
		w.pool.logger.Info("Worker pool statistics update",
			"total_jobs_processed", w.pool.stats.TotalJobsProcessed,
			"successful_jobs", w.pool.stats.SuccessfulJobs,
			"failed_jobs", w.pool.stats.FailedJobs,
			"average_job_time", w.pool.stats.AverageJobTime,
			"active_workers", len(w.pool.workers))
	}
}

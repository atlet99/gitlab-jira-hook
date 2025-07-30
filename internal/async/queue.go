package async

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// JobPriority is the priority of a job in the queue.
type JobPriority int

// Job priority levels from lowest to highest
const (
	PriorityLow      JobPriority = iota // Low priority
	PriorityNormal                      // Normal priority
	PriorityHigh                        // High priority
	PriorityCritical                    // Critical priority
)

// JobStatus is the status of a job in the queue.
type JobStatus int

// Job status values
const (
	StatusPending    JobStatus = iota // Pending
	StatusProcessing                  // Processing
	StatusCompleted                   // Completed
	StatusFailed                      // Failed
	StatusRetrying                    // Retrying
)

const (
	queueDivisorLow          = 4
	queueDivisorNormal       = 2
	queueDivisorHigh         = 4
	queueDivisorCritical     = 8
	averageProcessingDivisor = 2
	magic100                 = 100
	magic90                  = 90
	magic2                   = 2
	healthErrorRateThreshold = 10.0
	healthQueueUtilThreshold = 90.0
)

// Job represents a webhook processing job with enhanced features
type Job struct {
	ID          string
	Event       *webhook.Event
	Handler     webhook.EventHandler
	Priority    JobPriority
	Status      JobStatus
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	RetryCount  int
	MaxRetries  int
	LastError   error
	Attempts    []JobAttempt
	Metadata    map[string]interface{}

	// Rate limiting
	rateLimiter *rate.Limiter

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// JobAttempt represents a single attempt to process a job
type JobAttempt struct {
	AttemptNumber int
	StartedAt     time.Time
	CompletedAt   *time.Time
	Duration      time.Duration
	Error         error
	Success       bool
}

// QueueStats represents queue statistics
type QueueStats struct {
	TotalJobs      int64
	PendingJobs    int64
	ProcessingJobs int64
	CompletedJobs  int64
	FailedJobs     int64
	RetryingJobs   int64

	// Priority breakdown
	LowPriorityJobs      int64
	NormalPriorityJobs   int64
	HighPriorityJobs     int64
	CriticalPriorityJobs int64

	// Performance metrics
	AverageProcessingTime time.Duration
	AverageWaitTime       time.Duration
	ThroughputPerSecond   float64

	// Error rates
	ErrorRate float64

	// Queue health
	QueueUtilization  float64
	WorkerUtilization float64

	// Timestamps
	LastJobProcessedAt *time.Time
	LastJobFailedAt    *time.Time
}

// PriorityDecider determines the priority of a job at runtime
// Can be injected via DI
// event, handler, payload -> JobPriority
type PriorityDecider interface {
	DecidePriority(event *webhook.Event, handler webhook.EventHandler) JobPriority
}

// DefaultPriorityDecider implements the default strategy (by event type)
type DefaultPriorityDecider struct{}

// DecidePriority determines job priority based on event type
func (d *DefaultPriorityDecider) DecidePriority(event *webhook.Event, _ webhook.EventHandler) JobPriority {
	switch event.Type {
	case "merge_request":
		return PriorityHigh
	case "push":
		return PriorityNormal
	case "comment":
		return PriorityLow
	default:
		return PriorityNormal
	}
}

// PriorityQueue now supports dynamic prioritization
type PriorityQueue struct {
	config     *config.Config
	queues     map[JobPriority]*jobQueue
	stats      *QueueStats
	statsMutex sync.RWMutex
	decider    PriorityDecider

	// Rate limiting
	globalRateLimiter *rate.Limiter

	// Monitoring
	metricsEnabled bool
	healthChecker  *HealthChecker

	// Logging
	logger *slog.Logger

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// jobQueue represents a single priority queue
type jobQueue struct {
	priority JobPriority
	jobs     chan *Job
	stats    *QueueStats
}

// NewPriorityQueue creates a new priority-based job queue
func NewPriorityQueue(cfg *config.Config, decider PriorityDecider, logger *slog.Logger) *PriorityQueue {
	if decider == nil {
		decider = &DefaultPriorityDecider{}
	}
	ctx, cancel := context.WithCancel(context.Background())

	pq := &PriorityQueue{
		config:            cfg,
		queues:            make(map[JobPriority]*jobQueue),
		stats:             &QueueStats{},
		decider:           decider,
		globalRateLimiter: rate.NewLimiter(rate.Limit(cfg.MaxConcurrentJobs), cfg.MaxConcurrentJobs),
		metricsEnabled:    cfg.MetricsEnabled,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Initialize priority queues
	queueSizes := map[JobPriority]int{
		PriorityLow:      cfg.JobQueueSize / queueDivisorLow,
		PriorityNormal:   cfg.JobQueueSize / queueDivisorNormal,
		PriorityHigh:     cfg.JobQueueSize / queueDivisorHigh,
		PriorityCritical: cfg.JobQueueSize / queueDivisorCritical,
	}

	for priority, size := range queueSizes {
		pq.queues[priority] = &jobQueue{
			priority: priority,
			jobs:     make(chan *Job, size),
			stats:    &QueueStats{},
		}
	}

	// Start health checker if metrics are enabled
	if pq.metricsEnabled {
		pq.healthChecker = NewHealthChecker(cfg, pq)
		pq.healthChecker.Start()
	}

	return pq
}

// SubmitJob submits a job to the appropriate priority queue
func (pq *PriorityQueue) SubmitJob(
	event *webhook.Event,
	handler webhook.EventHandler,
	priorityOverride ...JobPriority,
) error {
	priority := PriorityNormal
	if len(priorityOverride) > 0 {
		priority = priorityOverride[0]
	} else if pq.decider != nil {
		priority = pq.decider.DecidePriority(event, handler)
	}

	job := &Job{
		ID:          generateJobID(),
		Event:       event,
		Handler:     handler,
		Priority:    priority,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		MaxRetries:  pq.config.MaxRetries,
		Attempts:    make([]JobAttempt, 0),
		Metadata:    make(map[string]interface{}),
		rateLimiter: rate.NewLimiter(rate.Limit(pq.config.JiraRateLimit), pq.config.JiraRateLimit),
	}

	job.ctx, job.cancel = context.WithTimeout(pq.ctx, time.Duration(pq.config.JobTimeoutSeconds)*time.Second)

	// Select the appropriate queue
	queue, exists := pq.queues[priority]
	if !exists {
		return fmt.Errorf("invalid priority: %d", priority)
	}

	// Try to submit with timeout
	select {
	case queue.jobs <- job:
		pq.updateStats(job, "submitted")
		return nil
	case <-time.After(time.Duration(pq.config.QueueTimeoutMs) * time.Millisecond):
		return fmt.Errorf("queue timeout: unable to submit job within %dms", pq.config.QueueTimeoutMs)
	case <-pq.ctx.Done():
		return fmt.Errorf("queue shutdown: unable to submit job")
	}
}

// GetJob retrieves the next job from the highest priority non-empty queue
func (pq *PriorityQueue) GetJob() (*Job, error) {
	// Check queues in priority order (Critical -> High -> Normal -> Low)
	priorities := []JobPriority{PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow}

	for _, priority := range priorities {
		queue, exists := pq.queues[priority]
		if !exists {
			continue
		}

		select {
		case job := <-queue.jobs:
			job.Status = StatusProcessing
			now := time.Now()
			job.StartedAt = &now
			pq.updateStats(job, "processing")
			return job, nil
		default:
			// Queue is empty, try next priority
			continue
		}
	}

	return nil, fmt.Errorf("no jobs available")
}

// CompleteJob marks a job as completed
func (pq *PriorityQueue) CompleteJob(job *Job, err error) {
	now := time.Now()
	job.CompletedAt = &now

	if err != nil {
		job.LastError = err
		job.Status = StatusFailed
		pq.updateStats(job, "failed")

		// Check if we should retry (don't retry context cancellation errors)
		if job.RetryCount < job.MaxRetries && job.ctx.Err() != context.Canceled {
			job.Status = StatusRetrying
			job.RetryCount++
			pq.scheduleRetry(job)
		} else if job.ctx.Err() == context.Canceled {
			// Log context cancellation without retry
			pq.logger.Warn("Job cancelled, skipping retry",
				"job_id", job.ID,
				"event_type", job.Event.Type,
				"retry_count", job.RetryCount)
		}
	} else {
		job.Status = StatusCompleted
		pq.updateStats(job, "completed")
	}

	// Record attempt
	attempt := JobAttempt{
		AttemptNumber: job.RetryCount + 1,
		CompletedAt:   &now,
		Error:         err,
		Success:       err == nil,
	}

	// Set StartedAt and Duration if available
	if job.StartedAt != nil {
		attempt.StartedAt = *job.StartedAt
		attempt.Duration = now.Sub(*job.StartedAt)
	} else {
		attempt.StartedAt = now
		attempt.Duration = 0
	}
	job.Attempts = append(job.Attempts, attempt)

	// Clean up
	job.cancel()
}

// scheduleRetry schedules a job for retry with exponential backoff
func (pq *PriorityQueue) scheduleRetry(job *Job) {
	delay := time.Duration(pq.config.RetryDelayMs) * time.Millisecond

	// Apply exponential backoff
	for i := 0; i < job.RetryCount-1; i++ {
		delay = time.Duration(float64(delay) * pq.config.BackoffMultiplier)
		if delay > time.Duration(pq.config.MaxBackoffMs)*time.Millisecond {
			delay = time.Duration(pq.config.MaxBackoffMs) * time.Millisecond
			break
		}
	}

	// Reset job for retry
	job.Status = StatusPending
	job.StartedAt = nil
	job.CompletedAt = nil
	job.LastError = nil

	// Schedule retry
	go func() {
		time.Sleep(delay)

		// Check if queue is still running
		select {
		case <-pq.ctx.Done():
			return
		default:
		}

		// Re-submit to queue
		queue := pq.queues[job.Priority]
		select {
		case queue.jobs <- job:
			pq.updateStats(job, "retry_scheduled")
		default:
			// Queue is full, mark as failed
			job.Status = StatusFailed
			job.LastError = fmt.Errorf("queue full during retry")
			pq.updateStats(job, "retry_failed")
		}
	}()
}

// GetStats returns current queue statistics
func (pq *PriorityQueue) GetStats() QueueStats {
	pq.statsMutex.RLock()
	defer pq.statsMutex.RUnlock()

	stats := *pq.stats

	// Calculate derived metrics
	if stats.TotalJobs > 0 {
		stats.ErrorRate = float64(stats.FailedJobs) / float64(stats.TotalJobs) * magic100
	}

	// Calculate queue utilization
	totalCapacity := 0
	for _, queue := range pq.queues {
		totalCapacity += cap(queue.jobs)
	}
	if totalCapacity > 0 {
		stats.QueueUtilization = float64(stats.PendingJobs) / float64(totalCapacity) * magic100
	}

	return stats
}

// updateStats updates queue statistics
func (pq *PriorityQueue) updateStats(job *Job, action string) {
	pq.statsMutex.Lock()
	defer pq.statsMutex.Unlock()

	now := time.Now()

	switch action {
	case "submitted":
		pq.stats.TotalJobs++
		pq.stats.PendingJobs++
		pq.updatePriorityStats(job.Priority, 1)

	case "processing":
		pq.stats.PendingJobs--
		pq.stats.ProcessingJobs++

	case "completed":
		pq.stats.ProcessingJobs--
		pq.stats.CompletedJobs++
		pq.stats.LastJobProcessedAt = &now
		if job.StartedAt != nil {
			duration := now.Sub(*job.StartedAt)
			if pq.stats.AverageProcessingTime == 0 {
				pq.stats.AverageProcessingTime = duration
			} else {
				pq.stats.AverageProcessingTime = (pq.stats.AverageProcessingTime + duration) / averageProcessingDivisor
			}
		}

	case "failed":
		pq.stats.ProcessingJobs--
		pq.stats.FailedJobs++
		pq.stats.LastJobFailedAt = &now

	case "retry_scheduled":
		pq.stats.RetryingJobs++

	case "retry_failed":
		pq.stats.RetryingJobs--
		pq.stats.FailedJobs++
	}
}

// updatePriorityStats updates priority-specific statistics
func (pq *PriorityQueue) updatePriorityStats(priority JobPriority, delta int64) {
	switch priority {
	case PriorityLow:
		pq.stats.LowPriorityJobs += delta
	case PriorityNormal:
		pq.stats.NormalPriorityJobs += delta
	case PriorityHigh:
		pq.stats.HighPriorityJobs += delta
	case PriorityCritical:
		pq.stats.CriticalPriorityJobs += delta
	}
}

// Shutdown gracefully shuts down the queue
func (pq *PriorityQueue) Shutdown() {
	pq.cancel()

	// Stop health checker
	if pq.healthChecker != nil {
		pq.healthChecker.Stop()
	}

	// Wait for all goroutines to complete
	pq.wg.Wait()
}

// HealthChecker monitors queue health and performance
type HealthChecker struct {
	config *config.Config
	queue  *PriorityQueue
	ticker *time.Ticker
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(cfg *config.Config, queue *PriorityQueue) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		config: cfg,
		queue:  queue,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	// Use default interval if HealthCheckInterval is 0 or negative
	interval := hc.config.HealthCheckInterval
	if interval <= 0 {
		interval = 30 // Default to 30 seconds
	}
	hc.ticker = time.NewTicker(time.Duration(interval) * time.Second)

	hc.wg.Add(1)
	go func() {
		defer hc.wg.Done()
		defer hc.ticker.Stop()

		for {
			select {
			case <-hc.ticker.C:
				hc.checkHealth()
			case <-hc.ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.cancel()
	hc.wg.Wait()
}

// checkHealth performs health checks on the queue
func (hc *HealthChecker) checkHealth() {
	stats := hc.queue.GetStats()

	// Check for high error rates
	if stats.ErrorRate > healthErrorRateThreshold { // 10% error rate threshold
		// TODO: Log warning or trigger alert
		_ = stats.ErrorRate // Suppress unused variable warning
	}

	// Check for queue overflow
	if stats.QueueUtilization > healthQueueUtilThreshold { // 90% utilization threshold
		// TODO: Log warning or trigger alert
		_ = stats.QueueUtilization // Suppress unused variable warning
	}

	// Check for stuck jobs
	if stats.ProcessingJobs > 0 {
		// TODO: Check if any jobs have been processing for too long
		// This would require additional tracking in the Job struct
		_ = stats.ProcessingJobs // Suppress unused variable warning
	}
}

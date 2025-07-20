// Package async provides asynchronous webhook processing functionality.
package async

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/monitoring"
	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

const (
	// DefaultWorkerPoolSize is the default number of workers in the pool
	DefaultWorkerPoolSize = 10
	// DefaultQueueSize is the default size of the job queue
	DefaultQueueSize = 100
	// DefaultWorkerTimeout is the default timeout for worker operations
	DefaultWorkerTimeout = 30 * time.Second
	// AverageJobTimeDivisor is used for calculating average job time
	AverageJobTimeDivisor = 2
	// JobIDRandomLength is the length of random part in job ID
	JobIDRandomLength = 8
)

// DefaultMaxWorkerMultiplier is the default multiplier for max workers if not set
const DefaultMaxWorkerMultiplier = 2

// WebhookJob represents a webhook processing job (legacy)
type WebhookJob struct {
	Event     *webhook.Event
	Handler   webhook.EventHandler
	StartTime time.Time
	ID        string
}

// WorkerPool manages a pool of workers for processing webhook jobs
type WorkerPool struct {
	config   *config.Config
	logger   *slog.Logger
	monitor  *monitoring.WebhookMonitor
	jobQueue chan *WebhookJob
	workers  []*Worker
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *webhook.PoolStats
	statsMu  sync.RWMutex
}

// Worker represents a single worker in the pool
type Worker struct {
	id       int
	pool     *WorkerPool
	jobQueue <-chan *WebhookJob
	ctx      context.Context
	active   bool
	mu       sync.RWMutex
}

// NewWorkerPool creates a new worker pool with the specified configuration
func NewWorkerPool(cfg *config.Config, logger *slog.Logger, monitor *monitoring.WebhookMonitor) *WorkerPool {
	poolSize := cfg.MinWorkers
	if poolSize <= 0 {
		poolSize = cfg.WorkerPoolSize
		if poolSize <= 0 {
			poolSize = DefaultWorkerPoolSize
		}
	}
	maxWorkers := cfg.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = poolSize * DefaultMaxWorkerMultiplier
	}
	queueSize := cfg.JobQueueSize
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		config:   cfg,
		logger:   logger,
		monitor:  monitor,
		jobQueue: make(chan *WebhookJob, queueSize),
		workers:  make([]*Worker, 0, maxWorkers),
		ctx:      ctx,
		cancel:   cancel,
		stats: &webhook.PoolStats{
			QueueCapacity:  queueSize,
			CurrentWorkers: 0,
		},
	}
	return pool
}

// Start launches the worker pool and scaling monitor
func (wp *WorkerPool) Start() {
	wp.logger.Info("Starting worker pool", "min_workers", wp.config.MinWorkers, "max_workers", wp.config.MaxWorkers)
	// Start initial workers
	for i := 0; i < wp.config.MinWorkers; i++ {
		wp.startWorker(i)
	}
	// Start scaling monitor
	go wp.scalingMonitor()
}

// scalingMonitor periodically checks the queue length and triggers scaling
func (wp *WorkerPool) scalingMonitor() {
	interval := wp.config.ScaleInterval
	if interval <= 0 {
		interval = 10
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			queueLen := len(wp.jobQueue)
			wp.statsMu.Lock()
			wp.stats.QueueLength = queueLen
			wp.stats.CurrentWorkers = len(wp.workers)
			wp.statsMu.Unlock()
			wp.logger.Debug("Scaling monitor tick", "queue_length", queueLen, "workers", len(wp.workers))
			if queueLen > wp.config.ScaleUpThreshold {
				wp.scaleUp()
			} else if queueLen < wp.config.ScaleDownThreshold {
				wp.scaleDown()
			}
		case <-wp.ctx.Done():
			return
		}
	}
}

// scaleUp increases the number of workers if below max
func (wp *WorkerPool) scaleUp() {
	wp.statsMu.Lock()
	defer wp.statsMu.Unlock()
	current := len(wp.workers)
	if current >= wp.config.MaxWorkers {
		wp.logger.Debug("Worker pool at max capacity", "current_workers", current)
		return
	}
	newCount := current + 1
	if newCount > wp.config.MaxWorkers {
		newCount = wp.config.MaxWorkers
	}
	wp.logger.Info("Scaling up worker pool", "from", current, "to", newCount)
	for i := current; i < newCount; i++ {
		wp.startWorker(i)
	}
	wp.stats.ScaleUpEvents++
	wp.stats.CurrentWorkers = newCount
}

// scaleDown decreases the number of workers if above min
func (wp *WorkerPool) scaleDown() {
	wp.statsMu.Lock()
	defer wp.statsMu.Unlock()
	current := len(wp.workers)
	if current <= wp.config.MinWorkers {
		wp.logger.Debug("Worker pool at min capacity", "current_workers", current)
		return
	}
	newCount := current - 1
	if newCount < wp.config.MinWorkers {
		newCount = wp.config.MinWorkers
	}
	wp.logger.Info("Scaling down worker pool", "from", current, "to", newCount)
	// Remove extra workers from the slice
	wp.workers = wp.workers[:newCount]
	wp.stats.ScaleDownEvents++
	wp.stats.CurrentWorkers = newCount
}

// Stop stops the worker pool and waits for all workers to finish
func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}

// SubmitJob submits a webhook job for processing
func (wp *WorkerPool) SubmitJob(event *webhook.Event, handler webhook.EventHandler) error {
	job := &WebhookJob{
		Event:     event,
		Handler:   handler,
		StartTime: time.Now(),
		ID:        generateJobID(),
	}

	select {
	case wp.jobQueue <- job:
		wp.updateQueueStats(1)
		wp.logger.Debug("Job submitted", "jobID", job.ID, "eventType", event.Type)
		return nil
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	default:
		wp.updateQueueStats(0)
		wp.logger.Warn("Job queue full, dropping job", "jobID", job.ID, "eventType", event.Type)
		return ErrQueueFull
	}
}

// GetStats returns current pool statistics
func (wp *WorkerPool) GetStats() webhook.PoolStats {
	wp.statsMu.RLock()
	defer wp.statsMu.RUnlock()
	stats := *wp.stats
	// Update current workers count based on actual workers
	stats.CurrentWorkers = len(wp.workers)
	// Count active workers
	activeWorkers := 0
	for _, worker := range wp.workers {
		worker.mu.RLock()
		if worker.active {
			activeWorkers++
		}
		worker.mu.RUnlock()
	}
	stats.ActiveWorkers = activeWorkers
	stats.QueueSize = len(wp.jobQueue)
	return stats
}

// updateQueueStats updates queue-related statistics
func (wp *WorkerPool) updateQueueStats(_ int) {
	// This is a simplified update - in real implementation you can add more logic
	// For now, we don't need to lock since we're not updating any stats
}

// startWorker launches a new worker with the given index and adds it to the pool
func (wp *WorkerPool) startWorker(index int) {
	worker := &Worker{
		id:   index,
		pool: wp,
		ctx:  wp.ctx,
	}
	wp.workers = append(wp.workers, worker)
	wp.wg.Add(1)
	go worker.start()
}

// start starts the worker
func (w *Worker) start() {
	defer w.pool.wg.Done()

	w.pool.logger.Debug("Worker started", "workerID", w.id)

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				// Channel closed, worker should exit
				w.pool.logger.Debug("Worker exiting", "workerID", w.id)
				return
			}

			w.processJob(job)

		case <-w.ctx.Done():
			w.pool.logger.Debug("Worker context canceled", "workerID", w.id)
			return
		}
	}
}

// processJob processes a webhook job
func (w *Worker) processJob(job *WebhookJob) {
	start := time.Now()

	// Mark worker as active
	w.mu.Lock()
	w.active = true
	w.mu.Unlock()

	defer func() {
		// Mark worker as inactive
		w.mu.Lock()
		w.active = false
		w.mu.Unlock()

		// Update statistics
		w.updateStats(job, time.Since(start))
	}()

	w.pool.logger.Debug("Processing job",
		"workerID", w.id,
		"jobID", job.ID,
		"eventType", job.Event.Type)

	// Create context with timeout for job processing
	ctx, cancel := context.WithTimeout(w.ctx, DefaultWorkerTimeout)
	defer cancel()

	// Process the event
	err := job.Handler.ProcessEventAsync(ctx, job.Event)

	if err != nil {
		w.pool.logger.Error("Job processing failed",
			"workerID", w.id,
			"jobID", job.ID,
			"eventType", job.Event.Type,
			"error", err)
	} else {
		w.pool.logger.Debug("Job processed successfully",
			"workerID", w.id,
			"jobID", job.ID,
			"eventType", job.Event.Type)
	}
}

// updateStats updates pool statistics
func (w *Worker) updateStats(_ *WebhookJob, duration time.Duration) {
	w.pool.statsMu.Lock()
	defer w.pool.statsMu.Unlock()

	w.pool.stats.TotalJobsProcessed++

	// Update average job time
	if w.pool.stats.AverageJobTime == 0 {
		w.pool.stats.AverageJobTime = duration
	} else {
		w.pool.stats.AverageJobTime = (w.pool.stats.AverageJobTime + duration) / AverageJobTimeDivisor
	}

	w.pool.stats.SuccessfulJobs++
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(JobIDRandomLength)
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

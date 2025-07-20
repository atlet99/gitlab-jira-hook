// Package async provides asynchronous webhook processing functionality.
package async

import (
	"context"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// WorkerPoolInterface defines the interface for worker pool operations
type WorkerPoolInterface interface {
	SubmitJob(event *webhook.Event, handler webhook.EventHandler) error
	SubmitDelayedJob(event *webhook.Event, handler webhook.EventHandler, delay time.Duration, priority ...JobPriority) error
	GetStats() webhook.PoolStats
	GetDelayedQueueStats() map[string]interface{}
	GetPendingDelayedJobs() []*DelayedJob
	WaitForReadyJobs(timeout time.Duration) error
	Start()
	Stop()
	IsHealthy() bool
}

// PriorityWorkerPoolInterface extends WorkerPoolInterface with priority-specific operations
type PriorityWorkerPoolInterface interface {
	WorkerPoolInterface
	UseMiddleware(middleware JobMiddleware) PriorityWorkerPoolInterface
	BuildMiddleware()
	ScaleUp()
	ScaleDown()
	GetQueueStats() QueueStats
}

// HandlerInterface defines the interface for event processing
type HandlerInterface interface {
	ProcessEventAsync(ctx context.Context, event *webhook.Event) error
}

// QueueInterface defines the interface for job queue operations
type QueueInterface interface {
	SubmitJob(job *Job) error
	GetJob() (*Job, error)
	CompleteJob(job *Job, err error)
	Shutdown()
	GetStats() map[string]interface{}
	IsEmpty() bool
	Size() int
	Capacity() int
}

// PriorityQueueInterface extends QueueInterface with priority-specific operations
type PriorityQueueInterface interface {
	QueueInterface
	SubmitJobWithPriority(job *Job, priority JobPriority) error
	GetJobWithTimeout(timeout time.Duration) (*Job, error)
	GetJobCount() int
	GetJobCountByPriority(priority JobPriority) int
}

// DelayedQueueInterface defines the interface for delayed job queue operations
type DelayedQueueInterface interface {
	SubmitDelayedJob(event *webhook.Event, handler webhook.EventHandler, delay time.Duration, priority ...JobPriority) error
	GetStats() map[string]interface{}
	GetPendingJobs() []*DelayedJob
	Shutdown()
	IsEmpty() bool
	Size() int
}

// JobProcessorInterface defines the interface for job processing
type JobProcessorInterface interface {
	ProcessJob(ctx context.Context, job *Job) error
	ProcessJobWithMiddleware(ctx context.Context, job *Job, middleware JobMiddleware) error
}

// ScalingInterface defines the interface for worker pool scaling
type ScalingInterface interface {
	ScaleUp() error
	ScaleDown() error
	GetScalingStats() ScalingStats
	IsScalingEnabled() bool
	SetScalingEnabled(enabled bool)
}

// MonitoringInterface defines the interface for monitoring and metrics
type MonitoringInterface interface {
	RecordJobStart(job *Job)
	RecordJobComplete(job *Job, duration time.Duration, err error)
	RecordJobRetry(job *Job, attempt int)
	RecordScalingEvent(eventType string)
	GetMetrics() map[string]interface{}
}

// ErrorHandlerInterface defines the interface for error handling
type ErrorHandlerInterface interface {
	HandleError(ctx context.Context, job *Job, err error) error
	ShouldRetry(job *Job, err error) bool
	GetRetryDelay(job *Job, attempt int) time.Duration
	GetMaxRetries(job *Job) int
}

// ConfigurationInterface defines the interface for configuration management
type ConfigurationInterface interface {
	GetWorkerPoolConfig() *WorkerPoolConfig
	GetQueueConfig() *QueueConfig
	GetMiddlewareConfig() *MiddlewareConfig
	Validate() error
}

// WorkerPoolConfig holds configuration for worker pools
type WorkerPoolConfig struct {
	MinWorkers          int           `json:"min_workers"`
	MaxWorkers          int           `json:"max_workers"`
	MaxConcurrentJobs   int           `json:"max_concurrent_jobs"`
	JobTimeoutSeconds   int           `json:"job_timeout_seconds"`
	QueueTimeoutMs      int           `json:"queue_timeout_ms"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelayMs        int           `json:"retry_delay_ms"`
	BackoffMultiplier   float64       `json:"backoff_multiplier"`
	MaxBackoffMs        int           `json:"max_backoff_ms"`
	MetricsEnabled      bool          `json:"metrics_enabled"`
	HealthCheckInterval int           `json:"health_check_interval"`
	ScalingEnabled      bool          `json:"scaling_enabled"`
	ScalingThreshold    int           `json:"scaling_threshold"`
	ScalingCooldown     time.Duration `json:"scaling_cooldown"`
}

// QueueConfig holds configuration for job queues
type QueueConfig struct {
	JobQueueSize       int           `json:"job_queue_size"`
	PriorityQueueSize  int           `json:"priority_queue_size"`
	DelayedQueueSize   int           `json:"delayed_queue_size"`
	QueueTimeoutMs     int           `json:"queue_timeout_ms"`
	MaxJobSize         int           `json:"max_job_size"`
	EnablePriority     bool          `json:"enable_priority"`
	EnableDelayedJobs  bool          `json:"enable_delayed_jobs"`
	MaxDelayedJobDelay time.Duration `json:"max_delayed_job_delay"`
}

// MiddlewareConfig holds configuration for middleware
type MiddlewareConfig struct {
	EnableLogging           bool          `json:"enable_logging"`
	EnableMetrics           bool          `json:"enable_metrics"`
	EnableRetry             bool          `json:"enable_retry"`
	EnableTimeout           bool          `json:"enable_timeout"`
	EnableCircuitBreaker    bool          `json:"enable_circuit_breaker"`
	EnableTracing           bool          `json:"enable_tracing"`
	EnableRateLimiting      bool          `json:"enable_rate_limiting"`
	DefaultTimeout          time.Duration `json:"default_timeout"`
	MaxRetries              int           `json:"max_retries"`
	RetryDelay              time.Duration `json:"retry_delay"`
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `json:"circuit_breaker_timeout"`
	RateLimitPerSecond      int           `json:"rate_limit_per_second"`
}

// ScalingStats holds statistics for scaling operations
type ScalingStats struct {
	ScaleUpEvents   int           `json:"scale_up_events"`
	ScaleDownEvents int           `json:"scale_down_events"`
	LastScaleUp     *time.Time    `json:"last_scale_up"`
	LastScaleDown   *time.Time    `json:"last_scale_down"`
	ScalingEnabled  bool          `json:"scaling_enabled"`
	CurrentWorkers  int           `json:"current_workers"`
	TargetWorkers   int           `json:"target_workers"`
	ScalingCooldown time.Duration `json:"scaling_cooldown"`
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Healthy   bool                   `json:"healthy"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	LastCheck time.Time              `json:"last_check"`
	Details   map[string]interface{} `json:"details"`
	Component string                 `json:"component"`
}

// HealthCheckerInterface defines the interface for health checking
type HealthCheckerInterface interface {
	CheckHealth() HealthStatus
	IsHealthy() bool
	GetHealthDetails() map[string]interface{}
}

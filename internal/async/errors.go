// Package async provides asynchronous webhook processing functionality.
package async

import "errors"

var (
	// ErrQueueFull is returned when the job queue is full
	ErrQueueFull = errors.New("job queue is full")
	// ErrWorkerPoolStopped is returned when the worker pool is stopped
	ErrWorkerPoolStopped = errors.New("worker pool is stopped")
	// ErrJobTimeout is returned when a job processing times out
	ErrJobTimeout = errors.New("job processing timeout")
)

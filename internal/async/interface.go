// Package async provides asynchronous webhook processing functionality.
package async

import (
	"context"

	"github.com/atlet99/gitlab-jira-hook/internal/webhook"
)

// WorkerPoolInterface defines the interface for worker pool operations
type WorkerPoolInterface interface {
	SubmitJob(event *webhook.Event, handler webhook.EventHandler) error
	GetStats() webhook.PoolStats
	Start()
	Stop()
}

// HandlerInterface defines the interface for event processing
type HandlerInterface interface {
	ProcessEventAsync(ctx context.Context, event *webhook.Event) error
}

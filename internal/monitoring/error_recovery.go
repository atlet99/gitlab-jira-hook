package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// StrategyRetry implements retry with exponential backoff
	StrategyRetry RecoveryStrategy = iota
	// StrategyCircuitBreaker implements circuit breaker pattern
	StrategyCircuitBreaker
	// StrategyFallback implements fallback to alternative implementation
	StrategyFallback
	// StrategyGracefulDegradation implements graceful degradation
	StrategyGracefulDegradation
	// StrategyRestart implements service restart
	StrategyRestart

	// Default values
	defaultMaxAttempts  = 3
	defaultBackoff      = 100 * time.Millisecond
	circuitBreakerDelay = 200 * time.Millisecond
	fallbackDelay       = 150 * time.Millisecond
	gracefulDelay       = 300 * time.Millisecond
	restartDelay        = 500 * time.Millisecond
	restartBackoff      = 2 * time.Second
	performanceBackoff  = 500 * time.Millisecond
)

// RecoveryStrategy defines the type of recovery strategy
type RecoveryStrategy int

// RecoveryAction defines a recovery action
type RecoveryAction struct {
	Strategy    RecoveryStrategy
	Description string
	Handler     func(ctx context.Context, err error) error
	MaxAttempts int
	Backoff     time.Duration
}

// ErrorRecoveryManager manages error recovery strategies
type ErrorRecoveryManager struct {
	actions map[string]*RecoveryAction
	history map[string][]RecoveryAttempt
	stats   map[string]*RecoveryStats
	logger  *slog.Logger
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// RecoveryAttempt represents a single recovery attempt
type RecoveryAttempt struct {
	Timestamp  time.Time
	Error      error
	Action     string
	Strategy   RecoveryStrategy
	Success    bool
	Duration   time.Duration
	AttemptNum int
}

// RecoveryStats holds statistics for recovery attempts
type RecoveryStats struct {
	TotalAttempts        int64
	SuccessfulRecoveries int64
	FailedRecoveries     int64
	LastAttemptTime      time.Time
	AverageRecoveryTime  time.Duration
	TotalRecoveryTime    time.Duration
}

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(logger *slog.Logger) *ErrorRecoveryManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ErrorRecoveryManager{
		actions: make(map[string]*RecoveryAction),
		history: make(map[string][]RecoveryAttempt),
		stats:   make(map[string]*RecoveryStats),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// RegisterAction registers a recovery action for a specific error type
func (erm *ErrorRecoveryManager) RegisterAction(errorType string, action *RecoveryAction) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.actions[errorType] = action
	erm.stats[errorType] = &RecoveryStats{}
	erm.history[errorType] = make([]RecoveryAttempt, 0)
}

// Recover attempts to recover from an error using registered strategies
func (erm *ErrorRecoveryManager) Recover(ctx context.Context, errorType string, err error) error {
	erm.mu.RLock()
	action, exists := erm.actions[errorType]
	erm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no recovery action registered for error type: %s", errorType)
	}

	attempt := &RecoveryAttempt{
		Timestamp: time.Now(),
		Error:     err,
		Action:    errorType,
		Strategy:  action.Strategy,
	}

	switch action.Strategy {
	case StrategyRetry:
		return erm.handleRetry(ctx, action, attempt)
	case StrategyCircuitBreaker:
		return erm.handleStrategy(ctx, action, attempt, "Circuit breaker")
	case StrategyFallback:
		return erm.handleStrategy(ctx, action, attempt, "Fallback")
	case StrategyGracefulDegradation:
		return erm.handleStrategy(ctx, action, attempt, "Graceful degradation")
	case StrategyRestart:
		return erm.handleStrategy(ctx, action, attempt, "Restart")
	default:
		return fmt.Errorf("unknown recovery strategy: %d", action.Strategy)
	}
}

// handleRetry implements retry with exponential backoff
func (erm *ErrorRecoveryManager) handleRetry(ctx context.Context, action *RecoveryAction,
	attempt *RecoveryAttempt) error {
	maxAttempts := action.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}

	backoff := action.Backoff
	if backoff == 0 {
		backoff = defaultBackoff
	}

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		attempt.AttemptNum = attemptNum
		start := time.Now()

		handlerErr := action.Handler(ctx, attempt.Error)
		if handlerErr == nil {
			attempt.Success = true
			attempt.Duration = time.Since(start)
			erm.recordAttempt(attempt)
			erm.logger.Info("Retry recovery successful", "attempt", attemptNum)
			return nil
		}

		attempt.Error = handlerErr
		erm.logger.Warn("Retry attempt failed", "attempt", attemptNum, "error", handlerErr)

		if attemptNum < maxAttempts {
			// Wait before next attempt
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff *= 2 // Exponential backoff
		}
	}

	attempt.Success = false
	attempt.Duration = time.Since(attempt.Timestamp)
	erm.recordAttempt(attempt)
	erm.logger.Error("All retry attempts failed", "max_attempts", maxAttempts)
	return attempt.Error
}

// handleStrategy is a common handler for non-retry strategies
func (erm *ErrorRecoveryManager) handleStrategy(ctx context.Context, action *RecoveryAction,
	attempt *RecoveryAttempt, strategyName string) error {
	erm.logger.Info(strategyName + " recovery attempt")

	if err := action.Handler(ctx, attempt.Error); err == nil {
		attempt.Success = true
		attempt.Duration = time.Since(attempt.Timestamp)
		erm.recordAttempt(attempt)
		erm.logger.Info(strategyName + " recovery successful")
		return nil
	}

	attempt.Success = false
	attempt.Duration = time.Since(attempt.Timestamp)
	erm.recordAttempt(attempt)
	erm.logger.Error(strategyName + " recovery failed")
	return attempt.Error
}

// recordAttempt records a recovery attempt
func (erm *ErrorRecoveryManager) recordAttempt(attempt *RecoveryAttempt) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	// Add to history
	erm.history[attempt.Action] = append(erm.history[attempt.Action], *attempt)

	// Update stats
	stats := erm.stats[attempt.Action]
	stats.TotalAttempts++
	stats.LastAttemptTime = attempt.Timestamp
	stats.TotalRecoveryTime += attempt.Duration

	if attempt.Success {
		stats.SuccessfulRecoveries++
	} else {
		stats.FailedRecoveries++
	}

	// Calculate average recovery time
	if stats.TotalAttempts > 0 {
		stats.AverageRecoveryTime = stats.TotalRecoveryTime / time.Duration(stats.TotalAttempts)
	}
}

// GetRecoveryHistory returns recovery history for a specific error type
func (erm *ErrorRecoveryManager) GetRecoveryHistory(errorType string) []RecoveryAttempt {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	if history, exists := erm.history[errorType]; exists {
		// Return a copy to avoid race conditions
		result := make([]RecoveryAttempt, len(history))
		copy(result, history)
		return result
	}
	return nil
}

// GetRecoveryStats returns recovery statistics for a specific error type
func (erm *ErrorRecoveryManager) GetRecoveryStats(errorType string) *RecoveryStats {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	if stats, exists := erm.stats[errorType]; exists {
		// Return a copy to avoid race conditions
		result := *stats
		return &result
	}
	return nil
}

// GetAllStats returns all recovery statistics
func (erm *ErrorRecoveryManager) GetAllStats() map[string]*RecoveryStats {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	result := make(map[string]*RecoveryStats)
	for errorType, stats := range erm.stats {
		result[errorType] = &RecoveryStats{}
		*result[errorType] = *stats
	}
	return result
}

// ClearHistory clears recovery history for a specific error type
func (erm *ErrorRecoveryManager) ClearHistory(errorType string) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	if _, exists := erm.history[errorType]; exists {
		erm.history[errorType] = make([]RecoveryAttempt, 0)
		erm.stats[errorType] = &RecoveryStats{}
	}
}

// ClearAllHistory clears all recovery history
func (erm *ErrorRecoveryManager) ClearAllHistory() {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	for errorType := range erm.history {
		erm.history[errorType] = make([]RecoveryAttempt, 0)
		erm.stats[errorType] = &RecoveryStats{}
	}
}

// Shutdown gracefully shuts down the error recovery manager
func (erm *ErrorRecoveryManager) Shutdown() {
	erm.cancel()
}

// RegisterDefaultActions registers predefined recovery actions
func (erm *ErrorRecoveryManager) RegisterDefaultActions() {
	// Retry strategy for network errors
	erm.RegisterAction("network_error", &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Retry network operations with exponential backoff",
		Handler: func(_ context.Context, _ error) error {
			// Simulate network recovery
			time.Sleep(defaultBackoff)
			return nil
		},
		MaxAttempts: defaultMaxAttempts,
		Backoff:     defaultBackoff,
	})

	// Circuit breaker for service errors
	erm.RegisterAction("service_error", &RecoveryAction{
		Strategy:    StrategyCircuitBreaker,
		Description: "Circuit breaker for service failures",
		Handler: func(_ context.Context, _ error) error {
			// Simulate circuit breaker recovery
			time.Sleep(circuitBreakerDelay)
			return nil
		},
		MaxAttempts: defaultMaxAttempts - 1,
		Backoff:     circuitBreakerDelay,
	})

	// Fallback for critical errors
	erm.RegisterAction("critical_error", &RecoveryAction{
		Strategy:    StrategyFallback,
		Description: "Fallback to alternative service",
		Handler: func(_ context.Context, _ error) error {
			// Simulate fallback recovery
			time.Sleep(fallbackDelay)
			return nil
		},
		MaxAttempts: defaultMaxAttempts - 1,
		Backoff:     fallbackDelay,
	})

	// Graceful degradation for performance issues
	erm.RegisterAction("performance_error", &RecoveryAction{
		Strategy:    StrategyGracefulDegradation,
		Description: "Graceful degradation for performance issues",
		Handler: func(_ context.Context, _ error) error {
			// Simulate graceful degradation
			time.Sleep(gracefulDelay)
			return nil
		},
		MaxAttempts: 1,
		Backoff:     performanceBackoff,
	})

	// Restart for system errors
	erm.RegisterAction("system_error", &RecoveryAction{
		Strategy:    StrategyRestart,
		Description: "Restart system components",
		Handler: func(_ context.Context, _ error) error {
			// Simulate restart recovery
			time.Sleep(restartDelay)
			return nil
		},
		MaxAttempts: 1,
		Backoff:     restartBackoff,
	})
}

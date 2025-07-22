package monitoring

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorRecoveryManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.actions)
	assert.NotNil(t, manager.history)
	assert.NotNil(t, manager.stats)
	assert.NotNil(t, manager.logger)
}

func TestErrorRecoveryManager_RegisterAction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test recovery action",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 3,
		Backoff:     time.Second,
	}

	manager.RegisterAction("test_error", action)

	// Verify action was registered
	manager.mu.RLock()
	registeredAction, exists := manager.actions["test_error"]
	manager.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, action, registeredAction)
	assert.NotNil(t, manager.history["test_error"])
	assert.NotNil(t, manager.stats["test_error"])
}

func TestErrorRecoveryManager_Recover_RetryStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	attemptCount := 0
	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test retry recovery",
		Handler: func(ctx context.Context, err error) error {
			attemptCount++
			if attemptCount < 3 {
				return errors.New("temporary error")
			}
			return nil
		},
		MaxAttempts: 3,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("retry_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "retry_error", errors.New("initial error"))

	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)

	// Verify stats
	stats := manager.GetRecoveryStats("retry_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_RetryStrategy_MaxAttemptsExceeded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test retry recovery with max attempts",
		Handler: func(ctx context.Context, err error) error {
			return errors.New("persistent error")
		},
		MaxAttempts: 2,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("persistent_error", action)

	ctx := context.Background()
	originalErr := errors.New("initial error")
	err := manager.Recover(ctx, "persistent_error", originalErr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persistent error")

	// Verify stats
	stats := manager.GetRecoveryStats("persistent_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(0), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(1), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_CircuitBreakerStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyCircuitBreaker,
		Description: "Test circuit breaker recovery",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("circuit_breaker_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "circuit_breaker_error", errors.New("initial error"))

	assert.NoError(t, err)

	// Verify stats
	stats := manager.GetRecoveryStats("circuit_breaker_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_FallbackStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyFallback,
		Description: "Test fallback recovery",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("fallback_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "fallback_error", errors.New("initial error"))

	assert.NoError(t, err)

	// Verify stats
	stats := manager.GetRecoveryStats("fallback_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_GracefulDegradationStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyGracefulDegradation,
		Description: "Test graceful degradation recovery",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("graceful_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "graceful_error", errors.New("initial error"))

	assert.NoError(t, err)

	// Verify stats
	stats := manager.GetRecoveryStats("graceful_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_RestartStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRestart,
		Description: "Test restart recovery",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("restart_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "restart_error", errors.New("initial error"))

	assert.NoError(t, err)

	// Verify stats
	stats := manager.GetRecoveryStats("restart_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
}

func TestErrorRecoveryManager_Recover_UnknownErrorType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	ctx := context.Background()
	originalErr := errors.New("unknown error")
	err := manager.Recover(ctx, "unknown_error_type", originalErr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recovery action registered for error type")
}

func TestErrorRecoveryManager_Recover_ContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test retry recovery with context cancellation",
		Handler: func(ctx context.Context, err error) error {
			return errors.New("persistent error")
		},
		MaxAttempts: 5,
		Backoff:     time.Millisecond * 50,
	}

	manager.RegisterAction("context_error", action)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(time.Millisecond * 25)
		cancel()
	}()

	err := manager.Recover(ctx, "context_error", errors.New("initial error"))

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestErrorRecoveryManager_GetRecoveryHistory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test recovery history",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("history_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "history_error", errors.New("initial error"))
	require.NoError(t, err)

	history := manager.GetRecoveryHistory("history_error")
	assert.Len(t, history, 1)
	assert.Equal(t, "history_error", history[0].Action)
	assert.Equal(t, StrategyRetry, history[0].Strategy)
	assert.True(t, history[0].Success)
	assert.Greater(t, history[0].Duration, time.Duration(0))
}

func TestErrorRecoveryManager_GetRecoveryStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test recovery stats",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("stats_error", action)

	ctx := context.Background()
	err := manager.Recover(ctx, "stats_error", errors.New("initial error"))
	require.NoError(t, err)

	stats := manager.GetRecoveryStats("stats_error")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalAttempts)
	assert.Equal(t, int64(1), stats.SuccessfulRecoveries)
	assert.Equal(t, int64(0), stats.FailedRecoveries)
	assert.GreaterOrEqual(t, stats.AverageRecoveryTime, time.Duration(0))
}

func TestErrorRecoveryManager_GetAllStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	// Register multiple actions
	action1 := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test action 1",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	action2 := &RecoveryAction{
		Strategy:    StrategyFallback,
		Description: "Test action 2",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("error1", action1)
	manager.RegisterAction("error2", action2)

	ctx := context.Background()
	_ = manager.Recover(ctx, "error1", errors.New("error1"))
	_ = manager.Recover(ctx, "error2", errors.New("error2"))

	allStats := manager.GetAllStats()
	assert.Len(t, allStats, 2)
	assert.NotNil(t, allStats["error1"])
	assert.NotNil(t, allStats["error2"])
}

func TestErrorRecoveryManager_ClearHistory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	action := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test clear history",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("clear_error", action)

	ctx := context.Background()
	_ = manager.Recover(ctx, "clear_error", errors.New("initial error"))

	// Verify history exists
	history := manager.GetRecoveryHistory("clear_error")
	assert.Len(t, history, 1)

	// Clear history
	manager.ClearHistory("clear_error")

	// Verify history is cleared
	history = manager.GetRecoveryHistory("clear_error")
	assert.Len(t, history, 0)

	stats := manager.GetRecoveryStats("clear_error")
	assert.Equal(t, int64(0), stats.TotalAttempts)
}

func TestErrorRecoveryManager_ClearAllHistory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	// Register multiple actions
	action1 := &RecoveryAction{
		Strategy:    StrategyRetry,
		Description: "Test action 1",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	action2 := &RecoveryAction{
		Strategy:    StrategyFallback,
		Description: "Test action 2",
		Handler: func(ctx context.Context, err error) error {
			return nil
		},
		MaxAttempts: 1,
		Backoff:     time.Millisecond * 10,
	}

	manager.RegisterAction("error1", action1)
	manager.RegisterAction("error2", action2)

	ctx := context.Background()
	_ = manager.Recover(ctx, "error1", errors.New("error1"))
	_ = manager.Recover(ctx, "error2", errors.New("error2"))

	// Verify history exists
	history1 := manager.GetRecoveryHistory("error1")
	history2 := manager.GetRecoveryHistory("error2")
	assert.Len(t, history1, 1)
	assert.Len(t, history2, 1)

	// Clear all history
	manager.ClearAllHistory()

	// Verify all history is cleared
	history1 = manager.GetRecoveryHistory("error1")
	history2 = manager.GetRecoveryHistory("error2")
	assert.Len(t, history1, 0)
	assert.Len(t, history2, 0)
}

func TestErrorRecoveryManager_RegisterDefaultActions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	manager.RegisterDefaultActions()

	// Verify default actions are registered
	manager.mu.RLock()
	assert.NotNil(t, manager.actions["network_error"])
	assert.NotNil(t, manager.actions["service_error"])
	assert.NotNil(t, manager.actions["critical_error"])
	assert.NotNil(t, manager.actions["performance_error"])
	assert.NotNil(t, manager.actions["system_error"])
	manager.mu.RUnlock()
}

func TestErrorRecoveryManager_Shutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	manager := NewErrorRecoveryManager(logger)

	// Shutdown should not panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})
}

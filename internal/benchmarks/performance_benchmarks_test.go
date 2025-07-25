package benchmarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"io"
	"log/slog"

	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/config"
	"github.com/atlet99/gitlab-jira-hook/internal/server"
)

// BenchmarkCachePerformance benchmarks cache operations
func BenchmarkCachePerformance(b *testing.B) {
	// Use smaller cache size for benchmarks
	cacheSize := 1000
	if testing.Short() {
		cacheSize = 100
	}

	cache := cache.NewMemoryCache(cacheSize)
	defer cache.Close()

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		// Limit iterations to prevent memory explosion
		maxIterations := 1000
		if testing.Short() {
			maxIterations = 100
		}

		for i := 0; i < b.N && i < maxIterations; i++ {
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, "value", 1*time.Hour)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate cache with smaller dataset
		prePopulateCount := 100
		if testing.Short() {
			prePopulateCount = 10
		}

		for i := 0; i < prePopulateCount; i++ {
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, "value", 1*time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%prePopulateCount)
			cache.Get(key)
		}
	})
}

// BenchmarkRateLimiter benchmarks rate limiter performance
func BenchmarkRateLimiter(b *testing.B) {
	rateLimiter := server.NewHTTPRateLimiter(&server.RateLimiterConfig{
		DefaultRate:  1000.0, // High rate for benchmarking
		DefaultBurst: 2000,
		PerIP:        true,
		PerEndpoint:  true,
	})

	req, _ := http.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	b.Run("Allow", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rateLimiter.Allow(req)
		}
	})

	b.Run("AllowWithContext", func(b *testing.B) {
		ctx := req.Context()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = rateLimiter.AllowWithContext(ctx, req)
		}
	})
}

// BenchmarkWebhookProcessing benchmarks webhook processing performance
func BenchmarkWebhookProcessing(b *testing.B) {
	cfg := &config.Config{
		JobQueueSize:        1000,
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 30,
		ScaleInterval:       10,
		MinWorkers:          2,
		MaxWorkers:          10,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(cfg, logger)

	// Create test webhook payload
	webhookPayload := map[string]interface{}{
		"object_kind": "push",
		"project": map[string]interface{}{
			"name":    "test-project",
			"web_url": "https://gitlab.com/test/project",
		},
		"commits": []map[string]interface{}{
			{
				"id":      "abc123",
				"message": "Test commit for ABC-123",
				"author": map[string]interface{}{
					"name":  "Test User",
					"email": "test@example.com",
				},
			},
		},
		"ref": "refs/heads/main",
	}

	payloadBytes, _ := json.Marshal(webhookPayload)

	b.Run("WebhookHandler", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/gitlab-hook", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Gitlab-Token", "test-token")

			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, req)
		}
	})
}

// BenchmarkMemoryUsagePatterns benchmarks memory usage patterns
func BenchmarkMemoryUsagePatterns(b *testing.B) {
	// Skip this benchmark if it's too heavy
	if testing.Short() {
		b.Skip("Skipping memory usage patterns benchmark in short mode")
	}

	b.Run("CacheMemoryUsage", func(b *testing.B) {
		// Use smaller cache size for benchmarks
		cacheSize := 1000
		if testing.Short() {
			cacheSize = 100
		}

		cache := cache.NewMemoryCache(cacheSize)
		defer cache.Close()

		b.ResetTimer()
		// Limit iterations to prevent memory explosion
		maxIterations := 1000
		if testing.Short() {
			maxIterations = 100
		}

		for i := 0; i < b.N && i < maxIterations; i++ {
			key := fmt.Sprintf("key%d", i)
			value := make([]byte, 512) // Reduced from 1KB to 512B
			cache.Set(key, value, 1*time.Hour)
		}
	})

	b.Run("RateLimiterMemoryUsage", func(b *testing.B) {
		rateLimiter := server.NewHTTPRateLimiter(&server.RateLimiterConfig{
			DefaultRate:  100.0,
			DefaultBurst: 200,
			PerIP:        true,
			PerEndpoint:  true,
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("POST", "/test", nil)
			req.RemoteAddr = "127.0.0.1:" + string(rune(i))
			rateLimiter.Allow(req)
		}
	})
}

// BenchmarkConcurrentRequests benchmarks concurrent request handling
func BenchmarkConcurrentRequests(b *testing.B) {
	// Skip this benchmark if it's too heavy
	if testing.Short() {
		b.Skip("Skipping concurrent requests benchmark in short mode")
	}

	cfg := &config.Config{
		JobQueueSize:        500, // Reduced from 1000
		MaxRetries:          3,
		RetryDelayMs:        100,
		BackoffMultiplier:   2.0,
		MaxBackoffMs:        1000,
		MetricsEnabled:      false,
		HealthCheckInterval: 30,
		ScaleInterval:       10,
		MinWorkers:          2,
		MaxWorkers:          5, // Reduced from 10
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := server.New(cfg, logger)

	webhookPayload := map[string]interface{}{
		"object_kind": "push",
		"project": map[string]interface{}{
			"name":    "test-project",
			"web_url": "https://gitlab.com/test/project",
		},
		"commits": []map[string]interface{}{
			{
				"id":      "abc123",
				"message": "Test commit for ABC-123",
				"author": map[string]interface{}{
					"name":  "Test User",
					"email": "test@example.com",
				},
			},
		},
		"ref": "refs/heads/main",
	}

	payloadBytes, _ := json.Marshal(webhookPayload)

	b.Run("ConcurrentWebhooks", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				req := httptest.NewRequest("POST", "/gitlab-hook", bytes.NewReader(payloadBytes))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Gitlab-Token", "test-token")

				w := httptest.NewRecorder()
				srv.Handler.ServeHTTP(w, req)
			}
		})
	})
}

package benchmarks

import (
	"net/http"
	"testing"
	"time"

	"github.com/atlet99/gitlab-jira-hook/internal/cache"
	"github.com/atlet99/gitlab-jira-hook/internal/server"
)

// BenchmarkCacheOnly benchmarks cache operations only
func BenchmarkCacheOnly(b *testing.B) {
	cache := cache.NewMemoryCache(10000)
	defer cache.Close()

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := "key" + string(rune(i))
			cache.Set(key, "value", 1*time.Hour)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			key := "key" + string(rune(i))
			cache.Set(key, "value", 1*time.Hour)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := "key" + string(rune(i%1000))
			cache.Get(key)
		}
	})
}

// BenchmarkRateLimiterOnly benchmarks rate limiter performance only
func BenchmarkRateLimiterOnly(b *testing.B) {
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

// BenchmarkMemoryUsageOnly benchmarks memory usage patterns only
func BenchmarkMemoryUsageOnly(b *testing.B) {
	b.Run("CacheMemoryUsage", func(b *testing.B) {
		cache := cache.NewMemoryCache(10000)
		defer cache.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := "key" + string(rune(i))
			value := make([]byte, 1024) // 1KB value
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

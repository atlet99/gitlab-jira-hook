package server

import (
	"context"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

const (
	// Default rate limiting values
	defaultRate  = 10.0 // 10 requests per second
	defaultBurst = 20   // burst of 20 requests

	// Adaptive rate limiting factors
	rateReductionFactor = 0.8
	rateIncreaseFactor  = 1.1
	burstMultiplier     = 2.0
	baseRateMultiplier  = 0.5
)

// HTTPRateLimiter provides rate limiting for HTTP requests
type HTTPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   *RateLimiterConfig
}

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	DefaultRate  float64 // requests per second
	DefaultBurst int     // burst limit
	PerIP        bool    // whether to limit per IP address
	PerEndpoint  bool    // whether to limit per endpoint
}

// NewHTTPRateLimiter creates a new HTTP rate limiter
func NewHTTPRateLimiter(config *RateLimiterConfig) *HTTPRateLimiter {
	if config == nil {
		config = &RateLimiterConfig{
			DefaultRate:  defaultRate,
			DefaultBurst: defaultBurst,
			PerIP:        true,
			PerEndpoint:  true,
		}
	}

	return &HTTPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// getLimiterKey generates a key for the rate limiter based on configuration
func (rl *HTTPRateLimiter) getLimiterKey(r *http.Request) string {
	var key string

	if rl.config.PerEndpoint {
		key = r.URL.Path
	} else {
		key = "global"
	}

	if rl.config.PerIP {
		key += ":" + getClientIP(r)
	}

	return key
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header (for proxy scenarios)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	// Check for X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// getOrCreateLimiter gets or creates a rate limiter for the given key
func (rl *HTTPRateLimiter) getOrCreateLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, exists := rl.limiters[key]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rl.config.DefaultRate), rl.config.DefaultBurst)
	rl.limiters[key] = limiter
	return limiter
}

// Allow checks if the request is allowed based on rate limiting
func (rl *HTTPRateLimiter) Allow(r *http.Request) bool {
	key := rl.getLimiterKey(r)
	limiter := rl.getOrCreateLimiter(key)
	return limiter.Allow()
}

// AllowWithContext checks if the request is allowed with context support
func (rl *HTTPRateLimiter) AllowWithContext(ctx context.Context, r *http.Request) error {
	key := rl.getLimiterKey(r)
	limiter := rl.getOrCreateLimiter(key)
	return limiter.Wait(ctx)
}

// RateLimitMiddleware creates a middleware that applies rate limiting
func RateLimitMiddleware(limiter *HTTPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow(r) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddlewareWithContext creates a middleware that applies rate limiting with context support
func RateLimitMiddlewareWithContext(limiter *HTTPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := limiter.AllowWithContext(r.Context(), r); err != nil {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AdaptiveRateLimiter implements adaptive rate limiting based on system load
type AdaptiveRateLimiter struct {
	*HTTPRateLimiter
	baseRate    float64
	maxRate     float64
	currentRate float64
	mu          sync.RWMutex
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(baseRate, maxRate float64) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		HTTPRateLimiter: NewHTTPRateLimiter(&RateLimiterConfig{
			DefaultRate:  baseRate,
			DefaultBurst: int(baseRate * burstMultiplier),
			PerIP:        true,
			PerEndpoint:  true,
		}),
		baseRate:    baseRate,
		maxRate:     maxRate,
		currentRate: baseRate,
	}
}

// AdjustRate adjusts the rate based on system metrics
func (arl *AdaptiveRateLimiter) AdjustRate(cpuUsage, memoryUsage float64) {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	// Simple adaptive algorithm: reduce rate if system is under load
	if cpuUsage > 80.0 || memoryUsage > 80.0 {
		// Reduce rate if system is under high load
		arl.currentRate *= rateReductionFactor
	} else if cpuUsage < 50.0 && memoryUsage < 50.0 {
		// Increase rate if system has capacity
		arl.currentRate *= rateIncreaseFactor
	}

	// Ensure rate stays within bounds
	if arl.currentRate < arl.baseRate*baseRateMultiplier {
		arl.currentRate = arl.baseRate * baseRateMultiplier
	}
	if arl.currentRate > arl.maxRate {
		arl.currentRate = arl.maxRate
	}

	// Update all existing limiters
	for key := range arl.limiters {
		newLimiter := rate.NewLimiter(rate.Limit(arl.currentRate), int(arl.currentRate*burstMultiplier))
		arl.limiters[key] = newLimiter
	}
}

// GetCurrentRate returns the current rate limit
func (arl *AdaptiveRateLimiter) GetCurrentRate() float64 {
	arl.mu.RLock()
	defer arl.mu.RUnlock()
	return arl.currentRate
}

package cache

import (
	"context"
	"sync"
	"time"
)

const (
	// Default TTL for cache promotion
	defaultPromotionTTL = 5 * time.Minute
)

// Cache defines the interface for cache operations
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	GetStats() Stats
}

// Stats holds cache statistics.
// Note: This type name stutters with package name but is kept for API compatibility.
type Stats struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	Size        int
	MaxSize     int
	HitRate     float64
	MemoryUsage int64
}

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	data          map[string]*cacheItem
	mu            sync.RWMutex
	maxSize       int
	stats         *Stats
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
}

// cacheItem represents a cached item with metadata
type cacheItem struct {
	value       interface{}
	expiresAt   time.Time
	createdAt   time.Time
	accessCount int64
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int) *MemoryCache {
	ctx, cancel := context.WithCancel(context.Background())

	cache := &MemoryCache{
		data:    make(map[string]*cacheItem),
		maxSize: maxSize,
		stats:   &Stats{MaxSize: maxSize},
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start cleanup goroutine
	cache.startCleanup()

	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, exists := c.data[key]
	c.mu.RUnlock()

	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.expiresAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		c.stats.Misses++
		return nil, false
	}

	// Update access count
	item.accessCount++
	c.stats.Hits++
	return item.value, true
}

// Set stores a value in the cache with TTL
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict items
	if len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	expiresAt := time.Now().Add(ttl)
	c.data[key] = &cacheItem{
		value:       value,
		expiresAt:   expiresAt,
		createdAt:   time.Now(),
		accessCount: 0,
	}

	c.stats.Size = len(c.data)
}

// Delete removes a key from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.data[key]; exists {
		delete(c.data, key)
		c.stats.Size = len(c.data)
	}
}

// Clear removes all items from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*cacheItem)
	c.stats.Size = 0
}

// GetStats returns cache statistics
func (c *MemoryCache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}

	return *c.stats
}

// evictLRU removes the least recently used item
func (c *MemoryCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	var lowestAccess int64 = -1

	for key, item := range c.data {
		if lowestAccess == -1 || item.accessCount < lowestAccess {
			oldestKey = key
			lowestAccess = item.accessCount
			oldestTime = item.createdAt
		} else if item.accessCount == lowestAccess && item.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.createdAt
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
		c.stats.Evictions++
	}
}

// startCleanup starts the cleanup goroutine
func (c *MemoryCache) startCleanup() {
	c.cleanupTicker = time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanup()
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

// cleanup removes expired items
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.data {
		if now.After(item.expiresAt) {
			delete(c.data, key)
			c.stats.Evictions++
		}
	}

	c.stats.Size = len(c.data)
}

// Close stops the cache and cleans up resources
func (c *MemoryCache) Close() {
	c.cancel()
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
	}
}

// MultiLevelCache implements a multi-level cache system
type MultiLevelCache struct {
	L1 *MemoryCache // Fast, small cache
	L2 *MemoryCache // Slower, larger cache
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(l1Size, l2Size int) *MultiLevelCache {
	return &MultiLevelCache{
		L1: NewMemoryCache(l1Size),
		L2: NewMemoryCache(l2Size),
	}
}

// Get retrieves a value from the multi-level cache
func (mlc *MultiLevelCache) Get(key string) (interface{}, bool) {
	// Try L1 first
	if value, found := mlc.L1.Get(key); found {
		return value, true
	}

	// Try L2
	if value, found := mlc.L2.Get(key); found {
		// Promote to L1
		mlc.L1.Set(key, value, defaultPromotionTTL)
		return value, true
	}

	return nil, false
}

// Set stores a value in both cache levels
func (mlc *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) {
	mlc.L1.Set(key, value, ttl)
	mlc.L2.Set(key, value, ttl)
}

// Delete removes a key from both cache levels
func (mlc *MultiLevelCache) Delete(key string) {
	mlc.L1.Delete(key)
	mlc.L2.Delete(key)
}

// Clear clears both cache levels
func (mlc *MultiLevelCache) Clear() {
	mlc.L1.Clear()
	mlc.L2.Clear()
}

// GetStats returns combined statistics from both cache levels
func (mlc *MultiLevelCache) GetStats() Stats {
	l1Stats := mlc.L1.GetStats()
	l2Stats := mlc.L2.GetStats()

	return Stats{
		Hits:      l1Stats.Hits + l2Stats.Hits,
		Misses:    l1Stats.Misses + l2Stats.Misses,
		Evictions: l1Stats.Evictions + l2Stats.Evictions,
		Size:      l1Stats.Size + l2Stats.Size,
		MaxSize:   l1Stats.MaxSize + l2Stats.MaxSize,
		HitRate:   (float64(l1Stats.Hits+l2Stats.Hits) / float64(l1Stats.Hits+l2Stats.Hits+l1Stats.Misses+l2Stats.Misses)),
	}
}

// Close closes both cache levels
func (mlc *MultiLevelCache) Close() {
	mlc.L1.Close()
	mlc.L2.Close()
}

// Package cache provides advanced caching functionality with multiple strategies,
// distributed caching support, and comprehensive monitoring capabilities.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"log/slog"
)

const (
	// Default cache configuration values
	defaultMaxSize         = 1000
	defaultMaxMemoryMB     = 100
	defaultCleanupInterval = 5 * time.Minute

	// Memory calculation constants
	memoryMBDivisor = 1024 * 1024

	// Adaptive eviction threshold
	adaptiveHitRateThreshold = 0.8

	// Default cleanup interval fallback
	defaultCleanupIntervalFallback = 30 * time.Second

	// Size estimation constants
	defaultIntSize     = 8
	defaultComplexSize = 64

	// Integer overflow protection
	maxUint32Value = 1 << 31
)

// CacheStrategy defines the caching strategy.
// Note: This type name stutters with package name but is kept for API compatibility.
//
//nolint:revive // API compatibility
type CacheStrategy int

const (
	// StrategyLRU implements Least Recently Used eviction
	StrategyLRU CacheStrategy = iota
	// StrategyLFU implements Least Frequently Used eviction
	StrategyLFU
	// StrategyFIFO implements First In First Out eviction
	StrategyFIFO
	// StrategyTTL implements Time To Live eviction
	StrategyTTL
	// StrategyAdaptive implements adaptive eviction based on access patterns
	StrategyAdaptive
)

// Policy defines cache eviction and behavior policies.
// Note: This type name stutters with package name but is kept for API compatibility.
type Policy struct {
	Strategy           CacheStrategy `json:"strategy"`
	MaxSize            int           `json:"max_size"`
	MaxMemoryMB        int           `json:"max_memory_mb"`
	TTL                time.Duration `json:"ttl"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	CompressionEnabled bool          `json:"compression_enabled"`
	EncryptionEnabled  bool          `json:"encryption_enabled"`
}

// AdvancedCache provides advanced caching capabilities
type AdvancedCache struct {
	policy        *Policy
	logger        *slog.Logger
	data          map[string]*AdvancedCacheItem
	mu            sync.RWMutex
	stats         *AdvancedCacheStats
	ctx           context.Context
	cancel        context.CancelFunc
	cleanupTicker *time.Ticker
}

// AdvancedCacheItem represents an item in the advanced cache
type AdvancedCacheItem struct {
	Key         string
	Value       interface{}
	CreatedAt   time.Time
	AccessedAt  time.Time
	ExpiresAt   *time.Time
	AccessCount int64
	Size        int64
	Compressed  bool
	Encrypted   bool
	Hash        string
}

// AdvancedCacheStats holds advanced cache statistics
type AdvancedCacheStats struct {
	Hits              int64
	Misses            int64
	Evictions         int64
	Compressions      int64
	Decompressions    int64
	Encryptions       int64
	Decryptions       int64
	Size              int
	MaxSize           int
	MemoryUsage       int64
	HitRate           float64
	AverageAccessTime time.Duration
}

// NewAdvancedCache creates a new advanced cache
func NewAdvancedCache(policy *Policy, logger *slog.Logger) *AdvancedCache {
	if policy == nil {
		policy = &Policy{
			Strategy:           StrategyLRU,
			MaxSize:            defaultMaxSize,
			MaxMemoryMB:        defaultMaxMemoryMB,
			TTL:                1 * time.Hour,
			CleanupInterval:    defaultCleanupInterval,
			CompressionEnabled: false,
			EncryptionEnabled:  false,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	cache := &AdvancedCache{
		policy: policy,
		logger: logger,
		data:   make(map[string]*AdvancedCacheItem),
		stats: &AdvancedCacheStats{
			MaxSize: policy.MaxSize,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	cache.startCleanup()
	return cache
}

// Get retrieves a value from the cache
func (ac *AdvancedCache) Get(key string) (interface{}, bool) {
	ac.mu.RLock()
	item, exists := ac.data[key]
	ac.mu.RUnlock()

	if !exists {
		ac.stats.Misses++
		return nil, false
	}

	// Check if item has expired
	if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
		ac.mu.Lock()
		delete(ac.data, key)
		ac.stats.Size--
		ac.mu.Unlock()
		ac.stats.Misses++
		return nil, false
	}

	// Update access statistics
	ac.mu.Lock()
	item.AccessedAt = time.Now()
	item.AccessCount++
	ac.mu.Unlock()

	ac.stats.Hits++
	return item.Value, true
}

// Set stores a value in the cache
func (ac *AdvancedCache) Set(key string, value interface{}, ttl time.Duration) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Calculate item size
	size := ac.calculateSize(value)

	// Check memory limits
	currentMemoryMB := ac.stats.MemoryUsage / memoryMBDivisor
	if currentMemoryMB+size/memoryMBDivisor > int64(ac.policy.MaxMemoryMB) {
		ac.evictItems()
	}

	// Create expiration time if TTL is provided
	var expiresAt *time.Time
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		expiresAt = &exp
	}

	// Calculate hash for integrity checking
	hash := ac.calculateHash(key, value)

	item := &AdvancedCacheItem{
		Key:         key,
		Value:       value,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		ExpiresAt:   expiresAt,
		AccessCount: 1,
		Size:        size,
		Hash:        hash,
	}

	// Check if key already exists
	if _, exists := ac.data[key]; !exists {
		ac.stats.Size++
	}

	ac.data[key] = item
	ac.stats.MemoryUsage += size

	// Evict items if cache is full
	if ac.stats.Size > ac.policy.MaxSize {
		ac.evictItems()
	}
}

// Delete removes an item from the cache
func (ac *AdvancedCache) Delete(key string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if item, exists := ac.data[key]; exists {
		ac.stats.MemoryUsage -= item.Size
		delete(ac.data, key)
		ac.stats.Size--
	}
}

// Clear removes all items from the cache
func (ac *AdvancedCache) Clear() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.data = make(map[string]*AdvancedCacheItem)
	ac.stats.Size = 0
	ac.stats.MemoryUsage = 0
}

// GetStats returns cache statistics
func (ac *AdvancedCache) GetStats() AdvancedCacheStats {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	stats := *ac.stats
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	}

	return stats
}

// evictItems removes items based on the configured strategy
func (ac *AdvancedCache) evictItems() {
	switch ac.policy.Strategy {
	case StrategyLRU:
		ac.evictLRU()
	case StrategyLFU:
		ac.evictLFU()
	case StrategyFIFO:
		ac.evictFIFO()
	case StrategyTTL:
		ac.evictExpired()
	case StrategyAdaptive:
		ac.evictAdaptive()
	}
}

// evictLRU removes the least recently used items
func (ac *AdvancedCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range ac.data {
		if oldestKey == "" || item.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessedAt
		}
	}

	if oldestKey != "" {
		ac.Delete(oldestKey)
		ac.stats.Evictions++
	}
}

// evictLFU removes the least frequently used items
func (ac *AdvancedCache) evictLFU() {
	var leastUsedKey string
	var minAccessCount int64

	for key, item := range ac.data {
		if leastUsedKey == "" || item.AccessCount < minAccessCount {
			leastUsedKey = key
			minAccessCount = item.AccessCount
		}
	}

	if leastUsedKey != "" {
		ac.Delete(leastUsedKey)
		ac.stats.Evictions++
	}
}

// evictFIFO removes items in first-in-first-out order
func (ac *AdvancedCache) evictFIFO() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range ac.data {
		if oldestKey == "" || item.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.CreatedAt
		}
	}

	if oldestKey != "" {
		ac.Delete(oldestKey)
		ac.stats.Evictions++
	}
}

// evictExpired removes expired items
func (ac *AdvancedCache) evictExpired() {
	now := time.Now()
	for key, item := range ac.data {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			ac.Delete(key)
			ac.stats.Evictions++
		}
	}
}

// evictAdaptive removes items based on adaptive criteria
func (ac *AdvancedCache) evictAdaptive() {
	hitRate := ac.stats.HitRate
	if hitRate > adaptiveHitRateThreshold {
		// High hit rate, use LRU
		ac.evictLRU()
	} else {
		// Low hit rate, use LFU
		ac.evictLFU()
	}
}

// startCleanup starts the cleanup goroutine
func (ac *AdvancedCache) startCleanup() {
	interval := ac.policy.CleanupInterval
	if interval == 0 {
		interval = defaultCleanupIntervalFallback
	}

	ac.cleanupTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ac.cleanupTicker.C:
				ac.cleanup()
			case <-ac.ctx.Done():
				return
			}
		}
	}()
}

// cleanup performs periodic cache cleanup
func (ac *AdvancedCache) cleanup() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Remove expired items
	now := time.Now()
	for key, item := range ac.data {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			ac.stats.MemoryUsage -= item.Size
			delete(ac.data, key)
			ac.stats.Size--
			ac.stats.Evictions++
		}
	}

	// Evict items if cache is still full
	if ac.stats.Size > ac.policy.MaxSize {
		ac.evictItems()
	}
}

// calculateSize estimates the memory size of a value
func (ac *AdvancedCache) calculateSize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64, bool:
		return defaultIntSize
	default:
		// For complex types, estimate based on JSON size
		if data, err := json.Marshal(value); err == nil {
			return int64(len(data))
		}
		return defaultComplexSize // Default size
	}
}

// calculateHash calculates a hash for the key-value pair
func (ac *AdvancedCache) calculateHash(key string, value interface{}) string {
	data := fmt.Sprintf("%s:%v", key, value)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Close stops the cache and cleans up resources
func (ac *AdvancedCache) Close() {
	ac.cancel()
	if ac.cleanupTicker != nil {
		ac.cleanupTicker.Stop()
	}
}

// DistributedCache provides distributed caching capabilities
type DistributedCache struct {
	*AdvancedCache
	nodes    map[string]*Node
	nodeMu   sync.RWMutex
	hashRing *ConsistentHashRing
}

// Node represents a cache node in the distributed system.
// Note: This type name stutters with package name but is kept for API compatibility.
type Node struct {
	ID       string
	Address  string
	Port     int
	Weight   int
	Healthy  bool
	LastSeen time.Time
}

// NewDistributedCache creates a new distributed cache
func NewDistributedCache(policy *Policy, logger *slog.Logger) *DistributedCache {
	return &DistributedCache{
		AdvancedCache: NewAdvancedCache(policy, logger),
		nodes:         make(map[string]*Node),
		hashRing:      NewConsistentHashRing(),
	}
}

// AddNode adds a cache node to the distributed system
func (dc *DistributedCache) AddNode(id, address string, port, weight int) {
	dc.nodeMu.Lock()
	defer dc.nodeMu.Unlock()

	node := &Node{
		ID:       id,
		Address:  address,
		Port:     port,
		Weight:   weight,
		Healthy:  true,
		LastSeen: time.Now(),
	}

	dc.nodes[id] = node
	dc.hashRing.AddNode(id, weight)
}

// RemoveNode removes a cache node from the distributed system
func (dc *DistributedCache) RemoveNode(id string) {
	dc.nodeMu.Lock()
	defer dc.nodeMu.Unlock()

	delete(dc.nodes, id)
	dc.hashRing.RemoveNode(id)
}

// GetNodeForKey returns the node responsible for a given key
func (dc *DistributedCache) GetNodeForKey(key string) *Node {
	dc.nodeMu.RLock()
	defer dc.nodeMu.RUnlock()

	nodeID := dc.hashRing.GetNode(key)
	if nodeID == "" {
		return nil
	}

	return dc.nodes[nodeID]
}

// ConsistentHashRing implements consistent hashing for distributed caching
type ConsistentHashRing struct {
	nodes      map[uint32]string
	sortedKeys []uint32
	mu         sync.RWMutex
}

// NewConsistentHashRing creates a new consistent hash ring
func NewConsistentHashRing() *ConsistentHashRing {
	return &ConsistentHashRing{
		nodes:      make(map[uint32]string),
		sortedKeys: make([]uint32, 0),
	}
}

// AddNode adds a node to the hash ring
func (chr *ConsistentHashRing) AddNode(nodeID string, weight int) {
	chr.mu.Lock()
	defer chr.mu.Unlock()

	// Create virtual nodes for better distribution
	for i := 0; i < weight; i++ {
		virtualNodeID := fmt.Sprintf("%s-%d", nodeID, i)
		hash := chr.hash(virtualNodeID)
		chr.nodes[hash] = nodeID
		chr.sortedKeys = append(chr.sortedKeys, hash)
	}

	// Sort keys for binary search
	chr.sortKeys()
}

// RemoveNode removes a node from the hash ring
func (chr *ConsistentHashRing) RemoveNode(nodeID string) {
	chr.mu.Lock()
	defer chr.mu.Unlock()

	// Remove all virtual nodes
	keysToRemove := make([]uint32, 0)
	for hash, id := range chr.nodes {
		if id == nodeID {
			keysToRemove = append(keysToRemove, hash)
		}
	}

	for _, hash := range keysToRemove {
		delete(chr.nodes, hash)
	}

	// Rebuild sorted keys
	chr.sortedKeys = make([]uint32, 0, len(chr.nodes))
	for hash := range chr.nodes {
		chr.sortedKeys = append(chr.sortedKeys, hash)
	}
	chr.sortKeys()
}

// GetNode returns the node responsible for a given key
func (chr *ConsistentHashRing) GetNode(key string) string {
	chr.mu.RLock()
	defer chr.mu.RUnlock()

	if len(chr.sortedKeys) == 0 {
		return ""
	}

	hash := chr.hash(key)

	// Find the first node with hash >= key hash
	for _, nodeHash := range chr.sortedKeys {
		if nodeHash >= hash {
			return chr.nodes[nodeHash]
		}
	}

	// Wrap around to the first node
	return chr.nodes[chr.sortedKeys[0]]
}

// hash calculates a hash for the given key
func (chr *ConsistentHashRing) hash(key string) uint32 {
	h := fnv.New32a()
	if _, err := h.Write([]byte(key)); err != nil {
		// Fallback to simple hash if write fails
		// Use modulo to prevent integer overflow
		keyLen := len(key)
		if keyLen > maxUint32Value {
			keyLen = maxUint32Value
		}
		return uint32(keyLen & (maxUint32Value - 1))
	}
	return h.Sum32()
}

// sortKeys sorts the hash keys for binary search
func (chr *ConsistentHashRing) sortKeys() {
	// Simple bubble sort for small datasets
	// In production, use a more efficient sorting algorithm
	for i := 0; i < len(chr.sortedKeys)-1; i++ {
		for j := 0; j < len(chr.sortedKeys)-i-1; j++ {
			if chr.sortedKeys[j] > chr.sortedKeys[j+1] {
				chr.sortedKeys[j], chr.sortedKeys[j+1] = chr.sortedKeys[j+1], chr.sortedKeys[j]
			}
		}
	}
}

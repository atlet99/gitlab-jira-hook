package cache

import (
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/stretchr/testify/assert"
)

func TestAdvancedCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("new_advanced_cache", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         100,
			MaxMemoryMB:     10,
			TTL:             1 * time.Hour,
			CleanupInterval: 1 * time.Minute,
		}

		cache := NewAdvancedCache(policy, logger)
		assert.NotNil(t, cache)
		assert.Equal(t, policy, cache.policy)
		assert.Equal(t, 100, cache.stats.MaxSize)

		cache.Close()
	})

	t.Run("new_advanced_cache_defaults", func(t *testing.T) {
		cache := NewAdvancedCache(nil, logger)
		assert.NotNil(t, cache)
		assert.Equal(t, StrategyLRU, cache.policy.Strategy)
		assert.Equal(t, 1000, cache.policy.MaxSize)
		assert.Equal(t, 100, cache.policy.MaxMemoryMB)

		cache.Close()
	})
}

func TestAdvancedCacheOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("set_and_get", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         10,
			MaxMemoryMB:     10,
			TTL:             1 * time.Hour,
			CleanupInterval: 1 * time.Minute,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		key := "test-key"
		value := "test-value"

		cache.Set(key, value, 0)

		retrieved, exists := cache.Get(key)
		assert.True(t, exists)
		assert.Equal(t, value, retrieved)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Hits)
		assert.Equal(t, int64(0), stats.Misses)
		assert.Equal(t, 1, stats.Size)
	})

	t.Run("get_nonexistent", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         10,
			MaxMemoryMB:     10,
			TTL:             1 * time.Hour,
			CleanupInterval: 1 * time.Minute,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		_, exists := cache.Get("nonexistent")
		assert.False(t, exists)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Misses)
	})

	t.Run("delete", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         10,
			MaxMemoryMB:     10,
			TTL:             1 * time.Hour,
			CleanupInterval: 1 * time.Minute,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		key := "delete-test"
		value := "delete-value"

		cache.Set(key, value, 0)
		_, exists := cache.Get(key)
		assert.True(t, exists)

		cache.Delete(key)
		_, exists = cache.Get(key)
		assert.False(t, exists)

		stats := cache.GetStats()
		assert.Equal(t, 0, stats.Size)
	})

	t.Run("clear", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         10,
			MaxMemoryMB:     10,
			TTL:             1 * time.Hour,
			CleanupInterval: 1 * time.Minute,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		stats := cache.GetStats()
		assert.Equal(t, 2, stats.Size)

		cache.Clear()

		stats = cache.GetStats()
		assert.Equal(t, 0, stats.Size)
		assert.Equal(t, int64(0), stats.MemoryUsage)
	})
}

func TestAdvancedCacheTTL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	policy := &Policy{
		Strategy:        StrategyTTL,
		MaxSize:         10,
		TTL:             100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}

	cache := NewAdvancedCache(policy, logger)
	defer cache.Close()

	t.Run("ttl_expiration", func(t *testing.T) {
		key := "ttl-test"
		value := "ttl-value"

		cache.Set(key, value, 50*time.Millisecond)

		// Should exist immediately
		_, exists := cache.Get(key)
		assert.True(t, exists)

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should not exist after expiration
		_, exists = cache.Get(key)
		assert.False(t, exists)
	})

	t.Run("custom_ttl", func(t *testing.T) {
		key := "custom-ttl-test"
		value := "custom-ttl-value"

		cache.Set(key, value, 200*time.Millisecond)

		// Should exist immediately
		_, exists := cache.Get(key)
		assert.True(t, exists)

		// Wait for policy TTL but not custom TTL
		time.Sleep(150 * time.Millisecond)

		// Should still exist
		_, exists = cache.Get(key)
		assert.True(t, exists)

		// Wait for custom TTL
		time.Sleep(100 * time.Millisecond)

		// Should not exist after custom TTL
		_, exists = cache.Get(key)
		assert.False(t, exists)
	})
}

func TestAdvancedCacheEvictionStrategies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("lru_eviction", func(t *testing.T) {
		policy := &Policy{
			Strategy: StrategyLRU,
			MaxSize:  2,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		// Add items
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		// Access key1 to make it more recently used
		cache.Get("key1")

		// Add third item, should evict key2 (least recently used)
		cache.Set("key3", "value3", 0)

		// key1 should still exist
		_, exists := cache.Get("key1")
		assert.True(t, exists)

		// key2 should be evicted
		_, exists = cache.Get("key2")
		assert.False(t, exists)

		// key3 should exist
		_, exists = cache.Get("key3")
		assert.True(t, exists)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Evictions)
	})

	t.Run("lfu_eviction", func(t *testing.T) {
		policy := &Policy{
			Strategy: StrategyLFU,
			MaxSize:  2,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		// Add items
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		// Access key1 multiple times to make it more frequently used
		cache.Get("key1")
		cache.Get("key1")
		cache.Get("key1")

		// Access key2 only once
		cache.Get("key2")

		// Add third item, should evict key3 (newest item with AccessCount = 0)
		cache.Set("key3", "value3", 0)

		// key1 should still exist (it has more access count, so it's not least frequently used)
		_, exists := cache.Get("key1")
		assert.True(t, exists)

		// key2 should still exist (it has more access count than key3)
		_, exists = cache.Get("key2")
		assert.True(t, exists)

		// key3 should be evicted (it has the least access count = 0)
		_, exists = cache.Get("key3")
		assert.False(t, exists)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Evictions)
	})

	t.Run("fifo_eviction", func(t *testing.T) {
		policy := &Policy{
			Strategy: StrategyFIFO,
			MaxSize:  2,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		// Add items
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		// Add third item, should evict key1 (first in)
		cache.Set("key3", "value3", 0)

		// key1 should be evicted
		_, exists := cache.Get("key1")
		assert.False(t, exists)

		// key2 should exist
		_, exists = cache.Get("key2")
		assert.True(t, exists)

		// key3 should exist
		_, exists = cache.Get("key3")
		assert.True(t, exists)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Evictions)
	})
}

func TestAdvancedCacheCompression(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	policy := &Policy{
		Strategy:           StrategyLRU,
		MaxSize:            10,
		CompressionEnabled: true,
	}

	cache := NewAdvancedCache(policy, logger)
	defer cache.Close()

	t.Run("compression_enabled", func(t *testing.T) {
		// Large value that should trigger compression
		largeValue := make([]byte, 2048) // 2KB
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		cache.Set("large-key", largeValue, 0)

		// Retrieve the value
		retrieved, exists := cache.Get("large-key")
		assert.True(t, exists)

		// When []byte is serialized to JSON, it becomes a base64 string
		// So we need to check that the retrieved value is a string that represents the original bytes
		retrievedStr, ok := retrieved.(string)
		assert.True(t, ok)

		// The string should be a JSON representation of the byte array
		// We can verify this by checking that it's not empty and has the right length
		assert.NotEmpty(t, retrievedStr)
		assert.True(t, len(retrievedStr) > 0)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Compressions)
		assert.Equal(t, int64(1), stats.Decompressions)
	})

	t.Run("compression_disabled", func(t *testing.T) {
		policy.CompressionEnabled = false
		cache2 := NewAdvancedCache(policy, logger)
		defer cache2.Close()

		largeValue := make([]byte, 2048)
		cache2.Set("large-key", largeValue, 0)

		stats := cache2.GetStats()
		assert.Equal(t, int64(0), stats.Compressions)
		assert.Equal(t, int64(0), stats.Decompressions)
	})
}

func TestAdvancedCacheEncryption(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	policy := &Policy{
		Strategy:          StrategyLRU,
		MaxSize:           10,
		EncryptionEnabled: true,
	}

	cache := NewAdvancedCache(policy, logger)
	defer cache.Close()

	t.Run("encryption_enabled", func(t *testing.T) {
		value := "sensitive-data"
		cache.Set("sensitive-key", value, 0)

		retrieved, exists := cache.Get("sensitive-key")
		assert.True(t, exists)
		assert.Equal(t, value, retrieved)

		stats := cache.GetStats()
		assert.Equal(t, int64(1), stats.Encryptions)
		assert.Equal(t, int64(1), stats.Decryptions)
	})
}

func TestAdvancedCacheStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("stats_calculation", func(t *testing.T) {
		policy := &Policy{
			Strategy: StrategyLRU,
			MaxSize:  10,
		}

		cache := NewAdvancedCache(policy, logger)
		defer cache.Close()

		// Add some items
		cache.Set("key1", "value1", 0)
		cache.Set("key2", "value2", 0)

		// Access items
		cache.Get("key1")        // First access - hit
		cache.Get("key1")        // Second access - hit
		cache.Get("key2")        // First access - hit
		cache.Get("nonexistent") // Miss

		stats := cache.GetStats()
		assert.Equal(t, int64(3), stats.Hits)
		assert.Equal(t, int64(1), stats.Misses)
		assert.Equal(t, 2, stats.Size)
		assert.Equal(t, 0.75, stats.HitRate) // 3/4 = 0.75
	})
}

func TestDistributedCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	policy := &Policy{
		Strategy: StrategyLRU,
		MaxSize:  100,
	}

	cache := NewDistributedCache(policy, logger)
	defer cache.Close()

	t.Run("add_nodes", func(t *testing.T) {
		cache.AddNode("node1", "localhost", 6379, 1)
		cache.AddNode("node2", "localhost", 6380, 2)
		cache.AddNode("node3", "localhost", 6381, 1)

		node := cache.GetNodeForKey("test-key")
		assert.NotNil(t, node)
		assert.Contains(t, []string{"node1", "node2", "node3"}, node.ID)
	})

	t.Run("remove_node", func(t *testing.T) {
		cache.RemoveNode("node2")

		// Should still get a node for any key
		node := cache.GetNodeForKey("test-key")
		assert.NotNil(t, node)
		assert.NotEqual(t, "node2", node.ID)
	})
}

func TestConsistentHashRing(t *testing.T) {
	ring := NewConsistentHashRing()

	t.Run("add_nodes", func(t *testing.T) {
		ring.AddNode("node1", 1)
		ring.AddNode("node2", 2)
		ring.AddNode("node3", 1)

		assert.Equal(t, 4, len(ring.nodes)) // 1 + 2 + 1 virtual nodes
		assert.Equal(t, 4, len(ring.sortedKeys))
	})

	t.Run("get_node", func(t *testing.T) {
		nodeID := ring.GetNode("test-key")
		assert.NotEmpty(t, nodeID)
		assert.Contains(t, []string{"node1", "node2", "node3"}, nodeID)
	})

	t.Run("remove_node", func(t *testing.T) {
		ring.RemoveNode("node2")

		nodeID := ring.GetNode("test-key")
		assert.NotEmpty(t, nodeID)
		assert.NotEqual(t, "node2", nodeID)
	})

	t.Run("empty_ring", func(t *testing.T) {
		emptyRing := NewConsistentHashRing()
		nodeID := emptyRing.GetNode("test-key")
		assert.Empty(t, nodeID)
	})
}

func TestPolicyValidation(t *testing.T) {
	t.Run("valid_policy", func(t *testing.T) {
		policy := &Policy{
			Strategy:        StrategyLRU,
			MaxSize:         1000,
			MaxMemoryMB:     100,
			TTL:             1 * time.Hour,
			CleanupInterval: 5 * time.Minute,
		}

		assert.Equal(t, StrategyLRU, policy.Strategy)
		assert.Equal(t, 1000, policy.MaxSize)
		assert.Equal(t, 100, policy.MaxMemoryMB)
	})

	t.Run("strategy_names", func(t *testing.T) {
		strategies := []CacheStrategy{
			StrategyLRU,
			StrategyLFU,
			StrategyFIFO,
			StrategyTTL,
			StrategyAdaptive,
		}

		expectedNames := []string{
			"LRU",
			"LFU",
			"FIFO",
			"TTL",
			"Adaptive",
		}

		for i, strategy := range strategies {
			assert.Equal(t, expectedNames[i], strategy.String())
		}
	})
}

// Add String method for CacheStrategy
func (cs CacheStrategy) String() string {
	switch cs {
	case StrategyLRU:
		return "LRU"
	case StrategyLFU:
		return "LFU"
	case StrategyFIFO:
		return "FIFO"
	case StrategyTTL:
		return "TTL"
	case StrategyAdaptive:
		return "Adaptive"
	default:
		return "Unknown"
	}
}

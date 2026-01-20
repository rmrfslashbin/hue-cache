package backends

import (
	"context"
	"strings"
	"sync"
	"time"

	cache "github.com/rmrfslashbin/hue-cache"
)

// Memory implements an in-memory cache backend using sync.Map.
// It supports TTL expiration, memory limits, and LRU eviction.
type Memory struct {
	// data stores cache entries
	data sync.Map

	// stats tracks cache statistics
	stats *cache.StatsCollector

	// config holds configuration
	config *MemoryConfig

	// cleanup manages background cleanup
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}

	// mu protects size tracking and eviction
	mu         sync.RWMutex
	totalSize  int64
	entryCount int64

	// closed tracks if backend is closed
	closed bool
}

// MemoryConfig contains configuration options for the memory backend.
type MemoryConfig struct {
	// MaxMemory is the maximum memory in bytes (0 = unlimited).
	MaxMemory int64

	// MaxEntries is the maximum number of entries (0 = unlimited).
	MaxEntries int64

	// CleanupInterval is how often to run TTL cleanup.
	// Default: 1 minute
	CleanupInterval time.Duration

	// EvictionPolicy determines how to evict entries when limits are reached.
	// Default: LRU
	EvictionPolicy EvictionPolicy
}

// EvictionPolicy determines how entries are evicted when limits are reached.
type EvictionPolicy int

const (
	// EvictionLRU evicts least recently used entries.
	EvictionLRU EvictionPolicy = iota

	// EvictionLFU evicts least frequently used entries.
	EvictionLFU

	// EvictionFIFO evicts oldest entries first.
	EvictionFIFO
)

// DefaultMemoryConfig returns default configuration.
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxMemory:       0, // Unlimited
		MaxEntries:      0, // Unlimited
		CleanupInterval: 1 * time.Minute,
		EvictionPolicy:  EvictionLRU,
	}
}

// NewMemory creates a new in-memory cache backend.
func NewMemory(config ...*MemoryConfig) *Memory {
	cfg := DefaultMemoryConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	m := &Memory{
		stats:       cache.NewStatsCollector(),
		config:      cfg,
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup if interval is set
	if cfg.CleanupInterval > 0 {
		m.cleanupTicker = time.NewTicker(cfg.CleanupInterval)
		go m.cleanupLoop()
	}

	return m
}

// Get retrieves a value from the cache.
func (m *Memory) Get(ctx context.Context, key string) (*cache.Entry, error) {
	if m.closed {
		return nil, cache.NewError("Get", key, cache.ErrBackendClosed)
	}

	if key == "" {
		return nil, cache.NewError("Get", key, cache.ErrInvalidKey)
	}

	value, ok := m.data.Load(key)
	if !ok {
		m.stats.RecordMiss()
		return nil, cache.NewError("Get", key, cache.ErrNotFound)
	}

	entry := value.(*cache.Entry)

	// Check expiration
	if entry.IsExpired() {
		m.stats.RecordMiss()
		m.data.Delete(key)
		m.updateSize(-entry.Size)
		m.stats.RecordEviction()
		return nil, cache.NewError("Get", key, cache.ErrExpired)
	}

	// Update hit counter and timestamp
	entry.Hits++
	entry.UpdatedAt = time.Now()

	m.stats.RecordHit()
	return entry.Clone(), nil
}

// Set stores a value in the cache.
func (m *Memory) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.closed {
		return cache.NewError("Set", key, cache.ErrBackendClosed)
	}

	if key == "" {
		return cache.NewError("Set", key, cache.ErrInvalidKey)
	}

	if value == nil {
		return cache.NewError("Set", key, cache.ErrInvalidValue)
	}

	entry := cache.NewEntry(key, value, ttl)

	// Check if we need to evict
	if err := m.makeRoom(entry.Size); err != nil {
		return cache.NewError("Set", key, err)
	}

	// Check if key already exists
	if oldValue, ok := m.data.Load(key); ok {
		oldEntry := oldValue.(*cache.Entry)
		m.updateSize(-oldEntry.Size)
	}

	m.data.Store(key, entry)
	m.updateSize(entry.Size)

	return nil
}

// Delete removes a key from the cache.
func (m *Memory) Delete(ctx context.Context, key string) error {
	if m.closed {
		return cache.NewError("Delete", key, cache.ErrBackendClosed)
	}

	if value, ok := m.data.LoadAndDelete(key); ok {
		entry := value.(*cache.Entry)
		m.updateSize(-entry.Size)
	}

	return nil
}

// Clear removes all entries from the cache.
func (m *Memory) Clear(ctx context.Context) error {
	if m.closed {
		return cache.NewError("Clear", "", cache.ErrBackendClosed)
	}

	m.data.Range(func(key, value interface{}) bool {
		m.data.Delete(key)
		return true
	})

	m.mu.Lock()
	m.totalSize = 0
	m.entryCount = 0
	m.mu.Unlock()

	m.stats.SetSize(0)
	m.stats.SetEntries(0)

	return nil
}

// Keys returns all keys matching the pattern.
func (m *Memory) Keys(ctx context.Context, pattern string) ([]string, error) {
	if m.closed {
		return nil, cache.NewError("Keys", "", cache.ErrBackendClosed)
	}

	var keys []string

	m.data.Range(func(key, value interface{}) bool {
		k := key.(string)
		if matchPattern(k, pattern) {
			entry := value.(*cache.Entry)
			// Skip expired entries
			if !entry.IsExpired() {
				keys = append(keys, k)
			}
		}
		return true
	})

	return keys, nil
}

// Stats returns cache statistics.
func (m *Memory) Stats(ctx context.Context) (*cache.Stats, error) {
	if m.closed {
		return nil, cache.NewError("Stats", "", cache.ErrBackendClosed)
	}

	stats := m.stats.Stats()

	m.mu.RLock()
	stats.Entries = m.entryCount
	stats.Size = m.totalSize
	m.mu.RUnlock()

	return stats, nil
}

// Close releases resources held by the backend.
func (m *Memory) Close() error {
	if m.closed {
		return nil
	}

	m.closed = true

	// Stop cleanup goroutine
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
		close(m.cleanupDone)
	}

	// Clear all data
	m.Clear(context.Background())

	return nil
}

// cleanupLoop runs periodic TTL cleanup.
func (m *Memory) cleanupLoop() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.cleanupExpired()
		case <-m.cleanupDone:
			return
		}
	}
}

// cleanupExpired removes expired entries.
func (m *Memory) cleanupExpired() {
	var toDelete []string

	m.data.Range(func(key, value interface{}) bool {
		entry := value.(*cache.Entry)
		if entry.IsExpired() {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})

	for _, key := range toDelete {
		if value, ok := m.data.LoadAndDelete(key); ok {
			entry := value.(*cache.Entry)
			m.updateSize(-entry.Size)
			m.stats.RecordEviction()
		}
	}
}

// makeRoom evicts entries if necessary to make room for new entry.
func (m *Memory) makeRoom(newSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check entry count limit
	if m.config.MaxEntries > 0 && m.entryCount >= m.config.MaxEntries {
		if err := m.evictOne(); err != nil {
			return err
		}
	}

	// Check memory limit
	if m.config.MaxMemory > 0 {
		for m.totalSize+newSize > m.config.MaxMemory {
			if err := m.evictOne(); err != nil {
				return err
			}
		}
	}

	return nil
}

// evictOne evicts a single entry based on the eviction policy.
// Must be called with mu held.
func (m *Memory) evictOne() error {
	var evictKey string
	var evictEntry *cache.Entry

	switch m.config.EvictionPolicy {
	case EvictionLRU:
		// Find least recently used
		var oldestTime time.Time
		m.data.Range(func(key, value interface{}) bool {
			entry := value.(*cache.Entry)
			if oldestTime.IsZero() || entry.UpdatedAt.Before(oldestTime) {
				oldestTime = entry.UpdatedAt
				evictKey = key.(string)
				evictEntry = entry
			}
			return true
		})

	case EvictionLFU:
		// Find least frequently used
		var lowestHits int64 = -1
		m.data.Range(func(key, value interface{}) bool {
			entry := value.(*cache.Entry)
			if lowestHits == -1 || entry.Hits < lowestHits {
				lowestHits = entry.Hits
				evictKey = key.(string)
				evictEntry = entry
			}
			return true
		})

	case EvictionFIFO:
		// Find oldest created
		var oldestTime time.Time
		m.data.Range(func(key, value interface{}) bool {
			entry := value.(*cache.Entry)
			if oldestTime.IsZero() || entry.CreatedAt.Before(oldestTime) {
				oldestTime = entry.CreatedAt
				evictKey = key.(string)
				evictEntry = entry
			}
			return true
		})
	}

	if evictKey == "" {
		return cache.ErrMemoryLimit
	}

	m.data.Delete(evictKey)
	m.totalSize -= evictEntry.Size
	m.entryCount--
	m.stats.RecordEviction()

	return nil
}

// updateSize updates the total size and entry count.
func (m *Memory) updateSize(delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalSize += delta
	if delta > 0 {
		m.entryCount++
	} else if delta < 0 {
		m.entryCount--
	}

	m.stats.SetSize(m.totalSize)
	m.stats.SetEntries(m.entryCount)
}

// matchPattern matches a key against a pattern.
// Supports: "*" (all), "prefix:*", "*:suffix", "exact"
func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(key, suffix)
	}

	return key == pattern
}

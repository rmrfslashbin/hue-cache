package backends

import (
	"context"
	"testing"
	"time"

	cache "github.com/rmrfslashbin/hue-cache"
)

func TestMemory_BackendContract(t *testing.T) {
	suite := cache.BackendTestSuite{
		NewBackend: func(t *testing.T) cache.Backend {
			return NewMemory()
		},
	}

	cache.RunBackendTests(t, suite)
}

func TestMemory_WithConfig(t *testing.T) {
	config := &MemoryConfig{
		MaxMemory:       1024 * 1024, // 1MB
		MaxEntries:      100,
		CleanupInterval: 100 * time.Millisecond,
		EvictionPolicy:  EvictionLRU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	if backend.config.MaxMemory != config.MaxMemory {
		t.Errorf("MaxMemory = %v, want %v", backend.config.MaxMemory, config.MaxMemory)
	}

	if backend.config.MaxEntries != config.MaxEntries {
		t.Errorf("MaxEntries = %v, want %v", backend.config.MaxEntries, config.MaxEntries)
	}
}

func TestMemory_DefaultConfig(t *testing.T) {
	backend := NewMemory()
	defer backend.Close()

	if backend.config.MaxMemory != 0 {
		t.Errorf("Default MaxMemory = %v, want 0 (unlimited)", backend.config.MaxMemory)
	}

	if backend.config.MaxEntries != 0 {
		t.Errorf("Default MaxEntries = %v, want 0 (unlimited)", backend.config.MaxEntries)
	}

	if backend.config.CleanupInterval != 1*time.Minute {
		t.Errorf("Default CleanupInterval = %v, want 1m", backend.config.CleanupInterval)
	}

	if backend.config.EvictionPolicy != EvictionLRU {
		t.Errorf("Default EvictionPolicy = %v, want LRU", backend.config.EvictionPolicy)
	}
}

func TestMemory_TTLCleanup(t *testing.T) {
	config := &MemoryConfig{
		CleanupInterval: 50 * time.Millisecond,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Set entry with short TTL
	err := backend.Set(ctx, "test:expires", []byte("value"), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Should exist immediately
	_, err = backend.Get(ctx, "test:expires")
	if err != nil {
		t.Errorf("Get() failed immediately: %v", err)
	}

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Should be cleaned up
	_, err = backend.Get(ctx, "test:expires")
	if err == nil {
		t.Error("Get() should fail after cleanup")
	}

	// Check that eviction was recorded
	stats, _ := backend.Stats(ctx)
	if stats.Evictions == 0 {
		t.Error("Evictions should be > 0 after TTL cleanup")
	}
}

func TestMemory_MaxEntries(t *testing.T) {
	config := &MemoryConfig{
		MaxEntries:     5,
		EvictionPolicy: EvictionFIFO,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Add entries up to limit
	for i := 0; i < 5; i++ {
		key := "test:" + string(rune('0'+i))
		err := backend.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set() #%d failed: %v", i, err)
		}
	}

	stats, _ := backend.Stats(ctx)
	if stats.Entries != 5 {
		t.Errorf("Entries = %v, want 5", stats.Entries)
	}

	// Add one more - should evict oldest
	err := backend.Set(ctx, "test:6", []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set() over limit failed: %v", err)
	}

	stats, _ = backend.Stats(ctx)
	if stats.Entries != 5 {
		t.Errorf("Entries after eviction = %v, want 5", stats.Entries)
	}

	if stats.Evictions != 1 {
		t.Errorf("Evictions = %v, want 1", stats.Evictions)
	}

	// First entry should be gone (FIFO)
	_, err = backend.Get(ctx, "test:0")
	if err == nil {
		t.Error("First entry should have been evicted")
	}

	// Last entry should exist
	_, err = backend.Get(ctx, "test:6")
	if err != nil {
		t.Errorf("Last entry should exist: %v", err)
	}
}

func TestMemory_MaxMemory(t *testing.T) {
	config := &MemoryConfig{
		MaxMemory:      200, // 200 bytes
		EvictionPolicy: EvictionLRU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Add entries that total ~150 bytes
	value := make([]byte, 50)
	for i := 0; i < 3; i++ {
		key := "test:" + string(rune('0'+i))
		err := backend.Set(ctx, key, value, 0)
		if err != nil {
			t.Fatalf("Set() #%d failed: %v", i, err)
		}
	}

	stats, _ := backend.Stats(ctx)
	if stats.Size != 150 {
		t.Errorf("Size = %v, want 150", stats.Size)
	}

	// Add larger entry - should evict to make room
	largeValue := make([]byte, 100)
	err := backend.Set(ctx, "test:large", largeValue, 0)
	if err != nil {
		t.Fatalf("Set() large failed: %v", err)
	}

	stats, _ = backend.Stats(ctx)
	if stats.Size > 200 {
		t.Errorf("Size = %v, should not exceed MaxMemory (200)", stats.Size)
	}

	if stats.Evictions == 0 {
		t.Error("Evictions should be > 0 after exceeding MaxMemory")
	}
}

func TestMemory_EvictionLRU(t *testing.T) {
	config := &MemoryConfig{
		MaxEntries:     3,
		EvictionPolicy: EvictionLRU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Add 3 entries
	backend.Set(ctx, "test:1", []byte("value1"), 0)
	time.Sleep(10 * time.Millisecond)
	backend.Set(ctx, "test:2", []byte("value2"), 0)
	time.Sleep(10 * time.Millisecond)
	backend.Set(ctx, "test:3", []byte("value3"), 0)
	time.Sleep(10 * time.Millisecond)

	// Access entry 1 (makes it most recently used)
	backend.Get(ctx, "test:1")
	time.Sleep(10 * time.Millisecond)

	// Add entry 4 - should evict test:2 (least recently used)
	backend.Set(ctx, "test:4", []byte("value4"), 0)

	// test:2 should be gone
	_, err := backend.Get(ctx, "test:2")
	if err == nil {
		t.Error("test:2 should have been evicted (LRU)")
	}

	// test:1 should still exist (was accessed)
	_, err = backend.Get(ctx, "test:1")
	if err != nil {
		t.Errorf("test:1 should exist (was recently used): %v", err)
	}
}

func TestMemory_EvictionLFU(t *testing.T) {
	config := &MemoryConfig{
		MaxEntries:     3,
		EvictionPolicy: EvictionLFU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Add 3 entries
	backend.Set(ctx, "test:1", []byte("value1"), 0)
	backend.Set(ctx, "test:2", []byte("value2"), 0)
	backend.Set(ctx, "test:3", []byte("value3"), 0)

	// Access entry 1 multiple times
	backend.Get(ctx, "test:1")
	backend.Get(ctx, "test:1")
	backend.Get(ctx, "test:1")

	// Access entry 2 once
	backend.Get(ctx, "test:2")

	// Don't access entry 3 at all

	// Add entry 4 - should evict test:3 (least frequently used)
	backend.Set(ctx, "test:4", []byte("value4"), 0)

	// test:3 should be gone
	_, err := backend.Get(ctx, "test:3")
	if err == nil {
		t.Error("test:3 should have been evicted (LFU)")
	}

	// test:1 should still exist (most frequently used)
	_, err = backend.Get(ctx, "test:1")
	if err != nil {
		t.Errorf("test:1 should exist (most frequently used): %v", err)
	}
}

func TestMemory_EvictionFIFO(t *testing.T) {
	config := &MemoryConfig{
		MaxEntries:     3,
		EvictionPolicy: EvictionFIFO,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()

	// Add 3 entries
	backend.Set(ctx, "test:1", []byte("value1"), 0)
	time.Sleep(10 * time.Millisecond)
	backend.Set(ctx, "test:2", []byte("value2"), 0)
	time.Sleep(10 * time.Millisecond)
	backend.Set(ctx, "test:3", []byte("value3"), 0)
	time.Sleep(10 * time.Millisecond)

	// Add entry 4 - should evict test:1 (oldest)
	backend.Set(ctx, "test:4", []byte("value4"), 0)

	// test:1 should be gone
	_, err := backend.Get(ctx, "test:1")
	if err == nil {
		t.Error("test:1 should have been evicted (FIFO)")
	}

	// test:2 should still exist
	_, err = backend.Get(ctx, "test:2")
	if err != nil {
		t.Errorf("test:2 should exist: %v", err)
	}
}

func TestMemory_Close(t *testing.T) {
	backend := NewMemory()
	ctx := context.Background()

	// Add some data
	backend.Set(ctx, "test:1", []byte("value"), 0)

	// Close
	err := backend.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Operations after close should fail
	_, err = backend.Get(ctx, "test:1")
	if err == nil {
		t.Error("Get() should fail after Close()")
	}

	err = backend.Set(ctx, "test:2", []byte("value"), 0)
	if err == nil {
		t.Error("Set() should fail after Close()")
	}

	// Close again should be idempotent
	err = backend.Close()
	if err != nil {
		t.Errorf("Second Close() should not error: %v", err)
	}
}

func TestMemory_PatternMatching(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		pattern string
		want    bool
	}{
		{
			name:    "wildcard matches all",
			key:     "light:123",
			pattern: "*",
			want:    true,
		},
		{
			name:    "prefix match",
			key:     "light:123",
			pattern: "light:*",
			want:    true,
		},
		{
			name:    "prefix no match",
			key:     "room:123",
			pattern: "light:*",
			want:    false,
		},
		{
			name:    "suffix match",
			key:     "light:123",
			pattern: "*:123",
			want:    true,
		},
		{
			name:    "exact match",
			key:     "light:123",
			pattern: "light:123",
			want:    true,
		},
		{
			name:    "exact no match",
			key:     "light:123",
			pattern: "light:456",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.key, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v",
					tt.key, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMemory_UpdateExistingKey(t *testing.T) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()

	// Set initial value
	err := backend.Set(ctx, "test:1", []byte("value1"), 0)
	if err != nil {
		t.Fatalf("Initial Set() failed: %v", err)
	}

	stats, _ := backend.Stats(ctx)
	initialSize := stats.Size

	// Update with different size value
	err = backend.Set(ctx, "test:1", []byte("longer value"), 0)
	if err != nil {
		t.Fatalf("Update Set() failed: %v", err)
	}

	// Check that size was updated correctly
	stats, _ = backend.Stats(ctx)
	if stats.Size == initialSize {
		t.Error("Size should have changed after updating with different size value")
	}

	// Entry count should not change
	if stats.Entries != 1 {
		t.Errorf("Entries = %v, want 1 (update shouldn't add new entry)", stats.Entries)
	}

	// Get updated value
	entry, err := backend.Get(ctx, "test:1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if string(entry.Value) != "longer value" {
		t.Errorf("Value = %q, want %q", entry.Value, "longer value")
	}
}

package cache

import (
	"context"
	"testing"
	"time"
)

// BackendTestSuite provides a standard test suite for Backend implementations.
// Backend implementations should call RunBackendTests with their implementation.
//
// Example usage in a backend implementation:
//
//	func TestMemory_BackendContract(t *testing.T) {
//		suite := cache.BackendTestSuite{
//			NewBackend: func(t *testing.T) cache.Backend {
//				return NewMemory()
//			},
//		}
//		cache.RunBackendTests(t, suite)
//	}
type BackendTestSuite struct {
	// NewBackend creates a new backend instance for testing.
	NewBackend func(t *testing.T) Backend
}

// RunBackendTests runs the complete backend test suite.
// This validates that a Backend implementation correctly implements
// the contract defined by the Backend interface.
func RunBackendTests(t *testing.T, suite BackendTestSuite) {
	t.Run("Get", func(t *testing.T) { testBackendGet(t, suite) })
	t.Run("Set", func(t *testing.T) { testBackendSet(t, suite) })
	t.Run("Delete", func(t *testing.T) { testBackendDelete(t, suite) })
	t.Run("Clear", func(t *testing.T) { testBackendClear(t, suite) })
	t.Run("Keys", func(t *testing.T) { testBackendKeys(t, suite) })
	t.Run("Stats", func(t *testing.T) { testBackendStats(t, suite) })
	t.Run("TTL", func(t *testing.T) { testBackendTTL(t, suite) })
	t.Run("Concurrency", func(t *testing.T) { testBackendConcurrency(t, suite) })
}

func testBackendGet(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Get non-existent key
	_, err := backend.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get() should return error for non-existent key")
	}

	// Set and get
	value := []byte("test value")
	err = backend.Set(ctx, "test:1", value, 0)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	entry, err := backend.Get(ctx, "test:1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if string(entry.Value) != string(value) {
		t.Errorf("Get() value = %v, want %v", entry.Value, value)
	}
}

func testBackendSet(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Basic set
	err := backend.Set(ctx, "test:1", []byte("value1"), 0)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Overwrite
	err = backend.Set(ctx, "test:1", []byte("value2"), 0)
	if err != nil {
		t.Fatalf("Set() overwrite failed: %v", err)
	}

	entry, err := backend.Get(ctx, "test:1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if string(entry.Value) != "value2" {
		t.Errorf("After overwrite, value = %v, want value2", entry.Value)
	}

	// Set with TTL
	err = backend.Set(ctx, "test:2", []byte("expires"), 1*time.Hour)
	if err != nil {
		t.Fatalf("Set() with TTL failed: %v", err)
	}

	entry, err = backend.Get(ctx, "test:2")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if entry.ExpiresAt.IsZero() {
		t.Error("Entry with TTL should have ExpiresAt set")
	}
}

func testBackendDelete(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Set a key
	err := backend.Set(ctx, "test:1", []byte("value"), 0)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Delete it
	err = backend.Delete(ctx, "test:1")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify it's gone
	_, err = backend.Get(ctx, "test:1")
	if err == nil {
		t.Error("Get() should fail after Delete()")
	}

	// Delete non-existent key (should be idempotent)
	err = backend.Delete(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Delete() of non-existent key should not error: %v", err)
	}
}

func testBackendClear(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Set multiple keys
	for i := 0; i < 5; i++ {
		key := "test:" + string(rune('0'+i))
		err := backend.Set(ctx, key, []byte("value"), 0)
		if err != nil {
			t.Fatalf("Set() failed: %v", err)
		}
	}

	// Clear all
	err := backend.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	// Verify all are gone
	keys, err := backend.Keys(ctx, "*")
	if err != nil {
		t.Fatalf("Keys() failed: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("After Clear(), found %d keys, want 0", len(keys))
	}
}

func testBackendKeys(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Set test data
	testData := map[string]string{
		"light:1": "value1",
		"light:2": "value2",
		"room:1":  "value3",
		"room:2":  "value4",
		"scene:1": "value5",
	}

	for key, value := range testData {
		err := backend.Set(ctx, key, []byte(value), 0)
		if err != nil {
			t.Fatalf("Set() failed: %v", err)
		}
	}

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{
			name:    "all keys",
			pattern: "*",
			want:    5,
		},
		{
			name:    "light prefix",
			pattern: "light:*",
			want:    2,
		},
		{
			name:    "room prefix",
			pattern: "room:*",
			want:    2,
		},
		{
			name:    "scene prefix",
			pattern: "scene:*",
			want:    1,
		},
		{
			name:    "no matches",
			pattern: "device:*",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := backend.Keys(ctx, tt.pattern)
			if err != nil {
				t.Fatalf("Keys() failed: %v", err)
			}

			if len(keys) != tt.want {
				t.Errorf("Keys(%q) returned %d keys, want %d", tt.pattern, len(keys), tt.want)
			}
		})
	}
}

func testBackendStats(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Get initial stats
	stats, err := backend.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() failed: %v", err)
	}

	if stats == nil {
		t.Fatal("Stats() returned nil")
	}

	// Stats should be available (values may vary by implementation)
	_ = stats.Hits
	_ = stats.Misses
	_ = stats.Entries
}

func testBackendTTL(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()

	// Set with short TTL
	err := backend.Set(ctx, "test:expires", []byte("value"), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Should exist immediately
	_, err = backend.Get(ctx, "test:expires")
	if err != nil {
		t.Errorf("Get() failed immediately after Set(): %v", err)
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Should be expired or not found
	_, err = backend.Get(ctx, "test:expires")
	if err == nil {
		t.Error("Get() should fail after TTL expiration")
	}
}

func testBackendConcurrency(t *testing.T, suite BackendTestSuite) {
	backend := suite.NewBackend(t)
	defer backend.Close()

	ctx := context.Background()
	const goroutines = 10
	const operations = 100

	// Concurrent writes
	errChan := make(chan error, goroutines*operations)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			for i := 0; i < operations; i++ {
				key := "concurrent:" + string(rune('0'+id))
				err := backend.Set(ctx, key, []byte("value"), 0)
				if err != nil {
					errChan <- err
				}
			}
		}(g)
	}

	// Concurrent reads
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			for i := 0; i < operations; i++ {
				key := "concurrent:" + string(rune('0'+id))
				_, _ = backend.Get(ctx, key) // May or may not exist
			}
		}(g)
	}

	// Check for errors
	select {
	case err := <-errChan:
		t.Fatalf("Concurrent operation failed: %v", err)
	case <-time.After(5 * time.Second):
		// No errors within timeout - good
	}
}

package backends

import (
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	cache "github.com/rmrfslashbin/hue-cache"
)

// File implements a file-based cache backend with periodic persistence.
// It stores cache entries in memory and periodically flushes to disk.
// On startup, it loads existing cache from disk for faster warm-up.
type File struct {
	memory           *Memory
	filePath         string
	autoSaveInterval time.Duration
	saveTicker       *time.Ticker
	saveStop         chan struct{}
	mu               sync.RWMutex
	closed           bool
}

// FileConfig contains configuration for the file backend.
type FileConfig struct {
	// FilePath is the path to the cache file.
	// Default: "./hue-cache.gob"
	FilePath string

	// AutoSaveInterval is how often to automatically flush to disk.
	// Set to 0 to disable auto-save (manual Save() only).
	// Default: 5 minutes
	AutoSaveInterval time.Duration

	// LoadOnStart loads existing cache from disk on initialization.
	// Default: true
	LoadOnStart bool

	// MemoryConfig is the configuration for the underlying memory backend.
	// If nil, defaults are used.
	MemoryConfig *MemoryConfig
}

// DefaultFileConfig returns default configuration for file backend.
func DefaultFileConfig() *FileConfig {
	return &FileConfig{
		FilePath:         "./hue-cache.gob",
		AutoSaveInterval: 5 * time.Minute,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}
}

// NewFile creates a new file-based cache backend.
// The backend stores data in memory and periodically flushes to disk.
//
// Example:
//
//	config := backends.DefaultFileConfig()
//	config.FilePath = "/var/cache/hue/cache.gob"
//	config.AutoSaveInterval = 10 * time.Minute
//	backend := backends.NewFile(config)
//	defer backend.Close() // Important: saves cache on shutdown
func NewFile(config *FileConfig) (*File, error) {
	if config == nil {
		config = DefaultFileConfig()
	}

	if config.MemoryConfig == nil {
		config.MemoryConfig = DefaultMemoryConfig()
	}

	f := &File{
		memory:           NewMemory(config.MemoryConfig),
		filePath:         config.FilePath,
		autoSaveInterval: config.AutoSaveInterval,
		saveStop:         make(chan struct{}),
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	// Load existing cache from disk
	if config.LoadOnStart {
		if err := f.Load(); err != nil {
			// Log error but continue - cache will be empty
			// This is expected on first run when file doesn't exist
		}
	}

	// Start auto-save ticker if enabled
	if config.AutoSaveInterval > 0 {
		f.saveTicker = time.NewTicker(config.AutoSaveInterval)
		go f.autoSaveLoop()
	}

	return f, nil
}

// autoSaveLoop periodically saves the cache to disk.
func (f *File) autoSaveLoop() {
	for {
		select {
		case <-f.saveTicker.C:
			f.mu.RLock()
			if f.closed {
				f.mu.RUnlock()
				return
			}
			f.mu.RUnlock()

			_ = f.Save()
		case <-f.saveStop:
			return
		}
	}
}

// Get retrieves an entry from the cache.
func (f *File) Get(ctx context.Context, key string) (*cache.Entry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, cache.NewError("Get", key, cache.ErrBackendClosed)
	}

	return f.memory.Get(ctx, key)
}

// Set stores an entry in the cache.
func (f *File) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return cache.NewError("Set", key, cache.ErrBackendClosed)
	}

	return f.memory.Set(ctx, key, value, ttl)
}

// Delete removes an entry from the cache.
func (f *File) Delete(ctx context.Context, key string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return cache.NewError("Delete", key, cache.ErrBackendClosed)
	}

	return f.memory.Delete(ctx, key)
}

// Clear removes all entries from the cache.
func (f *File) Clear(ctx context.Context) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return cache.ErrBackendClosed
	}

	return f.memory.Clear(ctx)
}

// Keys returns all keys matching the pattern.
func (f *File) Keys(ctx context.Context, pattern string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, cache.ErrBackendClosed
	}

	return f.memory.Keys(ctx, pattern)
}

// Stats returns cache statistics.
func (f *File) Stats(ctx context.Context) (*cache.Stats, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return nil, cache.ErrBackendClosed
	}

	return f.memory.Stats(ctx)
}

// Save writes the current cache state to disk.
// This is called automatically based on AutoSaveInterval, but can also
// be called manually for immediate persistence.
//
// Example:
//
//	// Manually save cache to disk
//	if err := backend.Save(); err != nil {
//	    log.Printf("Failed to save cache: %v", err)
//	}
func (f *File) Save() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return cache.ErrBackendClosed
	}

	// Create temporary file for atomic write
	tmpPath := f.filePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer file.Close()

	// Collect all entries
	ctx := context.Background()
	keys, err := f.memory.Keys(ctx, "*")
	if err != nil {
		return fmt.Errorf("getting keys: %w", err)
	}

	entries := make([]*cache.Entry, 0, len(keys))
	for _, key := range keys {
		entry, err := f.memory.Get(ctx, key)
		if err != nil {
			continue // Skip entries that error
		}
		entries = append(entries, entry)
	}

	// Encode to GOB
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(entries); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("encoding cache: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("syncing file: %w", err)
	}

	// Close file before rename
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, f.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// Load reads the cache state from disk.
// This is called automatically on startup if LoadOnStart is true,
// but can also be called manually to reload cache.
//
// Example:
//
//	// Manually reload cache from disk
//	if err := backend.Load(); err != nil {
//	    log.Printf("Failed to load cache: %v", err)
//	}
func (f *File) Load() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return cache.ErrBackendClosed
	}

	// Check if file exists
	if _, err := os.Stat(f.filePath); os.IsNotExist(err) {
		return nil // Not an error - file doesn't exist yet
	}

	file, err := os.Open(f.filePath)
	if err != nil {
		return fmt.Errorf("opening cache file: %w", err)
	}
	defer file.Close()

	// Decode from GOB
	var entries []*cache.Entry
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("decoding cache: %w", err)
	}

	// Load entries into memory
	ctx := context.Background()
	for _, entry := range entries {
		// Skip expired entries
		if entry.IsExpired() {
			continue
		}

		// Calculate remaining TTL
		var ttl time.Duration
		if !entry.ExpiresAt.IsZero() {
			ttl = time.Until(entry.ExpiresAt)
			if ttl < 0 {
				continue // Expired
			}
		}

		_ = f.memory.Set(ctx, entry.Key, entry.Value, ttl)
	}

	return nil
}

// Close stops auto-save and saves final state to disk.
func (f *File) Close() error {
	// Check if already closed
	f.mu.RLock()
	if f.closed {
		f.mu.RUnlock()
		return nil
	}
	f.mu.RUnlock()

	// Stop auto-save ticker
	if f.saveTicker != nil {
		f.saveTicker.Stop()
		close(f.saveStop) // Signal autoSaveLoop to stop
	}

	// Save final state (before marking as closed)
	saveErr := f.Save()

	// Mark as closed
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()

	// Close memory backend
	closeErr := f.memory.Close()

	// Return first error encountered
	if saveErr != nil {
		return fmt.Errorf("final save: %w", saveErr)
	}
	return closeErr
}

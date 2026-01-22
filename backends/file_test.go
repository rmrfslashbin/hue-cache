package backends

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFile_BasicOperations(t *testing.T) {
	tmpDir := t.TempDir()
	config := &FileConfig{
		FilePath:         filepath.Join(tmpDir, "test.gob"),
		AutoSaveInterval: 0, // Disable auto-save for test
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Test Set
	err = backend.Set(ctx, "test:1", []byte("value1"), 0)
	if err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Test Get
	entry, err := backend.Get(ctx, "test:1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if string(entry.Value) != "value1" {
		t.Errorf("Expected value1, got %s", entry.Value)
	}

	// Test Delete
	err = backend.Delete(ctx, "test:1")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify deleted
	_, err = backend.Get(ctx, "test:1")
	if err == nil {
		t.Error("Expected error after delete")
	}
}

func TestFile_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.gob")

	ctx := context.Background()

	// Create backend and populate
	config1 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend1, err := NewFile(config1)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	// Add entries
	backend1.Set(ctx, "light:1", []byte("light1"), 0)
	backend1.Set(ctx, "light:2", []byte("light2"), 0)
	backend1.Set(ctx, "room:1", []byte("room1"), 0)

	// Save to disk
	err = backend1.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	backend1.Close()

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// Create new backend and load
	config2 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend2, err := NewFile(config2)
	if err != nil {
		t.Fatalf("NewFile() with load failed: %v", err)
	}
	defer backend2.Close()

	// Verify entries were loaded
	entry1, err := backend2.Get(ctx, "light:1")
	if err != nil {
		t.Errorf("Get() after load failed: %v", err)
	} else if string(entry1.Value) != "light1" {
		t.Errorf("Expected light1, got %s", entry1.Value)
	}

	entry2, err := backend2.Get(ctx, "light:2")
	if err != nil {
		t.Errorf("Get() after load failed: %v", err)
	} else if string(entry2.Value) != "light2" {
		t.Errorf("Expected light2, got %s", entry2.Value)
	}

	entry3, err := backend2.Get(ctx, "room:1")
	if err != nil {
		t.Errorf("Get() after load failed: %v", err)
	} else if string(entry3.Value) != "room1" {
		t.Errorf("Expected room1, got %s", entry3.Value)
	}
}

func TestFile_AutoSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "autosave.gob")

	ctx := context.Background()

	// Create backend with short auto-save interval
	config := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 500 * time.Millisecond,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	// Add entry
	backend.Set(ctx, "test:1", []byte("value1"), 0)

	// Wait for auto-save
	time.Sleep(700 * time.Millisecond)

	// Close backend
	backend.Close()

	// Verify file was created by auto-save
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Auto-save did not create file")
	}

	// Load and verify
	config2 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend2, err := NewFile(config2)
	if err != nil {
		t.Fatalf("NewFile() with load failed: %v", err)
	}
	defer backend2.Close()

	entry, err := backend2.Get(ctx, "test:1")
	if err != nil {
		t.Errorf("Get() after auto-save failed: %v", err)
	} else if string(entry.Value) != "value1" {
		t.Errorf("Expected value1, got %s", entry.Value)
	}
}

func TestFile_TTLPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "ttl.gob")

	ctx := context.Background()

	// Create backend and add entry with TTL
	config1 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend1, err := NewFile(config1)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	// Add entry with 2 second TTL
	backend1.Set(ctx, "test:1", []byte("value1"), 2*time.Second)
	backend1.Save()
	backend1.Close()

	// Wait for TTL to expire
	time.Sleep(3 * time.Second)

	// Load cache - expired entries should not be loaded
	config2 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend2, err := NewFile(config2)
	if err != nil {
		t.Fatalf("NewFile() with load failed: %v", err)
	}
	defer backend2.Close()

	// Entry should not exist (expired)
	_, err = backend2.Get(ctx, "test:1")
	if err == nil {
		t.Error("Expected error for expired entry")
	}
}

func TestFile_ClearAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clear.gob")

	ctx := context.Background()

	// Create backend and populate
	config := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}
	defer backend.Close()

	// Add entries
	backend.Set(ctx, "test:1", []byte("value1"), 0)
	backend.Set(ctx, "test:2", []byte("value2"), 0)

	// Clear cache
	err = backend.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	// Save empty cache
	err = backend.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists but is empty
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// Reload and verify empty
	backend.Close()

	config2 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend2, err := NewFile(config2)
	if err != nil {
		t.Fatalf("NewFile() with load failed: %v", err)
	}
	defer backend2.Close()

	stats, _ := backend2.Stats(ctx)
	if stats.Entries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.Entries)
	}
}

func TestFile_CloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	config := &FileConfig{
		FilePath:         filepath.Join(tmpDir, "test.gob"),
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	// Close multiple times should not error
	err = backend.Close()
	if err != nil {
		t.Errorf("First Close() failed: %v", err)
	}

	err = backend.Close()
	if err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

func TestFile_OperationsAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	config := &FileConfig{
		FilePath:         filepath.Join(tmpDir, "test.gob"),
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	backend.Close()

	ctx := context.Background()

	// Operations after close should error
	err = backend.Set(ctx, "test:1", []byte("value"), 0)
	if err == nil {
		t.Error("Expected error for Set() after close")
	}

	_, err = backend.Get(ctx, "test:1")
	if err == nil {
		t.Error("Expected error for Get() after close")
	}

	err = backend.Delete(ctx, "test:1")
	if err == nil {
		t.Error("Expected error for Delete() after close")
	}

	err = backend.Clear(ctx)
	if err == nil {
		t.Error("Expected error for Clear() after close")
	}

	_, err = backend.Keys(ctx, "*")
	if err == nil {
		t.Error("Expected error for Keys() after close")
	}

	_, err = backend.Stats(ctx)
	if err == nil {
		t.Error("Expected error for Stats() after close")
	}
}

func TestFile_LargeCache(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large.gob")

	ctx := context.Background()

	config := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      false,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend, err := NewFile(config)
	if err != nil {
		t.Fatalf("NewFile() failed: %v", err)
	}

	// Add 1000 entries
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("test:%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		backend.Set(ctx, key, value, 0)
	}

	// Save
	err = backend.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	backend.Close()

	// Load
	config2 := &FileConfig{
		FilePath:         filePath,
		AutoSaveInterval: 0,
		LoadOnStart:      true,
		MemoryConfig:     DefaultMemoryConfig(),
	}

	backend2, err := NewFile(config2)
	if err != nil {
		t.Fatalf("NewFile() with load failed: %v", err)
	}
	defer backend2.Close()

	// Verify count
	stats, _ := backend2.Stats(ctx)
	if stats.Entries != 1000 {
		t.Errorf("Expected 1000 entries, got %d", stats.Entries)
	}

	// Spot check a few entries
	for i := 0; i < 1000; i += 100 {
		key := fmt.Sprintf("test:%d", i)
		entry, err := backend2.Get(ctx, key)
		if err != nil {
			t.Errorf("Get(%s) failed: %v", key, err)
			continue
		}
		expected := fmt.Sprintf("value%d", i)
		if string(entry.Value) != expected {
			t.Errorf("Expected %s, got %s", expected, entry.Value)
		}
	}
}

func TestFile_DefaultConfig(t *testing.T) {
	config := DefaultFileConfig()

	if config.FilePath != "./hue-cache.gob" {
		t.Errorf("Expected default FilePath ./hue-cache.gob, got %s", config.FilePath)
	}

	if config.AutoSaveInterval != 5*time.Minute {
		t.Errorf("Expected default AutoSaveInterval 5m, got %v", config.AutoSaveInterval)
	}

	if !config.LoadOnStart {
		t.Error("Expected LoadOnStart to be true")
	}

	if config.MemoryConfig == nil {
		t.Error("Expected non-nil MemoryConfig")
	}
}

package cache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rmrfslashbin/hue-sdk/resources"
)

// mockBackend is a simple in-memory backend for testing.
type mockBackend struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		data: make(map[string]*Entry),
	}
}

func (m *mockBackend) Get(ctx context.Context, key string) (*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return entry.Clone(), nil
}

func (m *mockBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = NewEntry(key, value, ttl)
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

func (m *mockBackend) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]*Entry)
	return nil
}

func (m *mockBackend) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockBackend) Stats(ctx context.Context) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &Stats{
		Entries: int64(len(m.data)),
	}, nil
}

func (m *mockBackend) Close() error {
	return nil
}

func TestSyncEngine_NewSyncEngine(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	// SDK client would be created here, but we'll test without it for now
	// since we can't easily mock the full SDK

	config := &SyncConfig{
		EnableAutoSync: true,
		SyncOnStart:    false,
	}

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     config,
		done:       make(chan struct{}),
	}

	if engine.backend == nil {
		t.Error("NewSyncEngine() backend is nil")
	}

	if engine.config.EnableAutoSync != true {
		t.Error("NewSyncEngine() EnableAutoSync not set correctly")
	}
}

func TestSyncEngine_ProcessEventData_Add(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     DefaultSyncConfig(),
	}

	// Create test event data
	lightData := map[string]interface{}{
		"id":   "light-123",
		"type": "light",
		"on":   map[string]interface{}{"on": true},
		"metadata": map[string]interface{}{
			"name": "Test Light",
		},
	}

	rawData, _ := json.Marshal(lightData)

	eventData := &resources.EventData{
		ID:      "light-123",
		Type:    "light",
		RawData: json.RawMessage(rawData),
	}

	// Process add event
	err := engine.processEventData(resources.EventTypeAdd, eventData)
	if err != nil {
		t.Fatalf("processEventData() failed: %v", err)
	}

	// Verify it was added to cache
	key := engine.keyBuilder.Light("light-123")
	entry, err := backend.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Cache Get() failed: %v", err)
	}

	if entry == nil {
		t.Fatal("Entry not found in cache after add event")
	}

	// Verify stats
	if engine.stats.AddEvents != 1 {
		t.Errorf("AddEvents = %d, want 1", engine.stats.AddEvents)
	}
}

func TestSyncEngine_ProcessEventData_Update(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     DefaultSyncConfig(),
	}

	// Create initial data
	initialData := map[string]interface{}{
		"id":   "light-123",
		"type": "light",
		"on":   map[string]interface{}{"on": false},
	}
	initialRaw, _ := json.Marshal(initialData)

	eventData := &resources.EventData{
		ID:      "light-123",
		Type:    "light",
		RawData: json.RawMessage(initialRaw),
	}

	// Add initial entry
	engine.processEventData(resources.EventTypeAdd, eventData)

	// Update with new data
	updatedData := map[string]interface{}{
		"id":   "light-123",
		"type": "light",
		"on":   map[string]interface{}{"on": true},
	}
	updatedRaw, _ := json.Marshal(updatedData)

	updatedEventData := &resources.EventData{
		ID:      "light-123",
		Type:    "light",
		RawData: json.RawMessage(updatedRaw),
	}

	// Process update event
	err := engine.processEventData(resources.EventTypeUpdate, updatedEventData)
	if err != nil {
		t.Fatalf("processEventData() update failed: %v", err)
	}

	// Verify cache was updated
	key := engine.keyBuilder.Light("light-123")
	entry, err := backend.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Cache Get() failed: %v", err)
	}

	var cached map[string]interface{}
	json.Unmarshal(entry.Value, &cached)

	onState := cached["on"].(map[string]interface{})
	if onState["on"].(bool) != true {
		t.Error("Cached value was not updated")
	}

	// Verify stats
	if engine.stats.UpdateEvents != 1 {
		t.Errorf("UpdateEvents = %d, want 1", engine.stats.UpdateEvents)
	}
}

func TestSyncEngine_ProcessEventData_Delete(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     DefaultSyncConfig(),
	}

	// Add initial data
	lightData := map[string]interface{}{
		"id":   "light-123",
		"type": "light",
	}
	rawData, _ := json.Marshal(lightData)

	eventData := &resources.EventData{
		ID:      "light-123",
		Type:    "light",
		RawData: json.RawMessage(rawData),
	}

	engine.processEventData(resources.EventTypeAdd, eventData)

	// Process delete event
	err := engine.processEventData(resources.EventTypeDelete, eventData)
	if err != nil {
		t.Fatalf("processEventData() delete failed: %v", err)
	}

	// Verify it was removed from cache
	key := engine.keyBuilder.Light("light-123")
	_, err = backend.Get(context.Background(), key)
	if err == nil {
		t.Error("Entry should have been deleted from cache")
	}

	// Verify stats
	if engine.stats.DeleteEvents != 1 {
		t.Errorf("DeleteEvents = %d, want 1", engine.stats.DeleteEvents)
	}
}

func TestSyncEngine_ProcessEvent(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	var handlerCalled bool
	config := &SyncConfig{
		EnableAutoSync: true,
		EventHandler: func(event *resources.Event) {
			handlerCalled = true
		},
	}

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     config,
	}

	// Create test event
	lightData := map[string]interface{}{
		"id":   "light-123",
		"type": "light",
	}
	rawData, _ := json.Marshal(lightData)

	event := &resources.Event{
		Type:         resources.EventTypeAdd,
		CreationTime: time.Now().Format(time.RFC3339),
		ID:           "event-123",
		Data: []resources.EventData{
			{
				ID:      "light-123",
				Type:    "light",
				RawData: json.RawMessage(rawData),
			},
		},
	}

	// Process event
	engine.processEvent(event)

	// Verify event handler was called
	if !handlerCalled {
		t.Error("Event handler was not called")
	}

	// Verify statistics
	if engine.stats.EventsProcessed != 1 {
		t.Errorf("EventsProcessed = %d, want 1", engine.stats.EventsProcessed)
	}

	if engine.stats.LastEventTime.IsZero() {
		t.Error("LastEventTime was not set")
	}

	if engine.stats.AvgLatency == 0 {
		t.Error("AvgLatency was not calculated")
	}
}

func TestSyncEngine_HandleError(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	var errorHandlerCalled bool
	var capturedError error

	config := &SyncConfig{
		EnableAutoSync: true,
		ErrorHandler: func(err error) {
			errorHandlerCalled = true
			capturedError = err
		},
	}

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     config,
	}

	testErr := ErrInvalidKey
	engine.handleError(testErr)

	// Verify error handler was called
	if !errorHandlerCalled {
		t.Error("Error handler was not called")
	}

	if capturedError != testErr {
		t.Errorf("Captured error = %v, want %v", capturedError, testErr)
	}

	// Verify stats
	if engine.stats.SyncErrors != 1 {
		t.Errorf("SyncErrors = %d, want 1", engine.stats.SyncErrors)
	}

	if engine.stats.LastError == "" {
		t.Error("LastError was not set")
	}

	if engine.stats.LastErrorTime.IsZero() {
		t.Error("LastErrorTime was not set")
	}
}

func TestSyncEngine_Stats(t *testing.T) {
	backend := newMockBackend()
	defer backend.Close()

	engine := &SyncEngine{
		backend:    backend,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     DefaultSyncConfig(),
	}

	// Set some stats
	engine.stats.mu.Lock()
	engine.stats.EventsProcessed = 100
	engine.stats.AddEvents = 30
	engine.stats.UpdateEvents = 50
	engine.stats.DeleteEvents = 20
	engine.stats.mu.Unlock()

	// Get stats
	stats := engine.Stats()

	if stats.EventsProcessed != 100 {
		t.Errorf("EventsProcessed = %d, want 100", stats.EventsProcessed)
	}

	if stats.AddEvents != 30 {
		t.Errorf("AddEvents = %d, want 30", stats.AddEvents)
	}

	if stats.UpdateEvents != 50 {
		t.Errorf("UpdateEvents = %d, want 50", stats.UpdateEvents)
	}

	if stats.DeleteEvents != 20 {
		t.Errorf("DeleteEvents = %d, want 20", stats.DeleteEvents)
	}
}

func TestSyncStats_Clone(t *testing.T) {
	original := &SyncStats{
		EventsProcessed: 100,
		AddEvents:       30,
		UpdateEvents:    50,
		DeleteEvents:    20,
		SyncErrors:      5,
		LastError:       "test error",
		AvgLatency:      time.Millisecond,
	}

	clone := original.Clone()

	// Check values are equal
	if clone.EventsProcessed != original.EventsProcessed {
		t.Error("Clone EventsProcessed mismatch")
	}

	if clone.AddEvents != original.AddEvents {
		t.Error("Clone AddEvents mismatch")
	}

	// Modify clone
	clone.EventsProcessed = 999

	// Original should be unchanged
	if original.EventsProcessed == 999 {
		t.Error("Modifying clone affected original")
	}
}

func TestSyncEngine_KeyBuilding(t *testing.T) {
	engine := &SyncEngine{
		keyBuilder: NewKeyBuilder(),
	}

	tests := []struct {
		resourceType string
		id           string
		want         string
	}{
		{
			resourceType: "light",
			id:           "abc-123",
			want:         "light:abc-123",
		},
		{
			resourceType: "room",
			id:           "def-456",
			want:         "room:def-456",
		},
		{
			resourceType: "scene",
			id:           "ghi-789",
			want:         "scene:ghi-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			got := engine.keyBuilder.Resource(tt.resourceType, tt.id)
			if got != tt.want {
				t.Errorf("keyBuilder.Resource(%q, %q) = %q, want %q",
					tt.resourceType, tt.id, got, tt.want)
			}
		})
	}
}

func TestDefaultSyncConfig(t *testing.T) {
	config := DefaultSyncConfig()

	if config.EnableAutoSync != true {
		t.Error("Default EnableAutoSync should be true")
	}

	if config.SyncOnStart != false {
		t.Error("Default SyncOnStart should be false")
	}

	if config.ErrorHandler != nil {
		t.Error("Default ErrorHandler should be nil")
	}

	if config.EventHandler != nil {
		t.Error("Default EventHandler should be nil")
	}
}

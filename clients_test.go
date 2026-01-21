package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rmrfslashbin/hue-sdk/resources"
)

// mockLightClient implements hue.LightClient for testing
type mockLightClient struct {
	lights map[string]*resources.Light
	calls  map[string]int
}

func newMockLightClient() *mockLightClient {
	return &mockLightClient{
		lights: make(map[string]*resources.Light),
		calls:  make(map[string]int),
	}
}

func (m *mockLightClient) List(ctx context.Context) ([]resources.Light, error) {
	m.calls["List"]++
	var lights []resources.Light
	for _, light := range m.lights {
		lights = append(lights, *light)
	}
	return lights, nil
}

func (m *mockLightClient) Get(ctx context.Context, id string) (*resources.Light, error) {
	m.calls["Get"]++
	light, ok := m.lights[id]
	if !ok {
		return nil, ErrNotFound
	}
	return light, nil
}

func (m *mockLightClient) Update(ctx context.Context, id string, update resources.LightUpdate) error {
	m.calls["Update"]++
	light, ok := m.lights[id]
	if !ok {
		return ErrNotFound
	}
	// Apply update
	if update.On != nil {
		light.On = *update.On
	}
	return nil
}

func TestCachedLightClient_Get_CacheHit(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockLightClient()

	// Populate SDK with a light
	mockSDK.lights["light-1"] = &resources.Light{
		ID:   "light-1",
		Type: "light",
		On:   resources.OnState{On: true},
	}

	client := NewCachedLightClient(backend, mockSDK, 5*time.Minute)

	// First call - cache miss, should call SDK
	light1, err := client.Get(context.Background(), "light-1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if mockSDK.calls["Get"] != 1 {
		t.Errorf("Expected 1 SDK call, got %d", mockSDK.calls["Get"])
	}

	// Second call - cache hit, should NOT call SDK
	light2, err := client.Get(context.Background(), "light-1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if mockSDK.calls["Get"] != 1 {
		t.Errorf("Expected 1 SDK call (cached), got %d", mockSDK.calls["Get"])
	}

	// Verify returned data is correct
	if light1.ID != light2.ID {
		t.Error("Cache returned different data")
	}
}

func TestCachedLightClient_Get_CacheMiss(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockLightClient()

	// Populate SDK with a light
	mockSDK.lights["light-1"] = &resources.Light{
		ID:   "light-1",
		Type: "light",
		On:   resources.OnState{On: true},
	}

	client := NewCachedLightClient(backend, mockSDK, 5*time.Minute)

	// Cache miss - should call SDK
	_, err := client.Get(context.Background(), "light-1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if mockSDK.calls["Get"] != 1 {
		t.Errorf("Expected 1 SDK call, got %d", mockSDK.calls["Get"])
	}

	// Verify data was cached
	kb := NewKeyBuilder()
	key := kb.Light("light-1")
	entry, err := backend.Get(context.Background(), key)
	if err != nil {
		t.Errorf("Expected entry to be cached, got error: %v", err)
	}

	var cached resources.Light
	if err := json.Unmarshal(entry.Value, &cached); err != nil {
		t.Fatalf("Failed to unmarshal cached data: %v", err)
	}

	if cached.ID != "light-1" {
		t.Error("Cached data doesn't match")
	}
}

func TestCachedLightClient_List_CacheHit(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockLightClient()

	// Populate SDK with lights
	mockSDK.lights["light-1"] = &resources.Light{ID: "light-1", Type: "light"}
	mockSDK.lights["light-2"] = &resources.Light{ID: "light-2", Type: "light"}

	client := NewCachedLightClient(backend, mockSDK, 5*time.Minute)

	// First call - cache miss
	lights1, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(lights1) != 2 {
		t.Errorf("Expected 2 lights, got %d", len(lights1))
	}

	if mockSDK.calls["List"] != 1 {
		t.Errorf("Expected 1 SDK call, got %d", mockSDK.calls["List"])
	}

	// Second call - cache hit
	lights2, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(lights2) != 2 {
		t.Errorf("Expected 2 lights, got %d", len(lights2))
	}

	// Should still be only 1 SDK call (cached)
	if mockSDK.calls["List"] != 1 {
		t.Errorf("Expected 1 SDK call (cached), got %d", mockSDK.calls["List"])
	}
}

func TestCachedLightClient_Update_InvalidatesCache(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockLightClient()

	// Populate SDK with a light
	mockSDK.lights["light-1"] = &resources.Light{
		ID:   "light-1",
		Type: "light",
		On:   resources.OnState{On: false},
	}

	client := NewCachedLightClient(backend, mockSDK, 5*time.Minute)

	// Get the light to populate cache
	_, err := client.Get(context.Background(), "light-1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	// Verify it's in cache
	kb := NewKeyBuilder()
	key := kb.Light("light-1")
	_, err = backend.Get(context.Background(), key)
	if err != nil {
		t.Error("Expected entry to be in cache")
	}

	// Update the light
	err = client.Update(context.Background(), "light-1", resources.LightUpdate{
		On: &resources.OnState{On: true},
	})
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Verify cache was invalidated
	_, err = backend.Get(context.Background(), key)
	if err == nil {
		t.Error("Expected cache entry to be deleted")
	}

	if mockSDK.calls["Update"] != 1 {
		t.Errorf("Expected 1 SDK Update call, got %d", mockSDK.calls["Update"])
	}
}

func TestCachedLightClient_EmptyID(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockLightClient()

	client := NewCachedLightClient(backend, mockSDK, 5*time.Minute)

	// Test Get with empty ID
	_, err := client.Get(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty ID")
	}

	// Test Update with empty ID
	err = client.Update(context.Background(), "", resources.LightUpdate{})
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

// mockRoomClient implements hue.RoomClient for testing
type mockRoomClient struct {
	rooms map[string]*resources.Room
	calls map[string]int
}

func newMockRoomClient() *mockRoomClient {
	return &mockRoomClient{
		rooms: make(map[string]*resources.Room),
		calls: make(map[string]int),
	}
}

func (m *mockRoomClient) List(ctx context.Context) ([]resources.Room, error) {
	m.calls["List"]++
	var rooms []resources.Room
	for _, room := range m.rooms {
		rooms = append(rooms, *room)
	}
	return rooms, nil
}

func (m *mockRoomClient) Get(ctx context.Context, id string) (*resources.Room, error) {
	m.calls["Get"]++
	room, ok := m.rooms[id]
	if !ok {
		return nil, ErrNotFound
	}
	return room, nil
}

func (m *mockRoomClient) Create(ctx context.Context, room resources.RoomCreate) (string, error) {
	m.calls["Create"]++
	id := "new-room-id"
	m.rooms[id] = &resources.Room{
		ID:   id,
		Type: "room",
	}
	return id, nil
}

func (m *mockRoomClient) Update(ctx context.Context, id string, update resources.RoomUpdate) error {
	m.calls["Update"]++
	_, ok := m.rooms[id]
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (m *mockRoomClient) Delete(ctx context.Context, id string) error {
	m.calls["Delete"]++
	delete(m.rooms, id)
	return nil
}

func TestCachedRoomClient_Create(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockRoomClient()

	client := NewCachedRoomClient(backend, mockSDK, 5*time.Minute)

	// Create a room
	id, err := client.Create(context.Background(), resources.RoomCreate{})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if id == "" {
		t.Error("Expected non-empty ID")
	}

	if mockSDK.calls["Create"] != 1 {
		t.Errorf("Expected 1 SDK Create call, got %d", mockSDK.calls["Create"])
	}
}

func TestCachedRoomClient_Delete(t *testing.T) {
	backend := newMockBackend()
	mockSDK := newMockRoomClient()

	// Populate SDK with a room
	mockSDK.rooms["room-1"] = &resources.Room{ID: "room-1", Type: "room"}

	client := NewCachedRoomClient(backend, mockSDK, 5*time.Minute)

	// Get room to populate cache
	_, err := client.Get(context.Background(), "room-1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	// Delete the room
	err = client.Delete(context.Background(), "room-1")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify cache was cleared
	kb := NewKeyBuilder()
	key := kb.Room("room-1")
	_, err = backend.Get(context.Background(), key)
	if err == nil {
		t.Error("Expected cache entry to be deleted")
	}

	if mockSDK.calls["Delete"] != 1 {
		t.Errorf("Expected 1 SDK Delete call, got %d", mockSDK.calls["Delete"])
	}
}

func TestCachedClient_LazyInitialization(t *testing.T) {
	backend := newMockBackend()

	// Create a basic hue.Client wrapper for testing
	// Note: In real usage, this would be a real hue.Client
	// For testing, we're just verifying lazy initialization

	cachedClient := &CachedClient{
		backend: backend,
		ttl:     5 * time.Minute,
	}

	// Lights client should be nil initially
	if cachedClient.lights != nil {
		t.Error("Expected lights client to be nil initially")
	}

	// Note: We can't fully test Lights() method without a real SDK client,
	// but we've verified the structure is correct
}

func TestKeyBuilder_AllGroupedLights(t *testing.T) {
	kb := NewKeyBuilder()

	pattern := kb.AllGroupedLights()
	if pattern != "grouped_light:*" {
		t.Errorf("AllGroupedLights() = %q, want \"grouped_light:*\"", pattern)
	}
}

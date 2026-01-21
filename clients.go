package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rmrfslashbin/hue-sdk"
	"github.com/rmrfslashbin/hue-sdk/resources"
)

// CachedLightClient wraps the SDK LightClient with caching.
// It implements the same interface as hue.LightClient for drop-in replacement.
type CachedLightClient struct {
	backend    Backend
	client     hue.LightClient
	keyBuilder *KeyBuilder
	ttl        time.Duration
}

// NewCachedLightClient creates a new cached light client.
// If ttl is 0, cached entries never expire (rely on SSE updates).
func NewCachedLightClient(backend Backend, client hue.LightClient, ttl time.Duration) *CachedLightClient {
	return &CachedLightClient{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		ttl:        ttl,
	}
}

// List returns all lights, using cache when possible.
// On cache miss, fetches from SDK and populates cache.
func (c *CachedLightClient) List(ctx context.Context) ([]resources.Light, error) {
	// Try to get all lights from cache using pattern
	pattern := c.keyBuilder.AllLights()
	keys, err := c.backend.Keys(ctx, pattern)
	if err == nil && len(keys) > 0 {
		// Attempt to get all lights from cache
		var lights []resources.Light
		allFound := true

		for _, key := range keys {
			entry, err := c.backend.Get(ctx, key)
			if err != nil {
				allFound = false
				break
			}

			var light resources.Light
			if err := json.Unmarshal(entry.Value, &light); err != nil {
				allFound = false
				break
			}

			lights = append(lights, light)
		}

		if allFound {
			return lights, nil
		}
	}

	// Cache miss - fetch from SDK
	lights, err := c.client.List(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache
	for _, light := range lights {
		key := c.keyBuilder.Light(light.ID)
		data, err := json.Marshal(light)
		if err != nil {
			continue // Skip this light, don't fail entire operation
		}
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return lights, nil
}

// Get returns a single light by ID, using cache when possible.
// On cache miss, fetches from SDK and populates cache.
func (c *CachedLightClient) Get(ctx context.Context, id string) (*resources.Light, error) {
	if id == "" {
		return nil, fmt.Errorf("invalid light ID")
	}

	// Try cache first
	key := c.keyBuilder.Light(id)
	entry, err := c.backend.Get(ctx, key)
	if err == nil {
		var light resources.Light
		if err := json.Unmarshal(entry.Value, &light); err == nil {
			return &light, nil
		}
	}

	// Cache miss - fetch from SDK
	light, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache
	data, err := json.Marshal(light)
	if err == nil {
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return light, nil
}

// Update updates a light's state in both SDK and cache.
// This is write-through caching - update SDK first, then invalidate cache.
func (c *CachedLightClient) Update(ctx context.Context, id string, update resources.LightUpdate) error {
	if id == "" {
		return fmt.Errorf("invalid light ID")
	}

	// Update SDK first
	if err := c.client.Update(ctx, id, update); err != nil {
		return err
	}

	// Invalidate cache entry (SSE event will repopulate it)
	key := c.keyBuilder.Light(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// CachedRoomClient wraps the SDK RoomClient with caching.
// It implements the same interface as hue.RoomClient for drop-in replacement.
type CachedRoomClient struct {
	backend    Backend
	client     hue.RoomClient
	keyBuilder *KeyBuilder
	ttl        time.Duration
}

// NewCachedRoomClient creates a new cached room client.
func NewCachedRoomClient(backend Backend, client hue.RoomClient, ttl time.Duration) *CachedRoomClient {
	return &CachedRoomClient{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		ttl:        ttl,
	}
}

// List returns all rooms, using cache when possible.
func (c *CachedRoomClient) List(ctx context.Context) ([]resources.Room, error) {
	pattern := c.keyBuilder.AllRooms()
	keys, err := c.backend.Keys(ctx, pattern)
	if err == nil && len(keys) > 0 {
		var rooms []resources.Room
		allFound := true

		for _, key := range keys {
			entry, err := c.backend.Get(ctx, key)
			if err != nil {
				allFound = false
				break
			}

			var room resources.Room
			if err := json.Unmarshal(entry.Value, &room); err != nil {
				allFound = false
				break
			}

			rooms = append(rooms, room)
		}

		if allFound {
			return rooms, nil
		}
	}

	// Cache miss - fetch from SDK
	rooms, err := c.client.List(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache
	for _, room := range rooms {
		key := c.keyBuilder.Room(room.ID)
		data, err := json.Marshal(room)
		if err != nil {
			continue
		}
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return rooms, nil
}

// Get returns a single room by ID, using cache when possible.
func (c *CachedRoomClient) Get(ctx context.Context, id string) (*resources.Room, error) {
	if id == "" {
		return nil, fmt.Errorf("invalid room ID")
	}

	// Try cache first
	key := c.keyBuilder.Room(id)
	entry, err := c.backend.Get(ctx, key)
	if err == nil {
		var room resources.Room
		if err := json.Unmarshal(entry.Value, &room); err == nil {
			return &room, nil
		}
	}

	// Cache miss - fetch from SDK
	room, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache
	data, err := json.Marshal(room)
	if err == nil {
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return room, nil
}

// Create creates a new room in the SDK and invalidates cache.
func (c *CachedRoomClient) Create(ctx context.Context, room resources.RoomCreate) (string, error) {
	// Create in SDK
	id, err := c.client.Create(ctx, room)
	if err != nil {
		return "", err
	}

	// Don't cache yet - SSE event will populate it
	return id, nil
}

// Update updates a room in SDK and invalidates cache.
func (c *CachedRoomClient) Update(ctx context.Context, id string, update resources.RoomUpdate) error {
	if id == "" {
		return fmt.Errorf("invalid room ID")
	}

	// Update SDK first
	if err := c.client.Update(ctx, id, update); err != nil {
		return err
	}

	// Invalidate cache entry
	key := c.keyBuilder.Room(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// Delete deletes a room from SDK and cache.
func (c *CachedRoomClient) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("invalid room ID")
	}

	// Delete from SDK first
	if err := c.client.Delete(ctx, id); err != nil {
		return err
	}

	// Remove from cache
	key := c.keyBuilder.Room(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// CachedZoneClient wraps the SDK ZoneClient with caching.
type CachedZoneClient struct {
	backend    Backend
	client     hue.ZoneClient
	keyBuilder *KeyBuilder
	ttl        time.Duration
}

// NewCachedZoneClient creates a new cached zone client.
func NewCachedZoneClient(backend Backend, client hue.ZoneClient, ttl time.Duration) *CachedZoneClient {
	return &CachedZoneClient{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		ttl:        ttl,
	}
}

// List returns all zones, using cache when possible.
func (c *CachedZoneClient) List(ctx context.Context) ([]resources.Zone, error) {
	pattern := c.keyBuilder.AllZones()
	keys, err := c.backend.Keys(ctx, pattern)
	if err == nil && len(keys) > 0 {
		var zones []resources.Zone
		allFound := true

		for _, key := range keys {
			entry, err := c.backend.Get(ctx, key)
			if err != nil {
				allFound = false
				break
			}

			var zone resources.Zone
			if err := json.Unmarshal(entry.Value, &zone); err != nil {
				allFound = false
				break
			}

			zones = append(zones, zone)
		}

		if allFound {
			return zones, nil
		}
	}

	// Cache miss - fetch from SDK
	zones, err := c.client.List(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache
	for _, zone := range zones {
		key := c.keyBuilder.Zone(zone.ID)
		data, err := json.Marshal(zone)
		if err != nil {
			continue
		}
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return zones, nil
}

// Get returns a single zone by ID, using cache when possible.
func (c *CachedZoneClient) Get(ctx context.Context, id string) (*resources.Zone, error) {
	if id == "" {
		return nil, fmt.Errorf("invalid zone ID")
	}

	// Try cache first
	key := c.keyBuilder.Zone(id)
	entry, err := c.backend.Get(ctx, key)
	if err == nil {
		var zone resources.Zone
		if err := json.Unmarshal(entry.Value, &zone); err == nil {
			return &zone, nil
		}
	}

	// Cache miss - fetch from SDK
	zone, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache
	data, err := json.Marshal(zone)
	if err == nil {
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return zone, nil
}

// Create creates a new zone in SDK and invalidates cache.
func (c *CachedZoneClient) Create(ctx context.Context, zone resources.ZoneCreate) (string, error) {
	id, err := c.client.Create(ctx, zone)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Update updates a zone in SDK and invalidates cache.
func (c *CachedZoneClient) Update(ctx context.Context, id string, update resources.ZoneUpdate) error {
	if id == "" {
		return fmt.Errorf("invalid zone ID")
	}

	if err := c.client.Update(ctx, id, update); err != nil {
		return err
	}

	key := c.keyBuilder.Zone(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// Delete deletes a zone from SDK and cache.
func (c *CachedZoneClient) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("invalid zone ID")
	}

	if err := c.client.Delete(ctx, id); err != nil {
		return err
	}

	key := c.keyBuilder.Zone(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// CachedSceneClient wraps the SDK SceneClient with caching.
type CachedSceneClient struct {
	backend    Backend
	client     hue.SceneClient
	keyBuilder *KeyBuilder
	ttl        time.Duration
}

// NewCachedSceneClient creates a new cached scene client.
func NewCachedSceneClient(backend Backend, client hue.SceneClient, ttl time.Duration) *CachedSceneClient {
	return &CachedSceneClient{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		ttl:        ttl,
	}
}

// List returns all scenes, using cache when possible.
func (c *CachedSceneClient) List(ctx context.Context) ([]resources.Scene, error) {
	pattern := c.keyBuilder.AllScenes()
	keys, err := c.backend.Keys(ctx, pattern)
	if err == nil && len(keys) > 0 {
		var scenes []resources.Scene
		allFound := true

		for _, key := range keys {
			entry, err := c.backend.Get(ctx, key)
			if err != nil {
				allFound = false
				break
			}

			var scene resources.Scene
			if err := json.Unmarshal(entry.Value, &scene); err != nil {
				allFound = false
				break
			}

			scenes = append(scenes, scene)
		}

		if allFound {
			return scenes, nil
		}
	}

	// Cache miss - fetch from SDK
	scenes, err := c.client.List(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache
	for _, scene := range scenes {
		key := c.keyBuilder.Scene(scene.ID)
		data, err := json.Marshal(scene)
		if err != nil {
			continue
		}
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return scenes, nil
}

// Get returns a single scene by ID, using cache when possible.
func (c *CachedSceneClient) Get(ctx context.Context, id string) (*resources.Scene, error) {
	if id == "" {
		return nil, fmt.Errorf("invalid scene ID")
	}

	// Try cache first
	key := c.keyBuilder.Scene(id)
	entry, err := c.backend.Get(ctx, key)
	if err == nil {
		var scene resources.Scene
		if err := json.Unmarshal(entry.Value, &scene); err == nil {
			return &scene, nil
		}
	}

	// Cache miss - fetch from SDK
	scene, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache
	data, err := json.Marshal(scene)
	if err == nil {
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return scene, nil
}

// Create creates a new scene in SDK.
func (c *CachedSceneClient) Create(ctx context.Context, scene resources.SceneCreate) (string, error) {
	id, err := c.client.Create(ctx, scene)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Update updates a scene in SDK and invalidates cache.
func (c *CachedSceneClient) Update(ctx context.Context, id string, update resources.SceneUpdate) error {
	if id == "" {
		return fmt.Errorf("invalid scene ID")
	}

	if err := c.client.Update(ctx, id, update); err != nil {
		return err
	}

	key := c.keyBuilder.Scene(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// Delete deletes a scene from SDK and cache.
func (c *CachedSceneClient) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("invalid scene ID")
	}

	if err := c.client.Delete(ctx, id); err != nil {
		return err
	}

	key := c.keyBuilder.Scene(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

// CachedGroupedLightClient wraps the SDK GroupedLightClient with caching.
type CachedGroupedLightClient struct {
	backend    Backend
	client     hue.GroupedLightClient
	keyBuilder *KeyBuilder
	ttl        time.Duration
}

// NewCachedGroupedLightClient creates a new cached grouped light client.
func NewCachedGroupedLightClient(backend Backend, client hue.GroupedLightClient, ttl time.Duration) *CachedGroupedLightClient {
	return &CachedGroupedLightClient{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		ttl:        ttl,
	}
}

// List returns all grouped lights, using cache when possible.
func (c *CachedGroupedLightClient) List(ctx context.Context) ([]resources.GroupedLight, error) {
	pattern := c.keyBuilder.AllGroupedLights()
	keys, err := c.backend.Keys(ctx, pattern)
	if err == nil && len(keys) > 0 {
		var groupedLights []resources.GroupedLight
		allFound := true

		for _, key := range keys {
			entry, err := c.backend.Get(ctx, key)
			if err != nil {
				allFound = false
				break
			}

			var gl resources.GroupedLight
			if err := json.Unmarshal(entry.Value, &gl); err != nil {
				allFound = false
				break
			}

			groupedLights = append(groupedLights, gl)
		}

		if allFound {
			return groupedLights, nil
		}
	}

	// Cache miss - fetch from SDK
	groupedLights, err := c.client.List(ctx)
	if err != nil {
		return nil, err
	}

	// Populate cache
	for _, gl := range groupedLights {
		key := c.keyBuilder.GroupedLight(gl.ID)
		data, err := json.Marshal(gl)
		if err != nil {
			continue
		}
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return groupedLights, nil
}

// Get returns a single grouped light by ID, using cache when possible.
func (c *CachedGroupedLightClient) Get(ctx context.Context, id string) (*resources.GroupedLight, error) {
	if id == "" {
		return nil, fmt.Errorf("invalid grouped light ID")
	}

	// Try cache first
	key := c.keyBuilder.GroupedLight(id)
	entry, err := c.backend.Get(ctx, key)
	if err == nil {
		var gl resources.GroupedLight
		if err := json.Unmarshal(entry.Value, &gl); err == nil {
			return &gl, nil
		}
	}

	// Cache miss - fetch from SDK
	gl, err := c.client.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache
	data, err := json.Marshal(gl)
	if err == nil {
		_ = c.backend.Set(ctx, key, data, c.ttl)
	}

	return gl, nil
}

// Update updates a grouped light in SDK and invalidates cache.
func (c *CachedGroupedLightClient) Update(ctx context.Context, id string, update resources.GroupedLightUpdate) error {
	if id == "" {
		return fmt.Errorf("invalid grouped light ID")
	}

	if err := c.client.Update(ctx, id, update); err != nil {
		return err
	}

	key := c.keyBuilder.GroupedLight(id)
	_ = c.backend.Delete(ctx, key)

	return nil
}

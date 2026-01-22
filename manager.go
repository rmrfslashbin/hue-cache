package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rmrfslashbin/hue-sdk"
)

// CacheManager provides high-level cache management operations.
// It wraps a backend and provides convenience methods for bulk operations,
// cache warming, and manual invalidation.
type CacheManager struct {
	backend    Backend
	client     *hue.Client
	keyBuilder *KeyBuilder
	mu         sync.RWMutex
}

// NewCacheManager creates a new cache manager.
func NewCacheManager(backend Backend, client *hue.Client) *CacheManager {
	return &CacheManager{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
	}
}

// ClearAll clears all entries from the cache.
func (m *CacheManager) ClearAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backend.Clear(ctx)
}

// ClearPattern clears all entries matching the pattern.
// Pattern uses glob syntax: * matches any sequence of characters.
//
// Examples:
//   - "light:*" - clear all lights
//   - "room:*" - clear all rooms
//   - "*" - clear everything
func (m *CacheManager) ClearPattern(ctx context.Context, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys, err := m.backend.Keys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("getting keys for pattern %q: %w", pattern, err)
	}

	for _, key := range keys {
		if err := m.backend.Delete(ctx, key); err != nil {
			// Continue on error, try to delete as many as possible
			continue
		}
	}

	return nil
}

// ClearLights clears all light entries from the cache.
func (m *CacheManager) ClearLights(ctx context.Context) error {
	return m.ClearPattern(ctx, m.keyBuilder.AllLights())
}

// ClearRooms clears all room entries from the cache.
func (m *CacheManager) ClearRooms(ctx context.Context) error {
	return m.ClearPattern(ctx, m.keyBuilder.AllRooms())
}

// ClearZones clears all zone entries from the cache.
func (m *CacheManager) ClearZones(ctx context.Context) error {
	return m.ClearPattern(ctx, m.keyBuilder.AllZones())
}

// ClearScenes clears all scene entries from the cache.
func (m *CacheManager) ClearScenes(ctx context.Context) error {
	return m.ClearPattern(ctx, m.keyBuilder.AllScenes())
}

// ClearGroupedLights clears all grouped light entries from the cache.
func (m *CacheManager) ClearGroupedLights(ctx context.Context) error {
	return m.ClearPattern(ctx, m.keyBuilder.AllGroupedLights())
}

// ClearResourceType clears all entries of a specific resource type.
func (m *CacheManager) ClearResourceType(ctx context.Context, resourceType string) error {
	pattern := m.keyBuilder.AllResources(resourceType)
	return m.ClearPattern(ctx, pattern)
}

// WarmConfig configures cache warming behavior.
type WarmConfig struct {
	// WarmLights warms the lights cache on startup.
	WarmLights bool

	// WarmRooms warms the rooms cache on startup.
	WarmRooms bool

	// WarmZones warms the zones cache on startup.
	WarmZones bool

	// WarmScenes warms the scenes cache on startup.
	WarmScenes bool

	// WarmGroupedLights warms the grouped lights cache on startup.
	WarmGroupedLights bool

	// TTL is the time-to-live for warmed entries.
	// Set to 0 for no expiration (rely on SSE sync).
	TTL time.Duration

	// OnError is called when warming fails for a resource type.
	OnError func(resourceType string, err error)
}

// DefaultWarmConfig returns default warming configuration.
// Warms all resource types with no TTL (rely on SSE).
func DefaultWarmConfig() *WarmConfig {
	return &WarmConfig{
		WarmLights:        true,
		WarmRooms:         true,
		WarmZones:         true,
		WarmScenes:        true,
		WarmGroupedLights: true,
		TTL:               0,
		OnError: func(resourceType string, err error) {
			// Default: silent failure (cache warming is best-effort)
		},
	}
}

// WarmCache pre-populates the cache with all resources from the bridge.
// This is useful for reducing cold-start latency.
//
// Example:
//
//	config := DefaultWarmConfig()
//	config.OnError = func(resourceType string, err error) {
//	    log.Printf("Failed to warm %s: %v", resourceType, err)
//	}
//	stats, err := manager.WarmCache(ctx, config)
func (m *CacheManager) WarmCache(ctx context.Context, config *WarmConfig) (*WarmStats, error) {
	if config == nil {
		config = DefaultWarmConfig()
	}

	stats := &WarmStats{
		StartTime: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Warm lights
	if config.WarmLights {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := m.warmLights(ctx, config.TTL)
			mu.Lock()
			stats.LightsWarmed = count
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Errorf("lights: %w", err))
				if config.OnError != nil {
					config.OnError("lights", err)
				}
			}
			mu.Unlock()
		}()
	}

	// Warm rooms
	if config.WarmRooms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := m.warmRooms(ctx, config.TTL)
			mu.Lock()
			stats.RoomsWarmed = count
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Errorf("rooms: %w", err))
				if config.OnError != nil {
					config.OnError("rooms", err)
				}
			}
			mu.Unlock()
		}()
	}

	// Warm zones
	if config.WarmZones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := m.warmZones(ctx, config.TTL)
			mu.Lock()
			stats.ZonesWarmed = count
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Errorf("zones: %w", err))
				if config.OnError != nil {
					config.OnError("zones", err)
				}
			}
			mu.Unlock()
		}()
	}

	// Warm scenes
	if config.WarmScenes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := m.warmScenes(ctx, config.TTL)
			mu.Lock()
			stats.ScenesWarmed = count
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Errorf("scenes: %w", err))
				if config.OnError != nil {
					config.OnError("scenes", err)
				}
			}
			mu.Unlock()
		}()
	}

	// Warm grouped lights
	if config.WarmGroupedLights {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := m.warmGroupedLights(ctx, config.TTL)
			mu.Lock()
			stats.GroupedLightsWarmed = count
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Errorf("grouped_lights: %w", err))
				if config.OnError != nil {
					config.OnError("grouped_lights", err)
				}
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	stats.Duration = time.Since(stats.StartTime)
	stats.TotalWarmed = stats.LightsWarmed + stats.RoomsWarmed +
		stats.ZonesWarmed + stats.ScenesWarmed + stats.GroupedLightsWarmed

	return stats, nil
}

// WarmStats contains statistics about cache warming operations.
type WarmStats struct {
	StartTime           time.Time
	Duration            time.Duration
	LightsWarmed        int
	RoomsWarmed         int
	ZonesWarmed         int
	ScenesWarmed        int
	GroupedLightsWarmed int
	TotalWarmed         int
	Errors              []error
}

// warmLights populates the cache with all lights from the bridge.
func (m *CacheManager) warmLights(ctx context.Context, ttl time.Duration) (int, error) {
	lights, err := m.client.Lights().List(ctx)
	if err != nil {
		return 0, err
	}

	cached := NewCachedLightClient(m.backend, m.client.Lights(), ttl)
	for _, light := range lights {
		// Use Get to populate cache (which handles serialization)
		_, _ = cached.Get(ctx, light.ID)
	}

	return len(lights), nil
}

// warmRooms populates the cache with all rooms from the bridge.
func (m *CacheManager) warmRooms(ctx context.Context, ttl time.Duration) (int, error) {
	rooms, err := m.client.Rooms().List(ctx)
	if err != nil {
		return 0, err
	}

	cached := NewCachedRoomClient(m.backend, m.client.Rooms(), ttl)
	for _, room := range rooms {
		_, _ = cached.Get(ctx, room.ID)
	}

	return len(rooms), nil
}

// warmZones populates the cache with all zones from the bridge.
func (m *CacheManager) warmZones(ctx context.Context, ttl time.Duration) (int, error) {
	zones, err := m.client.Zones().List(ctx)
	if err != nil {
		return 0, err
	}

	cached := NewCachedZoneClient(m.backend, m.client.Zones(), ttl)
	for _, zone := range zones {
		_, _ = cached.Get(ctx, zone.ID)
	}

	return len(zones), nil
}

// warmScenes populates the cache with all scenes from the bridge.
func (m *CacheManager) warmScenes(ctx context.Context, ttl time.Duration) (int, error) {
	scenes, err := m.client.Scenes().List(ctx)
	if err != nil {
		return 0, err
	}

	cached := NewCachedSceneClient(m.backend, m.client.Scenes(), ttl)
	for _, scene := range scenes {
		_, _ = cached.Get(ctx, scene.ID)
	}

	return len(scenes), nil
}

// warmGroupedLights populates the cache with all grouped lights from the bridge.
func (m *CacheManager) warmGroupedLights(ctx context.Context, ttl time.Duration) (int, error) {
	groupedLights, err := m.client.GroupedLights().List(ctx)
	if err != nil {
		return 0, err
	}

	cached := NewCachedGroupedLightClient(m.backend, m.client.GroupedLights(), ttl)
	for _, gl := range groupedLights {
		_, _ = cached.Get(ctx, gl.ID)
	}

	return len(groupedLights), nil
}

// GetStats returns current cache statistics.
func (m *CacheManager) GetStats(ctx context.Context) (*Stats, error) {
	return m.backend.Stats(ctx)
}

// CountByType returns the number of cached entries by resource type.
func (m *CacheManager) CountByType(ctx context.Context) (*TypeCounts, error) {
	counts := &TypeCounts{}

	// Count lights
	if keys, err := m.backend.Keys(ctx, m.keyBuilder.AllLights()); err == nil {
		counts.Lights = len(keys)
	}

	// Count rooms
	if keys, err := m.backend.Keys(ctx, m.keyBuilder.AllRooms()); err == nil {
		counts.Rooms = len(keys)
	}

	// Count zones
	if keys, err := m.backend.Keys(ctx, m.keyBuilder.AllZones()); err == nil {
		counts.Zones = len(keys)
	}

	// Count scenes
	if keys, err := m.backend.Keys(ctx, m.keyBuilder.AllScenes()); err == nil {
		counts.Scenes = len(keys)
	}

	// Count grouped lights
	if keys, err := m.backend.Keys(ctx, m.keyBuilder.AllGroupedLights()); err == nil {
		counts.GroupedLights = len(keys)
	}

	counts.Total = counts.Lights + counts.Rooms + counts.Zones +
		counts.Scenes + counts.GroupedLights

	return counts, nil
}

// TypeCounts contains counts of cached entries by type.
type TypeCounts struct {
	Lights        int
	Rooms         int
	Zones         int
	Scenes        int
	GroupedLights int
	Total         int
}

package cache

import (
	"time"

	"github.com/rmrfslashbin/hue-sdk"
)

// CachedClient wraps an SDK client with caching for all resource types.
// It provides the same interface as hue.Client but with automatic caching.
//
// Example usage:
//
//	backend := backends.NewMemory()
//	sdkClient, _ := hue.NewClient(...)
//	cachedClient := cache.NewCachedClient(backend, sdkClient, 5*time.Minute)
//
//	// Use just like the SDK client
//	lights, err := cachedClient.Lights().List(ctx)
type CachedClient struct {
	backend   Backend
	sdkClient *hue.Client
	ttl       time.Duration

	// Cached resource clients
	lights        *CachedLightClient
	rooms         *CachedRoomClient
	zones         *CachedZoneClient
	scenes        *CachedSceneClient
	groupedLights *CachedGroupedLightClient
}

// CachedClientConfig contains configuration for the cached client.
type CachedClientConfig struct {
	// TTL is the default time-to-live for cache entries.
	// Set to 0 to never expire (rely on SSE sync).
	// Default: 0 (no expiration, rely on SSE)
	TTL time.Duration

	// EnableSync enables automatic SSE synchronization.
	// If false, you must manually sync or rely on TTL expiration.
	// Default: true
	EnableSync bool

	// SyncConfig is passed to the sync engine if EnableSync is true.
	SyncConfig *SyncConfig
}

// DefaultCachedClientConfig returns default configuration.
func DefaultCachedClientConfig() *CachedClientConfig {
	return &CachedClientConfig{
		TTL:        0, // No expiration by default
		EnableSync: true,
		SyncConfig: DefaultSyncConfig(),
	}
}

// NewCachedClient creates a new cached client that wraps an SDK client.
// If config is nil, defaults are used.
//
// Example with defaults:
//
//	cachedClient := NewCachedClient(backend, sdkClient, nil)
//
// Example with custom config:
//
//	config := &CachedClientConfig{
//	    TTL: 5 * time.Minute,
//	    EnableSync: true,
//	    SyncConfig: &SyncConfig{
//	        EnableAutoSync: true,
//	        SyncOnStart:    true,
//	    },
//	}
//	cachedClient := NewCachedClient(backend, sdkClient, config)
func NewCachedClient(backend Backend, sdkClient *hue.Client, config *CachedClientConfig) *CachedClient {
	if config == nil {
		config = DefaultCachedClientConfig()
	}

	return &CachedClient{
		backend:   backend,
		sdkClient: sdkClient,
		ttl:       config.TTL,
	}
}

// Lights returns a cached light client.
func (c *CachedClient) Lights() hue.LightClient {
	if c.lights == nil {
		c.lights = NewCachedLightClient(c.backend, c.sdkClient.Lights(), c.ttl)
	}
	return c.lights
}

// Rooms returns a cached room client.
func (c *CachedClient) Rooms() hue.RoomClient {
	if c.rooms == nil {
		c.rooms = NewCachedRoomClient(c.backend, c.sdkClient.Rooms(), c.ttl)
	}
	return c.rooms
}

// Zones returns a cached zone client.
func (c *CachedClient) Zones() hue.ZoneClient {
	if c.zones == nil {
		c.zones = NewCachedZoneClient(c.backend, c.sdkClient.Zones(), c.ttl)
	}
	return c.zones
}

// Scenes returns a cached scene client.
func (c *CachedClient) Scenes() hue.SceneClient {
	if c.scenes == nil {
		c.scenes = NewCachedSceneClient(c.backend, c.sdkClient.Scenes(), c.ttl)
	}
	return c.scenes
}

// GroupedLights returns a cached grouped light client.
func (c *CachedClient) GroupedLights() hue.GroupedLightClient {
	if c.groupedLights == nil {
		c.groupedLights = NewCachedGroupedLightClient(c.backend, c.sdkClient.GroupedLights(), c.ttl)
	}
	return c.groupedLights
}

// Backend returns the underlying cache backend.
// Useful for accessing cache statistics or performing manual operations.
func (c *CachedClient) Backend() Backend {
	return c.backend
}

// SDKClient returns the underlying SDK client.
// Useful for operations that should bypass the cache.
func (c *CachedClient) SDKClient() *hue.Client {
	return c.sdkClient
}

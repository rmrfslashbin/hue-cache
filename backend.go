package cache

import (
	"context"
	"time"
)

// Backend defines the interface for cache storage implementations.
// All methods must be thread-safe and support concurrent access.
//
// Implementations include:
//   - Memory: Fast in-memory cache with TTL support
//   - SQLite: Persistent cache with query capabilities
//   - Disk: Simple file-based cache
type Backend interface {
	// Get retrieves a value from the cache by key.
	// Returns ErrNotFound if the key doesn't exist or has expired.
	// Returns ErrExpired if the key exists but TTL has elapsed.
	Get(ctx context.Context, key string) (*Entry, error)

	// Set stores a value in the cache with the specified TTL.
	// A TTL of 0 means no expiration.
	// If the key already exists, it is overwritten.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	// Returns nil if the key doesn't exist (idempotent).
	Delete(ctx context.Context, key string) error

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error

	// Keys returns all keys matching the given pattern.
	// Pattern syntax:
	//   - "*" matches all keys
	//   - "prefix:*" matches all keys with prefix
	//   - "*:suffix" matches all keys with suffix
	//   - "exact" matches exact key
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Stats returns current cache statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Close releases any resources held by the backend.
	// The backend should not be used after calling Close.
	Close() error
}

// KeyBuilder provides helper methods for constructing cache keys.
type KeyBuilder struct{}

// NewKeyBuilder creates a new KeyBuilder.
func NewKeyBuilder() *KeyBuilder {
	return &KeyBuilder{}
}

// Light creates a cache key for a light resource.
func (kb *KeyBuilder) Light(id string) string {
	return "light:" + id
}

// Room creates a cache key for a room resource.
func (kb *KeyBuilder) Room(id string) string {
	return "room:" + id
}

// Zone creates a cache key for a zone resource.
func (kb *KeyBuilder) Zone(id string) string {
	return "zone:" + id
}

// Scene creates a cache key for a scene resource.
func (kb *KeyBuilder) Scene(id string) string {
	return "scene:" + id
}

// SmartScene creates a cache key for a smart scene resource.
func (kb *KeyBuilder) SmartScene(id string) string {
	return "smart_scene:" + id
}

// GroupedLight creates a cache key for a grouped light resource.
func (kb *KeyBuilder) GroupedLight(id string) string {
	return "grouped_light:" + id
}

// Device creates a cache key for a device resource.
func (kb *KeyBuilder) Device(id string) string {
	return "device:" + id
}

// Bridge creates a cache key for a bridge resource.
func (kb *KeyBuilder) Bridge(id string) string {
	return "bridge:" + id
}

// BridgeHome creates a cache key for a bridge home resource.
func (kb *KeyBuilder) BridgeHome(id string) string {
	return "bridge_home:" + id
}

// Resource creates a cache key for any resource type.
func (kb *KeyBuilder) Resource(resourceType, id string) string {
	return resourceType + ":" + id
}

// AllLights returns the pattern for all light keys.
func (kb *KeyBuilder) AllLights() string {
	return "light:*"
}

// AllRooms returns the pattern for all room keys.
func (kb *KeyBuilder) AllRooms() string {
	return "room:*"
}

// AllZones returns the pattern for all zone keys.
func (kb *KeyBuilder) AllZones() string {
	return "zone:*"
}

// AllScenes returns the pattern for all scene keys.
func (kb *KeyBuilder) AllScenes() string {
	return "scene:*"
}

// AllResources returns the pattern for all resource types.
func (kb *KeyBuilder) AllResources(resourceType string) string {
	return resourceType + ":*"
}

// All returns the pattern for all keys.
func (kb *KeyBuilder) All() string {
	return "*"
}

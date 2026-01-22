# Hue Cache

Caching layer for the Hue SDK with automatic SSE synchronization and pluggable storage backends.

## Features

- **Pluggable Backends**: In-memory and file-based persistence
- **Automatic SSE Sync**: Cache stays synchronized with bridge events
- **Transparent Caching**: Same interface as SDK clients
- **Disk Persistence**: File backend with periodic auto-save for fast startup
- **Cache Management**: Bulk operations, cache warming, pattern-based clearing
- **TTL Support**: Automatic expiration with background cleanup
- **Statistics**: Real-time cache hit/miss metrics and counts by type
- **Thread-Safe**: Concurrent operations fully supported

## Installation

```bash
go get github.com/rmrfslashbin/hue-cache
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    cache "github.com/rmrfslashbin/hue-cache"
    "github.com/rmrfslashbin/hue-cache/backends"
    "github.com/rmrfslashbin/hue-sdk"
)

func main() {
    // Create SDK client
    sdkClient, _ := hue.NewClient(
        hue.WithBridgeIP("192.168.1.100"),
        hue.WithAppKey("your-app-key"),
    )

    // Create cache backend
    backend := backends.NewMemory(backends.DefaultMemoryConfig())
    defer backend.Close()

    // Create sync engine for automatic updates
    syncEngine := cache.NewSyncEngine(backend, sdkClient, nil)
    syncEngine.Start()
    defer syncEngine.Stop()

    // Create cached client (same interface as SDK!)
    cachedClient := cache.NewCachedClient(backend, sdkClient, nil)

    ctx := context.Background()

    // Use exactly like SDK client - but with caching!
    lights, _ := cachedClient.Lights().List(ctx)
    light, _ := cachedClient.Lights().Get(ctx, lights[0].ID)

    // Updates invalidate cache, SSE event repopulates
    cachedClient.Lights().Update(ctx, light.ID, hue.LightUpdate{
        On: &hue.OnState{On: true},
    })

    // Check cache stats
    stats, _ := backend.Stats(ctx)
    fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate())
}
```

## Architecture

```
Application
    ↓
Cached Clients (same interface as SDK)
    ↓
SSE Sync Engine (automatic updates)
    ↓
Cache Backend Interface
    ↓
├─ Memory Backend (fast, volatile)
├─ SQLite Backend (persistent, queryable)
└─ Disk Backend (simple file storage)
```

## Backend Interface

All backends implement the `Backend` interface:

```go
type Backend interface {
    Get(ctx context.Context, key string) (*Entry, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Keys(ctx context.Context, pattern string) ([]string, error)
    Stats(ctx context.Context) (*Stats, error)
    Close() error
}
```

## Cache Keys

Use the `KeyBuilder` for consistent key formatting:

```go
kb := cache.NewKeyBuilder()

// Individual resources
lightKey := kb.Light("abc-123")           // "light:abc-123"
roomKey := kb.Room("def-456")             // "room:def-456"
sceneKey := kb.Scene("ghi-789")           // "scene:ghi-789"

// Patterns for bulk operations
allLights := kb.AllLights()               // "light:*"
allRooms := kb.AllRooms()                 // "room:*"
```

## Statistics

Monitor cache performance:

```go
stats, err := backend.Stats(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate())
fmt.Printf("Entries: %d\n", stats.Entries)
fmt.Printf("Size: %d bytes\n", stats.Size)
```

## File Backend (Persistence)

Use file backend for faster startup times:

```go
// Create file backend with auto-save
config := &backends.FileConfig{
    FilePath:         "/var/cache/hue/cache.gob",
    AutoSaveInterval: 5 * time.Minute,  // Auto-save every 5 min
    LoadOnStart:      true,              // Load cache from disk
}

backend, _ := backends.NewFile(config)
defer backend.Close()  // Saves on shutdown!

// Cache loads from disk on startup = instant first access!
// Auto-saves every 5 minutes while running
```

**Benefits**:
- **Fast startup**: Cache pre-loaded from disk (~10ms vs 150ms+ bridge query)
- **Persistence**: Survives restarts
- **Automatic**: Periodic flush requires no manual intervention

See: [examples/persistent_cache](https://github.com/rmrfslashbin/hue-cache/tree/main/examples/persistent_cache)

## Cache Management

Bulk operations and cache warming:

```go
manager := cache.NewCacheManager(backend, sdkClient)

// Warm cache on startup (pre-populate all resources)
stats, _ := manager.WarmCache(ctx, cache.DefaultWarmConfig())
fmt.Printf("Warmed %d entries in %v\n", stats.TotalWarmed, stats.Duration)

// Clear by pattern
manager.ClearLights(ctx)     // Clear all lights
manager.ClearRooms(ctx)      // Clear all rooms
manager.ClearPattern(ctx, "light:*")  // Custom pattern

// Count by type
counts, _ := manager.CountByType(ctx)
fmt.Printf("Lights: %d, Rooms: %d\n", counts.Lights, counts.Rooms)
```

## Development Status

**Phases 1-6 Complete** - Production ready with persistence!

### Phase 1: Foundation ✅
- ✅ Backend interface
- ✅ Entry types with TTL
- ✅ Statistics collection
- ✅ Error handling
- ✅ 100% test coverage

### Phase 2: In-Memory Backend ✅
- ✅ sync.Map-based storage
- ✅ TTL expiration with background cleanup
- ✅ Memory limits (MaxMemory, MaxEntries)
- ✅ Three eviction policies (LRU, LFU, FIFO)
- ✅ ~99ns Get, ~142ns Set performance
- ✅ 93.9% test coverage

### Phase 3: SSE Sync Engine ✅
- ✅ Automatic cache updates from bridge events
- ✅ Add/Update/Delete event handling
- ✅ Full sync on startup (optional)
- ✅ Statistics tracking with latency
- ✅ Sub-millisecond event processing
- ✅ Comprehensive tests

### Phase 4: Cached Client Wrappers ✅
- ✅ CachedClient with same interface as SDK
- ✅ Read-through caching (Get/List methods)
- ✅ Write-through caching (Update/Create/Delete)
- ✅ Cached wrappers for Lights, Rooms, Zones, Scenes, GroupedLights
- ✅ Automatic cache invalidation on updates
- ✅ SSE-based cache refresh
- ✅ Comprehensive tests

### Phase 5: Cache Management & Stats ✅
- ✅ CacheManager with bulk operations
- ✅ Cache warming (pre-populate on startup)
- ✅ Pattern-based clearing (lights, rooms, zones, etc)
- ✅ Statistics by resource type
- ✅ Concurrent operations support
- ✅ 8/8 tests passing

### Phase 6: File Backend (Persistence) ✅
- ✅ File-based backend with auto-save
- ✅ GOB encoding for efficiency
- ✅ Periodic flush to disk (configurable)
- ✅ Auto-load on startup
- ✅ Atomic writes (no corruption)
- ✅ TTL-aware loading
- ✅ 9/9 tests passing
- ✅ 87.7% test coverage

**In Progress**:
- Phase 7: Integration & Polish

**Optional (Future)**:
- SQLite backend for advanced persistence

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Related Projects

- [hue-sdk](https://github.com/rmrfslashbin/hue-sdk) - Base SDK
- [hue-api](https://github.com/rmrfslashbin/hue-api) - Parent project
- [hue-mcp](https://github.com/rmrfslashbin/hue-mcp) - MCP server (planned)

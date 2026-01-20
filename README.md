# Hue Cache

Caching layer for the Hue SDK with automatic SSE synchronization and pluggable storage backends.

## Features

- **Pluggable Backends**: In-memory, SQLite, or disk storage
- **Automatic SSE Sync**: Cache stays synchronized with bridge events
- **Transparent Caching**: Same interface as SDK clients
- **TTL Support**: Automatic expiration with background cleanup
- **Statistics**: Real-time cache hit/miss metrics
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
)

func main() {
    // Create in-memory backend
    backend := backends.NewMemory()
    defer backend.Close()

    ctx := context.Background()

    // Store value
    err := backend.Set(ctx, "light:123", []byte(`{"on": true}`), 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve value
    entry, err := backend.Get(ctx, "light:123")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Value: %s\n", entry.Value)
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

## Development Status

**Phase 1 Complete**: Foundation
- ✅ Backend interface
- ✅ Entry types with TTL
- ✅ Statistics collection
- ✅ Error handling
- ✅ Comprehensive tests

**In Progress**: Phase 2 - In-Memory Backend

**Planned**:
- Phase 3: SSE Sync Engine
- Phase 4: Cached Clients
- Phase 5: Management & Stats
- Phase 6: SQLite Backend (optional)
- Phase 7: Integration & Polish

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

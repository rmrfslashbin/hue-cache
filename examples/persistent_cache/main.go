package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cache "github.com/rmrfslashbin/hue-cache"
	"github.com/rmrfslashbin/hue-cache/backends"
	"github.com/rmrfslashbin/hue-sdk"
)

func main() {
	// Create SDK client
	sdkClient, err := hue.NewClient(
		hue.WithBridgeIP("192.168.1.100"),
		hue.WithAppKey("your-app-key-here"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create file backend with persistence
	// Cache will load from disk on startup and auto-save every 5 minutes
	fileConfig := &backends.FileConfig{
		FilePath:         "/var/cache/hue/bridge-cache.gob",
		AutoSaveInterval: 5 * time.Minute,
		LoadOnStart:      true, // Load existing cache from disk
		MemoryConfig:     backends.DefaultMemoryConfig(),
	}

	backend, err := backends.NewFile(fileConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer backend.Close() // Important: saves final state on shutdown

	// Create cache manager for bulk operations
	manager := cache.NewCacheManager(backend, sdkClient)

	ctx := context.Background()

	// Warm cache on startup (optional - useful for first run)
	fmt.Println("Warming cache...")
	warmConfig := cache.DefaultWarmConfig()
	warmConfig.OnError = func(resourceType string, err error) {
		log.Printf("Warning: Failed to warm %s: %v", resourceType, err)
	}

	warmStats, err := manager.WarmCache(ctx, warmConfig)
	if err != nil {
		log.Printf("Cache warming had errors: %v", err)
	}

	fmt.Printf("Cache warmed in %v:\n", warmStats.Duration)
	fmt.Printf("  Lights: %d\n", warmStats.LightsWarmed)
	fmt.Printf("  Rooms: %d\n", warmStats.RoomsWarmed)
	fmt.Printf("  Zones: %d\n", warmStats.ZonesWarmed)
	fmt.Printf("  Scenes: %d\n", warmStats.ScenesWarmed)
	fmt.Printf("  GroupedLights: %d\n", warmStats.GroupedLightsWarmed)
	fmt.Printf("  Total: %d entries\n", warmStats.TotalWarmed)

	// Start SSE sync engine for automatic updates
	syncEngine := cache.NewSyncEngine(backend, sdkClient, nil)
	if err := syncEngine.Start(); err != nil {
		log.Fatal(err)
	}
	defer syncEngine.Stop()

	// Create cached client
	cachedClient := cache.NewCachedClient(backend, sdkClient, &cache.CachedClientConfig{
		TTL:        0, // No expiration, rely on SSE sync
		EnableSync: true,
	})

	// Use cached client - first access after warmup is instant!
	fmt.Println("\nGetting lights from cache...")
	start := time.Now()
	lights, err := cachedClient.Lights().List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved %d lights in %v (from cache!)\n", len(lights), time.Since(start))

	// Show cache statistics
	stats, _ := backend.Stats(ctx)
	fmt.Printf("\nCache Statistics:\n")
	fmt.Printf("  Entries: %d\n", stats.Entries)
	fmt.Printf("  Hits: %d\n", stats.Hits)
	fmt.Printf("  Misses: %d\n", stats.Misses)
	fmt.Printf("  Hit Rate: %.2f%%\n", stats.HitRate())

	// Show counts by type
	counts, _ := manager.CountByType(ctx)
	fmt.Printf("\nCached Resources:\n")
	fmt.Printf("  Lights: %d\n", counts.Lights)
	fmt.Printf("  Rooms: %d\n", counts.Rooms)
	fmt.Printf("  Zones: %d\n", counts.Zones)
	fmt.Printf("  Scenes: %d\n", counts.Scenes)
	fmt.Printf("  GroupedLights: %d\n", counts.GroupedLights)
	fmt.Printf("  Total: %d\n", counts.Total)

	// Manually save cache (optional - auto-save will do this periodically)
	fmt.Println("\nManually saving cache to disk...")
	if err := backend.Save(); err != nil {
		log.Printf("Failed to save cache: %v", err)
	} else {
		fmt.Println("Cache saved successfully!")
	}

	// Cache will auto-save every 5 minutes while running
	// On shutdown (defer backend.Close()), final state is saved

	fmt.Println("\nCache is now running with:")
	fmt.Println("  - Automatic SSE synchronization")
	fmt.Println("  - Periodic saves to disk (every 5 minutes)")
	fmt.Println("  - Fast startup on next run (loads from disk)")

	// Example: Clear specific resource types
	// manager.ClearLights(ctx)  // Clear all lights
	// manager.ClearRooms(ctx)   // Clear all rooms
	// manager.ClearAll(ctx)     // Clear everything
}

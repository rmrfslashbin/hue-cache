package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cache "github.com/rmrfslashbin/hue-cache"
	"github.com/rmrfslashbin/hue-cache/backends"
	"github.com/rmrfslashbin/hue-sdk"
	"github.com/rmrfslashbin/hue-sdk/resources"
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

	// Create cache backend
	backend := backends.NewMemory(backends.DefaultMemoryConfig())
	defer backend.Close()

	// Create sync engine for automatic cache updates
	syncEngine := cache.NewSyncEngine(backend, sdkClient, nil)
	if err := syncEngine.Start(); err != nil {
		log.Fatal(err)
	}
	defer syncEngine.Stop()

	// Create cached client with same interface as SDK client
	config := &cache.CachedClientConfig{
		TTL:        0, // No expiration, rely on SSE sync
		EnableSync: true,
		SyncConfig: cache.DefaultSyncConfig(),
	}
	cachedClient := cache.NewCachedClient(backend, sdkClient, config)

	ctx := context.Background()

	// Use cached client exactly like SDK client
	// First call hits SDK and populates cache
	fmt.Println("Getting lights (first call - cache miss)...")
	lights, err := cachedClient.Lights().List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d lights\n", len(lights))

	// Second call hits cache (much faster!)
	fmt.Println("\nGetting lights (second call - cache hit)...")
	start := time.Now()
	lights, err = cachedClient.Lights().List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	duration := time.Since(start)
	fmt.Printf("Found %d lights in %v (from cache!)\n", len(lights), duration)

	// Get a specific light
	if len(lights) > 0 {
		lightID := lights[0].ID
		fmt.Printf("\nGetting light %s...\n", lightID)

		light, err := cachedClient.Lights().Get(ctx, lightID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Light: %s, On: %v\n", light.Metadata.Name, light.On.On)
	}

	// Update a light (write-through - updates SDK and invalidates cache)
	// SSE event will repopulate the cache automatically
	if len(lights) > 0 {
		lightID := lights[0].ID
		fmt.Printf("\nUpdating light %s...\n", lightID)

		err := cachedClient.Lights().Update(ctx, lightID, resources.LightUpdate{
			On: &resources.OnState{On: true},
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Light updated! (Cache will be refreshed via SSE)")
	}

	// Check cache statistics
	stats, err := backend.Stats(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nCache Statistics:\n")
	fmt.Printf("  Entries: %d\n", stats.Entries)
	fmt.Printf("  Hits: %d\n", stats.Hits)
	fmt.Printf("  Misses: %d\n", stats.Misses)
	fmt.Printf("  Hit Rate: %.2f%%\n", stats.HitRate())

	// Check sync statistics
	syncStats := syncEngine.Stats()
	fmt.Printf("\nSync Statistics:\n")
	fmt.Printf("  Events Processed: %d\n", syncStats.EventsProcessed)
	fmt.Printf("  Add Events: %d\n", syncStats.AddEvents)
	fmt.Printf("  Update Events: %d\n", syncStats.UpdateEvents)
	fmt.Printf("  Delete Events: %d\n", syncStats.DeleteEvents)
	fmt.Printf("  Avg Latency: %v\n", syncStats.AvgLatency)
}

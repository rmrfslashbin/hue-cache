package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rmrfslashbin/hue-sdk"
	"github.com/rmrfslashbin/hue-sdk/resources"
)

// SyncEngine automatically synchronizes a cache backend with SSE events
// from a Hue Bridge. It listens to add/update/delete events and updates
// the cache accordingly, keeping it in sync with the bridge state.
type SyncEngine struct {
	// backend is the cache to synchronize
	backend Backend

	// client is the Hue SDK client
	client *hue.Client

	// keyBuilder helps construct cache keys
	keyBuilder *KeyBuilder

	// stats tracks sync statistics
	stats *SyncStats

	// config holds configuration
	config *SyncConfig

	// ctx is the context for the sync loop
	ctx    context.Context
	cancel context.CancelFunc

	// done signals when the sync loop has stopped
	done chan struct{}

	// mu protects the running state
	mu      sync.RWMutex
	running bool
}

// SyncConfig contains configuration for the sync engine.
type SyncConfig struct {
	// EnableAutoSync enables automatic cache updates from events.
	// Default: true
	EnableAutoSync bool

	// SyncOnStart performs a full sync when the engine starts.
	// Default: false
	SyncOnStart bool

	// ErrorHandler is called when sync errors occur.
	// If nil, errors are silently ignored.
	ErrorHandler func(error)

	// EventHandler is called for each event (for debugging/logging).
	// If nil, events are not logged.
	EventHandler func(*resources.Event)
}

// DefaultSyncConfig returns default sync configuration.
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		EnableAutoSync: true,
		SyncOnStart:    false,
		ErrorHandler:   nil,
		EventHandler:   nil,
	}
}

// SyncStats contains synchronization statistics.
type SyncStats struct {
	mu sync.RWMutex

	// EventsProcessed is the total number of events processed.
	EventsProcessed int64

	// AddEvents is the number of "add" events.
	AddEvents int64

	// UpdateEvents is the number of "update" events.
	UpdateEvents int64

	// DeleteEvents is the number of "delete" events.
	DeleteEvents int64

	// SyncErrors is the number of sync errors encountered.
	SyncErrors int64

	// LastEventTime is when the last event was processed.
	LastEventTime time.Time

	// LastError is the most recent error.
	LastError string

	// LastErrorTime is when the last error occurred.
	LastErrorTime time.Time

	// Latency is the average event processing latency.
	AvgLatency time.Duration
}

// Clone creates a copy of the stats.
func (s *SyncStats) Clone() *SyncStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &SyncStats{
		EventsProcessed: s.EventsProcessed,
		AddEvents:       s.AddEvents,
		UpdateEvents:    s.UpdateEvents,
		DeleteEvents:    s.DeleteEvents,
		SyncErrors:      s.SyncErrors,
		LastEventTime:   s.LastEventTime,
		LastError:       s.LastError,
		LastErrorTime:   s.LastErrorTime,
		AvgLatency:      s.AvgLatency,
	}
}

// NewSyncEngine creates a new sync engine.
func NewSyncEngine(backend Backend, client *hue.Client, config ...*SyncConfig) *SyncEngine {
	cfg := DefaultSyncConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SyncEngine{
		backend:    backend,
		client:     client,
		keyBuilder: NewKeyBuilder(),
		stats:      &SyncStats{},
		config:     cfg,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
}

// Start begins synchronizing the cache with SSE events.
func (s *SyncEngine) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("sync engine already running")
	}
	s.running = true
	s.mu.Unlock()

	// Perform initial sync if configured
	if s.config.SyncOnStart {
		if err := s.fullSync(); err != nil {
			s.handleError(fmt.Errorf("initial sync failed: %w", err))
		}
	}

	// Start event subscription
	if s.config.EnableAutoSync {
		go s.syncLoop()
	}

	return nil
}

// Stop stops the sync engine and waits for cleanup.
func (s *SyncEngine) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	// Cancel context and wait for sync loop to finish
	s.cancel()
	<-s.done

	return nil
}

// Stats returns current synchronization statistics.
func (s *SyncEngine) Stats() *SyncStats {
	return s.stats.Clone()
}

// syncLoop subscribes to events and processes them.
func (s *SyncEngine) syncLoop() {
	defer close(s.done)

	// Subscribe to events
	events, err := s.client.Events().Subscribe(s.ctx)
	if err != nil {
		s.handleError(fmt.Errorf("failed to subscribe to events: %w", err))
		return
	}

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Event channel closed
				return
			}

			s.processEvent(&event)

		case <-s.ctx.Done():
			return
		}
	}
}

// processEvent processes a single SSE event.
func (s *SyncEngine) processEvent(event *resources.Event) {
	start := time.Now()

	// Call event handler if configured
	if s.config.EventHandler != nil {
		s.config.EventHandler(event)
	}

	// Update statistics
	s.stats.mu.Lock()
	s.stats.EventsProcessed++
	s.stats.LastEventTime = time.Now()
	s.stats.mu.Unlock()

	// Process each data element
	for _, data := range event.Data {
		if err := s.processEventData(event.Type, &data); err != nil {
			s.handleError(fmt.Errorf("failed to process event data: %w", err))
		}
	}

	// Update latency
	latency := time.Since(start)
	s.stats.mu.Lock()
	if s.stats.AvgLatency == 0 {
		s.stats.AvgLatency = latency
	} else {
		// Exponential moving average
		s.stats.AvgLatency = (s.stats.AvgLatency*9 + latency) / 10
	}
	s.stats.mu.Unlock()
}

// processEventData processes a single event data element.
func (s *SyncEngine) processEventData(eventType string, data *resources.EventData) error {
	ctx := context.Background()

	// Build cache key
	key := s.keyBuilder.Resource(data.Type, data.ID)

	switch eventType {
	case resources.EventTypeAdd:
		s.stats.mu.Lock()
		s.stats.AddEvents++
		s.stats.mu.Unlock()
		return s.handleAdd(ctx, key, data)

	case resources.EventTypeUpdate:
		s.stats.mu.Lock()
		s.stats.UpdateEvents++
		s.stats.mu.Unlock()
		return s.handleUpdate(ctx, key, data)

	case resources.EventTypeDelete:
		s.stats.mu.Lock()
		s.stats.DeleteEvents++
		s.stats.mu.Unlock()
		return s.handleDelete(ctx, key)

	default:
		return fmt.Errorf("unknown event type: %s", eventType)
	}
}

// handleAdd handles an "add" event by caching the new resource.
func (s *SyncEngine) handleAdd(ctx context.Context, key string, data *resources.EventData) error {
	// Marshal the event data to JSON
	jsonData, err := json.Marshal(data.RawData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Store in cache with no TTL (stays until deleted or updated)
	return s.backend.Set(ctx, key, jsonData, 0)
}

// handleUpdate handles an "update" event by updating the cached resource.
func (s *SyncEngine) handleUpdate(ctx context.Context, key string, data *resources.EventData) error {
	// Marshal the event data to JSON
	jsonData, err := json.Marshal(data.RawData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Update in cache with no TTL
	return s.backend.Set(ctx, key, jsonData, 0)
}

// handleDelete handles a "delete" event by removing the resource from cache.
func (s *SyncEngine) handleDelete(ctx context.Context, key string) error {
	return s.backend.Delete(ctx, key)
}

// handleError handles sync errors according to configuration.
func (s *SyncEngine) handleError(err error) {
	s.stats.mu.Lock()
	s.stats.SyncErrors++
	s.stats.LastError = err.Error()
	s.stats.LastErrorTime = time.Now()
	s.stats.mu.Unlock()

	if s.config.ErrorHandler != nil {
		s.config.ErrorHandler(err)
	}
}

// fullSync performs a full synchronization of all resources.
// This is used for the initial sync when SyncOnStart is true.
func (s *SyncEngine) fullSync() error {
	ctx := context.Background()

	// Sync lights
	if err := s.syncLights(ctx); err != nil {
		return fmt.Errorf("failed to sync lights: %w", err)
	}

	// Sync rooms
	if err := s.syncRooms(ctx); err != nil {
		return fmt.Errorf("failed to sync rooms: %w", err)
	}

	// Sync zones
	if err := s.syncZones(ctx); err != nil {
		return fmt.Errorf("failed to sync zones: %w", err)
	}

	// Sync scenes
	if err := s.syncScenes(ctx); err != nil {
		return fmt.Errorf("failed to sync scenes: %w", err)
	}

	// Sync grouped lights
	if err := s.syncGroupedLights(ctx); err != nil {
		return fmt.Errorf("failed to sync grouped lights: %w", err)
	}

	return nil
}

// syncLights syncs all lights to the cache.
func (s *SyncEngine) syncLights(ctx context.Context) error {
	lights, err := s.client.Lights().List(ctx)
	if err != nil {
		return err
	}

	for _, light := range lights {
		key := s.keyBuilder.Light(light.ID)
		data, err := json.Marshal(light)
		if err != nil {
			return err
		}
		if err := s.backend.Set(ctx, key, data, 0); err != nil {
			return err
		}
	}

	return nil
}

// syncRooms syncs all rooms to the cache.
func (s *SyncEngine) syncRooms(ctx context.Context) error {
	rooms, err := s.client.Rooms().List(ctx)
	if err != nil {
		return err
	}

	for _, room := range rooms {
		key := s.keyBuilder.Room(room.ID)
		data, err := json.Marshal(room)
		if err != nil {
			return err
		}
		if err := s.backend.Set(ctx, key, data, 0); err != nil {
			return err
		}
	}

	return nil
}

// syncZones syncs all zones to the cache.
func (s *SyncEngine) syncZones(ctx context.Context) error {
	zones, err := s.client.Zones().List(ctx)
	if err != nil {
		return err
	}

	for _, zone := range zones {
		key := s.keyBuilder.Zone(zone.ID)
		data, err := json.Marshal(zone)
		if err != nil {
			return err
		}
		if err := s.backend.Set(ctx, key, data, 0); err != nil {
			return err
		}
	}

	return nil
}

// syncScenes syncs all scenes to the cache.
func (s *SyncEngine) syncScenes(ctx context.Context) error {
	scenes, err := s.client.Scenes().List(ctx)
	if err != nil {
		return err
	}

	for _, scene := range scenes {
		key := s.keyBuilder.Scene(scene.ID)
		data, err := json.Marshal(scene)
		if err != nil {
			return err
		}
		if err := s.backend.Set(ctx, key, data, 0); err != nil {
			return err
		}
	}

	return nil
}

// syncGroupedLights syncs all grouped lights to the cache.
func (s *SyncEngine) syncGroupedLights(ctx context.Context) error {
	groupedLights, err := s.client.GroupedLights().List(ctx)
	if err != nil {
		return err
	}

	for _, gl := range groupedLights {
		key := s.keyBuilder.GroupedLight(gl.ID)
		data, err := json.Marshal(gl)
		if err != nil {
			return err
		}
		if err := s.backend.Set(ctx, key, data, 0); err != nil {
			return err
		}
	}

	return nil
}

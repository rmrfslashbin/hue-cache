package cache

import (
	"sync/atomic"
	"time"
)

// Stats contains cache statistics and metrics.
type Stats struct {
	// Hits is the number of successful cache retrievals.
	Hits int64

	// Misses is the number of cache misses.
	Misses int64

	// Evictions is the number of entries evicted (TTL or memory pressure).
	Evictions int64

	// Entries is the current number of entries in the cache.
	Entries int64

	// Size is the total size of cached data in bytes.
	Size int64

	// Errors is the number of cache errors encountered.
	Errors int64

	// LastError is the most recent error message.
	LastError string

	// LastErrorTime is when the last error occurred.
	LastErrorTime time.Time
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// MissRate returns the cache miss rate as a percentage (0-100).
func (s *Stats) MissRate() float64 {
	return 100 - s.HitRate()
}

// Clone creates a copy of the stats.
func (s *Stats) Clone() *Stats {
	return &Stats{
		Hits:          s.Hits,
		Misses:        s.Misses,
		Evictions:     s.Evictions,
		Entries:       s.Entries,
		Size:          s.Size,
		Errors:        s.Errors,
		LastError:     s.LastError,
		LastErrorTime: s.LastErrorTime,
	}
}

// StatsCollector provides thread-safe statistics collection.
type StatsCollector struct {
	hits          atomic.Int64
	misses        atomic.Int64
	evictions     atomic.Int64
	entries       atomic.Int64
	size          atomic.Int64
	errors        atomic.Int64
	lastError     atomic.Value // string
	lastErrorTime atomic.Value // time.Time
}

// NewStatsCollector creates a new statistics collector.
func NewStatsCollector() *StatsCollector {
	sc := &StatsCollector{}
	sc.lastError.Store("")
	sc.lastErrorTime.Store(time.Time{})
	return sc
}

// RecordHit increments the hit counter.
func (sc *StatsCollector) RecordHit() {
	sc.hits.Add(1)
}

// RecordMiss increments the miss counter.
func (sc *StatsCollector) RecordMiss() {
	sc.misses.Add(1)
}

// RecordEviction increments the eviction counter.
func (sc *StatsCollector) RecordEviction() {
	sc.evictions.Add(1)
}

// RecordError increments the error counter and stores the error.
func (sc *StatsCollector) RecordError(err error) {
	sc.errors.Add(1)
	sc.lastError.Store(err.Error())
	sc.lastErrorTime.Store(time.Now())
}

// SetEntries sets the current entry count.
func (sc *StatsCollector) SetEntries(count int64) {
	sc.entries.Store(count)
}

// SetSize sets the current total size.
func (sc *StatsCollector) SetSize(size int64) {
	sc.size.Store(size)
}

// AddSize adds to the total size.
func (sc *StatsCollector) AddSize(delta int64) {
	sc.size.Add(delta)
}

// Stats returns the current statistics.
func (sc *StatsCollector) Stats() *Stats {
	lastErr := sc.lastError.Load().(string)
	lastErrTime := sc.lastErrorTime.Load().(time.Time)

	return &Stats{
		Hits:          sc.hits.Load(),
		Misses:        sc.misses.Load(),
		Evictions:     sc.evictions.Load(),
		Entries:       sc.entries.Load(),
		Size:          sc.size.Load(),
		Errors:        sc.errors.Load(),
		LastError:     lastErr,
		LastErrorTime: lastErrTime,
	}
}

// Reset resets all statistics to zero.
func (sc *StatsCollector) Reset() {
	sc.hits.Store(0)
	sc.misses.Store(0)
	sc.evictions.Store(0)
	sc.entries.Store(0)
	sc.size.Store(0)
	sc.errors.Store(0)
	sc.lastError.Store("")
	sc.lastErrorTime.Store(time.Time{})
}

package cache

import (
	"sync"
	"testing"
	"time"
)

func TestStats_HitRate(t *testing.T) {
	tests := []struct {
		name   string
		stats  *Stats
		want   float64
	}{
		{
			name:  "no requests",
			stats: &Stats{Hits: 0, Misses: 0},
			want:  0,
		},
		{
			name:  "all hits",
			stats: &Stats{Hits: 100, Misses: 0},
			want:  100,
		},
		{
			name:  "all misses",
			stats: &Stats{Hits: 0, Misses: 100},
			want:  0,
		},
		{
			name:  "50% hit rate",
			stats: &Stats{Hits: 50, Misses: 50},
			want:  50,
		},
		{
			name:  "90% hit rate",
			stats: &Stats{Hits: 90, Misses: 10},
			want:  90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.HitRate()
			if got != tt.want {
				t.Errorf("HitRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStats_MissRate(t *testing.T) {
	stats := &Stats{Hits: 90, Misses: 10}
	got := stats.MissRate()
	want := 10.0

	if got != want {
		t.Errorf("MissRate() = %v, want %v", got, want)
	}
}

func TestStats_Clone(t *testing.T) {
	original := &Stats{
		Hits:      100,
		Misses:    20,
		Evictions: 5,
		Entries:   50,
		Size:      1024,
		Errors:    3,
		LastError: "test error",
	}

	clone := original.Clone()

	// Check values are equal
	if clone.Hits != original.Hits {
		t.Errorf("Clone Hits = %v, want %v", clone.Hits, original.Hits)
	}
	if clone.Misses != original.Misses {
		t.Errorf("Clone Misses = %v, want %v", clone.Misses, original.Misses)
	}
	if clone.LastError != original.LastError {
		t.Errorf("Clone LastError = %v, want %v", clone.LastError, original.LastError)
	}

	// Check that modifying clone doesn't affect original
	clone.Hits = 999
	if original.Hits == 999 {
		t.Error("Modifying clone affected original")
	}
}

func TestStatsCollector_RecordHit(t *testing.T) {
	sc := NewStatsCollector()

	for i := 0; i < 100; i++ {
		sc.RecordHit()
	}

	stats := sc.Stats()
	if stats.Hits != 100 {
		t.Errorf("Hits = %v, want 100", stats.Hits)
	}
}

func TestStatsCollector_RecordMiss(t *testing.T) {
	sc := NewStatsCollector()

	for i := 0; i < 50; i++ {
		sc.RecordMiss()
	}

	stats := sc.Stats()
	if stats.Misses != 50 {
		t.Errorf("Misses = %v, want 50", stats.Misses)
	}
}

func TestStatsCollector_RecordEviction(t *testing.T) {
	sc := NewStatsCollector()

	for i := 0; i < 10; i++ {
		sc.RecordEviction()
	}

	stats := sc.Stats()
	if stats.Evictions != 10 {
		t.Errorf("Evictions = %v, want 10", stats.Evictions)
	}
}

func TestStatsCollector_RecordError(t *testing.T) {
	sc := NewStatsCollector()

	err1 := ErrNotFound
	err2 := ErrExpired

	sc.RecordError(err1)
	stats := sc.Stats()

	if stats.Errors != 1 {
		t.Errorf("Errors = %v, want 1", stats.Errors)
	}
	if stats.LastError != err1.Error() {
		t.Errorf("LastError = %v, want %v", stats.LastError, err1.Error())
	}
	if stats.LastErrorTime.IsZero() {
		t.Error("LastErrorTime should not be zero")
	}

	time.Sleep(10 * time.Millisecond)
	firstErrorTime := stats.LastErrorTime

	sc.RecordError(err2)
	stats = sc.Stats()

	if stats.Errors != 2 {
		t.Errorf("Errors = %v, want 2", stats.Errors)
	}
	if stats.LastError != err2.Error() {
		t.Errorf("LastError = %v, want %v", stats.LastError, err2.Error())
	}
	if !stats.LastErrorTime.After(firstErrorTime) {
		t.Error("LastErrorTime should be updated to latest error")
	}
}

func TestStatsCollector_SetEntries(t *testing.T) {
	sc := NewStatsCollector()

	sc.SetEntries(42)
	stats := sc.Stats()

	if stats.Entries != 42 {
		t.Errorf("Entries = %v, want 42", stats.Entries)
	}
}

func TestStatsCollector_SetSize(t *testing.T) {
	sc := NewStatsCollector()

	sc.SetSize(1024)
	stats := sc.Stats()

	if stats.Size != 1024 {
		t.Errorf("Size = %v, want 1024", stats.Size)
	}
}

func TestStatsCollector_AddSize(t *testing.T) {
	sc := NewStatsCollector()

	sc.SetSize(100)
	sc.AddSize(50)
	sc.AddSize(25)

	stats := sc.Stats()
	if stats.Size != 175 {
		t.Errorf("Size = %v, want 175", stats.Size)
	}

	sc.AddSize(-75)
	stats = sc.Stats()
	if stats.Size != 100 {
		t.Errorf("Size = %v, want 100 after negative delta", stats.Size)
	}
}

func TestStatsCollector_Reset(t *testing.T) {
	sc := NewStatsCollector()

	// Populate with data
	sc.RecordHit()
	sc.RecordMiss()
	sc.RecordEviction()
	sc.RecordError(ErrNotFound)
	sc.SetEntries(10)
	sc.SetSize(1024)

	// Reset
	sc.Reset()

	stats := sc.Stats()
	if stats.Hits != 0 {
		t.Errorf("Hits after reset = %v, want 0", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Misses after reset = %v, want 0", stats.Misses)
	}
	if stats.Evictions != 0 {
		t.Errorf("Evictions after reset = %v, want 0", stats.Evictions)
	}
	if stats.Entries != 0 {
		t.Errorf("Entries after reset = %v, want 0", stats.Entries)
	}
	if stats.Size != 0 {
		t.Errorf("Size after reset = %v, want 0", stats.Size)
	}
	if stats.Errors != 0 {
		t.Errorf("Errors after reset = %v, want 0", stats.Errors)
	}
	if stats.LastError != "" {
		t.Errorf("LastError after reset = %v, want empty", stats.LastError)
	}
	if !stats.LastErrorTime.IsZero() {
		t.Error("LastErrorTime after reset should be zero")
	}
}

func TestStatsCollector_Concurrency(t *testing.T) {
	sc := NewStatsCollector()

	const goroutines = 10
	const operations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // hits, misses, evictions

	// Concurrent hits
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				sc.RecordHit()
			}
		}()
	}

	// Concurrent misses
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				sc.RecordMiss()
			}
		}()
	}

	// Concurrent evictions
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				sc.RecordEviction()
			}
		}()
	}

	wg.Wait()

	stats := sc.Stats()
	expectedCount := int64(goroutines * operations)

	if stats.Hits != expectedCount {
		t.Errorf("Hits = %v, want %v", stats.Hits, expectedCount)
	}
	if stats.Misses != expectedCount {
		t.Errorf("Misses = %v, want %v", stats.Misses, expectedCount)
	}
	if stats.Evictions != expectedCount {
		t.Errorf("Evictions = %v, want %v", stats.Evictions, expectedCount)
	}
}

package cache

import (
	"time"
)

// Entry represents a cached value with metadata.
type Entry struct {
	// Key is the cache key.
	Key string

	// Value is the cached data (typically JSON-encoded resource).
	Value []byte

	// CreatedAt is when this entry was first created.
	CreatedAt time.Time

	// UpdatedAt is when this entry was last updated.
	UpdatedAt time.Time

	// ExpiresAt is when this entry expires (zero means no expiration).
	ExpiresAt time.Time

	// TTL is the time-to-live duration for this entry.
	TTL time.Duration

	// Hits is the number of times this entry has been retrieved.
	Hits int64

	// Size is the size of the value in bytes.
	Size int64
}

// IsExpired returns true if the entry has expired.
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// Age returns how long the entry has existed.
func (e *Entry) Age() time.Duration {
	return time.Since(e.CreatedAt)
}

// TimeUntilExpiry returns how long until the entry expires.
// Returns 0 if already expired or no TTL set.
func (e *Entry) TimeUntilExpiry() time.Duration {
	if e.ExpiresAt.IsZero() {
		return 0
	}
	remaining := time.Until(e.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	valueCopy := make([]byte, len(e.Value))
	copy(valueCopy, e.Value)

	return &Entry{
		Key:       e.Key,
		Value:     valueCopy,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		ExpiresAt: e.ExpiresAt,
		TTL:       e.TTL,
		Hits:      e.Hits,
		Size:      e.Size,
	}
}

// NewEntry creates a new cache entry.
func NewEntry(key string, value []byte, ttl time.Duration) *Entry {
	now := time.Now()
	size := int64(len(value))

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	return &Entry{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
		TTL:       ttl,
		Hits:      0,
		Size:      size,
	}
}

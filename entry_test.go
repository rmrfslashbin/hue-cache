package cache

import (
	"testing"
	"time"
)

func TestNewEntry(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value []byte
		ttl   time.Duration
	}{
		{
			name:  "entry with ttl",
			key:   "test:123",
			value: []byte("test value"),
			ttl:   5 * time.Minute,
		},
		{
			name:  "entry without ttl",
			key:   "test:456",
			value: []byte("another value"),
			ttl:   0,
		},
		{
			name:  "empty value",
			key:   "test:789",
			value: []byte{},
			ttl:   1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewEntry(tt.key, tt.value, tt.ttl)

			if entry.Key != tt.key {
				t.Errorf("Key = %v, want %v", entry.Key, tt.key)
			}

			if string(entry.Value) != string(tt.value) {
				t.Errorf("Value = %v, want %v", entry.Value, tt.value)
			}

			if entry.TTL != tt.ttl {
				t.Errorf("TTL = %v, want %v", entry.TTL, tt.ttl)
			}

			if entry.Size != int64(len(tt.value)) {
				t.Errorf("Size = %v, want %v", entry.Size, len(tt.value))
			}

			if entry.Hits != 0 {
				t.Errorf("Hits = %v, want 0", entry.Hits)
			}

			if entry.CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}

			if entry.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should not be zero")
			}

			if tt.ttl > 0 {
				if entry.ExpiresAt.IsZero() {
					t.Error("ExpiresAt should not be zero when TTL > 0")
				}
				expectedExpiry := entry.CreatedAt.Add(tt.ttl)
				if !entry.ExpiresAt.Equal(expectedExpiry) {
					t.Errorf("ExpiresAt = %v, want %v", entry.ExpiresAt, expectedExpiry)
				}
			} else {
				if !entry.ExpiresAt.IsZero() {
					t.Error("ExpiresAt should be zero when TTL = 0")
				}
			}
		})
	}
}

func TestEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name    string
		entry   *Entry
		want    bool
		sleep   time.Duration
		wantMsg string
	}{
		{
			name: "not expired - no ttl",
			entry: &Entry{
				Key:       "test:1",
				Value:     []byte("value"),
				CreatedAt: time.Now(),
				ExpiresAt: time.Time{},
			},
			want:    false,
			wantMsg: "entry with no TTL should never expire",
		},
		{
			name: "not expired - future expiry",
			entry: &Entry{
				Key:       "test:2",
				Value:     []byte("value"),
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(10 * time.Second),
			},
			want:    false,
			wantMsg: "entry should not be expired before expiry time",
		},
		{
			name: "expired - past expiry",
			entry: &Entry{
				Key:       "test:3",
				Value:     []byte("value"),
				CreatedAt: time.Now().Add(-10 * time.Second),
				ExpiresAt: time.Now().Add(-5 * time.Second),
			},
			want:    true,
			wantMsg: "entry should be expired after expiry time",
		},
		{
			name: "expires after sleep",
			entry: NewEntry("test:4", []byte("value"), 50*time.Millisecond),
			sleep: 100 * time.Millisecond,
			want:  true,
			wantMsg: "entry should expire after TTL elapses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sleep > 0 {
				time.Sleep(tt.sleep)
			}

			got := tt.entry.IsExpired()
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v - %s", got, tt.want, tt.wantMsg)
			}
		})
	}
}

func TestEntry_Age(t *testing.T) {
	now := time.Now()
	entry := &Entry{
		CreatedAt: now.Add(-5 * time.Second),
	}

	age := entry.Age()
	if age < 4*time.Second || age > 6*time.Second {
		t.Errorf("Age() = %v, want approximately 5s", age)
	}
}

func TestEntry_TimeUntilExpiry(t *testing.T) {
	tests := []struct {
		name      string
		entry     *Entry
		wantRange [2]time.Duration // min, max
	}{
		{
			name: "no expiry",
			entry: &Entry{
				ExpiresAt: time.Time{},
			},
			wantRange: [2]time.Duration{0, 0},
		},
		{
			name: "already expired",
			entry: &Entry{
				ExpiresAt: time.Now().Add(-5 * time.Second),
			},
			wantRange: [2]time.Duration{0, 0},
		},
		{
			name: "expires in future",
			entry: &Entry{
				ExpiresAt: time.Now().Add(10 * time.Second),
			},
			wantRange: [2]time.Duration{9 * time.Second, 11 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.TimeUntilExpiry()

			if tt.wantRange[0] == 0 && tt.wantRange[1] == 0 {
				if got != 0 {
					t.Errorf("TimeUntilExpiry() = %v, want 0", got)
				}
				return
			}

			if got < tt.wantRange[0] || got > tt.wantRange[1] {
				t.Errorf("TimeUntilExpiry() = %v, want between %v and %v",
					got, tt.wantRange[0], tt.wantRange[1])
			}
		})
	}
}

func TestEntry_Clone(t *testing.T) {
	original := NewEntry("test:clone", []byte("original value"), 5*time.Minute)
	original.Hits = 42

	clone := original.Clone()

	// Check that values are equal
	if clone.Key != original.Key {
		t.Errorf("Clone Key = %v, want %v", clone.Key, original.Key)
	}
	if string(clone.Value) != string(original.Value) {
		t.Errorf("Clone Value = %v, want %v", clone.Value, original.Value)
	}
	if clone.Hits != original.Hits {
		t.Errorf("Clone Hits = %v, want %v", clone.Hits, original.Hits)
	}

	// Check that value is a deep copy
	clone.Value[0] = 'X'
	if original.Value[0] == 'X' {
		t.Error("Modifying clone Value affected original - not a deep copy")
	}

	// Check that modifying clone doesn't affect original
	clone.Hits = 100
	if original.Hits == 100 {
		t.Error("Modifying clone Hits affected original")
	}
}

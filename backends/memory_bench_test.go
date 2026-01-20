package backends

import (
	"context"
	"testing"
	"time"
)

func BenchmarkMemory_Set(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value for benchmarking")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+(i%10)))
		_ = backend.Set(ctx, key, value, 0)
	}
}

func BenchmarkMemory_Get(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := "light:" + string(rune('0'+i))
		backend.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+(i%100)))
		_, _ = backend.Get(ctx, key)
	}
}

func BenchmarkMemory_GetMiss(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = backend.Get(ctx, "nonexistent")
	}
}

func BenchmarkMemory_SetWithTTL(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")
	ttl := 5 * time.Minute

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+(i%10)))
		_ = backend.Set(ctx, key, value, ttl)
	}
}

func BenchmarkMemory_Delete(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	// Pre-populate
	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+i))
		backend.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+i))
		_ = backend.Delete(ctx, key)
	}
}

func BenchmarkMemory_Keys(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	// Pre-populate with different resource types
	for i := 0; i < 100; i++ {
		backend.Set(ctx, "light:"+string(rune('0'+i)), value, 0)
		backend.Set(ctx, "room:"+string(rune('0'+i)), value, 0)
		backend.Set(ctx, "scene:"+string(rune('0'+i)), value, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = backend.Keys(ctx, "light:*")
	}
}

func BenchmarkMemory_Stats(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = backend.Stats(ctx)
	}
}

func BenchmarkMemory_ParallelGet(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := "light:" + string(rune('0'+i))
		backend.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "light:" + string(rune('0'+(i%100)))
			_, _ = backend.Get(ctx, key)
			i++
		}
	})
}

func BenchmarkMemory_ParallelSet(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "light:" + string(rune('0'+(i%10)))
			_ = backend.Set(ctx, key, value, 0)
			i++
		}
	})
}

func BenchmarkMemory_ParallelMixed(b *testing.B) {
	backend := NewMemory()
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	// Pre-populate
	for i := 0; i < 100; i++ {
		key := "light:" + string(rune('0'+i))
		backend.Set(ctx, key, value, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "light:" + string(rune('0'+(i%100)))
			if i%2 == 0 {
				_, _ = backend.Get(ctx, key)
			} else {
				_ = backend.Set(ctx, key, value, 0)
			}
			i++
		}
	})
}

func BenchmarkMemory_EvictionLRU(b *testing.B) {
	config := &MemoryConfig{
		MaxEntries:     100,
		EvictionPolicy: EvictionLRU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+i))
		_ = backend.Set(ctx, key, value, 0)
	}
}

func BenchmarkMemory_EvictionLFU(b *testing.B) {
	config := &MemoryConfig{
		MaxEntries:     100,
		EvictionPolicy: EvictionLFU,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+i))
		_ = backend.Set(ctx, key, value, 0)
	}
}

func BenchmarkMemory_EvictionFIFO(b *testing.B) {
	config := &MemoryConfig{
		MaxEntries:     100,
		EvictionPolicy: EvictionFIFO,
	}

	backend := NewMemory(config)
	defer backend.Close()

	ctx := context.Background()
	value := []byte("test value")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := "light:" + string(rune('0'+i))
		_ = backend.Set(ctx, key, value, 0)
	}
}

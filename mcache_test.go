package mcache

import (
	"testing"
	"time"
)

var TTL = 100
var benchmarksSize = 5 * 1000 * 1000
var cache = New(int64(benchmarksSize), int64(TTL))

func TestAdd(t *testing.T) {
	cache.Store(0, 0, nanotime())
	v, ok := cache.Load(0)
	if !ok {
		t.Fatalf("Failed to load value from the cache")
	}
	if v != 0 {
		t.Fatalf("Wrong value %v instead of %v", v, 0)
	}
}

func TestRemove(t *testing.T) {
	time.Sleep(time.Second)
	nextExpiration, evicted := cache.Evict(Nanotime())
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v", nextExpiration)
	}
	_, ok := cache.Load(0)
	if ok {
		t.Fatalf("Failed to remove value from the cache")
	}
}

func BenchmarkStore(b *testing.B) {
	b.N = benchmarksSize
	now := nanotime()
	for i := 0; i < b.N; i++ {
		cache.Store(Key(i), Object(i), now)
	}
}

func BenchmarkLoad(b *testing.B) {
	b.N = benchmarksSize
	for i := 0; i < b.N; i++ {
		cache.Load(Key(i))
	}
}

func TestEvictSetup(t *testing.T) {
	time.Sleep(time.Duration(2*TTL) * time.Millisecond)
}

func BenchmarkEvict(b *testing.B) {
	now := nanotime()
	b.N = benchmarksSize
	for i := 0; i < b.N; i++ {
		_, ok := cache.Evict(now)
		if !ok {
			b.Fatalf("Failed to evict %v", i)
		}
	}
}

func TestRemove1(t *testing.T) {
	if len(cache.data) > 0 {
		t.Fatalf("Failed to remove all values from the cache, remains %d", len(cache.data))
	}
}

func TestAddCustomType(t *testing.T) {
	cache.Store(0, 0, nanotime())
	v, ok := cache.Load(0)
	if !ok {
		t.Fatalf("Failed to load value from the cache")
	}
	if v != 0 {
		t.Fatalf("Wrong value %v instead of %v", v, 0)
	}
}

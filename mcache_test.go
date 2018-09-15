package mcache

import (
	"testing"
	"time"
)

var TTL = 10
var smallCache = New(1, int64(TTL))

func TestAdd(t *testing.T) {
	smallCache.Store(0, 0, nanotime())
	v, ok := smallCache.Load(0)
	if !ok {
		t.Fatalf("Failed to load value from the cache")
	}
	if v != 0 {
		t.Fatalf("Wrong value %v instead of %v", v, 0)
	}
}

func TestRemove(t *testing.T) {
	time.Sleep(time.Second)
	nextExpiration, evicted := smallCache.Evict(Nanotime())
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v", nextExpiration)
	}
	_, ok := smallCache.Load(0)
	if ok {
		t.Fatalf("Failed to remove value from the cache")
	}
}

var cache = New(100*1000*1000, int64(TTL))

func BenchmarkStore(b *testing.B) {
	now := nanotime()
	cache.Reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ok := cache.Store(Key(i), Object(i), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
}

func BenchmarkLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cache.Load(Key(i))
	}
}

func BenchmarkEvict(b *testing.B) {
	cache.Reset()
	now := nanotime()
	for i := 0; i < b.N; i++ {
		if ok := cache.Store(Key(i), Object(i), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
	time.Sleep(time.Duration(TTL) * time.Millisecond)
	now = nanotime()
	b.ResetTimer()
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

package mcache

import (
	"reflect"
	"testing"
	"time"
	"unsafe"
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
	_, evicted, nextExpiration := smallCache.Evict(Nanotime())
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

type MyData struct {
	key int
}

func TestAddCustomType(t *testing.T) {
	pool := NewPool(reflect.TypeOf(new(MyData)), 1)
	ptr, ok := pool.Alloc()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	t.Logf("Allocated %p", ptr)

	myData := (*MyData)(ptr)
	myData.key = 1

	smallCache.Store(Key(myData.key), Object(ptr), Nanotime())
	time.Sleep(time.Duration(TTL) * time.Millisecond)
	o, evicted, nextExpiration := smallCache.Evict(Nanotime())
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v", nextExpiration)
	}
	if ok = pool.Free(unsafe.Pointer(o)); !ok {
		t.Fatalf("Failed to free ptr %v", o)
	}
	if ok = pool.Free(unsafe.Pointer(pool)); ok {
		t.Fatalf("Succeeded to add illegal pointer %p", pool)
	}
	if ok = pool.Free(unsafe.Pointer(uintptr(0))); ok {
		t.Fatalf("Succeeded to add illegal pointer 0")
	}
}

func BenchmarkPoolAlloc(b *testing.B) {
	poolSize := 10 * 1000 * 1000
	pool := NewPool(reflect.TypeOf(new(MyData)), poolSize)
	b.N = poolSize
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, ok := pool.Alloc()
		if !ok {
			b.Fatalf("Failed to allocate an object from the pool %d", i)
		}
	}
}

var cacheSize = 10 * 1000 * 1000
var cache = New(cacheSize, int64(TTL))

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
		_, expired, _ := cache.Evict(now)
		if !expired {
			b.Fatalf("Failed to evict %v", i)
		}
	}
}

func TestRemove1(t *testing.T) {
	if len(cache.data) > 0 {
		t.Fatalf("Failed to remove all values from the cache, remains %d", len(cache.data))
	}
}

package mcache

import (
	"hash/fnv"
	"reflect"
	"testing"
	"time"
	"unsafe"
)

var TTL int64 = 10
var smallCache = New(1, int64(TTL))

func TestAdd(t *testing.T) {
	if smallCache.Len() != 0 {
		t.Fatalf("Cache is not empty %d", smallCache.Len())
	}
	smallCache.Store(0, 0, nanotime())
	v, ok := smallCache.Load(0)
	if !ok {
		t.Fatalf("Failed to load value from the cache")
	}
	if v != 0 {
		t.Fatalf("Wrong value %v instead of %v", v, 0)
	}
	if smallCache.Len() != 1 {
		t.Fatalf("Got %d, expected 1", smallCache.Len())
	}
}

func TestRemove(t *testing.T) {
	smallCache.Reset()
	start := nanotime()
	smallCache.Store(0, 0, start)
	_, evicted, nextExpiration := smallCache.Evict(start)
	if evicted {
		t.Fatalf("Evicted entry before it expired")
	}
	expectedNextExpiration := ns * TTL
	if nextExpiration != expectedNextExpiration {
		t.Fatalf("Expected %d, got %d", expectedNextExpiration, nextExpiration)
	}
	time.Sleep(time.Second)
	_, evicted, nextExpiration = smallCache.Evict(Nanotime())
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

	_, evicted, nextExpiration = smallCache.Evict(Nanotime())
	if evicted {
		t.Fatalf("Evicted from empty cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v, should be zero", nextExpiration)
	}
}

func TestOverflow(t *testing.T) {
	smallCache.Reset()
	if ok := smallCache.Store(0, 0, nanotime()); !ok {
		t.Fatalf("Failed to store value in the cache")
	}
	if ok := smallCache.Store(0, 0, nanotime()); ok {
		t.Fatalf("Did not fail on overflow")
	}
}

type MyData struct {
	a int
	b int
}

func TestPoolAlloc(t *testing.T) {
	pool := NewPool(reflect.TypeOf(new(MyData)), 1)
	if _, ok := pool.Alloc(); !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	if _, ok := pool.Alloc(); ok {
		t.Fatalf("Did not fail on empty pool")
	}
}

func TestAddCustomType(t *testing.T) {
	pool := NewPool(reflect.TypeOf(new(MyData)), 1)
	ptr, ok := pool.Alloc()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	t.Logf("Allocated %p", ptr)

	myData := (*MyData)(ptr)
	myData.a = 1
	myData.b = 2

	smallCache.Store(0, Object(ptr), Nanotime())
	time.Sleep(time.Duration(TTL) * time.Millisecond)
	o, evicted, nextExpiration := smallCache.Evict(Nanotime())
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v", nextExpiration)
	}
	myData = (*MyData)(unsafe.Pointer(o))
	if myData.a != 1 || myData.b != 2 {
		t.Fatalf("Failed to recover the original data %v", myData)
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

var fnvSum uint32

// 40ns per hash
func BenchmarkHashFnv(b *testing.B) {
	s := "google.com."
	h := fnv.New32a()
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.Write([]byte(s))
		fnvSum = h.Sum32()
	}
}

// 10ns/allocation
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

// 150ns cache.Store()
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

// 120ns cache.Load()
func BenchmarkLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cache.Load(Key(i))
	}
}

// 180ns cache.Evict()
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

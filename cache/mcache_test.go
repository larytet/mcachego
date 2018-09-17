package mcache

import (
	"fmt"
	"github.com/cespare/xxhash"
	"hash/fnv"
	"mcachego/unsafepool"
	"reflect"
	"sync/atomic"
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
	smallCache.Store("0", 0, nanotime())
	v, ok := smallCache.Load("0")
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
	smallCache.Store("0", 0, start)
	_, evicted, nextExpiration := smallCache.Evict(start, false)
	if evicted {
		t.Fatalf("Evicted entry before it expired")
	}
	expectedNextExpiration := ns * TTL
	if nextExpiration != expectedNextExpiration {
		t.Fatalf("Expected %d, got %d", expectedNextExpiration, nextExpiration)
	}
	time.Sleep(time.Second)
	_, evicted, nextExpiration = smallCache.Evict(Nanotime(), false)
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v", nextExpiration)
	}
	_, ok := smallCache.Load("0")
	if ok {
		t.Fatalf("Failed to remove value from the cache")
	}

	_, evicted, nextExpiration = smallCache.Evict(Nanotime(), false)
	if evicted {
		t.Fatalf("Evicted from empty cache")
	}
	if nextExpiration != 0 {
		t.Fatalf("bad next expiration %v, should be zero", nextExpiration)
	}
}

func TestOverflow(t *testing.T) {
	smallCache.Reset()
	if ok := smallCache.Store("0", 0, nanotime()); !ok {
		t.Fatalf("Failed to store value in the cache")
	}
	if ok := smallCache.Store("0", 0, nanotime()); ok {
		t.Fatalf("Did not fail on overflow")
	}
}

type MyData struct {
	a int
	b int
}

func TestAddCustomType(t *testing.T) {
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), 1)
	ptr, ok := pool.Alloc()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	t.Logf("Allocated %p", ptr)

	myData := (*MyData)(ptr)
	myData.a = 1
	myData.b = 2

	smallCache.Store("0", Object(ptr), Nanotime())
	time.Sleep(time.Duration(TTL) * time.Millisecond)
	o, evicted, nextExpiration := smallCache.Evict(Nanotime(), false)
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
	if !pool.Belongs(unsafe.Pointer(o)) {
		t.Fatalf("Bad pointer %v is allocated from the pool", o)
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

func BenchmarkAllocStoreEvictFree(b *testing.B) {
	b.ReportAllocs()
	cache := New(b.N, int64(TTL))
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), b.N)
	now := nanotime()
	keys := make([]string, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("000000  %d", b.N-i)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p, ok := pool.Alloc()
		if !ok {
			b.Fatalf("Failed to allocate an object from the pool %d", i)
		}
		if p == unsafe.Pointer(uintptr(0)) {
			b.Fatalf("Nil is allocated from the pool %d", i)
		}
		if !pool.Belongs(p) {
			b.Fatalf("Bad pointer %p is allocated from the pool", p)
		}
		if ok := cache.Store(Key(keys[i]), Object(p), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
	now += 1000*1000*TTL + 1
	for i := 0; i < b.N; i++ {
		p, expired, _ := cache.Evict(now, false)
		if !expired {
			b.Fatalf("Failed to evict %v", i)
		}
		ok := pool.Free(unsafe.Pointer(p))
		if !ok {
			b.Fatalf("Failed to free an object %p to the pool in the iteration %d", unsafe.Pointer(p), i)
		}
	}
	s := pool.GetStatistics()
	if s.AllocLockCongested != 0 {
		b.Fatalf("Alloc congestion %d", s.AllocLockCongested)
	}
	if s.FreeLockCongested != 0 {
		b.Fatalf("Free congestion %d", s.AllocLockCongested)
	}
}

var fnvSum uint32

// 40ns per hash
func BenchmarkHashFnv(b *testing.B) {
	b.ReportAllocs()
	s := "google.com."
	h := fnv.New32a()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.Write([]byte(s))
		fnvSum = h.Sum32()
	}
}

// 10ns/allocation - suprisingly expensive
// 32/64 bits compare and swap do not impact the performance
func BenchmarkPoolAlloc(b *testing.B) {
	b.ReportAllocs()
	poolSize := 10 * 1000 * 1000
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), poolSize)
	b.N = poolSize
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, ok := pool.Alloc()
		if !ok {
			b.Fatalf("Failed to allocate an object from the pool %d", i)
		}
	}
}

func BenchmarkStackAllocationMap(b *testing.B) {
	now := nanotime()
	mapSize := 10 * 1000 * 1000
	b.ReportAllocs()
	b.N = mapSize
	m := make(map[uintptr]uintptr, mapSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		it := item{o: Object(i), expirationNs: now + TTL}
		m[uintptr(it.o)] = uintptr(it.o)
	}
}

func BenchmarkFifo(b *testing.B) {
	fifoSize := 10 * 1000 * 1000
	fifo := newFifo(fifoSize)
	b.ReportAllocs()
	b.N = fifoSize
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ok := fifo.add(Key(i))
		if !ok {
			b.Fatalf("Failed to add an object to the FIFO %d", i)
		}
	}
}

func BenchmarkPoolAllocFree(b *testing.B) {
	b.ReportAllocs()
	poolSize := 10 * 1000 * 1000
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), poolSize)
	b.N = poolSize
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p, ok := pool.Alloc()
		if !ok {
			b.Fatalf("Failed to allocate an object from the pool %d", i)
		}
		ok = pool.Free(p)
		if !ok {
			b.Fatalf("Failed to free an object to the pool %d", i)
		}
	}
}

func BenchmarkMapInt32Store(b *testing.B) {
	mapSize := 1 * 1000 * 1000
	b.N = mapSize
	m := make(map[int32]int32, mapSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[int32(i)] = 0
	}
}

func BenchmarkMapInt32Delete(b *testing.B) {
	mapSize := b.N
	m := make(map[int32]int32, mapSize)
	for i := 0; i < b.N; i++ {
		m[int32(i)] = int32(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delete(m, int32(i))
	}
}

func BenchmarkMapInt32Lookup(b *testing.B) {
	mapSize := b.N
	m := make(map[int32]int32, mapSize)
	for i := 0; i < b.N; i++ {
		m[int32(i)] = int32(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, ok := m[int32(i)]
		if !ok {
			b.Fatalf("Failed to find entry in the map %d", i)
		}
		if v != int32(i) {
			b.Fatalf("Bad entry in the map %d", i)
		}
	}
}

func BenchmarkMapStringStoreLookup(b *testing.B) {
	b.ReportAllocs()
	mapSize := b.N
	m := make(map[string]uintptr, mapSize)
	keys := make([]string, mapSize, mapSize)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("000000  %d", mapSize-i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[keys[i]] = uintptr(i)
	}
	for i := 0; i < b.N; i++ {
		if _, ok := m[keys[i]]; !ok {
			b.Fatalf("Missing entry in the map %d", i)
		}
	}
}

type mapItem struct {
	a int64
	b int64
}

func BenchmarkStackAllocationMapString(b *testing.B) {
	b.ReportAllocs()
	mapSize := b.N
	m := make(map[string]mapItem, mapSize)
	keys := make([]string, mapSize, mapSize)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("000000  %d", mapSize-i)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		it := mapItem{a: int64(i), b: int64(i)}
		m[keys[i]] = it
	}
}

func BenchmarkAtomicCompareAndSwap(b *testing.B) {
	idx := int32(0)
	for i := 0; i < b.N; i++ {
		atomic.CompareAndSwapInt32(&idx, idx, idx+1)
	}
}

func BenchmarkTimeNowUnixNano(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Now().UnixNano()
	}
}

func BenchmarkEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

var globalCounter int32

func BenchmarkGlobalCounter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		globalCounter += 1
	}
}

func BenchmarkLocalCounter(b *testing.B) {
	var localCounter int32
	for i := 0; i < b.N; i++ {
		localCounter += 1
	}
}

// 8ns/sum
// O(len(string)) ?
func BenchmarkXxhashSum64String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		xxhash.Sum64String("google.go.")
	}
}

// 150ns cache.Store()
func BenchmarkStore(b *testing.B) {
	b.ReportAllocs()
	now := nanotime()
	keys := make([]string, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("000000  %d", b.N-i)
	}
	cache := New(b.N, int64(TTL))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ok := cache.Store(Key(keys[i]), Object(i), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
}

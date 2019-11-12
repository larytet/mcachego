package mcache

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
	"github.com/larytet-go/fifo64"
	"github.com/larytet-go/nanotime"
	"github.com/larytet-go/unsafepool"
)

var TTL TimeMs = 10

func TestGetTime(t *testing.T) {
	t0 := GetTime()
	time.Sleep(time.Second)
	t1 := GetTime()
	d := t1 - t0
	if t1-t0 < 1000 {
		t.Fatalf("Sleep is shorter than expected %d", d)
	}
	if t1-t0 > 1001 {
		t.Fatalf("Sleep is longer than expected %d", d)
	}
}

func TestItemSize(t *testing.T) {
	itemSize := unsafe.Sizeof(*new(item))
	if itemSize != 8 {
		t.Fatalf("Cache item size %d is not 64 bits", itemSize)
	}
}

func TestAdd(t *testing.T) {
	var smallCache = New(Configuration{Size: 1, TTL: TTL, LoadFactor: 100})
	if smallCache.Len() != 0 {
		t.Fatalf("Cache is not empty %d", smallCache.Len())
	}
	smallCache.Store(0, 0, GetTime())
	v, _, ok := smallCache.Load(0)
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
	var smallCache = New(Configuration{Size: 1, TTL: TTL, LoadFactor: 100})
	start := GetTime()
	smallCache.Store(0, 0, start)
	_, evicted := smallCache.Evict(start, false)
	if evicted {
		t.Fatalf("Evicted entry before it expired")
	}
	time.Sleep(time.Second)
	now := GetTime()
	_, evicted = smallCache.Evict(now, false)
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	_, _, ok := smallCache.Load(0)
	if ok {
		t.Fatalf("Failed to remove value from the cache")
	}

	_, evicted = smallCache.Evict(now, false)
	if evicted {
		t.Fatalf("Evicted from empty cache")
	}
}

func TestRemoveByRef(t *testing.T) {
	var smallCache = New(Configuration{Size: 1, TTL: TTL, LoadFactor: 100})
	smallCache.Store(0, 0, GetTime())
	v, ref, ok := smallCache.Load(0)
	if !ok {
		t.Fatalf("Failed to find value in the cache")
	}
	if v != 0 {
		t.Fatalf("Bad entry in the map %v", v)
	}

	smallCache.EvictByRef(ref)
	_, _, ok = smallCache.Load(0)
	if ok {
		t.Fatalf("Failed to remove value from the cache")
	}
}

func TestOverflow(t *testing.T) {
	var smallCache = New(Configuration{Size: 1, TTL: TTL, LoadFactor: 100})
	if ok := smallCache.Store(0, 0, GetTime()); !ok {
		t.Fatalf("Failed to store value in the cache")
	}
	if ok := smallCache.Store(0, 0, GetTime()); ok {
		t.Fatalf("Did not fail on overflow")
	}
}

type MyData struct {
	a int
	b int
}

func TestAddCustomType(t *testing.T) {
	var smallCache = New(Configuration{Size: 1, TTL: TTL, LoadFactor: 100})
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), 1)
	ptr, ok := pool.Alloc()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	myData := (*MyData)(unsafe.Pointer(ptr))
	myData.a = 1
	myData.b = 2

	smallCache.Store(0, Object(uintptr(ptr)-pool.GetBase()), GetTime())
	time.Sleep(time.Duration(TTL) * time.Millisecond)
	o, evicted := smallCache.Evict(GetTime(), false)
	if !evicted {
		t.Fatalf("Failed to evict value from the cache")
	}
	oAddress := uintptr(o) + pool.GetBase()
	myData = (*MyData)(unsafe.Pointer(oAddress))
	if myData.a != 1 || myData.b != 2 {
		t.Fatalf("Failed to recover the original data %v", myData)
	}
	if !pool.Belongs(oAddress) {
		t.Fatalf("Bad pointer %v is allocated from the pool", o)
	}
	if ok = pool.Free(oAddress); !ok {
		t.Fatalf("Failed to free ptr %v", o)
	}
	if ok = pool.Free(uintptr(unsafe.Pointer(pool))); ok {
		t.Fatalf("Succeeded to add illegal pointer %p", pool)
	}
	if ok = pool.Free(uintptr(unsafe.Pointer(uintptr(0)))); ok {
		t.Fatalf("Succeeded to add illegal pointer 0")
	}
}

type TestData struct {
	i int
	k int
}

func getIfc(data TestData) interface{} {
	return data
}

func getMyData(b *testing.B, ifc interface{}, expected TestData) TestData {
	data := ifc.(TestData)
	if data.i != expected.i {
		b.Fatalf("%d instead of %d", data.i, data.i)
	}
	if data.k != expected.k {
		b.Fatalf("%d instead of %d", data.k, data.k)
	}
	return ifc.(TestData)
}

func getMyDataFast(ifc interface{}) TestData {
	return ifc.(TestData)
}

func BenchmarkTypeAssert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for k := 0; k < 1000; k++ {
			data := TestData{i, k}
			myIfc := getIfc(data)
			getMyData(b, myIfc, data)
		}
	}
}

func BenchmarkTypeAssertFast(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for k := 0; k < 1000; k++ {
			data := TestData{i, k}
			myIfc := getIfc(data)
			data = getMyDataFast(myIfc)
			if data.i != i {
				b.Fatalf("%d instead of %d", data.i, i)
			}
			if data.k != k {
				b.Fatalf("%d instead of %d", data.k, k)
			}
		}
	}
}

func BenchmarkTypeAssertEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for k := 0; k < 1000; k++ {
		}
	}
}

func BenchmarkAllocStoreEvictFree(b *testing.B) {
	b.ReportAllocs()
	cache := New(Configuration{Size: b.N, TTL: TTL, LoadFactor: 50})
	pool := unsafepool.New(reflect.TypeOf(new(MyData)), b.N)
	now := GetTime()
	keys := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = uint64(b.N - i)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p, ok := pool.Alloc()
		if !ok {
			b.Fatalf("Failed to allocate an object from the pool %d", i)
		}
		if p == uintptr(0) {
			b.Fatalf("Nil is allocated from the pool %d", i)
		}
		if !pool.Belongs(p) {
			b.Fatalf("Bad pointer %x is allocated from the pool", p)
		}
		if ok := cache.Store(keys[i], Object(uintptr(p)-pool.GetBase()), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
	now += 1000*1000*TTL + 1
	for i := 0; i < b.N; i++ {
		pOffset, expired := cache.Evict(now, false)
		p := unsafe.Pointer(uintptr(pOffset) + pool.GetBase())
		if !expired {
			b.Fatalf("Failed to evict %v", i)
		}
		ok := pool.Free(uintptr(p))
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

func BenchmarkStackAllocationMap(b *testing.B) {
	now := GetTime()
	mapSize := 10 * 1000 * 1000
	b.ReportAllocs()
	b.N = mapSize
	m := make(map[uintptr]uintptr, mapSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		it := item{o: Object(i), expirationMs: now + TTL}
		m[uintptr(it.o)] = uintptr(it.o)
	}
}

func BenchmarkFifo(b *testing.B) {
	fifoSize := 10 * 1000 * 1000
	fifo := fifo64.New(fifoSize)
	b.ReportAllocs()
	b.N = fifoSize
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ok := fifo.Add(uint64(i))
		if !ok {
			b.Fatalf("Failed to add an object to the FIFO %d", i)
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
		keys[i] = fmt.Sprintf("000000-%d", mapSize-i)
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
		keys[i] = fmt.Sprintf("000000-%d", mapSize-i)
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

func BenchmarkNanotime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		nanotime.Now()
	}
}

func BenchmarkEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

var globalCounter int32

func BenchmarkGlobalCounter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		globalCounter++
	}
}

func BenchmarkLocalCounter(b *testing.B) {
	var localCounter int32
	for i := 0; i < b.N; i++ {
		localCounter++
	}
}

// 8ns/sum
// O(len(string)) ?
func BenchmarkXxhashSum64String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		xxhash.Sum64String("google.go.")
	}
}

// 180ns cache.Store()
func BenchmarkStore(b *testing.B) {
	b.ReportAllocs()
	now := GetTime()
	keys := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = uint64(b.N - i)
	}
	cache := New(Configuration{Size: b.N, TTL: TTL, LoadFactor: 50})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ok := cache.Store(keys[i], Object(i), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
}

// 160ns cache.Load()
func BenchmarkLoad(b *testing.B) {
	b.ReportAllocs()
	now := GetTime()
	keys := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = uint64(b.N - i)
	}
	cache := New(Configuration{Size: b.N, TTL: TTL, LoadFactor: 50})
	for i := 0; i < b.N; i++ {
		if ok := cache.Store(keys[i], Object(i), now); !ok {
			b.Fatalf("Failed to add item %d", i)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if o, _, ok := cache.Load(keys[i]); !ok {
			b.Fatalf("Failed to find item %d", i)
		} else if int(o) != i {
			b.Fatalf("Bad entry %d instead of %d", int(o), i)
		}
	}
}

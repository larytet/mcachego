package hashtable

import (
	"fmt"
	"github.com/cespare/xxhash"
	"log"
	"math"
	"math/rand"
	"mcachego/hashtable/xorshift64star"
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

type A struct {
	str string
}

var firstptr = new(A)

func T1estUnsafe(t *testing.T) {
	firstptr.str = "first"
	firstUnsafe := uintptr(unsafe.Pointer(firstptr))
	secondptr := new(A)
	secondptr.str = "second"
	firstptr = secondptr
	runtime.GC()
	var allocated [][]int
	var i = 0
	for {
		block := make([]int, 10*1024*1024)
		allocated = append(allocated, block)
		if uintptr(firstUnsafe) >= uintptr(unsafe.Pointer(&block[0])) && uintptr(firstUnsafe) <= uintptr(unsafe.Pointer(&block[len(block)-1])) {
			break
		}
		runtime.GC()
		i++
		t.Errorf("Allocated %d", i)
	}
}

func TestRace(t *testing.T) {
	var done bool
	go func() {
		done = true
	}()
	for !done {
	}
}

func TestModulo(t *testing.T) {
	var testNumber = uint64(1) << 63
	for _, prime := range PrimeList {
		size := getSize(prime)
		if size != prime {
			t.Fatalf("Expected %d got %d", prime, size)
		}
		moduloSize := getModuloSizeFunction(size)
		if moduloSize == nil {
			t.Fatalf("Got nill for %d", prime)
		}
		modulo := moduloSize(testNumber)
		expectedModulo := int(testNumber % uint64(prime))
		if modulo != expectedModulo {
			t.Fatalf("Got %d instead for %d", modulo, expectedModulo)
		}
	}
}

func BenchmarkHashtableLoadMutlithread(b *testing.B) {
	b.ReportAllocs()
	h := New(2*b.N, 64)
	keys := make([]string, b.N, b.N)
	hashes := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%d", b.N-i)
		keys[i] = key
		hashes[i] = xxhash.Sum64String(key)
	}
	for i := 0; i < b.N; i++ {
		key := keys[i]
		if ok := h.Store(key, hashes[i], uintptr(i)); !ok {
			b.Fatalf("Failed to add item %d, %v", i, key)
		}
	}
	//	rand.Shuffle(len(keys), func(i, j int) {
	//		keys[i], keys[j] = keys[j], keys[i]
	//		hashes[i], hashes[j] = hashes[j], hashes[i]
	//	})
	threads := runtime.NumCPU()
	if b.N < threads {
		threads = 1
	}
	ch := make(chan int, threads)
	b.ResetTimer()
	for thread := 0; thread < threads; thread += 1 {
		go func(thread int) {
			for j := 0; j < b.N; j++ {
				v, ok, _ := h.Load(keys[j], hashes[j])
				if !ok {
					log.Printf("Failed to find key %v in the hashtable", keys[j])
				}
				if v != uintptr(j) {
					log.Printf("Got %v instead of %v from the hashtable", v, j)
				}
			}
			ch <- thread
		}(thread)
	}
	var completedThreads []int
	for threads > 0 {
		completed := <-ch
		completedThreads = append(completedThreads, completed)
		threads--
	}
	//b.Logf("Got completed %v from %d", completedThreads, threads)
}

var SmallTableSize = 100

// Run the same test with the Go map API for comparison
func BenchmarkSmallMapLookup(b *testing.B) {
	size := SmallTableSize
	keys := make([]string, size, size)
	for i := 0; i < size; i++ {
		keys[i] = fmt.Sprintf("%d", size-i)
	}
	samples := prepareNonuniform(size)
	m := make(map[string]uintptr, size)
	for i := 0; i < size; i++ {
		sample := samples[i]
		key := keys[sample]
		m[key] = 1
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < size; i++ {
			sample := samples[i]
			key := keys[sample]
			v := m[key]
			if v != 1 {
				b.Fatalf("Wrong value %v[%d]", key, v)
			}
		}
	}
}

func BenchmarkSmallHashtableLookup(b *testing.B) {
	size := SmallTableSize
	keys := make([]string, size, size)
	hashes := make([]uint64, size, size)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		keys[i] = key
		hashes[i] = xxhash.Sum64String(key)
	}
	h := New(4*size, 64)
	for i := 0; i < size; i++ {
		key := keys[i]
		hash := hashes[i]
		if ok := h.Store(key, hash, uintptr(i)); !ok {
			b.Fatalf("Failed to add item %d, %v", i, key)
		}
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < size; i++ {
			key := keys[i]
			hash := hashes[i]
			v, ok, _ := h.Load(key, hash)
			if !ok {
				b.Fatalf("Failed to find key %v hash %v in the hashtable", key, hash)
			}
			if v != uintptr(i) {
				b.Fatalf("Got %v instead of %v from the hashtable. b.N=%d", v, i, i)
			}
		}
	}
	b.StopTimer()
	b.Logf("Store collisions %d from %d, max collision chain %d", h.statistics.StoreCollision, h.statistics.Store, h.statistics.MaxCollisions)
}

func BenchmarkMapMutex(b *testing.B) {
	keysCount := 100
	keys := make([]string, keysCount, keysCount)
	for i := 0; i < keysCount; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	m := make(map[string]uintptr, keysCount)
	var mutex sync.Mutex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			mutex.Lock()
			m[key] = uintptr(i)
			mutex.Unlock()
		}
		for _, key := range keys {
			mutex.Lock()
			delete(m, key)
			mutex.Unlock()
		}
	}
}

func BenchmarkMapChannel(b *testing.B) {
	keysCount := 100
	keys := make([]string, keysCount, keysCount)
	for i := 0; i < keysCount; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	m := make(map[string]int, keysCount)
	insertCh := make(chan int)
	deleteCh := make(chan int)
	exitCh := make(chan int)
	go func() {
		for {
			select {
			case keyIdx := <-insertCh:
				key := keys[keyIdx]
				m[key] = keyIdx
			case keyIdx := <-deleteCh:
				key := keys[keyIdx]
				delete(m, key)
			case <-exitCh:
				return
			}
		}
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for idx, _ := range keys {
			insertCh <- idx
		}
		for idx, _ := range keys {
			deleteCh <- idx
		}
	}
	exitCh <- 0
}

func TestHashtable(t *testing.T) {
	itemSize := unsafe.Sizeof(*new(item))
	if itemSize%8 != 0 {
		t.Fatalf("Hashtable item size %d is not alligned", itemSize)
	}
	size := 10
	h := New(2*size, 4)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		ok := h.Store(key, xxhash.Sum64String(key), uintptr(i))
		if !ok {
			t.Fatalf("Failed to store value %v in the hashtable", i)
		}
	}
	ok := h.Store("0", xxhash.Sum64String("0"), uintptr(0))
	if ok {
		t.Fatalf("Added same key to the hashtable")
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok, _ := h.Load(key, xxhash.Sum64String(key))
		if !ok {
			t.Logf("%v", h.data)
			t.Fatalf("Failed to find key %v in the hashtable", key)
		}
		if v != uintptr(i) {
			t.Fatalf("Got %v instead of %v from the hashtable", v, i)
		}
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok := h.Remove(key, xxhash.Sum64String(key))
		if !ok {
			t.Fatalf("Failed to remove key %v from the hashtable", key)
		}
		if v != uintptr(i) {
			t.Fatalf("Got %v instead of %v from the hashtable", v, i)
		}
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok, _ := h.Load(key, xxhash.Sum64String(key))
		if ok {
			t.Fatalf("Found key %v in the empty hashtable", key)
		}
		if v != 0 {
			t.Fatalf("Got %v instead of 0 from the hashtable", v)
		}
	}
}

func prepareNonuniform(size int) []int {
	samples := make([]int, size, size)
	for i := 0; i < size; i++ {
		for {
			grg := gaussian()
			index := int(((2.0 + grg) / 4.0) * float64(size-1))
			if (index >= 0) && (index < size) {
				samples[i] = index
				break
			}
		}
	}
	return samples
}

// Run the same test with the Go map API for comparison
func BenchmarkMapStore(b *testing.B) {
	keys := make([]string, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	m := make(map[string]uintptr, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i]
		m[key] = uintptr(i)
	}
}

func BenchmarkMapStoreSameKey(b *testing.B) {
	m := make(map[string]uintptr, b.N)
	key := "0"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[key] = uintptr(i)
	}
}

func BenchmarkMapStoreNonuniform(b *testing.B) {
	keys := make([]string, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	samples := prepareNonuniform(b.N)
	m := make(map[string]uintptr, b.N)
	b.ResetTimer()
	skipped := 0
	for i := 0; i < b.N; i++ {
		sample := samples[i]
		if sample < 0 || sample >= b.N {
			//b.Logf("Skipped %d", sample)
			skipped++
			continue
		}
		key := keys[sample]
		m[key] = uintptr(i)
	}
}

func BenchmarkMapLoadNonuniform(b *testing.B) {
	keys := make([]string, b.N, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	samples := prepareNonuniform(b.N)
	m := make(map[string]uintptr, b.N)
	for i := 0; i < b.N; i++ {
		sample := samples[i]
		key := keys[sample]
		m[key] = 1
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sample := samples[i]
		key := keys[sample]
		v := m[key]
		if v != 1 {
			b.Fatalf("Wrong value %v[%d]", key, v)
		}
	}
}

// So far 150ns per Store() for large tables
func BenchmarkHashtableStore(b *testing.B) {
	b.ReportAllocs()
	//b.N = 100 * 1000
	h := New(2*b.N, 64)
	keys := make([]string, b.N, b.N)
	hashes := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%d", b.N-i)
		keys[i] = key
		hashes[i] = xxhash.Sum64String(key)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i]
		if ok := h.Store(key, hashes[i], uintptr(i)); !ok {
			b.Fatalf("Failed to add item %d, %v", i, key)
		}
	}
	b.StopTimer()
	b.Logf("Store collisions %d from %d, max collision chain %d", h.statistics.StoreCollision, h.statistics.Store, h.statistics.MaxCollisions)
}

func BenchmarkHashtableLoad(b *testing.B) {
	b.ReportAllocs()
	h := New(2*b.N, 64)
	keys := make([]string, b.N, b.N)
	hashes := make([]uint64, b.N, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%d", b.N-i)
		keys[i] = key
		hashes[i] = xxhash.Sum64String(key)
	}
	for i := 0; i < b.N; i++ {
		key := keys[i]
		if ok := h.Store(key, hashes[i], uintptr(i)); !ok {
			b.Fatalf("Failed to add item %d, %v", i, key)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i]
		v, ok, _ := h.Load(key, hashes[i])
		if !ok {
			b.Fatalf("Failed to find key %v in the hashtable", key)
		}
		if v != uintptr(i) {
			b.Fatalf("Got %v instead of %v from the hashtable. b.N=%d", v, i, b.N)
		}
	}
}

func float64Range(a, b float64) float64 {
	return a + rand.Float64()*(b-a)
}

// From https://github.com/leesper/go_rng/blob/master/gauss.go
func gaussian() float64 {
	// Box-Muller Transform
	var r, x, y float64
	for r >= 1 || r == 0 {
		x = float64Range(-1.0, 1.0)
		y = float64Range(-1.0, 1.0)
		r = x*x + y*y
	}
	return x * math.Sqrt(-2*math.Log(r)/r)
}

func BenchmarkRandomMemoryAccess(b *testing.B) {
	array := make([]int, b.N, b.N)
	prng := xorshift64star.New(1)
	for i := 0; i < b.N; i++ {
		hash := prng.Next()
		idx := int(hash % uint64(b.N))
		array[idx] = 1
	}

	var inUseCount = 0
	prng = xorshift64star.New(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash := prng.Next()
		idx := int(hash % uint64(b.N))
		// This line is responsible for 85% of the execution time
		it := array[idx]
		if it != 0 {
			inUseCount += 1
		}
	}
	b.StopTimer()
	b.Logf("InUse %d", inUseCount)
}

func BenchmarkModuloSize(b *testing.B) {
	size := getSize(10 * 1000 * 1000)
	var result int
	moduloSize := getModuloSizeFunction(size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result = moduloSize(uint64(i))
	}
	if result < 0 {
		b.Fatalf("Got bad modulo result %d", result)
	}

}

func BenchmarkModulo(b *testing.B) {
	size := getSize(10 * 1000 * 1000)
	var result int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result = int(i % size)
	}
	if result < 0 {
		b.Fatalf("Got bad modulo result %d", result)
	}
}

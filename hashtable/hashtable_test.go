package hashtable

import (
	"fmt"
	"github.com/cespare/xxhash"
	"mcachego/xorshift64star"
	"testing"
	"unsafe"
)

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
	//b.N = 100 * 1000
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

// Run the same test with the Go map API for comparison
func BenchmarkMapStore(b *testing.B) {
	b.ReportAllocs()
	//b.N = 100 * 1000
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
	array := make([]uint64, b.N, b.N)
	prng := xorshift64star.New(1)
	for i := 0; i < b.N; i++ {
		array[i] = prng.Next()
	}
	size := getSize(10 * 1000 * 1000)
	var result int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result = moduloSize(array[i], size)
	}
	if result <= 0 {
		b.Fatalf("Got bad modulo result %d", result)
	}

}

func BenchmarkModulo(b *testing.B) {
	array := make([]uint64, b.N, b.N)
	prng := xorshift64star.New(1)
	for i := 0; i < b.N; i++ {
		array[i] = prng.Next()
	}
	size := getSize(10 * 1000 * 1000)
	var result int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result = int(array[i] % uint64(size))
	}
	if result <= 0 {
		b.Fatalf("Got bad modulo result %d", result)
	}
}

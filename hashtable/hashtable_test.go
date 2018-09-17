package hashtable

import (
	"fmt"
	"mcachego/xorshift64star"
	"testing"
)

func TestHashtable(t *testing.T) {
	size := 10
	h := New(2*size, 4)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		ok := h.Store(key, uintptr(i))
		if !ok {
			t.Fatalf("Failed to store value %v in the hashtable", i)
		}
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok := h.Load(key)
		if !ok {
			t.Fatalf("Failed to find key %v in the hashtable", key)
		}
		if v != uintptr(i) {
			t.Fatalf("Got %v instead of %v from the hashtable", v, i)
		}
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok := h.Remove(key)
		if !ok {
			t.Fatalf("Failed to remove key %v from the hashtable", key)
		}
		if v != uintptr(i) {
			t.Fatalf("Got %v instead of %v from the hashtable", v, i)
		}
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%d", i)
		v, ok := h.Load(key)
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
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i]
		if ok := h.Store(key, uintptr(i)); !ok {
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
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("%d", b.N-i)
	}
	for i := 0; i < b.N; i++ {
		key := keys[i]
		if ok := h.Store(key, uintptr(i)); !ok {
			b.Fatalf("Failed to add item %d, %v", i, key)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i]
		v, ok := h.Load(key)
		if !ok {
			b.Fatalf("Failed to find key %v in the hashtable", key)
		}
		if v != uintptr(i) {
			b.Fatalf("Got %v instead of %v from the hashtable", v, i)
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

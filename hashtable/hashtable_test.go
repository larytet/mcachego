package hashtable

import (
	"fmt"
	"testing"
)

func TestHashtable(t *testing.T) {
	size := 10
	h := New(2*size, 2)
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

func BenchmarkStore(b *testing.B) {
	b.ReportAllocs()
	h := New(2*b.N, 128)
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

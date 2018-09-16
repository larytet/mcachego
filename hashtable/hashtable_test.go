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
}

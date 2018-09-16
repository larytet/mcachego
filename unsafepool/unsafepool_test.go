package unsafepool

import (
	"reflect"
	"testing"
)

type MyData struct {
	a int
	b int
}

func TestPoolAlloc(t *testing.T) {
	pool := New(reflect.TypeOf(new(MyData)), 1)
	if _, ok := pool.Alloc(); !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	if _, ok := pool.Alloc(); ok {
		t.Fatalf("Did not fail on empty pool")
	}
}

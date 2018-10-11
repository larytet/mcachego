package unsafepool

import (
	"reflect"
	"testing"
)

type MyData struct {
	a int
	b int
}

func TestPoolAllocSync(t *testing.T) {
	pool := New(reflect.TypeOf(new(MyData)), 1)
	p, ok := pool.AllocSync()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	if _, ok := pool.AllocSync(); ok {
		t.Fatalf("Did not fail on empty pool")
	}
	if ok := pool.FreeSync(p); !ok {
		t.Fatalf("Failed to free an object to the pool")
	}
}

func TestPoolAlloc(t *testing.T) {
	pool := New(reflect.TypeOf(new(MyData)), 1)
	p, ok := pool.Alloc()
	if !ok {
		t.Fatalf("Failed to allocate an object from the pool")
	}
	if _, ok := pool.Alloc(); ok {
		t.Fatalf("Did not fail on empty pool")
	}
	if ok := pool.Free(p); !ok {
		t.Fatalf("Failed to free an object to the pool")
	}
}

func BenchmarkPoolAllocFree(b *testing.B) {
	b.ReportAllocs()
	poolSize := 1000
	pool := New(reflect.TypeOf(new(MyData)), poolSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < poolSize; j++ {
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
}

// 10ns/allocation - suprisingly expensive
// 32/64 bits compare and swap do not impact the performance
func BenchmarkPoolAlloc(b *testing.B) {
	b.ReportAllocs()
	poolSize := 1000
	pool := New(reflect.TypeOf(new(MyData)), poolSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < poolSize; j++ {
			_, ok := pool.Alloc()
			if !ok {
				b.Fatalf("Failed to allocate an object from the pool %d", i)
			}
		}
		b.StopTimer()
		pool.Reset()
		b.StartTimer()
	}
}

func BenchmarkPoolAllocFreeSync(b *testing.B) {
	b.ReportAllocs()
	poolSize := 1000
	pool := New(reflect.TypeOf(new(MyData)), poolSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < poolSize; j++ {
			p, ok := pool.AllocSync()
			if !ok {
				b.Fatalf("Failed to allocate an object from the pool %d", i)
			}
			ok = pool.FreeSync(p)
			if !ok {
				b.Fatalf("Failed to free an object to the pool %d", i)
			}
		}
	}
}

// 10ns/allocation - suprisingly expensive
// 32/64 bits compare and swap do not impact the performance
func BenchmarkPoolAllocSync(b *testing.B) {
	b.ReportAllocs()
	poolSize := 1000
	pool := New(reflect.TypeOf(new(MyData)), poolSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < poolSize; j++ {
			_, ok := pool.AllocSync()
			if !ok {
				b.Fatalf("Failed to allocate an object from the pool %d", i)
			}
		}
		b.StopTimer()
		pool.Reset()
		b.StartTimer()
	}
}

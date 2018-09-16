## Introduction

This is yet another Go cache. I need the fastest possible implementation with optional synchronizaton. 

* Carefully avoid allocations from the heap in the Store()/Load() API
* Use runtime.nanotime()
* Synchronisation is optional
* All items have the same expiration period
* Minimalistic API
* Eviction only via expiration
* "Unsafe" memory pool 

## Benchmarks

	BenchmarkPoolAlloc-4   	10000000	         9.68 ns/op
	BenchmarkStore-4   	50000000	       292 ns/op
	BenchmarkLoad-4    	50000000	       129 ns/op
	BenchmarkEvict-4   	50000000	       222 ns/op
	BenchmarkAllocStoreEvictFree-4    	10000000	       358 ns/op	       0 B/op	       0 allocs/op


In the pprof the map API dominates the CPU consumption

      flat  flat%   sum%        cum   cum%
     9.03s 29.61% 29.61%      9.52s 31.21%  runtime.mapaccess1_fast64
     8.18s 26.82% 56.43%     13.20s 43.28%  runtime.mapassign_fast64
     5.52s 18.10% 74.52%      5.90s 19.34%  runtime.mapaccess2_fast64

It gives 5-10M cache operations/s on a single core. Round trip allocation from a pool-store in cache-evict from cache-free to the pool requires 350ns. 
A single core system theoretical peak is ~3M events/s. With packet size 64 bytes this code is expected to handle 100Mb/s line.


## Usage

The Cache API is a thin wrapper around Go map[int32]int32 and an expiration queue. The key is a string and data is unsafe.Pointer.

See TestAddCustomType() for usage.


## Similar projects 

* https://github.com/patrickmn/go-cache
* https://github.com/allegro/bigcache
* https://github.com/coocood/freecache
* https://github.com/koding/cache
* https://github.com/sch00lb0y/vegamcache
* https://github.com/OneOfOne/cmap
* https://github.com/golang/groupcache


## Links

* github.com/cespare/xxhash
* https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html

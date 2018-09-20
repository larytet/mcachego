## Introduction

This is yet another Go cache. I need the fastest possible implementation with optional synchronizaton. 

* Target DNS servers and domain names lookup
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

	BenchmarkHashtableStore-4       	20000000	        99.9 ns/op	       0 B/op	       0 allocs/op
	BenchmarkHashtableLoad-4        	20000000	       165 ns/op	       0 B/op	       0 allocs/op
	BenchmarkMapStore-4             	10000000	       144 ns/op	       0 B/op	       0 allocs/op
	BenchmarkRandomMemoryAccess-4   	50000000	        34.5 ns/op

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


## ToDo

I want the hash function to cluster for most popular keys. The idea is that popular lookups can fit a single 4K memory page and rarely trigger a data cache miss.
The simplest way to do this is to keep a small hashtable (cache) for the popular lookups. I can run two lookups - in the main and 
large table and in the small cache. The small cache can implement a very simple and fast hash function. For example, use 2 first characters as a hash

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
* https://dave.cheney.net/2018/05/29/how-the-go-runtime-implements-maps-efficiently-without-generics
* https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf
* https://dzone.com/articles/so-you-wanna-go-fast
* https://www.youtube.com/watch?time_continue=2&v=ncHmEUmJZf4
* https://probablydance.com/2017/02/26/i-wrote-the-fastest-hashtable/
* http://www.cs.cornell.edu/courses/cs3110/2008fa/lectures/lec21.html - start here 
* https://en.wikipedia.org/wiki/Hash_table
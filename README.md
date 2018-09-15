Yet another Go cache. I need the fastest possible implementation with optional synchronizaton

* Carefully avoid allocations from the heap in the Store()/Load() API
* Use runtime.nanotime()
* Synchronisation is optional
* Eviction of expired entries is up to the application
* Minimalistic API
* Eviction only via expiration


Benchmarks:

	BenchmarkPoolAlloc-4   	10000000	         9.68 ns/op
	BenchmarkStore-4   	50000000	       292 ns/op
	BenchmarkLoad-4    	50000000	       129 ns/op
	BenchmarkEvict-4   	50000000	       222 ns/op

It gives 1-2B operations/s on a single core

In the pprof map API dominates the CPU consumption

      flat  flat%   sum%        cum   cum%
     9.03s 29.61% 29.61%      9.52s 31.21%  runtime.mapaccess1_fast64
     8.18s 26.82% 56.43%     13.20s 43.28%  runtime.mapassign_fast64
     5.52s 18.10% 74.52%      5.90s 19.34%  runtime.mapaccess2_fast64

See TestAddCustomType() for usage.
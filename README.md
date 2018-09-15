Yet another Go cache 
I need the fastest possible implementation with optional synchronising

* Carefully avoid allocations from the heap in the Store()/Laod() API
* Use runtime.nanotime()
* Synchronisation is optional
* Eviction of expired entries is up to the application
* Minimalistic API
* Eviction only via expiration


Benchmarks:

	BenchmarkStore-4   	50000000	       292 ns/op
	BenchmarkLoad-4    	50000000	       129 ns/op
	BenchmarkEvict-4   	50000000	       222 ns/op

Yet another Go cache 
I need the fastest possible implementation with optional synchronising

* Allocate items from a pool, carefully avoid allocations from the heap in the Store()/Laod() API
* Use runtime.nanotime()
* Synchronisation is optional
* Eviction of expired entries is up to the application
* Minimalistic API
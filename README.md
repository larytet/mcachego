Yet another Go cache 
I need the fastest possible implementation with optional synchronising

* Allocate items from a pool
* Use runtime.nanotime()
* Synchronisation is optional
* Eviction of expired entries is up to the application
* Minimalistic API
Yet another Go cache 
I need the fastest possible implementation with optional synchronising

* Allocate items from a pool, carefully avoid allocations from the heap in the Store()/Laod() API
* Use runtime.nanotime()
* Synchronisation is optional
* Eviction of expired entries is up to the application
* Minimalistic API
* Eviction only via expiration


Benchmarks:

      flat  flat%   sum%        cum   cum%
     1.58s 28.11% 28.11%      1.66s 29.54%  runtime.mapaccess2_fast64
     0.87s 15.48% 43.59%      0.92s 16.37%  runtime.mapaccess1_fast64
     0.78s 13.88% 57.47%      2.35s 41.81%  dnsProxyWin/mcache.(*Cache).Evict
     0.67s 11.92% 69.40%      1.24s 22.06%  runtime.mapassign_fast64
     0.35s  6.23% 75.62%      0.62s 11.03%  dnsProxyWin/mcache.(*Cache).evict
     0.32s  5.69% 81.32%      0.49s  8.72%  runtime.evacuate_fast64
     0.27s  4.80% 86.12%      2.62s 46.62%  dnsProxyWin/mcache.BenchmarkEvict
     0.23s  4.09% 90.21%      0.23s  4.09%  dnsProxyWin/mcache.(*itemFifo).peek (inline)

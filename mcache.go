package mcache

import (
	//	"log"
	"runtime"
	"sync"
	"unsafe" // I need this for runtime.nanotime()

	"github.com/larytet-go/fifo64"
	"github.com/larytet-go/hashtable"
)

// Object I have three choices here:
//  * Allow the user to specify Object type
//  * Use type Object interface{}
//  * Use uintptr() (truncated to "enough for anybody" 32 bits) to the user defined structures
// 32 bits is not a mistake here, but a sad necessity allowing to reduce data cache miss
// Without generics I will need a separate cache for every user type
// If I use a type safe and GC safe interface{}, somewhere up the stack somebody will have to type assert Object
// and pay 20ns per Load()
// See https://stackoverflow.com/questions/28024884/does-a-type-assertion-type-switch-have-bad-performance-is-slow-in-go
// Can I use unsafe pointers to the users objects and cast to int64?
// See also insane runtime.noescape() discussion
//  in https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/
// The user is expected to allocate pointers from a pool like UnsafePool
type Object uint32

// TimeMs can be an offset from the beginning of the operation
// or truncated result of Nanotime()
// I would use 16 bits if only I could
type TimeMs int32

// GetTime returns a time stamp
// Application is expected to call this function to get "now". The cache API itself does
// not perform any time related calls. Application can call GetTime only once for a
// a bunch of operations
// time.Now() takes 45ns, runtime.nanotime is 20ns
// I can not create an exported symbol with //go:linkname
// I need a wrapper
// Go does not inline functions? https://lemire.me/blog/2017/09/05/go-does-not-inline-functions-when-it-should/
// The wrapper costs 5ns per call
func GetTime() TimeMs {
	res := TimeMs(uint64(nanotime()) / (1000 * 1000))
	return res
}

// Configuration of the cache
type Configuration struct {
	Size       int
	Shards     int
	TTL        TimeMs
	Collisions int
	// Try 50(%) load factor - size of Hashtable 2*Size
	LoadFactor int
}

// Cache keeps internal data
type Cache struct {
	// FIFO of the items to support eviction of the expired entries
	fifo          *fifo64.Fifo
	size          int
	shards        [](*shard)
	shardsMask    uint64
	statistics    *Statistics
	configuration Configuration
}

// Statistics is a placeholder for debug counters
type Statistics struct {
	EvictCalled       uint64
	EvictExpired      uint64
	EvictForce        uint64
	EvictNotExpired   uint64
	EvictLookupFailed uint64
	EvictPeekFailed   uint64
	MaxOccupancy      uint64
}

// New creates a new instance of Cache
// If 'shards' is zero the table will use 2*runtime.NumCPU()
func New(configuration Configuration) *Cache {
	c := new(Cache)

	if configuration.Shards == 0 {
		configuration.Shards = 2 * runtime.NumCPU()
	}
	// Force power of 2
	configuration.Shards = hashtable.GetPower2(configuration.Shards)
	c.shardsMask = uint64(configuration.Shards) - 1
	if configuration.LoadFactor == 0 {
		configuration.LoadFactor = 50
	}
	if configuration.Collisions == 0 {
		configuration.Collisions = 64
	}
	c.configuration = configuration
	c.size = (c.configuration.Size * 100) / c.configuration.LoadFactor
	c.shards = make([]*shard, configuration.Shards, configuration.Shards)
	shardSize := c.size / configuration.Shards
	for i := range c.shards {
		c.shards[i] = &shard{
			table: hashtable.New(shardSize, 64),
		}
	}
	c.Reset()
	return c
}

// Len returns occupancy
func (c *Cache) Len() int {
	return c.fifo.Len()
}

// Size returns accomodations
func (c *Cache) Size() int {
	return c.fifo.Size()
}

// Reset removes all items from the cache
// This API is not thread safe
func (c *Cache) Reset() {
	// Probably faster and more reliable is to allocate everything
	// than try to call delete()
	c.fifo = fifo64.New(c.size)
	for _, shard := range c.shards {
		shard.table.Reset()
	}
	c.statistics = new(Statistics)
}

// Store adds an object to the cache
// This is the single most expensive function in the code - 160ns/op for large tables
func (c *Cache) Store(key uint64, o Object, now TimeMs) bool {
	// Create an entry on the stack, copy 128 bits
	// These two lines of code add 20% overhead
	// because I use map[int]item instead of map[int]int

	// I can save an assignment here by using user prepared items
	// The idea is to require using of the UnsafePool() and pad 64 bits
	// expirationMs to the user structure
	// This is very C/C++ style

	// A temporary variable helps to profile the code
	i := item{o: o, expirationMs: now + c.configuration.TTL}
	iValue := *((*uintptr)(unsafe.Pointer(&i)))

	hash := key
	shardIdx := hash & c.shardsMask
	shard := c.shards[shardIdx]

	// 85% of the CPU cycles are spent here. Go lang map is rather slow
	// Trivial map[int32]int32 requires 90ns to add an entry
	// What about a custom implementation of map? Can I do better than
	// 120ns (400 CPU cycles)?
	shard.mutex.Lock()
	shard.table.Store(key, hash, iValue)
	ok := c.fifo.Add(key)
	count := c.fifo.Len()
	shard.mutex.Unlock()

	if c.statistics.MaxOccupancy < uint64(count) {
		c.statistics.MaxOccupancy = uint64(count)
	}
	return ok
}

// ItemRef is used for direct access to the entries in cache
// If ItemRef is a struct with two 64 bits fields I see 10ns overhead
// Can I return a single 64 bits word?
// hashtableRef can be 32 bits offset from the beginning of the hash
// TBD What if ItemRef is a struct of two 32 bits words?
type ItemRef struct {
	tableIdx uint32
	shardIdx uint32
}

// Load performs lookup in the cache
// Application can use "ref" in calls to EvictByRef()
// Allocation and return of ref costs 10ns/Load Should I use a dedicated API?
func (c *Cache) Load(key uint64) (o Object, ref ItemRef, ok bool) {
	hash := key
	shardIdx := hash & c.shardsMask
	shard := c.shards[shardIdx]

	shard.mutex.RLock()
	iValue, ok, hashtableRef := shard.table.Load(key, hash)
	shard.mutex.RUnlock()
	ref = ItemRef{
		tableIdx: hashtableRef,
		shardIdx: uint32(shardIdx),
	}

	i := *(*item)(unsafe.Pointer(&iValue))
	return i.o, ref, ok
}

// EvictByRef can save some CPU cycles if the application peforms
// lot of lookup-delete cycles
// This API breaks "eviction only by timeout" guarantee
// TODO I can keep the index (or reference) to the FIFO item in the map.
// It will alllow removing the entry from the eviction FIFO as well (mark as nil)
func (c *Cache) EvictByRef(ref ItemRef) {
	shardIdx := ref.shardIdx
	hashtableRef := ref.tableIdx
	// I can save this line (multiplication) if I compose ItemRef from the
	// shard address instead of index
	shard := c.shards[shardIdx]
	shard.mutex.Lock()
	shard.table.RemoveByRef(hashtableRef)
	shard.mutex.Unlock()
}

// Evict an expired - added before time "now" ms - entry
// Evict() will remove at most one entry
// If "force" is true evict the entry even if not expired
// Use force 'true' if you want to expire all entries periodically
func (c *Cache) Evict(now TimeMs, force bool) (o Object, expired bool) {
	c.statistics.EvictCalled++
	o, expired = 0, false
	// If there is a race I will pick a removed entry or fail to pick anything
	// or pick a not initialized ("") key
	key, ok := c.fifo.Pick()
	if ok {
		// I save hashing by keep the object hash in the FIFO instead of the object itself
		// I am going to call Evict() for every Store(). I assume that the Load()
		// performance is more important
		hash := key
		shardIdx := hash & c.shardsMask
		shard := c.shards[shardIdx]

		shard.mutex.Lock()

		if iValue, ok, ref := shard.table.Load(key, hash); ok {
			i := (*item)(unsafe.Pointer(&iValue))
			isExpired := force || ((i.expirationMs - now) <= 0)
			if isExpired {
				c.statistics.EvictExpired++
				if !expired {
					c.statistics.EvictForce++
				}
				c.fifo.Remove()
				shard.table.RemoveByRef(ref)
				o = i.o
				expired = true
			} else {
				c.statistics.EvictNotExpired++
			}
		} else {
			// This is bad - entry is in the eviction FIFO, but not in the hashtable
			// memory leak? Was removed not by eviction?
			// Currently EvictByRef() does not remove entries from the eviction FIFO
			c.statistics.EvictLookupFailed++
			c.fifo.Remove()
		}

		shard.mutex.Unlock()
	} else {
		// Probably expiration FIFO is empty - nothing to do
		c.statistics.EvictPeekFailed++
	}

	return o, expired
}

// GetStatistics returns a snapshot of debug counters
func (c *Cache) GetStatistics() Statistics {
	return *c.statistics
}

//go:noescape
//go:linkname nanotime runtime.nanotime
func nanotime() int64

// GC is going to poll the cache entries. I can try map[init]int and cast int to
// a (unsafe?) pointer in the arrays of strings and structures.
// Inside of the "item" I keep an address of the "item" allocated from a pool
// Insertion into the map[int]int is 20% faster than map[int]item :100ns vs 120ns
// The fastest in the benchmarks is map[string]uintptr
type shard struct {
	table *hashtable.Hashtable
	mutex sync.RWMutex
}

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
// If I keep the item struct small I can avoid memory pools for items
// The benchmark is clear here: copy of a small object is better than allocation
// from a pool and copy the pointer.
type item struct {
	expirationMs TimeMs
	o            Object
}

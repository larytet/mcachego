package mcache

import (
	"github.com/cespare/xxhash"
	"log"
	"mcachego/hashtable"
	"runtime"
	"sync"
	"unsafe" // I need this for runtime.nanotime()
)

// I have three choices here:
//  * Allow the user to specify Object type
//  * Use type Object interface{}
//  * Use uintptr() (truncated to 32 bits) to the user defined structures
// Without generics I will need a separate cache for every user type
// If I use a type safe and GC safe interface{}, somewhere up the stack somebody will have to type assert Object
// and pay 20ns per Load()
// See https://stackoverflow.com/questions/28024884/does-a-type-assertion-type-switch-have-bad-performance-is-slow-in-go
// Can I use unsafe pointers to the users objects and cast to int64?
// See also insane runtime.noescape() discussion
//  in https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/
// The user is expected to allocate pointers from a pool like UnsafePool
type Object uint32

// Can be an offset from the beginning of the operation
// or truncated result of Nanotime()
type TimeMs int32

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
// If I keep the item struct small I can avoid memory pools for items
// I want a benchmark here: copy vs custom memory pool
type item struct {
	expirationMs TimeMs
	o            Object
}

type itemFifo struct {
	head int
	tail int
	data []string
	size int
}

func newFifo(size int) *itemFifo {
	s := new(itemFifo)
	s.data = make([]string, size+1, size+1)
	s.size = size
	s.head = 0
	s.tail = 0
	return s
}

func (s *itemFifo) inc(v int) int {
	if v < s.size {
		v += 1
	} else {
		v = 0
	}
	return v
}

func (s *itemFifo) add(key string) (ok bool) {
	newTail := s.inc(s.tail)
	if s.head != newTail {
		s.data[s.tail] = key
		s.tail = newTail
		return true
	} else {
		return false
	}
}

func (s *itemFifo) remove() (key string, ok bool) {
	newHead := s.inc(s.head)
	if s.head != s.tail {
		key = s.data[s.head]
		s.head = newHead
		return key, true
	} else {
		return key, false
	}
}

// I assume that this API is "reasonably" tread safe. Will not cause
// problems if there is a race
// s.head is modified by remove() and is an atomic operation
// I do not care about valifity of s.tai
func (s *itemFifo) pick() (key string, ok bool) {
	if s.head != s.tail {
		key = s.data[s.head]
		return key, true
	} else {
		return key, false
	}
}

func (s *itemFifo) Len() int {
	if s.head <= s.tail {
		return s.tail - s.head
	} else {
		return s.size - s.head + s.tail
	}
}

//go:noescape
//go:linkname nanotime runtime.nanotime
func nanotime() int64

// time.Now() is 45ns, runtime.nanotime is 20ns
// I can not create an exported symbol with //go:linkname
// I need a wrapper
// Go does not inline functions? https://lemire.me/blog/2017/09/05/go-does-not-inline-functions-when-it-should/
// The wrapper costs 5ns per call
func GetTime() TimeMs {
	res := TimeMs(uint64(nanotime()) / (1000 * 1000))
	log.Printf("Time %d", res)
	return res
}

type Statistics struct {
	EvictCalled       uint64
	EvictExpired      uint64
	EvictForce        uint64
	EvictNotExpired   uint64
	EvictLookupFailed uint64
	EvictPeekFailed   uint64
	MaxOccupancy      uint64
}

// GC is going to poll the cache entries. I can try map[init]int and cast int to
// a (unsafe?) pointer in the arrays of strings and structures.
// Inside of the "item" I keep an address of the "item" allocated from a pool
// Insertion into the map[int]int is 20% faster than map[int]item :100ns vs 120ns
// The fastest in the benchmarks is map[string]uintptr
type shard struct {
	table *hashtable.Hashtable
	mutex sync.RWMutex
}

type Cache struct {
	ttl TimeMs
	// FIFO of the items to support eviction of the expired entries
	fifo       *itemFifo
	size       int
	shards     []shard
	shardsMask uint64
	statistics *Statistics
}

// Create a new instance of Cache
// If 'shards' is zero the table will use 2*runtime.NumCPU()
func New(size int, shards int, ttl TimeMs) *Cache {
	if shards == 0 {
		shards = 2 * runtime.NumCPU()
	}
	shards = hashtable.GetPower2(shards)
	c := new(Cache)
	c.size, c.ttl = size, ttl
	c.shards = make([]shard, shards, shards)
	c.shardsMask = uint64(len(c.shards)) - 1
	shardSize := size / shards
	for i, _ := range c.shards {
		c.shards[i].table = hashtable.New(shardSize, 32)
	}
	c.Reset()
	return c
}

// Occupancy
func (c *Cache) Len() int {
	return c.fifo.Len()
}

// Accomodations
func (c *Cache) Size() int {
	return c.fifo.size
}

// This API is not thread safe
func (c *Cache) Reset() {
	// Probably faster and more reliable is to allocate everything
	// than try to call delete()
	c.fifo = newFifo(c.size)
	for _, shard := range c.shards {
		shard.table.Reset()
	}
	c.statistics = new(Statistics)
}

// Add an object to the cache
// This is the single most expensive function in the code - 160ns/op
func (c *Cache) Store(key string, o Object, now TimeMs) bool {
	// Create an entry on the stack, copy 128 bits
	// These two lines of code add 20% overhead
	// because I use map[int]item instead of map[int]int

	// I can save an assignment here by using user prepared items
	// The idea is to require using of the UnsafePool() and pad 64 bits
	// expirationMs to the user structure
	// This is very C/C++ style

	// A temporary variable helps to profile the code
	i := item{o: o, expirationMs: now + c.ttl}
	iValue := *((*uintptr)(unsafe.Pointer(&i)))
	log.Printf("Store item %x %d %d", iValue, i.expirationMs, now)

	hash := xxhash.Sum64String(string(key))
	shardIdx := hash & c.shardsMask
	shard := &c.shards[shardIdx]

	// 85% of the CPU cycles are spent here. Go lang map is rather slow
	// Trivial map[int32]int32 requires 90ns to add an entry
	// What about a custom implementation of map? Can I do better than
	// 120ns (400 CPU cycles)?
	shard.mutex.Lock()
	shard.table.Store(key, hash, iValue)
	ok := c.fifo.add(key)
	count := c.fifo.Len()
	shard.mutex.Unlock()

	if c.statistics.MaxOccupancy < uint64(count) {
		c.statistics.MaxOccupancy = uint64(count)
	}
	return ok
}

// Lookup in the cache
func (c *Cache) Load(key string) (o Object, ok bool) {
	hash := xxhash.Sum64String(string(key))
	shardIdx := hash & c.shardsMask
	shard := &c.shards[shardIdx]

	shard.mutex.RLock()
	iValue, ok, _ := shard.table.Load(key, hash)
	shard.mutex.RUnlock()

	i := *(*item)(unsafe.Pointer(&iValue))
	return i.o, ok
}

// Evict an expired - added before time "now" ms - entry
// If "force" is true evict the entry even if not expired
func (c *Cache) Evict(now TimeMs, force bool) (o Object, expired bool) {
	c.statistics.EvictCalled += 1
	o, expired = 0, false
	// If there is a race I will pick a removed entry or fail to pick anything
	// or pick a not initialized ("") key
	key, ok := c.fifo.pick()
	if ok {
		hash := xxhash.Sum64String(string(key))
		shardIdx := hash & c.shardsMask
		shard := &c.shards[shardIdx]

		shard.mutex.Lock()

		// I can save hashing if I keep the hash in the FIFO
		if iValue, ok, ref := shard.table.Load(key, hash); ok {
			i := (*item)(unsafe.Pointer(&iValue))
			log.Printf("Pick item %x exp=%d now=%d", ref, i.expirationMs, now)
			expired := ((i.expirationMs - now) < 0)
			if expired || force {
				c.statistics.EvictExpired += 1
				if !expired {
					c.statistics.EvictForce += 1
				}
				c.fifo.remove()
				shard.table.RemoveByRef(ref)
				o, expired = i.o, true
			} else {
				c.statistics.EvictNotExpired += 1
			}
		} else {
			// This is bad - entry is in the eviction FIFO, but not in the map
			// memory leak?
			c.statistics.EvictLookupFailed += 1
		}

		shard.mutex.Unlock()
	} else {
		// Probably expiration FIFO is empty - nothing to do
		c.statistics.EvictPeekFailed += 1
	}

	return o, expired
}

func (c *Cache) GetStatistics() Statistics {
	return *c.statistics
}

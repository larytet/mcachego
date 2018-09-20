package mcache

import (
	"sync"
	_ "unsafe" // I need this for runtime.nanotime()
)

// a string key and int32 key have roughly the same benchmarks (!?)
type Key string

// I have three  choices here:
//  * Allow the user to specify Object type
//  * Use type Object interface{}
//  * Use uintptr() to the user defined structures
// Without generics I will need a separate cache for every user type
// If I use a type safe and GC safe interface{}, somewhere up the stack somebody will have to type assert Object
// and pay 20ns per Load()
// See https://stackoverflow.com/questions/28024884/does-a-type-assertion-type-switch-have-bad-performance-is-slow-in-go
// Can I use unsafe pointers to the users objects and cast to int64?
// See also insane runtime.noescape() discussion
//  in https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/
// The user is expected to allocate pointers from a pool like UnsafePool
type Object uintptr

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
// If I keep the item struct small I can avoid memory pools for items
// I want a benchmark here: copy vs custom memory pool
// If I use 32 bits for both  fields the smaller item delivers ~20% better
// performance. I can do it if I assume timeout in ms and an offset instead of absolute
// address for "o"
type item struct {
	expirationNs int64
	o            Object
}

// A naive cyclic buffer
// "head == tail" means empty
type itemFifo struct {
	head int
	tail int
	data []Key
	size int
}

func newFifo(size int) *itemFifo {
	s := new(itemFifo)
	s.data = make([]Key, size+1, size+1)
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

func (s *itemFifo) add(key Key) (ok bool) {
	newTail := s.inc(s.tail)
	if s.head != newTail {
		s.data[s.tail] = key
		s.tail = newTail
		return true
	} else {
		return false
	}
}

func (s *itemFifo) remove() (key Key, ok bool) {
	newHead := s.inc(s.head)
	if s.head != s.tail {
		key = s.data[s.head]
		s.head = newHead
		return key, true
	} else {
		return key, false
	}
}

func (s *itemFifo) peek() (key Key, ok bool) {
	if s.head != s.tail {
		key = s.data[s.head]
		return key, true
	} else {
		return key, false
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
func Nanotime() int64 {
	return nanotime()
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

type Cache struct {
	// GC is going to poll the cache entries. I can try map[init]int and cast int to
	// a (unsafe?) pointer in the arrays of strings and structures.
	// Inside of the "item" I keep an address of the "item" allocated from a pool
	// Insertion into the map[int]int is 20% faster than map[int]item :100ns vs 120ns
	// The fastest in the benchmarks is map[string]uintptr
	data  map[Key]item
	mutex sync.RWMutex
	ttl   int64
	// FIFO of the items to support eviction of the expired entries
	fifo       *itemFifo
	size       int
	statistics *Statistics
}

var ns = int64(1000 * 1000)

// I can not rely on time.Time API: every call to time.Now() is 40ns
// Nanotime() completes in 10-12ns
func New(size int, ttl int64) *Cache {
	c := new(Cache)
	c.size = size
	c.ttl = ns * ttl
	c.Reset()
	return c
}

// Occupancy
func (c *Cache) Len() int {
	return len(c.data)
}

// Accomodations
func (c *Cache) Size() int {
	return c.fifo.size
}

func (c *Cache) Reset() {
	// Probably faster and more reliable to allocate everything
	// than try to call delete()
	c.fifo = newFifo(c.size)
	c.data = make(map[Key]item, c.size)
	c.statistics = new(Statistics)
}

// Add an object to the cache
// This is the single most expensive function in the code - 160ns/op
func (c *Cache) Store(key Key, o Object, now int64) bool {
	// Create an entry on the stack, copy 128 bits
	// These two lines of code add 20% overhead
	// because I use map[int]item instead of map[int]int

	// I can save an assignment here by using user prepared items
	// The idea is to require using of the UnsafePool() and pad 64 bits
	// expirationNs to the user structure
	// This is very C/C++ style

	// A temporary variable helps to profile the code
	i := item{o: o, expirationNs: now + c.ttl}

	// 85% of the CPU cycles are spent here. Go lang map is rather slow
	// Trivial map[int32]int32 requires 90ns to add an entry
	// What about a custom implementation of map? Can I do better than
	// 120ns (400 CPU cycles)?
	c.data[key] = i
	ok := c.fifo.add(key)
	count := uint64(len(c.data))
	if c.statistics.MaxOccupancy < count {
		c.statistics.MaxOccupancy = count
	}
	return ok
}

// For highly congested systems most choose sharding. Should I?
func (c *Cache) StoreSync(key Key, o Object) bool {
	c.mutex.Lock()
	ok := c.Store(key, o, nanotime())
	c.mutex.Unlock()
	return ok
}

// Lookup in the cache
func (c *Cache) Load(key Key) (o Object, ok bool) {
	i, ok := c.data[key]
	return i.o, ok
}

func (c *Cache) LoadSync(key Key) (o Object, ok bool) {
	c.mutex.RLock()
	o, ok = c.Load(key)
	c.mutex.RUnlock()
	return o, ok
}

func (c *Cache) evict(now int64, force bool) (o Object, expired bool) {
	c.statistics.EvictCalled += 1
	if key, ok := c.fifo.peek(); ok {
		if i, ok := c.data[key]; ok {
			expired := ((i.expirationNs - now) < 0)
			if expired || force {
				c.statistics.EvictExpired += 1
				if !expired {
					c.statistics.EvictForce += 1
				}
				c.fifo.remove()
				delete(c.data, key)
				return i.o, true
			} else {
				c.statistics.EvictNotExpired += 1
			}
		} else {
			// This is bad - entry is in the eviction FIFO, but not in the map
			// memory leak?
			c.statistics.EvictLookupFailed += 1
		}
	} else {
		// Probably expiration FIFO is empty - nothing to do
		c.statistics.EvictPeekFailed += 1
	}
	return 0, false
}

func (c *Cache) GetStatistics() Statistics {
	return *c.statistics
}

// Evict an expired - added before time "now" ms - entry
// If "force" is true evict the entry even if not expired yet entry
func (c *Cache) Evict(now int64, force bool) (o Object, expired bool, nextExpirationNs int64) {
	nextExpirationNs = 0
	o, expired = c.evict(now, force)
	key, ok := c.fifo.peek()
	if ok {
		i := c.data[key]
		nextExpirationNs = i.expirationNs - now
	}
	return o, expired, nextExpirationNs
}

func (c *Cache) EvictSync(now int64, force bool) (o Object, expired bool, nextExpirationNs int64) {
	c.mutex.Lock()
	o, expired, nextExpirationNs = c.Evict(now, force)
	c.mutex.Unlock()
	return o, expired, nextExpirationNs
}

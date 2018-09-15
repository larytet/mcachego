package mcache

import (
	_ "fmt"
	"sync"
	_ "time"
	_ "unsafe"
)

type Key int64
type Object int64

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
// If I keep the item struct small I can avoid memory pools for items
type item struct {
	// somewhere up the stack somebody will have to type assert and pay 20ns
	// https://stackoverflow.com/questions/28024884/does-a-type-assertion-type-switch-have-bad-performance-is-slow-in-go
	// Can I use unsafe pointers here?
	// Object     interface{}
	o          Object
	expiration int64
}

type itemFifo struct {
	head int
	tail int
	data []Key
}

func newFifo(size int64) *itemFifo {
	s := new(itemFifo)
	s.data = make([]Key, size, size)
	s.head = 0
	s.tail = 0
	return s
}

func (s *itemFifo) inc(v int) int {
	v += 1
	if v >= len(s.data) {
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

type Cache struct {
	// GC is going to poll the cache entries. I can try map[init]int and cast int to
	// a (unsafe?) pointer in the arrays of strings and structures.
	// I keep an address of the "item" allocated from a pool
	data  map[Key]item
	mutex sync.RWMutex
	ttl   int64
	// pool of preallocted items
	fifo *itemFifo
}

var ns = int64(1000 * 1000)

func New(size int64, ttl int64) *Cache {
	c := new(Cache)
	c.data = make(map[Key]item)
	c.ttl = ns * ttl
	c.fifo = newFifo(size)
	return c
}

func (c *Cache) Len() int {
	return len(c.data)
}

func (c *Cache) Store(key Key, o Object, now int64) {
	c.data[key] = item{o: o, expiration: now + c.ttl}
	c.fifo.add(key)
}

func (c *Cache) StoreSync(key Key, o Object) {
	c.mutex.Lock()
	c.Store(key, o, nanotime())
	c.mutex.Unlock()
}

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

func (c *Cache) evict(now int64) (expired bool) {
	key, ok := c.fifo.peek()
	if ok {
		i := c.data[key]
		if (i.expiration - now) < 0 {
			c.fifo.remove()
			delete(c.data, key)
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

func (c *Cache) Evict(now int64) (nextExpiration int64, expired bool) {
	nextExpiration = 0
	expired = c.evict(now)
	key, ok := c.fifo.peek()
	if ok {
		i := c.data[key]
		nextExpiration = i.expiration - now
	}
	return nextExpiration, expired
}

func (c *Cache) EvictSync(now int64) (nextExpiration int64, expired bool) {
	c.mutex.Lock()
	nextExpiration, expired = c.Evict(now)
	c.mutex.Unlock()
	return nextExpiration, expired
}

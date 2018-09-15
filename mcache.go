package mcache

import (
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe" // I need this for runtime.nanotime()
)

type Key int64

// I have three  choices here:
//  * Allow the user to specify Object type
//  * Use interface{}
//  * Use uintptr() to the user defined structures
// If I use a type and GC safe interface{} somewhere up the stack somebody will have to type assert Object and pay 20ns
// See https://stackoverflow.com/questions/28024884/does-a-type-assertion-type-switch-have-bad-performance-is-slow-in-go
// Can I use unsafe pointers to the users objects and cast to int64?
// See also insane noescape() in https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/
// Object     interface{}
type Object uintptr

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
// If I keep the item struct small I can avoid memory pools for items
// I want a benchmark here: copy vs custom memory pool
type item struct {
	o          Object
	expiration int64
}

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

type Cache struct {
	// GC is going to poll the cache entries. I can try map[init]int and cast int to
	// a (unsafe?) pointer in the arrays of strings and structures.
	// I keep an address of the "item" allocated from a pool
	data  map[Key]item
	mutex sync.RWMutex
	ttl   int64
	// FIFO of the items to support eviction of the expired entries
	fifo *itemFifo
	size int
}

var ns = int64(1000 * 1000)

func New(size int, ttl int64) *Cache {
	c := new(Cache)
	c.size = size
	c.data = make(map[Key]item, size)
	c.ttl = ns * ttl
	c.fifo = newFifo(size)
	return c
}

func (c *Cache) Len() int {
	return len(c.data)
}

func (c *Cache) Reset() {
	c.fifo = newFifo(c.size)
	c.data = make(map[Key]item, c.size)
}

func (c *Cache) Store(key Key, o Object, now int64) bool {
	c.data[key] = item{o: o, expiration: now + c.ttl}
	ok := c.fifo.add(key)
	return ok
}

func (c *Cache) StoreSync(key Key, o Object) bool {
	c.mutex.Lock()
	ok := c.Store(key, o, nanotime())
	c.mutex.Unlock()
	return ok
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

func (c *Cache) evict(now int64) (o Object, expired bool) {
	key, ok := c.fifo.peek()
	if ok {
		i := c.data[key]
		if (i.expiration - now) < 0 {
			c.fifo.remove()
			delete(c.data, key)
			return i.o, true
		} else {
			return 0, false
		}
	} else {
		return 0, false
	}
}

func (c *Cache) Evict(now int64) (o Object, expired bool, nextExpiration int64) {
	nextExpiration = 0
	o, expired = c.evict(now)
	key, ok := c.fifo.peek()
	if ok {
		i := c.data[key]
		nextExpiration = i.expiration - now
	}
	return o, expired, nextExpiration
}

func (c *Cache) EvictSync(now int64) (o Object, expired bool, nextExpiration int64) {
	c.mutex.Lock()
	o, expired, nextExpiration = c.Evict(now)
	c.mutex.Unlock()
	return o, expired, nextExpiration
}

// I am replacing the whole Go  memory managemnt, It is safer (no pun)
// to provide
// an API for the application which demos a HowTo
// Application needs a pool to allocate users objects
// and keep the objects in the cache
// This is a lock free memory pool of objects of the same size
type Pool struct {
	top         int64
	stack       []unsafe.Pointer
	data        []byte
	objectSize  int
	objectCount int
}

func NewPool(t reflect.Type, objectCount int) (p *Pool) {
	objectSize := int(unsafe.Sizeof(t))
	p = new(Pool)
	p.objectSize, p.objectCount = objectSize, objectCount
	p.data = make([]byte, objectSize*objectCount, objectSize*objectCount)
	p.stack = make([]unsafe.Pointer, objectCount, objectCount)
	p.Reset()
	return p
}

func (p *Pool) Reset() {
	for i := 0; i < p.objectCount; i += 1 {
		p.stack[i] = unsafe.Pointer(&p.data[i*p.objectSize])
	}
	p.top = int64(p.objectCount)
}

func (p *Pool) Alloc() (ptr unsafe.Pointer, ok bool) {
	for p.top > 0 {
		top := p.top
		if atomic.CompareAndSwapInt64(&p.top, top, top-1) {
			// success, I decremented p.top
			return p.stack[top-1], true
		}
	}
	return nil, false
}

func (p *Pool) Free(ptr unsafe.Pointer) {
	for {
		top := p.top
		if atomic.CompareAndSwapInt64(&p.top, top, top+1) {
			// success, I incremented p.top
			p.stack[top] = ptr
			return
		}
	}
}

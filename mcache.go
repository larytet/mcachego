import (
	"container/list"
	"dnsProxyWin/utils"
	"sync"
	"sync/atomic"
	"time"
)

// Straight from https://github.com/patrickmn/go-cache
// Read also https://allegro.tech/2016/03/writing-fast-cache-service-in-go.html
type item struct {
	Object     interface{}
	Expiration int64
}

type itemStack struct {
	top  int
	data []*item
}

func newStack(size int64) *itemStack {
	s := new(itemStack)
	s.data = make([]*item, size, size)
	for i := 0; i < size; i++ {
		data[i] = &item{nil, 0}
	}
	s.top = size
	return s
}

func (s *itemStack) pop() (*item, bool) {
	if s.top > 0 {
		s.top -= 1
		return s.data[s.top], true
	} else {
		return nil, false
	}
}

func (s *itemStack) push() bool {
	if s.top < len(s.data) {
		s.data[s.top] = item
		s.top += 1
		return true
	} else {
		return false
	}
}

type Cache struct {
	// GC is going to poll the cache entries. I can try map[init]int and cast int to
	// a (unsafe?) pointer in the arrays of strings and structures.
	// I keep an address of the "item" allocated from a pool
	data  map[string]int64
	queue cyclicBufferItems
	mutex sync.RWMutex
	ttl   int64
	// pool of preallocted items
	pool *itemStack
}

func New(size int64, ttl int64) *Cache {
	c := new(Cache)
	c.queue.Init()
	c.data = make(map[string]*PolicyCacheEntry)
	c.ttl = ttl
	c.pool = newStack(size)
	return c
}

func (c *Cache) Len() int {
	return c.queue.Len()
}

func (c *Cache) Store(key string, o interface{}) {
}

func (c *Cache) StoreSync(key string, o interface{}) {
	c.mutex.Lock()
	c.Store(key, o)
	c.mutex.Unlock()
}

func (c *Cache) Load(key string) (o interface{}, ok bool) {
}

func (c *Cache) LoadSync(key string) (o interface{}, ok bool) {
	c.mutex.Lock()
	c.Load(key)
	c.mutex.Unlock()
	return nil, false
}

func (c *Cache) Remove(now int64) (key string, expired bool) {
}

func (c *Cache) RemoveSync(now int64) (key string, expired bool) {
}

func (c *Cache) evict(now int64) (nextExpiration int64, expired bool) {
}

func (c *Cache) Evict(now int64) (nextExpiration int64, expired bool) {
	c.mutex.Lock()
	c.evict()
	c.mutex.Unlock()
}

func (c *Cache) EvictAll(now int64) (nextExpiration int64, expired bool) {
	c.mutex.Lock()
	for {
		c.evict()
	}
	c.mutex.Unlock()
}

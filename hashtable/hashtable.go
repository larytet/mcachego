package hashtable

import (
	"github.com/cespare/xxhash"
	"log"
	"sync"
)

// An alternative for Go runtime implemenation of map[string]uintptr
// Requires to specify maximum number of hash collisions at the initialization time
// Insert can fail if there are too many collisions
// The goal is 3x improvement and true O(1) performance
// See also:
// * https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf
// * https://github.com/larytet/emcpp/blob/master/src/HashTable.h

type Statistics struct {
	Store          uint64
	StoreSuccess   uint64
	StoreCollision uint64
	Load           uint64
	LoadSuccess    uint64
	LoadFailed     uint64
	FindSuccess    uint64
	FindCollision  uint64
	FindFailed     uint64
	Remove         uint64
	RemoveSuccess  uint64
	RemoveFailed   uint64
}

type item struct {
	// I keep pointers to strings. This is bad for GC - triggers runtime.scanobject()
	// Can I copy the string to a large buffer and use an index in the buffer instead
	// of the string address? What are alternatives?
	// I can also rely on 64 bits (or 128 bits) hash and report collisions
	// key string
	// 64 bits hash of the key for quick compare
	hash  uint64
	value uintptr
	// I can "state int64" instead and atomic compareAndSwap to allocate the entry
	inUse bool
}

func (i *item) reset() {
	i.inUse = false
	//i.key = ""
	i.hash = 0
}

// This is copy&paste from https://github.com/larytet/emcpp/blob/master/src/HashTable.h
type Hashtable struct {
	size          int
	maxCollisions int
	count         int
	statistics    Statistics
	// Number of collisions in the table
	collisions int
	// Resize automatically if not zero
	ResizeFactor int
	data         []item
	mutex        sync.Mutex
	// Assume 64 bits reliable
	RelyOnHash bool
}

func New(size int, maxCollisions int) (h *Hashtable) {
	h = new(Hashtable)
	h.size = size
	h.maxCollisions = maxCollisions
	// allow collision for the last entry in the table
	count := size + maxCollisions
	h.data = make([]item, count, count)
	return h
}

func (h *Hashtable) Reset() {
	// GC will remove strings anyway. Better I will perform the loop
	for _, it := range h.data {
		it.reset()
	}
}

// Store a key:value pair in the hashtable
func (h *Hashtable) Store(key string, value uintptr) bool {
	h.statistics.Store += 1
	hash := xxhash.Sum64String(key)
	if h.RelyOnHash {
		key = ""
	}
	index := int(hash % uint64(h.size))
	collisions := 0
	for collisions := 0; collisions < h.maxCollisions; collisions++ {
		it := &h.data[index]
		if !it.inUse {
			h.statistics.StoreSuccess += 1
			it.inUse = true
			//it.key = key
			it.hash = hash
			it.value = value
			// This store added one collision
			if collisions > 0 {
				h.collisions += 1
			}
			return true
		} else {
			// should be  a rare occasion
			h.statistics.StoreCollision += 1
			index += 1
		}
	}
	log.Printf("Faied to add %v:%v, col=%d:%d, index=%d hash=%d", key, value, collisions, h.collisions, index, hash)
	return false
}

func (h *Hashtable) find(key string) (index int, ok bool, collisions int) {
	hash := xxhash.Sum64String(key)
	if h.RelyOnHash {
		key = ""
	}
	index = int(hash % uint64(h.size))
	collisions = 0
	for collisions < h.maxCollisions {
		it := h.data[index]
		if it.inUse && (hash == it.hash) { //&& (key == it.key)
			h.statistics.FindSuccess += 1
			return index, true, collisions
		} else {
			// should be  a rare occasion
			h.statistics.FindCollision += 1
			collisions += 1
			index += 1
		}
	}
	h.statistics.FindFailed += 1
	return 0, false, collisions
}

func (h *Hashtable) Load(key string) (value uintptr, ok bool) {
	h.statistics.Load += 1
	if index, ok, _ := h.find(key); ok {
		h.statistics.LoadSuccess += 1
		it := h.data[index]
		value = it.value
		return value, true
	}
	h.statistics.LoadFailed += 1
	return 0, false
}

func (h *Hashtable) Remove(key string) (value uintptr, ok bool) {
	h.statistics.Remove += 1
	if index, ok, collisions := h.find(key); ok {
		h.statistics.RemoveSuccess += 1
		if collisions > 0 {
			h.collisions -= 1
		}
		// TODO I can move all colliding items left here and find a match
		// faster next time.

		// I can save some races by paying a copy
		// it := h.data[index]
		// it.reset()
		// h.data[index] = it
		it := &h.data[index]
		value = it.value
		it.reset()
		return value, true
	}
	h.statistics.RemoveFailed += 1
	return 0, false
}

// Resize the table. Usually you call the function to make
// the table larger and reduce number of collisions
func (h *Hashtable) Resize(factor int) bool {
	return false
}

// Returns number of collisions in the table
func (h *Hashtable) Collisions() int {
	return h.collisions
}

package hashtable

import (
	"github.com/cespare/xxhash"
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
	LoadCollision  uint64
}

type item struct {
	// I keep pointers to strings. This is bad for GC.
	// Can I copy the string to a large buffer and use an index in the buffer instead
	// of the string address? What are alternatives?
	// I can also rely on 64 bits (or 128 bits) hash and report collisions
	key string
	// 64 bits hash of the key for quick compare
	hash  uint64
	value uintptr
	inUse bool
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
	return h
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
	for collisions < h.maxCollisions {
		it := h.data[index]
		if !it.inUse {
			h.statistics.StoreSuccess += 1
			h.data[index] = item{key: key, hash: hash, value: value, inUse: true}
			// This store added one collision
			if collisions > 0 {
				h.collisions += 1
			}
			return true
		} else {
			collisions += 1
			h.statistics.StoreCollision += 1
			index += 1
		}
	}
	return false
}

func (h *Hashtable) Load(key string) (value uintptr, ok bool) {
	h.statistics.Load += 1
	hash := xxhash.Sum64String(key)
	if h.RelyOnHash {
		key = ""
	}
	index := int(hash % uint64(h.size))
	collisions := 0
	for collisions < h.maxCollisions {
		item := h.data[index]
		if item.inUse && (hash == item.hash) && (key == item.key) {
			h.statistics.LoadSuccess += 1
			return item.value, true
		} else {
			h.statistics.LoadCollision += 1
			collisions += 1
			index += 1
		}
	}
	return 0, false
}

func (h *Hashtable) Remove(key string) (value uintptr, ok bool) {
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

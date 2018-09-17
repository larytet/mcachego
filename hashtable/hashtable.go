package hashtable

import (
	//	"encoding/binary"
	"github.com/cespare/xxhash"
	"log"
	"sync"
)

// An alternative for Go runtime implemenation of map[string]uintptr
// Requires to specify maximum number of hash collisions at the initialization time
// Insert can fail if there are too many collisions
// The goal is 3x improvement and true O(1) performance (what about cache miss?)
// See also:
// * https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf
// * https://github.com/larytet/emcpp/blob/master/src/HashTable.h

type Statistics struct {
	Store          uint64
	StoreSuccess   uint64
	StoreCollision uint64
	MaxCollisions  uint64
	Load           uint64
	LoadSuccess    uint64
	LoadSwap       uint64
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
	i.value = 0
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
	size = getPower2(size)
	//size = getPrime(size)
	h.size = size
	h.maxCollisions = maxCollisions
	// allow collision for the last entry in the table
	count := size + maxCollisions
	h.data = make([]item, count, count)
	h.Reset()
	return h
}

func (h *Hashtable) Reset() {
	// GC will remove strings anyway. Better I will perform the loop
	for i := 0; i < len(h.data); i++ {
		h.data[i].reset()
	}
}

type hashContext struct {
	index     int
	firstHash uint64
	key       string
	size      int
	step      int
}

// This is naive. What I want to do here is sharding based on 8 LSBs
// Bad choise of "size" will cause collisions
func (hc *hashContext) nextIndex() (index int) {
	if hc.step == 0 {
		// Collision attack is possible here
		// I should rotate hash functions
		// See also https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
		hash := xxhash.Sum64String(hc.key)
		hc.step += 1
		hc.firstHash = hash
		// 20% of the function is here. I want a switch/case with dividing by const
		// and let the compiler optimize modulo
		// See also https://probablydance.com/2017/02/26/i-wrote-the-fastest-hashtable/
		hc.index = int(hash % uint64(hc.size))
	} else {
		// rehash the hash ?
		//bs := []byte{0, 0, 0, 0, 0, 0, 0, 0} // https://stackoverflow.com/questions/16888357/convert-an-integer-to-a-byte-array
		//binary.LittleEndian.PutUint64(bs, hc.hash)
		//hash = xxhash.Sum64(bs)
		hc.index += 1
	}
	return hc.index
}

// Store a key:value pair in the hashtable
func (h *Hashtable) Store(key string, value uintptr) bool {
	h.statistics.Store += 1

	hc := hashContext{key: key, size: h.size}
	index := hc.nextIndex()
	var collisions int
	for collisions = 0; collisions < h.maxCollisions; collisions++ {
		// most expensive line in the code - likely a cache miss here
		it := &h.data[index]
		// The next line - first fetch - consumes lot of CPU cycles. Why?
		if !it.inUse {
			// I can swap the first item in the "chain" with this item and improve lookup time for freshly inserted items
			// See https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
			h.statistics.StoreSuccess += 1
			it.inUse = true
			//it.key = key
			it.hash = hc.firstHash
			it.value = value
			// This store added one collision
			if collisions > 0 {
				h.collisions += 1
			}
			if h.statistics.MaxCollisions < uint64(collisions) {
				h.statistics.MaxCollisions = uint64(collisions)
			}
			return true
		} else {
			// should be  a rare occasion
			h.statistics.StoreCollision += 1
			index = hc.nextIndex()
		}
	}
	log.Printf("Failed to add %v:%v, col=%d:%d, hash=%x size=%d", key, value, collisions, h.collisions, hc.firstHash, h.size)
	return false
}

func (h *Hashtable) find(key string) (index int, collisions int, chainStart int, ok bool) {
	hc := hashContext{key: key, size: h.size}
	index = hc.nextIndex()
	//log.Printf("Find hash %d", hc.firstHash)
	chainStart = index
	for collisions = 0; collisions < h.maxCollisions; collisions++ {
		it := h.data[index]
		//log.Printf("Find firstHash %d, act %d", hc.firstHash, it.hash)
		if it.inUse && (hc.firstHash == it.hash) { //&& (key == it.key)
			h.statistics.FindSuccess += 1
			//log.Printf("%v", h.data)
			return index, collisions, chainStart, true
		} else {
			// should be  a rare occasion
			h.statistics.FindCollision += 1
			index = hc.nextIndex()
		}
	}
	//log.Printf("%v", h.data)
	h.statistics.FindFailed += 1
	return 0, collisions, chainStart, false
}

func (h *Hashtable) Load(key string) (value uintptr, ok bool) {
	h.statistics.Load += 1
	if index, collisions, chainStart, ok := h.find(key); ok {
		h.statistics.LoadSuccess += 1
		it := h.data[index]
		// Swap the found item with the first in the "chain" and improve lookup next time
		// due to CPU caching
		if collisions > 0 {
			//log.Printf("Swap %v[%d] %v[%d]", h.data[index], index, h.data[chainStart], chainStart)
			h.data[index] = h.data[chainStart]
			h.data[chainStart] = it
			h.statistics.LoadSwap += 1
		}
		value = it.value
		return value, true
	}
	h.statistics.LoadFailed += 1
	return 0, false
}

func (h *Hashtable) Remove(key string) (value uintptr, ok bool) {
	h.statistics.Remove += 1
	if index, collisions, _, ok := h.find(key); ok {
		h.statistics.RemoveSuccess += 1
		if collisions > 0 {
			h.collisions -= 1
		}
		// TODO I can move all colliding items left and find a match
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

// Using 2^n-1 I have 20% collisions rate
// A real prime does not improve much
// See https://stackoverflow.com/questions/21854191/generating-prime-numbers-in-go
// https://github.com/agis/gofool/blob/master/atkin.go
func getPower2(N int) int {
	v := 1
	res := 1
	for i := 0; res < N; i++ {
		v = v << 1
		res = v - 1
	}
	return res
}

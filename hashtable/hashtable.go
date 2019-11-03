package hashtable

import (
	//	"encoding/binary"
	"log"
	"sync"
	"unsafe"
)

// An alternative for Go runtime implemenation of map[string]uintptr
// Requires to specify maximum number of hash collisions at the initialization time
// Insert fails if there are too many collisions
// Allows to use a custom hash funciton - Store/Load API requires both the key and the hash
//
// The goal is 3x improvement and true O(1) performance (what about data cache miss?)
// See also:
// * https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf
// * https://github.com/larytet/emcpp/blob/master/src/HashTable.h

// So far the performance for large tables (+100K) is 2x better than the Go built-in map
// For large tables - 100K+ items - random memory access dominates the performance
// The idea is probably a dead end unless I introduce more constraints on the key distribution
// My key is a domain name. There is not much special about domain names.
// A domain name is a UTF-8 string which can be rather long (~100 bytes), but usually
// (95%) is short (under 35 bytes) Popular domain names are very short, with specific
// distribution of character pairs
// I want popular domain names hit the same 4K memory page in the hashtable

type Statistics struct {
	Store            uint64
	StoreSuccess     uint64
	StoreCollision   uint64
	StoreMatchingKey uint64
	MaxCollisions    uint64
	Load             uint64
	LoadSuccess      uint64
	LoadSwap         uint64
	LoadFailed       uint64
	FindSuccess      uint64
	FindCollision    uint64
	FindFailed       uint64
	Remove           uint64
	RemoveSuccess    uint64
	RemoveFailed     uint64
}

const ITEM_IN_USE_MASK = (uint64(1) << 63)

// An item in the hashtable. I want this struct to be as small as possible
// to reduce data cache miss.
// Alternatively I can keep two keys (a bucket) in the same item
type item struct {
	// I keep pointers to strings. This is bad for GC - triggers runtime.scanobject()
	// Can I copy the string to a large buffer and use an index in the buffer instead
	// of the string address? What are alternatives?
	// I can also rely on 64 bits (or 128 bits) hash instead of the key itself and
	// do not keep the key in the table (see also RelyOnHash)
	key uint64

	// User value. Can I assume 32 bits here?
	value uintptr

	// hash of the key for quick compare
	// I can set the IN_USE bit with atomic.compareAndSwap() and lock the entry
	// I will need two bits LOCK and READY to avoid read of partial data
	hash uint64
	// Add padding for 64 bytes cache line?
}

func (i *item) reset() {
	i.key = 0
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
	// Not used
	ResizeFactor int
	data         []item
	// Mutex will be called in the LoadSync/StoreSync API
	// Not used
	mutex sync.Mutex
	// I can optimize modulo by size
	moduloSize ModuloSize
	// Assume 64 bits reliable
	// The idea is that if the hash function reliable
	// I can avoid collisions and skip comparing the key
	// Not used
	RelyOnHash bool
}

// size is the maximum hashtable capacity and usually is 2x-4x times larger than
// the number of items you want to keep in the hashtable
// For the "load factor" 0.5  capacity is 2x number of items
// If the hash function is perfect the load factor can be 1
// maxCollisions is the maximum number collisions before Load() gives up
// and returns an error.
// I do not provide automatic resizing
// I can not resize, because I do not know the hash function
// This is up to the application can try to create a new larger table
// and copy the elements there.
func New(size int, maxCollisions int) (h *Hashtable) {
	h = new(Hashtable)
	size = getSize(size)
	h.size = size
	h.moduloSize = getModuloSizeFunction(size)
	h.maxCollisions = maxCollisions
	// allow collision for the last entry in the table
	count := size + maxCollisions
	h.data = make([]item, count, count)
	h.Reset()
	return h
}

func (h *Hashtable) Reset() {
	// I have two alternatives here - reallocate the h.data or reset each
	// item in h.data separately. In both cases GC will work hard and
	// removes strings (keys). Better I will perform the loop
	// I want to touch all entries after New(). If the table is small
	// it will fit L3 data cache and first store will be fast
	// At the very least I get rid of memory page miss for the first Store()
	for i := 0; i < len(h.data); i++ {
		h.data[i].reset()
	}
}

func nextIndex(index int) int {
	return index + 1
}

func (h *Hashtable) GetStatistics() Statistics {
	return h.statistics
}

// Store a key:value pair in the hashtable
// 'hash' can be xxhash.Sum64String(key)
//
// Store() is not thread safe. I could CompareAndSwap and lock the items, but
// atomics are expensive. I assume that the application will use a mutex anyway
// or I can implement LoadSync()/StoreSync() API if I need it
// You want the hash function to hit the same 4K memory page for most frequent lookups.
// You want "clustering" for a few keys, and uniform distribution for most keys.
// This approach can potentially improve the peformance of the large hashtables by 50%-80%
// Hint. You calculate hash only once if you want to check if the element exists first
// and remove the existing element before storing a new one. In the golang map this code
// will require three calls to the hash function
// A bonus - you choose the hash function and can switch it in the run-time.
// See also https://github.com/golang/go/issues/21195
// https://stackoverflow.com/questions/29662003/go-map-with-user-defined-key-with-user-defined-equality
func (h *Hashtable) Store(key uint64, hash uint64, value uintptr) bool {
	h.statistics.Store++
	// I used a small struct HashContext with a couple of "methods" nextIndex/init/..
	// Appears that calling "methods" impacts performance (prevents inlining in Golang ?)
	index := h.moduloSize(hash)
	hash = hash | ITEM_IN_USE_MASK
	lookIt := item{key: key, hash: hash}
	var collisions int
	for collisions = 0; collisions < h.maxCollisions; collisions++ {
		it := &h.data[index]
		// The next line - random memory access - dominates execution time
		// for tables 100K entries and above
		// Data cache miss (and memory page miss?) sucks
		inUse := inUse(it)
		if !inUse {
			// TODO How can I make sure that the newly added item is in the possible best slot
			// for the following search? I can not just swap the elements because the best slot
			// can be occupied by an item from a different collision chain. I limit length of the
			// collisions chains. I can keep in the item it's distance from the perfect position
			// this way I can swap some elements when storing
			h.statistics.StoreSuccess++
			it.key = key
			it.hash = hash
			it.value = value
			if collisions > 0 {
				if h.statistics.MaxCollisions < uint64(collisions) {
					h.statistics.MaxCollisions = uint64(collisions)
				}
				// This store added one collision
				h.collisions++
			}
			return true
		} else {
			// should be a rare occasion
			if isSameAndInUse(it, &lookIt) {
				h.statistics.StoreMatchingKey++
				return false
			}
			h.statistics.StoreCollision++
			index = nextIndex(index)
		}
	}
	log.Printf("Failed to add '%v':'%v', col=%d:%d, hash=%x size=%d", key, value, collisions, h.collisions, lookIt.hash, h.size)
	return false
}

// 'other' is usually an automatic variable
// 'i' is a random address in the hashtable
func isSameAndInUse(i *item, other *item) bool {
	return inUse(i) &&
		(i.hash == other.hash) &&

		// this line consumes 50% of the CPU time
		// for tables smaller than a memory page
		// when comparing short strings
		// for long keys the line will dominate CPU
		// consumption. TODO Check RelyOnHash?
		(i.key == other.key)
}

// This is by far the most expensive single line in the Load() flow
// The line is responsible for 80% of the execution time in large hashtables
// 'i' is a random address in a potentially very large hashtable
// This function used to be a method of "item" object at cost of 10% of CPU
func inUse(i *item) bool {
	return (i.hash & ITEM_IN_USE_MASK) != 0
}

func (h *Hashtable) find(key uint64, hash uint64, index int) (int, bool) {
	hash = hash | ITEM_IN_USE_MASK
	lookIt := item{key: key, hash: hash}
	for collisions := 0; collisions < h.maxCollisions; collisions++ {
		it := &h.data[index]
		if isSameAndInUse(it, &lookIt) {
			h.statistics.FindSuccess++
			return index, true
		} else {
			// should be  a rare occasion
			h.statistics.FindCollision++
			index = nextIndex(index)
		}
	}
	h.statistics.FindFailed++
	return 0, false
}

// Load is not thread safe. There is a race if another tread removes the item
// while I am looking for it. I can find an item in inconsistent state
// Find the key in the table, return the object. It is pain to pay price of
// the atomic swap. What I want to do here?
// There is aso a race if Load() swaps elements
//
// Can I assume that Load() is more frequent than Store()?
// 'ref' can be used in the subsequent Remove() and save lookup
// Should I define type 'Ref'?
func (h *Hashtable) Load(key uint64, hash uint64) (value uintptr, ok bool, ref uint32) {
	h.statistics.Load++
	index0 := h.moduloSize(hash)
	if index, ok := h.find(key, hash, index0); ok {
		h.statistics.LoadSuccess++
		it := &h.data[index]
		value = it.value
		// If the found item is not in the perfect slot
		// swap the found item with the first in the "chain" and improve lookup for
		//  the same element if it happens again
		// See https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
		//if index0 != index {
		//	tmp := *it
		//	*it = h.data[index0]
		//	h.data[index0] = tmp
		//	h.statistics.LoadSwap++
		//}
		return value, true, uint32(uintptr(unsafe.Pointer(it)) - uintptr(unsafe.Pointer(&h.data[0])))
	}
	h.statistics.LoadFailed++
	return 0, false, 0
}

// Iterate through the hashtable. Firsr time use index 0
// I want to use 32 bits ref here?
func (h *Hashtable) GetNext(index int) (nextIndex int, value uintptr, key uint64, ok bool) {
	for i := index; i < len(h.data); i++ {
		it := &h.data[i]
		if inUse(it) {
			return (i + 1), it.value, it.key, true
		}
	}
	return len(h.data), 0, 0, false
}

// Fast removal by reference. Argument "ref" is an offest from the start of the allocated data
// This approach limits size of the hashtable by 4GB.The idea is the the user of the API
// implements some sharding scheme. The user composes an item ID (64 bits) from the shard ID
// and the hashtable ref
func (h *Hashtable) RemoveByRef(ref uint32) {
	it := (*item)(unsafe.Pointer(uintptr(ref) + uintptr(unsafe.Pointer(&h.data[0]))))
	it.reset()
}

func (h *Hashtable) Remove(key uint64, hash uint64) (value uintptr, ok bool) {
	h.statistics.Remove++
	index0 := h.moduloSize(hash)
	if index, ok := h.find(key, hash, index0); ok {
		h.statistics.RemoveSuccess++
		// Return yet another value 'collision' from find() is 10% performance for small tables
		// TODO: what to do here?
		if index0 != index { // collision?
			h.collisions--
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
	h.statistics.RemoveFailed++
	return 0, false
}

// Resize the table. Usually you call the function to make
// the table larger and reduce number of collisions
// You can call this function if you make run-time changes of the hash function
// (hash collision attack?)
// Not supported
func (h *Hashtable) Resize(factor int, maxCollisions int) bool {
	return false
}

// Returns number of collisions in the table
func (h *Hashtable) Collisions() int {
	return h.collisions
}

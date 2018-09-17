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
// The goal is 3x improvement and true O(1) performance (what about data cache miss?)
// See also:
// * https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf
// * https://github.com/larytet/emcpp/blob/master/src/HashTable.h

// So far the performance is similar to the Go built-in map
// For large tables - 100K+ items - random memory access dominates the performance
// The idea is probably a dead end unless I introduce more constraints on the key distribution
// My key is a domain name. There is not much special about domain names.
// A domain name is a UTF-8 string which can be rather long (~100 bytes), but usually
// (95%) is short (under 35 bytes) Popular domain names are very short, with specific
// distribution of character pairs

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

type item struct {
	// I keep pointers to strings. This is bad for GC - triggers runtime.scanobject()
	// Can I copy the string to a large buffer and use an index in the buffer instead
	// of the string address? What are alternatives?
	// I can also rely on 64 bits (or 128 bits) hash and report collisions
	// I can keep two keys (a bucket) in the same item which will reduce data cache miss
	key string

	// 64 bits hash of the key for quick compare
	hash  uint64
	value uintptr

	// I can "state int64" instead and atomic compareAndSwap to allocate the entry
	inUse bool

	// Add padding for 64 bytes cache line?
}

func (i *item) reset() {
	i.inUse = false
	i.key = ""
	i.hash = 0
	i.value = 0
}

func (i *item) isSame(other *item) bool {
	return i.inUse && other.inUse && (i.hash == other.hash) && (i.key == other.key)
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
	// size = getPower2(size)
	size = getSize(size)
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
	// I want to touch all entries after New(). If the table is small
	// it will fit L3 data cache and first store will be fast
	// At the very least I get rid of memory page miss on first Stor()
	for i := 0; i < len(h.data); i++ {
		h.data[i].reset()
	}
}

type hashContext struct {
	it    item
	index int
	size  int
	step  int
}

// This is naive. What I want to do here is sharding based on 8 LSBs
// Bad choise of "size" will cause collisions
func (hc *hashContext) nextIndex() (index int) {
	if hc.step == 0 {
		// Collision attack is possible here
		// I should rotate hash functions
		// See also https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
		hash := xxhash.Sum64String(hc.it.key)
		hc.step += 1
		hc.it.hash = hash
		// The modulo below consumes 50% of the function if the table fits L3 cache
		// 20% of the function for large tables.
		hc.index = int(hash % uint64(hc.size))
		hc.it.inUse = true
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

	hc := hashContext{it: item{key: key}, size: h.size}
	index := hc.nextIndex()
	var collisions int
	for collisions = 0; collisions < h.maxCollisions; collisions++ {
		it := &h.data[index]
		// The next line - random memory access - dominates CPU consumption
		// for tables 100K entries and above
		// Data cache miss (and memory page miss?) sucks
		inUse := it.inUse
		if !inUse {
			// I can swap the first item in the "chain" with this item and improve lookup time for freshly inserted items
			// See https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
			h.statistics.StoreSuccess += 1
			it.inUse = true
			it.key = key
			it.hash = hc.it.hash
			it.value = value
			if collisions > 0 {
				if h.statistics.MaxCollisions < uint64(collisions) {
					h.statistics.MaxCollisions = uint64(collisions)
				}
				// This store added one collision
				h.collisions += 1
			}
			return true
		} else {
			// should be a rare occasion
			if it.isSame(&hc.it) {
				h.statistics.StoreMatchingKey += 1
				return false
			}
			h.statistics.StoreCollision += 1
			index = hc.nextIndex()
		}
	}
	log.Printf("Failed to add %v:%v, col=%d:%d, hash=%x size=%d", key, value, collisions, h.collisions, hc.it.hash, h.size)
	return false
}

func (h *Hashtable) find(key string) (index int, collisions int, chainStart int, ok bool) {
	hc := hashContext{it: item{key: key}, size: h.size}
	index = hc.nextIndex()
	chainStart = index
	for collisions = 0; collisions < h.maxCollisions; collisions++ {
		it := &h.data[index]
		if it.isSame(&hc.it) {
			h.statistics.FindSuccess += 1
			return index, collisions, chainStart, true
		} else {
			// should be  a rare occasion
			h.statistics.FindCollision += 1
			index = hc.nextIndex()
		}
	}
	h.statistics.FindFailed += 1
	return 0, collisions, chainStart, false
}

// Find the key in the table, return the object
// Can I assume that Load() is more frequent than Store()?
func (h *Hashtable) Load(key string) (value uintptr, ok bool) {
	h.statistics.Load += 1
	if index, collisions, chainStart, ok := h.find(key); ok {
		h.statistics.LoadSuccess += 1
		it := h.data[index]
		// Swap the found item with the first in the "chain" and improve lookup next time
		// due to CPU caching
		if collisions > 0 {
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

// I want a switch/case with dividing by const
// and let the compiler optimize modulo
// See also https://probablydance.com/2017/02/26/i-wrote-the-fastest-hashtable/
// and source code https://github.com/skarupke/flat_hash_map/blob/master/flat_hash_map.hpp
func moduloSize(size int, modulo int) int {
	return 0
}

// From https://github.com/skarupke/flat_hash_map/blob/master/flat_hash_map.hpp
func getSize(N int) int {
	prime_list := []int{
		2, 3, 5, 7, 11, 13, 17, 23, 29, 37, 47,
		59, 73, 97, 127, 151, 197, 251, 313, 397,
		499, 631, 797, 1009, 1259, 1597, 2011, 2539,
		3203, 4027, 5087, 6421, 8089, 10193, 12853, 16193,
		20399, 25717, 32401, 40823, 51437, 64811, 81649,
		102877, 129607, 163307, 205759, 259229, 326617,
		411527, 518509, 653267, 823117, 1037059, 1306601,
		1646237, 2074129, 2613229, 3292489, 4148279, 5226491,
		6584983, 8296553, 10453007, 13169977, 16593127, 20906033,
		26339969, 33186281, 41812097, 52679969, 66372617,
		83624237, 105359939, 132745199, 167248483, 210719881,
		265490441, 334496971, 421439783, 530980861, 668993977,
		842879579, 1061961721, 1337987929, 1685759167, 2123923447,
		2675975881, 3371518343, 4247846927, 5351951779, 6743036717,
		8495693897, 10703903591, 13486073473, 16991387857,
		21407807219, 26972146961, 33982775741, 42815614441,
		53944293929, 67965551447, 85631228929, 107888587883,
		135931102921, 171262457903, 215777175787, 271862205833,
		342524915839, 431554351609, 543724411781, 685049831731,
		863108703229, 1087448823553, 1370099663459, 1726217406467,
		2174897647073, 2740199326961, 3452434812973, 4349795294267,
		5480398654009, 6904869625999, 8699590588571, 10960797308051,
		13809739252051, 17399181177241, 21921594616111, 27619478504183,
		34798362354533, 43843189232363, 55238957008387, 69596724709081,
		87686378464759, 110477914016779, 139193449418173,
		175372756929481, 220955828033581, 278386898836457,
		350745513859007, 441911656067171, 556773797672909,
		701491027718027, 883823312134381, 1113547595345903,
		1402982055436147, 1767646624268779, 2227095190691797,
		2805964110872297, 3535293248537579, 4454190381383713,
		5611928221744609, 7070586497075177, 8908380762767489,
		11223856443489329, 14141172994150357, 17816761525534927,
		22447712886978529, 28282345988300791, 35633523051069991,
		44895425773957261, 56564691976601587, 71267046102139967,
		89790851547914507, 113129383953203213, 142534092204280003,
		179581703095829107, 226258767906406483, 285068184408560057,
		359163406191658253, 452517535812813007, 570136368817120201,
		718326812383316683, 905035071625626043, 1140272737634240411,
		1436653624766633509, 1810070143251252131, 2280545475268481167,
		2873307249533267101, 3620140286502504283, 4561090950536962147,
		5746614499066534157, 7240280573005008577, 9122181901073924329} // 11493228998133068689, 14480561146010017169, 18446744073709551557
	for _, p := range prime_list {
		if p >= N {
			return p
		}
	}
	return getPower2(N)
}

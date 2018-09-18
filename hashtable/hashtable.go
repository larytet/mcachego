package hashtable

import (
	//	"encoding/binary"
	"github.com/cespare/xxhash"
	"log"
	"sync"
	"sync/atomic"
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

// An item in the hashtable. I want this struct to be as small as possible
// to reduce data cache miss. Cache miss dominates the performance for large tables
// Alternatively I can keep two keys (a bucket) in the same item
type item struct {
	// I keep pointers to strings. This is bad for GC - triggers runtime.scanobject()
	// Can I copy the string to a large buffer and use an index in the buffer instead
	// of the string address? What are alternatives?
	// I can also rely on 64 bits (or 128 bits) hash and report collisions
	key string

	// I use this field to lock the item with atomic.CompareAndSwap
	// I modify entries in Store() and Remove()
	// I need read access in Load()
	// I lock the entry first. If acquisition of the lock fails I try again
	// I set/clear IN_USE bit if needed and release the lock
	state uint32

	// Can be an index in a table or an offset from the base of the allocated
	// memory pool
	value uint32

	// 16 bits hash of the key for quick compare
	// I can set the IN_USE bit with atomic.compareAndSwap() and lock the entry
	// I will need two bits LOCK and READY to avoid read of partial data
	// I have two
	hash uint16

	// Add padding for 64 bytes cache line?
}

func (i *item) reset() {
	i.key = ""
	i.hash = 0
	i.value = 0
}

// This is by far the most expensive single line in the Load() flow
// The line is responsible for 80% of the execution time
// 'other' is an automatic variable
// 'i' is a random address in the hashtable
func (i *item) isSame(other *item) bool {
	return i.inUse() && other.inUse() &&
		(i.hash == other.hash) &&
		(i.key == other.key)
}

const (
	ITEM_IN_USE uint32 = 1
	ITEM_LOCKED uint32 = 2
)

func (i *item) free() bool {
	return ((i.state & (ITEM_IN_USE | ITEM_LOCKED)) != 0)
}

func (i *item) inUse() bool {
	return ((i.state & ITEM_IN_USE) == ITEM_IN_USE)
}

func (i *item) locked() bool {
	return ((i.state & ITEM_LOCKED) == ITEM_LOCKED)
}

// Returns true if succeeds to set both lock and inUse bits
func (i *item) lockAndSetInUse() bool {
	ok := false
	for !i.inUse() {
		ok = atomic.CompareAndSwapUint32(&i.state, i.state, ITEM_IN_USE|ITEM_LOCKED)
		if ok {
			break
		}
	}
	return ok
}

func (i *item) lockAndClearInUse() bool {
	ok := false
	for {
		ok = atomic.CompareAndSwapUint32(&i.state, i.state, ITEM_LOCKED)
		if ok {
			break
		}
	}
	return ok
}

// Returns true if succeeds to set both lock and inUse bits
func (i *item) lock() bool {
	ok := false
	for {
		ok = atomic.CompareAndSwapUint32(&i.state, i.state, i.state|ITEM_LOCKED)
		if ok {
			break
		}
	}
	return ok
}

func (i *item) unlock() {
	atomic.StoreUint32(&i.state, i.state&(^ITEM_LOCKED))
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
		hc.it.hash = uint16(hash)
		// The modulo 'hash % hc.size' consumes 50% of the function if the table fits L3 cache
		// and 20% of the function for large tables
		hc.index = moduloSize(hash, hc.size)
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
		// The next line - random memory access - dominates execution time
		// for tables 100K entries and above
		// Data cache miss (and memory page miss?) sucks
		inUse := it.inUse()
		if !inUse {
			// I can swap the first item in the "chain" with this item and improve lookup time for freshly inserted items
			// See https://www.sebastiansylvan.com/post/robin-hood-hashing-should-be-your-default-hash-table-implementation/
			h.statistics.StoreSuccess += 1
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

// I want a switch/case and division by const and let the compiler optimize modulo
// by generating the best assembler it can
// See also https://probablydance.com/2017/02/26/i-wrote-the-fastest-hashtable/
// and source code https://github.com/skarupke/flat_hash_map/blob/master/flat_hash_map.hpp
// This function shaves 10% off the Store() CPU consumption
func moduloSize(hash uint64, size int) int {
	switch size {
	case 5087:
		return int(hash % 5087)
	case 6421:
		return int(hash % 6421)
	case 8089:
		return int(hash % 8089)
	case 10193:
		return int(hash % 10193)
	case 12853:
		return int(hash % 12853)
	case 16193:
		return int(hash % 16193)
	case 20399:
		return int(hash % 20399)
	case 25717:
		return int(hash % 25717)
	case 32401:
		return int(hash % 32401)
	case 40823:
		return int(hash % 40823)
	case 51437:
		return int(hash % 51437)
	case 64811:
		return int(hash % 64811)
	case 81649:
		return int(hash % 81649)
	case 102877:
		return int(hash % 102877)
	case 129607:
		return int(hash % 129607)
	case 163307:
		return int(hash % 163307)
	case 205759:
		return int(hash % 205759)
	case 259229:
		return int(hash % 259229)
	case 326617:
		return int(hash % 326617)
	case 411527:
		return int(hash % 411527)
	case 518509:
		return int(hash % 518509)
	case 653267:
		return int(hash % 653267)
	case 823117:
		return int(hash % 823117)
	case 1037059:
		return int(hash % 1037059)
	case 0:
		return 0
	case 2:
		return int(hash % 2)
	case 3:
		return int(hash % 3)
	case 5:
		return int(hash % 5)
	case 7:
		return int(hash % 7)
	case 11:
		return int(hash % 11)
	case 13:
		return int(hash % 13)
	case 17:
		return int(hash % 17)
	case 23:
		return int(hash % 23)
	case 29:
		return int(hash % 29)
	case 37:
		return int(hash % 37)
	case 47:
		return int(hash % 47)
	case 59:
		return int(hash % 59)
	case 73:
		return int(hash % 73)
	case 97:
		return int(hash % 97)
	case 127:
		return int(hash % 127)
	case 151:
		return int(hash % 151)
	case 197:
		return int(hash % 197)
	case 251:
		return int(hash % 251)
	case 313:
		return int(hash % 313)
	case 397:
		return int(hash % 397)
	case 499:
		return int(hash % 499)
	case 631:
		return int(hash % 631)
	case 797:
		return int(hash % 797)
	case 1009:
		return int(hash % 1009)
	case 1259:
		return int(hash % 1259)
	case 1597:
		return int(hash % 1597)
	case 2011:
		return int(hash % 2011)
	case 2539:
		return int(hash % 2539)
	case 3203:
		return int(hash % 3203)
	case 4027:
		return int(hash % 4027)
	case 1306601:
		return int(hash % 1306601)
	case 1646237:
		return int(hash % 1646237)
	case 2074129:
		return int(hash % 2074129)
	case 2613229:
		return int(hash % 2613229)
	case 3292489:
		return int(hash % 3292489)
	case 4148279:
		return int(hash % 4148279)
	case 5226491:
		return int(hash % 5226491)
	case 6584983:
		return int(hash % 6584983)
	case 8296553:
		return int(hash % 8296553)
	case 10453007:
		return int(hash % 10453007)
	case 13169977:
		return int(hash % 13169977)
	case 16593127:
		return int(hash % 16593127)
	case 20906033:
		return int(hash % 20906033)
	case 26339969:
		return int(hash % 26339969)
	case 33186281:
		return int(hash % 33186281)
	case 41812097:
		return int(hash % 41812097)
	case 52679969:
		return int(hash % 52679969)
	case 66372617:
		return int(hash % 66372617)
	case 83624237:
		return int(hash % 83624237)
	case 105359939:
		return int(hash % 105359939)
	case 132745199:
		return int(hash % 132745199)
	case 167248483:
		return int(hash % 167248483)
	case 210719881:
		return int(hash % 210719881)
	case 265490441:
		return int(hash % 265490441)
	case 334496971:
		return int(hash % 334496971)
	case 421439783:
		return int(hash % 421439783)
	case 530980861:
		return int(hash % 530980861)
	case 668993977:
		return int(hash % 668993977)
	case 842879579:
		return int(hash % 842879579)
	case 1061961721:
		return int(hash % 1061961721)
	case 1337987929:
		return int(hash % 1337987929)
	case 1685759167:
		return int(hash % 1685759167)
	case 2123923447:
		return int(hash % 2123923447)
	case 2675975881:
		return int(hash % 2675975881)
	case 3371518343:
		return int(hash % 3371518343)
	case 4247846927:
		return int(hash % 4247846927)
	case 5351951779:
		return int(hash % 5351951779)
	case 6743036717:
		return int(hash % 6743036717)
	case 8495693897:
		return int(hash % 8495693897)
	case 10703903591:
		return int(hash % 10703903591)
	case 13486073473:
		return int(hash % 13486073473)
	case 16991387857:
		return int(hash % 16991387857)
	case 21407807219:
		return int(hash % 21407807219)
	case 26972146961:
		return int(hash % 26972146961)
	case 33982775741:
		return int(hash % 33982775741)
	case 42815614441:
		return int(hash % 42815614441)
	case 53944293929:
		return int(hash % 53944293929)
	case 67965551447:
		return int(hash % 67965551447)
	case 85631228929:
		return int(hash % 85631228929)
	case 107888587883:
		return int(hash % 107888587883)
	case 135931102921:
		return int(hash % 135931102921)
	case 171262457903:
		return int(hash % 171262457903)
	case 215777175787:
		return int(hash % 215777175787)
	case 271862205833:
		return int(hash % 271862205833)
	case 342524915839:
		return int(hash % 342524915839)
	case 431554351609:
		return int(hash % 431554351609)
	case 543724411781:
		return int(hash % 543724411781)
	case 685049831731:
		return int(hash % 685049831731)
	case 863108703229:
		return int(hash % 863108703229)
	case 1087448823553:
		return int(hash % 1087448823553)
	case 1370099663459:
		return int(hash % 1370099663459)
	case 1726217406467:
		return int(hash % 1726217406467)
	case 2174897647073:
		return int(hash % 2174897647073)
	case 2740199326961:
		return int(hash % 2740199326961)
	case 3452434812973:
		return int(hash % 3452434812973)
	case 4349795294267:
		return int(hash % 4349795294267)
	case 5480398654009:
		return int(hash % 5480398654009)
	case 6904869625999:
		return int(hash % 6904869625999)
	case 8699590588571:
		return int(hash % 8699590588571)
	case 10960797308051:
		return int(hash % 10960797308051)
	case 13809739252051:
		return int(hash % 13809739252051)
	case 17399181177241:
		return int(hash % 17399181177241)
	case 21921594616111:
		return int(hash % 21921594616111)
	case 27619478504183:
		return int(hash % 27619478504183)
	case 34798362354533:
		return int(hash % 34798362354533)
	case 43843189232363:
		return int(hash % 43843189232363)
	case 55238957008387:
		return int(hash % 55238957008387)
	case 69596724709081:
		return int(hash % 69596724709081)
	case 87686378464759:
		return int(hash % 87686378464759)
	case 110477914016779:
		return int(hash % 110477914016779)
	case 139193449418173:
		return int(hash % 139193449418173)
	case 175372756929481:
		return int(hash % 175372756929481)
	case 220955828033581:
		return int(hash % 220955828033581)
	case 278386898836457:
		return int(hash % 278386898836457)
	case 350745513859007:
		return int(hash % 350745513859007)
	case 441911656067171:
		return int(hash % 441911656067171)
	case 556773797672909:
		return int(hash % 556773797672909)
	case 701491027718027:
		return int(hash % 701491027718027)
	case 883823312134381:
		return int(hash % 883823312134381)
	case 1113547595345903:
		return int(hash % 1113547595345903)
	case 1402982055436147:
		return int(hash % 1402982055436147)
	case 1767646624268779:
		return int(hash % 1767646624268779)
	case 2227095190691797:
		return int(hash % 2227095190691797)
	case 2805964110872297:
		return int(hash % 2805964110872297)
	case 3535293248537579:
		return int(hash % 3535293248537579)
	case 4454190381383713:
		return int(hash % 4454190381383713)
	case 5611928221744609:
		return int(hash % 5611928221744609)
	case 7070586497075177:
		return int(hash % 7070586497075177)
	case 8908380762767489:
		return int(hash % 8908380762767489)
	case 11223856443489329:
		return int(hash % 11223856443489329)
	case 14141172994150357:
		return int(hash % 14141172994150357)
	case 17816761525534927:
		return int(hash % 17816761525534927)
	case 22447712886978529:
		return int(hash % 22447712886978529)
	case 28282345988300791:
		return int(hash % 28282345988300791)
	case 35633523051069991:
		return int(hash % 35633523051069991)
	case 44895425773957261:
		return int(hash % 44895425773957261)
	case 56564691976601587:
		return int(hash % 56564691976601587)
	case 71267046102139967:
		return int(hash % 71267046102139967)
	case 89790851547914507:
		return int(hash % 89790851547914507)
	case 113129383953203213:
		return int(hash % 113129383953203213)
	case 142534092204280003:
		return int(hash % 142534092204280003)
	case 179581703095829107:
		return int(hash % 179581703095829107)
	case 226258767906406483:
		return int(hash % 226258767906406483)
	case 285068184408560057:
		return int(hash % 285068184408560057)
	case 359163406191658253:
		return int(hash % 359163406191658253)
	case 452517535812813007:
		return int(hash % 452517535812813007)
	case 570136368817120201:
		return int(hash % 570136368817120201)
	case 718326812383316683:
		return int(hash % 718326812383316683)
	case 905035071625626043:
		return int(hash % 905035071625626043)
	case 1140272737634240411:
		return int(hash % 1140272737634240411)
	case 1436653624766633509:
		return int(hash % 1436653624766633509)
	case 1810070143251252131:
		return int(hash % 1810070143251252131)
	case 2280545475268481167:
		return int(hash % 2280545475268481167)
	case 2873307249533267101:
		return int(hash % 2873307249533267101)
	case 3620140286502504283:
		return int(hash % 3620140286502504283)
	case 4561090950536962147:
		return int(hash % 4561090950536962147)
	case 5746614499066534157:
		return int(hash % 5746614499066534157)
	case 7240280573005008577:
		return int(hash % 7240280573005008577)
	case 9122181901073924329:
		return int(hash % 9122181901073924329)
	}
	return int(hash % uint64(size))
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

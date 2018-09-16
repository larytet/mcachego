package hashtable

// An alternative for Go runtime implemenation of map[string]uintptr
// Requires to specify maximum number of hash collisions at the initialization time
// Insert can fail if there are too many collisions
// See also https://github.com/larytet/emcpp/blob/master/src/HashTable.h
// The goal is 3x improvement and true O(1) performance
// See also  https://medium.com/@ConnorPeet/go-maps-are-not-o-1-91c1e61110bf

type Statistics struct {
	InsertTotal uint64
}

// This is copy&paste from https://github.com/larytet/emcpp/blob/master/src/HashTable.h
type Hashtable struct {
	size       int
	count      int
	statistics Statistics
	// Number of collisions in the table
	collisions int
	// Resize automatically if not zero
	ResizeFactor int
}

func New(size int, maxCollisions int) (h *Hashtable) {
}

// Store a value in the hashtable
func (h *Hashtable) Store(key string, value uintptr) bool {
	return false
}

// Resize the table. Usually you call the function to make
// the table larger and reduce number of collisions
func (h *Hashtable) Resize() bool {
	return false
}

// Returns number of collisions in the table
func (h *Hashtable) Collisions() int {
	return h.collisions
}

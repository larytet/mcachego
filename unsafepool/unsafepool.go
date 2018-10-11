package unsafepool

import (
	"reflect"
	"sync/atomic"
	"unsafe"
)

// This is a fast (<4ns) Free/Alloc unsafe.Pointer memory pool

type Statistics struct {
	Alloc              uint64
	AllocLockCongested uint64
	Free               uint64
	FreeBadAddress     uint64
	FreeLockCongested  uint64
	MinAvailability    uint64
}

// In the cache API I am replacing the whole Go  memory managemnt,
// It is safer (no pun) to provide
// an API for the application which demos a HowTo
// Application needs a pool to allocate users objects
// and keep the objects in the cache
// This is a lock free memory pool of blocks of fixed size
type Pool struct {
	top         int32
	stack       []unsafe.Pointer
	data        []byte
	objectSize  int
	objectCount int
	maxAddr     uintptr
	minAddr     uintptr
	statistics  *Statistics
}

// Create a memory pool of objectCount objects of type objectType
func New(objectType reflect.Type, objectCount int) (p *Pool) {
	objectSize := int(unsafe.Sizeof(objectType))
	p = new(Pool)
	p.objectSize, p.objectCount = objectSize, objectCount
	p.data = make([]byte, objectSize*objectCount, objectSize*objectCount)
	p.stack = make([]unsafe.Pointer, objectCount, objectCount)
	p.maxAddr = uintptr(unsafe.Pointer(&p.data[objectSize*(objectCount-1)]))
	p.minAddr = uintptr(unsafe.Pointer(&p.data[0]))
	p.Reset()
	return p
}

// Maximum number of objects in the pool
func (p *Pool) Size() int {
	return p.objectCount
}

// Occupied memory
func (p *Pool) SizeBytes() int {
	var up unsafe.Pointer
	return len(p.data) + int(unsafe.Sizeof(up))*len(p.stack)
}

// Number of objects available for allocation
func (p *Pool) Availability() int {
	return int(p.top)
}

// Pool keeps objects in contiguous memory
// Application can keep only offset from the start of the range
func (p *Pool) GetBase() uintptr {
	return p.minAddr
}

func (p *Pool) Reset() {
	for i := 0; i < p.objectCount; i += 1 {
		p.stack[i] = unsafe.Pointer(&p.data[i*p.objectSize])
	}
	p.top = int32(p.objectCount)
	p.statistics = new(Statistics)
	p.statistics.MinAvailability = uint64(p.objectCount)
}

// Allocate a block from the pool
// This API is not thread safe. ~3ns
func (p *Pool) Alloc() (ptr unsafe.Pointer, ok bool) {
	p.statistics.Alloc += 1
	top := p.top - 1
	if top >= 0 {
		if p.statistics.MinAvailability > uint64(top) {
			p.statistics.MinAvailability = uint64(top)
		}
		p.top = top
		return p.stack[top], true
	}
	return nil, false
}

// Return previously allocated block to the pool
// This API is not thread safe ~4ns
func (p *Pool) Free(ptr unsafe.Pointer) bool {
	// I want a quick test that the pointer makes sense
	if (uintptr(ptr) < p.minAddr) || (uintptr(ptr) > p.maxAddr) {
		p.statistics.FreeBadAddress += 1
		return false
	}
	p.statistics.Free += 1
	top := p.top
	p.stack[top] = ptr
	p.top = top + 1
	return true
}

// Allocate a block from the pool
// This API is thread safe. ~10ns
func (p *Pool) AllocSync() (ptr unsafe.Pointer, ok bool) {
	p.statistics.Alloc += 1
	for p.top > 0 {
		top := p.top
		// CompareAndSwap dominates the CPU cycles
		if atomic.CompareAndSwapInt32(&p.top, top, top-1) {
			// success, I decremented p.top
			if p.statistics.MinAvailability > uint64(top) {
				p.statistics.MinAvailability = uint64(top)
			}
			return p.stack[top-1], true
		}
		// a rare event
		p.statistics.AllocLockCongested += 1
	}
	return nil, false
}

// Return previously allocated block to the pool
// The pool does not protect agains double free. I could mark the blocks
// as freed/allocated. Probably this is way too C/C++
// This API is thread safe. ~18ns
func (p *Pool) FreeSync(ptr unsafe.Pointer) bool {
	if (uintptr(ptr) < p.minAddr) || (uintptr(ptr) > p.maxAddr) {
		p.statistics.FreeBadAddress += 1
		return false
	}
	p.statistics.Free += 1
	for {
		top := p.top
		if atomic.CompareAndSwapInt32(&p.top, top, top+1) {
			// success, I incremented p.top
			p.stack[top] = ptr
			return true
		}
		// a rare event
		p.statistics.FreeLockCongested += 1
	}
}

// Returns true if the ptr is from the pool
func (p *Pool) Belongs(ptr unsafe.Pointer) bool {
	res := true
	res = res && (uintptr(ptr) >= p.minAddr)
	res = res && (uintptr(ptr) <= p.maxAddr)
	res = res && (((uintptr(ptr) - p.minAddr) % uintptr(p.objectSize)) == 0)
	return res
}

func (p *Pool) GetStatistics() Statistics {
	return *p.statistics
}

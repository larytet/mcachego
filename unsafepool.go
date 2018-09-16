package mcache

import (
	"reflect"
	"sync/atomic"
	"unsafe" // I need this for runtime.nanotime()
)

// I am replacing the whole Go  memory managemnt, It is safer (no pun)
// to provide
// an API for the application which demos a HowTo
// Application needs a pool to allocate users objects
// and keep the objects in the cache
// This is a lock free memory pool of objects of the same size
type UnsafePool struct {
	top         int32
	stack       []unsafe.Pointer
	data        []byte
	objectSize  int
	objectCount int
	maxAddr     uintptr
	minAddr     uintptr
}

func NewUnsafePool(t reflect.Type, objectCount int) (p *UnsafePool) {
	objectSize := int(unsafe.Sizeof(t))
	p = new(UnsafePool)
	p.objectSize, p.objectCount = objectSize, objectCount
	p.data = make([]byte, objectSize*objectCount, objectSize*objectCount)
	p.stack = make([]unsafe.Pointer, objectCount, objectCount)
	p.maxAddr = uintptr(unsafe.Pointer(&p.data[objectSize*(objectCount-1)]))
	p.minAddr = uintptr(unsafe.Pointer(&p.data[0]))
	p.Reset()
	return p
}

func (p *UnsafePool) Reset() {
	for i := 0; i < p.objectCount; i += 1 {
		p.stack[i] = unsafe.Pointer(&p.data[i*p.objectSize])
	}
	p.top = int32(p.objectCount)
}

func (p *UnsafePool) Alloc() (ptr unsafe.Pointer, ok bool) {
	for p.top > 0 {
		top := p.top
		if atomic.CompareAndSwapInt32(&p.top, top, top-1) {
			// success, I decremented p.top
			return p.stack[top-1], true
		}
	}
	return nil, false
}

func (p *UnsafePool) Free(ptr unsafe.Pointer) bool {
	if (uintptr(ptr) < p.minAddr) || (uintptr(ptr) > p.maxAddr) {
		return false
	}
	for {
		top := p.top
		if atomic.CompareAndSwapInt32(&p.top, top, top+1) {
			// success, I incremented p.top
			p.stack[top] = ptr
			return true
		}
	}
}

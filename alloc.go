package lua

import (
	"reflect"
	"unsafe"
)

// iface is an internal representation of the go-interface.
type iface struct {
	itab unsafe.Pointer
	word unsafe.Pointer
}

const preloadLimit LNumber = 128

var _fv float64
var _uv uintptr

var preloads [int(preloadLimit)]LValue

func init() {
	for i := 0; i < int(preloadLimit); i++ {
		preloads[i] = LNumber(i)
	}
}

// allocator is a fast bulk memory allocator for the LValue.
type allocator struct {
	top     int
	size    int
	fptrs   []float64
	fheader *reflect.SliceHeader

	scratchValue  LValue
	scratchValueP *iface
}

func newAllocator(size int) *allocator {
	al := &allocator{
		top:     0,
		size:    size,
		fptrs:   make([]float64, size),
		fheader: nil,
	}
	al.fheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.fptrs))
	al.scratchValue = LNumber(0)
	al.scratchValueP = (*iface)(unsafe.Pointer(&al.scratchValue))

	return al
}

// LNumber2I takes a number value and returns an interface LValue representing the same number.
// Converting an LNumber to a LValue naively, by doing:
// `var val LValue = myLNumber`
// will result in an individual heap alloc of 8 bytes for the float value. LNumber2I amortizes the cost and memory
// overhead of these allocs by allocating blocks of floats instead.
// The downside of this is that all of the floats on a given block have to become eligible for gc before the block
// as a whole can be gc-ed.
func (al *allocator) LNumber2I(v LNumber) LValue {
	// first check for shared preloaded numbers
	if v >= 0 && v < preloadLimit && float64(v) == float64(int64(v)) {
		return preloads[int(v)]
	}

	// check if we need a new alloc page
	if al.top == len(al.fptrs)-1 {
		al.top = 0
		al.fptrs = make([]float64, al.size)
		al.fheader = (*reflect.SliceHeader)(unsafe.Pointer(&al.fptrs))
	}

	// alloc a new float, and store our value into it
	fptr := (*float64)(unsafe.Pointer(al.fheader.Data + uintptr(al.top)*unsafe.Sizeof(_fv)))
	al.top++
	*fptr = float64(v)

	// hack our scratch LValue to point to our allocated value
	// this scratch lvalue is copied when this function returns meaning the scratch value can be reused
	// on the next call
	al.scratchValueP.word = unsafe.Pointer(fptr)

	return al.scratchValue
}

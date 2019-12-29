package lua

import (
	"context"
	"fmt"
	"os"
)

type LValueType int

const (
	LTNil LValueType = iota
	LTBool
	LTNumber
	LTString
	LTFunction
	LTUserData
	LTThread
	LTTable
	LTChannel
)

var lValueNames = [9]string{"nil", "boolean", "number", "string", "function", "userdata", "thread", "table", "channel"}

func (vt LValueType) String() string {
	return lValueNames[int(vt)]
}

type LValue interface {
	String() string
	Type() LValueType
	// to reduce `runtime.assertI2T2` costs, this method should be used instead of the type assertion in heavy paths(typically inside the VM).
	assertFloat64() (float64, bool)
	// to reduce `runtime.assertI2T2` costs, this method should be used instead of the type assertion in heavy paths(typically inside the VM).
	assertString() (string, bool)
	// to reduce `runtime.assertI2T2` costs, this method should be used instead of the type assertion in heavy paths(typically inside the VM).
	assertFunction() (*LFunction, bool)
}

// LVIsFalse returns true if a given LValue is a nil or false otherwise false.
func LVIsFalse(v LValue) bool { return v == LNil || v == LFalse }

// LVIsFalse returns false if a given LValue is a nil or false otherwise true.
func LVAsBool(v LValue) bool { return v != LNil && v != LFalse }

// LVAsString returns string representation of a given LValue
// if the LValue is a string or number, otherwise an empty string.
func LVAsString(v LValue) string {
	switch sn := v.(type) {
	case LString, LNumber:
		return sn.String()
	default:
		return ""
	}
}

// LVCanConvToString returns true if a given LValue is a string or number
// otherwise false.
func LVCanConvToString(v LValue) bool {
	switch v.(type) {
	case LString, LNumber:
		return true
	default:
		return false
	}
}

// LVAsNumber tries to convert a given LValue to a number.
func LVAsNumber(v LValue) LNumber {
	switch lv := v.(type) {
	case LNumber:
		return lv
	case LString:
		if num, err := parseNumber(string(lv)); err == nil {
			return num
		}
	}
	return LNumber(0)
}

type LNilType struct{}

func (nl *LNilType) String() string                     { return "nil" }
func (nl *LNilType) Type() LValueType                   { return LTNil }
func (nl *LNilType) assertFloat64() (float64, bool)     { return 0, false }
func (nl *LNilType) assertString() (string, bool)       { return "", false }
func (nl *LNilType) assertFunction() (*LFunction, bool) { return nil, false }

var LNil = LValue(&LNilType{})

type LBool bool

func (bl LBool) String() string {
	if bool(bl) {
		return "true"
	}
	return "false"
}
func (bl LBool) Type() LValueType                   { return LTBool }
func (bl LBool) assertFloat64() (float64, bool)     { return 0, false }
func (bl LBool) assertString() (string, bool)       { return "", false }
func (bl LBool) assertFunction() (*LFunction, bool) { return nil, false }

var LTrue = LBool(true)
var LFalse = LBool(false)

type LString string

func (st LString) String() string                     { return string(st) }
func (st LString) Type() LValueType                   { return LTString }
func (st LString) assertFloat64() (float64, bool)     { return 0, false }
func (st LString) assertString() (string, bool)       { return string(st), true }
func (st LString) assertFunction() (*LFunction, bool) { return nil, false }

// fmt.Formatter interface
func (st LString) Format(f fmt.State, c rune) {
	switch c {
	case 'd', 'i':
		if nm, err := parseNumber(string(st)); err != nil {
			defaultFormat(nm, f, 'd')
		} else {
			defaultFormat(string(st), f, 's')
		}
	default:
		defaultFormat(string(st), f, c)
	}
}

func (nm LNumber) String() string {
	if isInteger(nm) {
		return fmt.Sprint(int64(nm))
	}
	return fmt.Sprint(float64(nm))
}

func (nm LNumber) Type() LValueType                   { return LTNumber }
func (nm LNumber) assertFloat64() (float64, bool)     { return float64(nm), true }
func (nm LNumber) assertString() (string, bool)       { return "", false }
func (nm LNumber) assertFunction() (*LFunction, bool) { return nil, false }

// fmt.Formatter interface
func (nm LNumber) Format(f fmt.State, c rune) {
	switch c {
	case 'q', 's':
		defaultFormat(nm.String(), f, c)
	case 'b', 'c', 'd', 'o', 'x', 'X', 'U':
		defaultFormat(int64(nm), f, c)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		defaultFormat(float64(nm), f, c)
	case 'i':
		defaultFormat(int64(nm), f, 'd')
	default:
		if isInteger(nm) {
			defaultFormat(int64(nm), f, c)
		} else {
			defaultFormat(float64(nm), f, c)
		}
	}
}

// LTableAllocInfo is used to track the memory usage for one or more tables.
// We can't accurately nor easily track the total memory use of a table, as we don't know how many of its keys / values
// are shared with other tables. What we can track is the number of keys which is a good indication of memory use.
// Additionally, we allow multiple tables to contribute to the same alloc info struct, hence the `numTables` field.
// The values in this struct are adjusted using atomic operators, as the finalizer can run on any goroutine and will
// decrement these fields when it does.
// If the max quotas are exceeded then an error is raised in the LState.
// Implementation note regarding numKeys:
// numKeys tracks the lengths of the slices and maps in the underlying storage of the LTable. In Gopher Lua's current
// implementation, this means that when you set a high integer index it will allocate a contiguous slice of values,
// meaning that the number of keys used may go up by more than you expected:
// For example:
// a = {}
// a[100] = 5
// would allocate 100 keys, as keys 1-99 would be allocated and set to nil.
// Similarly, if a[100] is then set to nil, the key count will stay at 100, as the underlying storage remains allocated
// due to the Gopher Lua implementation.
// If table.remove() is used, then the underlying storage is truncated, so the key count will be reduced. Again, this
// is just an implementation detail of the Gopher Lua implementation.
// Using associative arrays, the lengths work exactly as expected:
// a = {}
// a["100"] = 5
// would allocate 1 key.
// From a Lua script's point of view, this tracking of the underlying storage, rather than the number of logical keys
// may be hard to reason about, but it's important to remember we're approximating the memory used by this
// implementation of Lua, not the logical number of keys used by a script.
// As we want an approximation of the memory usage, arguably, we should track the _capacity_ of the underlying slices
// rather than their _length_, as the capacity more accurately approximates the memory use. The reason this is not done
// is because the capacity of a slice is automatically managed by the go runtime and is free to change between versions
// of go. This would mean Gopher Lua could behave differently between different version of go, which is not desirable.
// We could switch to using capacity if we moved away from using the `append` built in for the table slice.
type LTableAllocInfo struct {
	numKeys   int32 // these can be read from other go routines safely. They are adjusted via atomic operations.
	numTables int32
	maxKeys   int32 // set maximums to MaxInt32 to have tracking without any quota enforcement.
	maxTables int32
}

// tableFinalizer is a struct which hangs off an LTable which the finalizer is attached to. We can't attach directly to
// the table as tables can reference themselves (directly or indirectly) and if a finalizer is attached to a self
// referencing chain of objects they will never be gc-ed. Therefore, it is important when attaching a finalizer that
// the struct being attached to cannot end up referencing itself.
// The tableFinalizer exists as typically LTableAllocInfos are shared between LTables, whereas tableFinalizer is
// unique per LTable, hence it is finalized when the LTable is.
type tableFinalizer struct {
	allocInfo *LTableAllocInfo
	numKeys   int32 // number of keys in this specific table (needed in finalization callback)
}

type LTable struct {
	Metatable LValue

	array   []LValue
	dict    map[LValue]LValue
	strdict map[string]LValue
	keys    []LValue
	k2i     map[LValue]int

	// optional finalizer used to track memory utilisation
	finalizer *tableFinalizer
}

func (tb *LTable) String() string                     { return fmt.Sprintf("table: %p", tb) }
func (tb *LTable) Type() LValueType                   { return LTTable }
func (tb *LTable) assertFloat64() (float64, bool)     { return 0, false }
func (tb *LTable) assertString() (string, bool)       { return "", false }
func (tb *LTable) assertFunction() (*LFunction, bool) { return nil, false }

type LFunction struct {
	IsG       bool
	Env       *LTable
	Proto     *FunctionProto
	GFunction LGFunction
	Upvalues  []*Upvalue
}
type LGFunction func(*LState) int

func (fn *LFunction) String() string                     { return fmt.Sprintf("function: %p", fn) }
func (fn *LFunction) Type() LValueType                   { return LTFunction }
func (fn *LFunction) assertFloat64() (float64, bool)     { return 0, false }
func (fn *LFunction) assertString() (string, bool)       { return "", false }
func (fn *LFunction) assertFunction() (*LFunction, bool) { return fn, true }

type Global struct {
	MainThread    *LState
	CurrentThread *LState
	Registry      *LTable
	Global        *LTable

	builtinMts map[int]LValue
	tempFiles  []*os.File
	gccount    int32
}

type LState struct {
	G       *Global
	Parent  *LState
	Env     *LTable
	Panic   func(*LState)
	Dead    bool
	Options Options

	stop         int32
	reg          *registry
	stack        callFrameStack
	alloc        *allocator
	tblAllocInfo *LTableAllocInfo
	currentFrame *callFrame
	wrapped      bool
	uvcache      *Upvalue
	hasErrorFunc bool
	mainLoop     func(*LState, *callFrame)
	ctx          context.Context
}

func (ls *LState) String() string                     { return fmt.Sprintf("thread: %p", ls) }
func (ls *LState) Type() LValueType                   { return LTThread }
func (ls *LState) assertFloat64() (float64, bool)     { return 0, false }
func (ls *LState) assertString() (string, bool)       { return "", false }
func (ls *LState) assertFunction() (*LFunction, bool) { return nil, false }

type LUserData struct {
	Value     interface{}
	Env       *LTable
	Metatable LValue
}

func (ud *LUserData) String() string                     { return fmt.Sprintf("userdata: %p", ud) }
func (ud *LUserData) Type() LValueType                   { return LTUserData }
func (ud *LUserData) assertFloat64() (float64, bool)     { return 0, false }
func (ud *LUserData) assertString() (string, bool)       { return "", false }
func (ud *LUserData) assertFunction() (*LFunction, bool) { return nil, false }

type LChannel chan LValue

func (ch LChannel) String() string                     { return fmt.Sprintf("channel: %p", ch) }
func (ch LChannel) Type() LValueType                   { return LTChannel }
func (ch LChannel) assertFloat64() (float64, bool)     { return 0, false }
func (ch LChannel) assertString() (string, bool)       { return "", false }
func (ch LChannel) assertFunction() (*LFunction, bool) { return nil, false }

package lua

import (
	"runtime"
	"sync/atomic"
)

const defaultArrayCap = 32
const defaultHashCap = 32

type lValueArraySorter struct {
	L      *LState
	Fn     *LFunction
	Values []LValue
}

func (lv lValueArraySorter) Len() int {
	return len(lv.Values)
}

func (lv lValueArraySorter) Swap(i, j int) {
	lv.Values[i], lv.Values[j] = lv.Values[j], lv.Values[i]
}

func (lv lValueArraySorter) Less(i, j int) bool {
	if lv.Fn != nil {
		lv.L.Push(lv.Fn)
		lv.L.Push(lv.Values[i])
		lv.L.Push(lv.Values[j])
		lv.L.Call(2, 1)
		return LVAsBool(lv.L.reg.Pop())
	}
	return lessThan(lv.L, lv.Values[i], lv.Values[j])
}

// newLTable creates an LTable without setting up the memory tracking on it. See L.CreateTable() for memory tracked
// version.
func newLTable(acap int, hcap int) *LTable {
	if acap < 0 {
		acap = 0
	}
	if hcap < 0 {
		hcap = 0
	}
	tb := &LTable{}
	tb.Metatable = LNil
	if acap != 0 {
		tb.array = make([]LValue, 0, acap)
	}
	if hcap != 0 {
		tb.strdict = make(map[string]LValue, hcap)
	}
	return tb
}

// Len returns length of this LTable.
func (tb *LTable) Len() int {
	if tb.array == nil {
		return 0
	}
	var prev LValue = LNil
	for i := len(tb.array) - 1; i >= 0; i-- {
		v := tb.array[i]
		if prev == LNil && v != LNil {
			return i + 1
		}
		prev = v
	}
	return 0
}

// Append appends a given LValue to this LTable.
func (tb *LTable) Append(value LValue) {
	if value == LNil {
		return
	}
	if tb.array == nil {
		tb.array = make([]LValue, 0, defaultArrayCap)
	}
	if len(tb.array) == 0 || tb.array[len(tb.array)-1] != LNil {
		tb.array = append(tb.array, value)
		if tb.finalizer != nil {
			tb.finalizer.adjustKeys(1)
		}
	} else {
		i := len(tb.array) - 2
		for ; i >= 0; i-- {
			if tb.array[i] != LNil {
				break
			}
		}
		tb.array[i+1] = value
	}
}

// Insert inserts a given LValue at position `i` in this table.
func (tb *LTable) Insert(i int, value LValue) {
	if tb.array == nil {
		tb.array = make([]LValue, 0, defaultArrayCap)
	}
	if i > len(tb.array) {
		tb.RawSetInt(i, value)
		return
	}
	if i <= 0 {
		tb.RawSet(LNumber(i), value)
		return
	}
	i -= 1
	tb.array = append(tb.array, LNil)
	copy(tb.array[i+1:], tb.array[i:])
	tb.array[i] = value
	if tb.finalizer != nil {
		tb.finalizer.adjustKeys(1)
	}
}

// MaxN returns a maximum number key that nil value does not exist before it.
func (tb *LTable) MaxN() int {
	if tb.array == nil {
		return 0
	}
	for i := len(tb.array) - 1; i >= 0; i-- {
		if tb.array[i] != LNil {
			return i + 1
		}
	}
	return 0
}

// Remove removes from this table the element at a given position.
func (tb *LTable) Remove(pos int) LValue {
	if tb.array == nil {
		return LNil
	}
	larray := len(tb.array)
	if larray == 0 {
		return LNil
	}
	i := pos - 1
	oldval := LNil
	switch {
	case i >= larray:
		// nothing to do
		return oldval
	case i == larray-1 || i < 0:
		oldval = tb.array[larray-1]
		tb.array = tb.array[:larray-1]
	default:
		oldval = tb.array[i]
		copy(tb.array[i:], tb.array[i+1:])
		tb.array[larray-1] = nil
		tb.array = tb.array[:larray-1]
	}
	if tb.finalizer != nil {
		tb.finalizer.adjustKeys(-1)
	}
	return oldval
}

// RawSet sets a given LValue to a given index without the __newindex metamethod.
// It is recommended to use `RawSetString` or `RawSetInt` for performance
// if you already know the given LValue is a string or number.
func (tb *LTable) RawSet(key LValue, value LValue) {
	switch v := key.(type) {
	case LNumber:
		if isInteger(v) {
			tb.RawSetInt(int(v), value)
			return
		}
	case LString:
		tb.RawSetString(string(v), value)
		return
	}

	tb.RawSetH(key, value)
}

// RawSetInt sets a given LValue at a position `key` without the __newindex metamethod.
func (tb *LTable) RawSetInt(key int, value LValue) {
	if key < 1 || key >= MaxArrayIndex {
		tb.RawSetH(LNumber(key), value)
		return
	}
	if tb.array == nil {
		tb.array = make([]LValue, 0, defaultArrayCap)
	}
	index := key - 1
	alen := len(tb.array)
	switch {
	case index == alen:
		tb.array = append(tb.array, value)
	case index > alen:
		for i := 0; i < (index - alen); i++ {
			tb.array = append(tb.array, LNil)
		}
		tb.array = append(tb.array, value)
	case index < alen:
		tb.array[index] = value
		return // no need to check for key count adjustment when replacing an existing key
	}
	if tb.finalizer != nil {
		delta := int32(len(tb.array) - alen)
		if delta != 0 {
			tb.finalizer.adjustKeys(delta)
		}
	}
}

// RawSetString sets a given LValue to a given string index without the __newindex metamethod.
func (tb *LTable) RawSetString(key string, value LValue) {
	if tb.strdict == nil {
		tb.strdict = make(map[string]LValue, defaultHashCap)
	}
	if tb.keys == nil {
		tb.keys = []LValue{}
		tb.k2i = map[LValue]int{}
	}

	if value == LNil {
		// TODO tb.keys and tb.k2i should also be removed
		if tb.finalizer != nil {
			_, existed := tb.strdict[key]
			if existed {
				delete(tb.strdict, key)
				tb.finalizer.adjustKeys(-1)
			}
		} else {
			delete(tb.strdict, key)
		}
	} else {
		if tb.finalizer != nil {
			_, existed := tb.strdict[key]
			tb.strdict[key] = value
			if !existed {
				tb.finalizer.adjustKeys(1)
			}
		} else {
			tb.strdict[key] = value
		}
		lkey := LString(key)
		if _, ok := tb.k2i[lkey]; !ok {
			tb.k2i[lkey] = len(tb.keys)
			tb.keys = append(tb.keys, lkey)
		}
	}
}

// RawSetH sets a given LValue to a given index without the __newindex metamethod.
func (tb *LTable) RawSetH(key LValue, value LValue) {
	if s, ok := key.(LString); ok {
		tb.RawSetString(string(s), value)
		return
	}
	if tb.dict == nil {
		tb.dict = make(map[LValue]LValue, len(tb.strdict))
	}
	if tb.keys == nil {
		tb.keys = []LValue{}
		tb.k2i = map[LValue]int{}
	}

	if value == LNil {
		// TODO tb.keys and tb.k2i should also be removed
		if tb.finalizer != nil {
			_, existed := tb.dict[key]
			if existed {
				delete(tb.dict, key)
				tb.finalizer.adjustKeys(-1)
			}
		} else {
			delete(tb.dict, key)
		}
	} else {
		if tb.finalizer != nil {
			_, existed := tb.dict[key]
			tb.dict[key] = value
			if !existed {
				tb.finalizer.adjustKeys(1)
			}
		} else {
			tb.dict[key] = value
		}
		if _, ok := tb.k2i[key]; !ok {
			tb.k2i[key] = len(tb.keys)
			tb.keys = append(tb.keys, key)
		}
	}
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGet(key LValue) LValue {
	switch v := key.(type) {
	case LNumber:
		if isArrayKey(v) {
			if tb.array == nil {
				return LNil
			}
			index := int(v) - 1
			if index >= len(tb.array) {
				return LNil
			}
			return tb.array[index]
		}
	case LString:
		if tb.strdict == nil {
			return LNil
		}
		if ret, ok := tb.strdict[string(v)]; ok {
			return ret
		}
		return LNil
	}
	if tb.dict == nil {
		return LNil
	}
	if v, ok := tb.dict[key]; ok {
		return v
	}
	return LNil
}

// RawGetInt returns an LValue at position `key` without __index metamethod.
func (tb *LTable) RawGetInt(key int) LValue {
	if tb.array == nil {
		return LNil
	}
	index := int(key) - 1
	if index >= len(tb.array) || index < 0 {
		return LNil
	}
	return tb.array[index]
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetH(key LValue) LValue {
	if s, sok := key.(LString); sok {
		if tb.strdict == nil {
			return LNil
		}
		if v, vok := tb.strdict[string(s)]; vok {
			return v
		}
		return LNil
	}
	if tb.dict == nil {
		return LNil
	}
	if v, ok := tb.dict[key]; ok {
		return v
	}
	return LNil
}

// RawGetString returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetString(key string) LValue {
	if tb.strdict == nil {
		return LNil
	}
	if v, vok := tb.strdict[string(key)]; vok {
		return v
	}
	return LNil
}

// ForEach iterates over this table of elements, yielding each in turn to a given function.
func (tb *LTable) ForEach(cb func(LValue, LValue)) {
	if tb.array != nil {
		for i, v := range tb.array {
			if v != LNil {
				cb(LNumber(i+1), v)
			}
		}
	}
	if tb.strdict != nil {
		for k, v := range tb.strdict {
			if v != LNil {
				cb(LString(k), v)
			}
		}
	}
	if tb.dict != nil {
		for k, v := range tb.dict {
			if v != LNil {
				cb(k, v)
			}
		}
	}
}

// This function is equivalent to lua_next ( http://www.lua.org/manual/5.1/manual.html#lua_next ).
func (tb *LTable) Next(key LValue) (LValue, LValue) {
	init := false
	if key == LNil {
		key = LNumber(0)
		init = true
	}

	if init || key != LNumber(0) {
		if kv, ok := key.(LNumber); ok && isInteger(kv) && int(kv) >= 0 && kv < LNumber(MaxArrayIndex) {
			index := int(kv)
			if tb.array != nil {
				for ; index < len(tb.array); index++ {
					if v := tb.array[index]; v != LNil {
						return LNumber(index + 1), v
					}
				}
			}
			if tb.array == nil || index == len(tb.array) {
				if (tb.dict == nil || len(tb.dict) == 0) && (tb.strdict == nil || len(tb.strdict) == 0) {
					return LNil, LNil
				}
				key = tb.keys[0]
				if v := tb.RawGetH(key); v != LNil {
					return key, v
				}
			}
		}
	}

	for i := tb.k2i[key] + 1; i < len(tb.keys); i++ {
		key := tb.keys[i]
		if v := tb.RawGetH(key); v != LNil {
			return key, v
		}
	}
	return LNil, LNil
}

// tableFinalized is called when the LTable associated with this tableFinalizer has been GC-ed
func tableFinalized(f *tableFinalizer) {
	atomic.AddInt32(&f.allocInfo.numTables, -1)
	atomic.AddInt32(&f.allocInfo.numKeys, -f.numKeys)
}

// adjustKeys is called from the LState's goroutine only
func (f *tableFinalizer) adjustKeys(delta int32) {
	f.numKeys += delta
	// num keys in alloc info must be adjusted atomically as it can be altered via finalizers which can be executed
	// from any go routine
	atomic.AddInt32(&f.allocInfo.numKeys, delta)
}

// CheckQuota checks if this table's alloc info's quotas have been exceeded and if so invoke's the LState's quota
// exceeded callback. CheckQuota is called automatically by vm operation which modify tables, so only needs to be
// invoked directly if you are writing code which has directly added keys to a table using the Raw set methods.
func (tb *LTable) CheckQuota(L *LState) {
	if tb.finalizer != nil {
		ai := tb.finalizer.allocInfo
		if ai.numKeys > ai.maxKeys {
			L.RaiseError("quota exceeded : too many table keys (max is %v)", ai.maxKeys)
		}
		if ai.numTables > ai.maxTables {
			L.RaiseError("quota exceeded : too many tables (max is %v)", ai.maxTables)
		}
	}
}

// SetAllocInfo associates a table alloc info with a table. This will cause this table to atomically increment and
// decrement the fields in the alloc info with the lifetime of the LTable. Note, that there are no guarantees when GC
// will collect an LTable.
// You can pass nil to remove a table from the allocation tracking.
// Pass an LState which an error should be raised in if assigning this the table to the alloc info causes a quota
// violation. You can pass nil if you do not want a quota violation to be raised even if the quota is exceeded.
func (tb *LTable) SetAllocInfo(L *LState, info *LTableAllocInfo) {
	if info != nil {
		if tb.finalizer != nil {
			atomic.AddInt32(&tb.finalizer.allocInfo.numTables, -1)
			atomic.AddInt32(&tb.finalizer.allocInfo.numKeys, -tb.finalizer.numKeys)
			atomic.AddInt32(&info.numTables, 1)
			atomic.AddInt32(&info.numKeys, tb.finalizer.numKeys)
			tb.finalizer.allocInfo = info
			if L != nil {
				tb.CheckQuota(L)
			}
		} else {
			tb.finalizer = &tableFinalizer{allocInfo: info}
			atomic.AddInt32(&info.numTables, 1)
			numKeys := int32(len(tb.strdict) + len(tb.array) + len(tb.dict))
			tb.finalizer.numKeys = numKeys
			atomic.AddInt32(&info.numKeys, numKeys)
			runtime.SetFinalizer(tb.finalizer, tableFinalized)
			if L != nil {
				tb.CheckQuota(L)
			}
		}
	} else if tb.finalizer != nil {
		runtime.SetFinalizer(tb.finalizer, nil)
		atomic.AddInt32(&tb.finalizer.allocInfo.numTables, -1)
		atomic.AddInt32(&tb.finalizer.allocInfo.numKeys, -tb.finalizer.numKeys)
		tb.finalizer = nil
	}
}

// NewTableAllocInfo will create  new LTableAllocInfo. Pass MaxInt32 if you want a value to be unlimited.
func NewTableAllocInfo(maxTables, maxTotalKeys int32) *LTableAllocInfo {
	return &LTableAllocInfo{maxTables: maxTables, maxKeys: maxTotalKeys}
}

// GetTableCount returns the number of tables tracked by this LTableAllocInfo. It is safe to call this from any
// go routine.
func (ti *LTableAllocInfo) GetTableCount() int32 {
	return ti.numTables
}

// GetTableKeyCount returns the number of keys tracked by this LTableAllocInfo. It is safe to call this from any
// go routine.
func (ti *LTableAllocInfo) GetTableKeyCount() int32 {
	return ti.numKeys
}

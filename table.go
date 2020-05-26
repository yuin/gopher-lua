package lua

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

func newLTable(acap int, hcap int) *LTable {
	if acap < 0 {
		acap = 0
	}
	if hcap < 0 {
		hcap = 0
	}
	tb := &LTable{Metatable: LNil}
	if acap != 0 {
		tb.array = make([]LValue, 0, acap)
	}
	if hcap != 0 {
		tb.dict = make(map[LValue]lTableDictValue, hcap)
		tb.keys = make([]*LValue, 0, hcap)
	}
	return tb
}

func (tb *LTable) DictionaryLen() int {
	return len(tb.dict)
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
	case i == larray-1 || i < 0:
		oldval = tb.array[larray-1]
		tb.array = tb.array[:larray-1]
	default:
		oldval = tb.array[i]
		copy(tb.array[i:], tb.array[i+1:])
		tb.array[larray-1] = nil
		tb.array = tb.array[:larray-1]
	}
	return oldval
}

// RawSet sets a given LValue to a given index without the __newindex metamethod.
// It is recommended to use `RawSetString` or `RawSetInt` for performance
// if you already know the given LValue is a string or number.
func (tb *LTable) RawSet(key LValue, value LValue) {
	if v, ok := key.(LNumber); ok && isArrayKey(v) {
		tb.RawSetInt(int(v), value)
	} else {
		tb.RawSetH(key, value)
	}
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
	}
}

// RawSetString sets a given LValue to a given string index without the __newindex metamethod.
func (tb *LTable) RawSetString(key string, value LValue) {
	tb.RawSetH(LString(key), value)
}

// RawSetH sets a given LValue to a given index without the __newindex metamethod.
func (tb *LTable) RawSetH(key LValue, value LValue) {
	if tb.dict == nil {
		tb.dict = make(map[LValue]lTableDictValue, defaultHashCap)
	}
	if tb.keys == nil {
		tb.keys = make([]*LValue, 0, defaultHashCap)
	}
	if value == LNil {
		// TODO tb.keys and tb.k2i should also be removed
		delete(tb.dict, key)
	} else {
		tv := lTableDictValue{value: value}
		if oldTV, ok := tb.dict[key]; ok {
			tv.keyIndex = oldTV.keyIndex
		} else {
			tv.keyIndex = len(tb.keys)
			tb.keys = append(tb.keys, &key)
		}
		tb.dict[key] = tv
	}
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGet(key LValue) LValue {
	if v, ok := key.(LNumber); ok && isArrayKey(v) {
		return tb.RawGetInt(int(v))
	}
	return tb.RawGetH(key)
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
	if tb.dict == nil {
		return LNil
	}
	if v, ok := tb.dict[key]; ok {
		return v.value
	}
	return LNil
}

// RawGetString returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetString(key string) LValue {
	return tb.RawGetH(LString(key))
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
	if tb.dict != nil {
		for k, v := range tb.dict {
			if v.value != LNil {
				cb(k, v.value)
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
				if (tb.dict == nil || len(tb.dict) == 0) {
					return LNil, LNil
				}
				key = *tb.keys[0]
				if v := tb.RawGetH(key); v != LNil {
					return key, v
				}
			}
		}
	}

	for i := tb.dict[key].keyIndex + 1; i < len(tb.keys); i++ {
		key := *tb.keys[i]
		if v := tb.RawGetH(key); v != LNil {
			return key, v
		}
	}
	return LNil, LNil
}

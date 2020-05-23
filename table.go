package lua

const defaultArrayCap = 0
const defaultHashCap = 0

func newLTable(acap int, hcap int) *LTable {
	if acap < 0 {
		acap = 0
	}
	if hcap < 0 {
		hcap = 0
	}
	tb := &LTable{}
	tb.Metatable = LNil
	initltable(&tb.tab, hcap)
	return tb
}

func (tb *LTable) sizearray() int {
	return int(tb.tab.sizearray)
}

func (tb *LTable) resizeArray(nasize int) {
	tb.tab.ResizeArray(nasize)
}

// Len returns length of this LTable.
func (tb *LTable) Len() int {
	return int(tb.tab.GetN())
}

// Append appends a given LValue to this LTable.
func (tb *LTable) Append(value LValue) {
	tb.Insert(-1, value)
}

// Insert inserts a given LValue at position `i` in this table.
func (tb *LTable) Insert(pos int, value LValue) {
	e := int(tb.tab.GetN()) + 1
	if pos == -1 {
		pos = e
	}

	// lua 5.1 implement
	if pos > e {
		e = pos // `grow' array if necessary
	}
	i := 0
	for i = e; i > pos; i-- { /* move up elements */
		pv := tb.tab.GetInt(int64(i) - 1)
		tb.tab.SetInt(int64(i), pv)
	}
	tb.tab.SetInt(int64(pos), value)
}

// MaxN returns a maximum number key that nil value does not exist before it.
func (tb *LTable) MaxN() int {
	k, _, ok := tb.tab.Next(LNil)
	max := LNumber(0)
	for ok {
		if kv, ok := k.(LNumber); ok {
			if kv > max {
				max = kv
			}
		}
		k, _, ok = tb.tab.Next(k)
	}
	return int(max)
}

// Remove removes from this table the element at a given position.
func (tb *LTable) Remove(pos int) LValue {
	size := int(tb.tab.GetN())
	if pos == -1 {
		pos = size
	}
	// lua 5.1 implement
	if !(1 <= pos && pos <= size) { // position is outside bounds?
		// nothing to remove
		return LNil
	}
	oldval := tb.tab.GetInt(int64(pos))
	for ; pos < size; pos++ {
		nv := tb.tab.GetInt(int64(pos) + 1)
		tb.tab.SetInt(int64(pos), nv)
	}
	tb.tab.SetInt(int64(pos), LNil)
	return oldval
}

// RawSet sets a given LValue to a given index without the __newindex metamethod.
// It is recommended to use `RawSetString` or `RawSetInt` for performance
// if you already know the given LValue is a string or number.
func (tb *LTable) RawSet(key LValue, value LValue) {
	tb.tab.Set(key, value)
}

// RawSetInt sets a given LValue at a position `key` without the __newindex metamethod.
func (tb *LTable) RawSetInt(key int, value LValue) {
	tb.tab.SetInt(int64(key), value)
}

// RawSetString sets a given LValue to a given string index without the __newindex metamethod.
func (tb *LTable) RawSetString(key string, value LValue) {
	tb.RawSet(LString(key), value)
}

// RawSetH sets a given LValue to a given index without the __newindex metamethod.
func (tb *LTable) RawSetH(key LValue, value LValue) {
	tb.RawSet(key, value)
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGet(key LValue) LValue {
	return tb.tab.Get(key)
}

// RawGetInt returns an LValue at position `key` without __index metamethod.
func (tb *LTable) RawGetInt(key int) LValue {
	return tb.tab.GetInt(int64(key))
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetH(key LValue) LValue {
	return tb.tab.Get(key)
}

// RawGetString returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetString(key string) LValue {
	return tb.tab.GetString(key)
}

// ForEach iterates over this table of elements, yielding each in turn to a given function.
func (tb *LTable) ForEach(cb func(LValue, LValue)) {
	k, v, ok := tb.tab.Next(LNil)
	for ok {
		cb(k, v)
		k, v, ok = tb.tab.Next(k)
	}
}

// This function is equivalent to lua_next ( http://www.lua.org/manual/5.1/manual.html#lua_next ).
func (tb *LTable) Next(key LValue) (LValue, LValue) {
	k, v, _ := tb.tab.Next(key)
	return k, v
}

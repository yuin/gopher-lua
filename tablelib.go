package lua

import "fmt"

func OpenTable(L *LState) int {
	tabmod := L.RegisterModule(TabLibName, tableFuncs)
	L.Push(tabmod)
	return 1
}

var tableFuncs = map[string]LGFunction{
	"getn":   tableGetN,
	"concat": tableConcat,
	"insert": tableInsert,
	"maxn":   tableMaxN,
	"remove": tableRemove,
	"sort":   tableSort,
}

// Adapt from lua 5.3 implementation.
// https://www.lua.org/source/5.3/ltablib.c.html

type lSortState struct {
	L   *LState
	tab *ltable
	fn  *LFunction
}

func (s lSortState) less(i, j int) bool {
	vi := s.tab.GetInt(int64(i))
	vj := s.tab.GetInt(int64(j))
	return s.lessV(vi, vj)
}

func (s lSortState) lessV(vi, vj LValue) bool {
	if s.fn != nil {
		s.L.Push(s.fn)
		s.L.Push(vi)
		s.L.Push(vj)
		s.L.Call(2, 1)
		return LVAsBool(s.L.reg.Pop())
	}
	return lessThan(s.L, vi, vj)
}

func (s lSortState) swap(i, j int) {
	err := s.tab.Swap(int64(i), int64(j))
	if err != nil {
		s.L.RaiseError("%v", err)
	}
}

func auxsort(state lSortState, l, u int) {
	for l < u {
		var i, j int
		if state.less(u, l) {
			state.swap(u, l)
		}
		if u-l == 1 {
			break
		}
		i = (l + u) / 2
		if state.less(i, l) {
			state.swap(i, l)
		} else {
			if state.less(u, i) {
				state.swap(u, i)
			}
		}
		if u-l == 2 {
			break
		}
		privot := state.tab.GetInt(int64(i))
		state.swap(i, u-1)
		// a[l] <= P == a[u-1] <= a[u], only need to sort from l+1 to u-2
		i = l
		j = u - 1
		for { // invariant: a[l..i] <= P <= a[j..u]
			// repeat ++i until a[i] >= P
			for {
				i++
				// !(i < privot) === i >= privote
				if !state.lessV(state.tab.GetInt(int64(i)), privot) {
					break
				}
				if i >= u {
					state.L.ArgError(2, "invalid order function for sorting")
				}
			}
			// repeat --j until a[j] <= P
			for {
				j--
				if !state.lessV(privot, state.tab.GetInt(int64(j))) {
					break
				}
				if j <= l {
					state.L.ArgError(2, "invalid order function for sorting")
				}
			}
			if j < i {
				break
			}
			state.swap(i, j)
		}
		// swap pivot (a[u-1]) with a[i]
		state.swap(u-1, i)
		// a[l..i-1] <= a[i] == P <= a[i+1..u]
		// adjust so that smaller half is in [j..i] and larger one in [l..u]
		if i-l < u-i {
			j = l
			i = i - 1
			l = i + 2
		} else {
			j = i + 1
			i = u
			u = j - 2
		}
		auxsort(state, j, i)
	}
}

func tableSort(L *LState) int {
	tbl := L.CheckTable(1)
	n := tbl.tab.GetN()
	state := lSortState{
		L:   L,
		tab: &tbl.tab,
	}
	if L.GetTop() != 1 {
		state.fn = L.CheckFunction(2)
	}
	auxsort(state, 1, int(n))
	return 0
}

func tableGetN(L *LState) int {
	L.Push(LNumber(L.CheckTable(1).Len()))
	return 1
}

func tableMaxN(L *LState) int {
	L.Push(LNumber(L.CheckTable(1).MaxN()))
	return 1
}

func wrapRemove(L *LState, tbl *LTable, idx int) LValue {
	defer func() {
		if r := recover(); r != nil {
			L.ArgError(2, fmt.Sprintf("%v", r))
		}
	}()
	return tbl.Remove(idx)
}

func tableRemove(L *LState) int {
	tbl := L.CheckTable(1)
	if L.GetTop() == 1 {
		L.Push(wrapRemove(L, tbl, -1))
	} else {
		L.Push(wrapRemove(L, tbl, L.CheckInt(2)))
	}
	return 1
}

func tableConcat(L *LState) int {
	tbl := L.CheckTable(1)
	sep := LString(L.OptString(2, ""))
	i := L.OptInt(3, 1)
	j := L.OptInt(4, tbl.Len())
	if L.GetTop() == 3 {
		if i > tbl.Len() || i < 1 {
			L.Push(emptyLString)
			return 1
		}
	}
	i = intMax(intMin(i, tbl.Len()), 1)
	j = intMin(intMin(j, tbl.Len()), tbl.Len())
	if i > j {
		L.Push(emptyLString)
		return 1
	}
	//TODO should flushing?
	retbottom := L.GetTop()
	for ; i <= j; i++ {
		v := tbl.RawGetInt(i)
		if !LVCanConvToString(v) {
			L.RaiseError("invalid value (%s) at index %d in table for concat", v.Type().String(), i)
		}
		L.Push(v)
		if i != j {
			L.Push(sep)
		}
	}
	L.Push(stringConcat(L, L.GetTop()-retbottom, L.reg.Top()-1))
	return 1
}

func wrapInsert(L *LState, tbl *LTable, idx int, value LValue) {
	defer func() {
		if r := recover(); r != nil {
			L.ArgError(2, fmt.Sprintf("%v", r))
		}
	}()

	tbl.Insert(idx, value)
}

func tableInsert(L *LState) int {
	tbl := L.CheckTable(1)
	nargs := L.GetTop()
	if nargs == 1 {
		L.RaiseError("wrong number of arguments")
	}
	if L.GetTop() == 2 {
		wrapInsert(L, tbl, -1, L.Get(2))
		return 0
	}
	wrapInsert(L, tbl, L.CheckInt(2), L.CheckAny(3))
	return 0
}

//

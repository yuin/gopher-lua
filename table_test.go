package lua

import (
	"fmt"
	"strings"
	"testing"
)

func TestTableNewLTable(t *testing.T) {
	tbl := newLTable(-1, -2)
	errorIfNotEqual(t, 0, cap(tbl.array))

	tbl = newLTable(10, 9)
	errorIfNotEqual(t, 10, cap(tbl.array))
}

func TestTableLen(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(10, LNil)
	tbl.RawSetInt(9, LNumber(10))
	tbl.RawSetInt(8, LNil)
	tbl.RawSetInt(7, LNumber(10))
	errorIfNotEqual(t, 9, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	errorIfNotEqual(t, 3, tbl.Len())
}

func TestTableLenType(t *testing.T) {
	L := NewState(Options{})
	err := L.DoString(`
        mt = {
            __index = mt,
            __len = function (self)
                return {hello = "world"}
            end
        }

        v = {}
        v.__index = v

        setmetatable(v, mt)

        assert(#v ~= 0, "#v should return a table reference in this case")

        print(#v)
    `)
	if err != nil {
		t.Error(err)
	}
}

func TestTableAppend(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(1, LNumber(1))
	tbl.RawSetInt(2, LNumber(2))
	tbl.RawSetInt(3, LNumber(3))
	errorIfNotEqual(t, 3, tbl.Len())

	tbl.RawSetInt(1, LNil)
	tbl.RawSetInt(2, LNil)
	errorIfNotEqual(t, 3, tbl.Len())

	tbl.Append(LNumber(4))
	errorIfNotEqual(t, 4, tbl.Len())

	tbl.RawSetInt(3, LNil)
	tbl.RawSetInt(4, LNil)
	errorIfNotEqual(t, 0, tbl.Len())

	tbl.Append(LNumber(5))
	errorIfNotEqual(t, 1, tbl.Len())
}

func TestTableInsert(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	tbl.Append(LTrue)

	tbl.Insert(5, LFalse)
	errorIfNotEqual(t, LFalse, tbl.RawGetInt(5))
	errorIfNotEqual(t, 5, tbl.Len())

	tbl.Insert(-10, LFalse)
	errorIfNotEqual(t, LFalse, tbl.RawGet(LNumber(-10)))
	errorIfNotEqual(t, 5, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Append(LNumber(1))
	tbl.Append(LNumber(2))
	tbl.Append(LNumber(3))
	tbl.Insert(1, LNumber(10))
	errorIfNotEqual(t, LNumber(10), tbl.RawGetInt(1))
	errorIfNotEqual(t, LNumber(1), tbl.RawGetInt(2))
	errorIfNotEqual(t, LNumber(2), tbl.RawGetInt(3))
	errorIfNotEqual(t, LNumber(3), tbl.RawGetInt(4))
	errorIfNotEqual(t, 4, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Insert(5, LNumber(10))
	errorIfNotEqual(t, LNumber(10), tbl.RawGetInt(5))

}

func TestTableMaxN(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	errorIfNotEqual(t, 3, tbl.MaxN())

	tbl = newLTable(0, 0)
	errorIfNotEqual(t, 0, tbl.MaxN())

	tbl = newLTable(10, 0)
	errorIfNotEqual(t, 0, tbl.MaxN())
}

func TestTableRemove(t *testing.T) {
	tbl := newLTable(0, 0)
	errorIfNotEqual(t, LNil, tbl.Remove(10))
	tbl.Append(LTrue)
	errorIfNotEqual(t, LNil, tbl.Remove(10))

	tbl.Append(LFalse)
	tbl.Append(LTrue)
	errorIfNotEqual(t, LFalse, tbl.Remove(2))
	errorIfNotEqual(t, 2, tbl.MaxN())
	tbl.Append(LFalse)
	errorIfNotEqual(t, LFalse, tbl.Remove(-1))
	errorIfNotEqual(t, 2, tbl.MaxN())

}

func TestTableRawSetInt(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(MaxArrayIndex+1, LTrue)
	errorIfNotEqual(t, 0, tbl.MaxN())
	errorIfNotEqual(t, LTrue, tbl.RawGet(LNumber(MaxArrayIndex+1)))

	tbl.RawSetInt(1, LTrue)
	tbl.RawSetInt(3, LTrue)
	errorIfNotEqual(t, 3, tbl.MaxN())
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(1))
	errorIfNotEqual(t, LNil, tbl.RawGetInt(2))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(3))
	tbl.RawSetInt(2, LTrue)
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(1))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(2))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(3))
}

func TestTableRawSetH(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetH(LString("key"), LTrue)
	tbl.RawSetH(LString("key"), LNil)
	_, found := tbl.dict[LString("key")]
	errorIfNotEqual(t, false, found)

	tbl.RawSetH(LTrue, LTrue)
	tbl.RawSetH(LTrue, LNil)
	_, foundb := tbl.dict[LTrue]
	errorIfNotEqual(t, false, foundb)
}

func TestTableRawGetH(t *testing.T) {
	tbl := newLTable(0, 0)
	errorIfNotEqual(t, LNil, tbl.RawGetH(LNumber(1)))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LString("key0")))
	tbl.RawSetH(LString("key0"), LTrue)
	tbl.RawSetH(LString("key1"), LFalse)
	tbl.RawSetH(LNumber(1), LTrue)
	errorIfNotEqual(t, LTrue, tbl.RawGetH(LString("key0")))
	errorIfNotEqual(t, LTrue, tbl.RawGetH(LNumber(1)))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LString("notexist")))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LTrue))
}

func TestTableForEach(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LNumber(1))
	tbl.Append(LNumber(2))
	tbl.Append(LNumber(3))
	tbl.Append(LNil)
	tbl.Append(LNumber(5))

	tbl.RawSetH(LString("a"), LString("a"))
	tbl.RawSetH(LString("b"), LString("b"))
	tbl.RawSetH(LString("c"), LString("c"))

	tbl.RawSetH(LTrue, LString("true"))
	tbl.RawSetH(LFalse, LString("false"))

	tbl.ForEach(func(key, value LValue) {
		switch k := key.(type) {
		case LBool:
			switch bool(k) {
			case true:
				errorIfNotEqual(t, LString("true"), value)
			case false:
				errorIfNotEqual(t, LString("false"), value)
			default:
				t.Fail()
			}
		case LNumber:
			switch int(k) {
			case 1:
				errorIfNotEqual(t, LNumber(1), value)
			case 2:
				errorIfNotEqual(t, LNumber(2), value)
			case 3:
				errorIfNotEqual(t, LNumber(3), value)
			case 4:
				errorIfNotEqual(t, LNumber(5), value)
			default:
				t.Fail()
			}
		case LString:
			switch string(k) {
			case "a":
				errorIfNotEqual(t, LString("a"), value)
			case "b":
				errorIfNotEqual(t, LString("b"), value)
			case "c":
				errorIfNotEqual(t, LString("c"), value)
			default:
				t.Fail()
			}
		}
	})
}

// genArrayLiteral builds `return { ["outer"] = { ["arr"] = { 1, 2, ..., n } } }`.
// The nested shape is important: it puts the array inside a parent table
// expression, which is what triggers the PropagateKMV cascade for the
// second bug (see compile.go compileTableExpr / extended OP_SETLIST).
func genArrayLiteral(n int) string {
	var b strings.Builder
	b.WriteString(`return { ["outer"] = { ["arr"] = {`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%d,", i+1)
	}
	b.WriteString("} } }")
	return b.String()
}

// TestCompileTableExpr_LargeArrayLiteral exercises OP_SETLIST's extended
// (C=0) form, which is required once a single array literal exceeds
// 25,550 elements (MAXARG_C * FieldsPerFlush). Pre-fix, the compiler
// emitted a 0 page-index instead of the real one AND the page-index
// word was incorrectly popped by PropagateKMV, dropping the parent
// key's SETTABLE entirely. We assert both the array contents and the
// parent key assignment survive.
func TestCompileTableExpr_LargeArrayLiteral(t *testing.T) {
	for _, n := range []int{
		FieldsPerFlush,                 // 50  — one full batch
		FieldsPerFlush * 511,           // 25550 — last single-instruction page (C=511)
		FieldsPerFlush*511 + 1,         // 25551 — first extended (C=0) page
		FieldsPerFlush * 600,           // 30000 — well past the threshold
		FieldsPerFlush*511 + (50 * 50), // hybrid: many extended pages
	} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			L := NewState()
			defer L.Close()
			if err := L.DoString(genArrayLiteral(n)); err != nil {
				t.Fatalf("DoString: %v", err)
			}
			root := L.Get(-1)
			if root.Type() != LTTable {
				t.Fatalf("root: want table, got %s", root.Type())
			}
			outer := root.(*LTable).RawGetString("outer")
			if outer.Type() != LTTable {
				t.Fatalf("outer: want table, got %s — parent SETTABLE was dropped", outer.Type())
			}
			arr := outer.(*LTable).RawGetString("arr")
			if arr.Type() != LTTable {
				t.Fatalf("outer.arr: want table, got %s — parent SETTABLE was dropped", arr.Type())
			}
			arrT := arr.(*LTable)
			errorIfNotEqual(t, n, arrT.Len())
			// Spot-check first, middle, last to confirm we didn't get a
			// zero-offset corruption (where elements land at index 0/-1).
			errorIfNotEqual(t, LNumber(1), arrT.RawGetInt(1))
			errorIfNotEqual(t, LNumber(n/2), arrT.RawGetInt(n/2))
			errorIfNotEqual(t, LNumber(n), arrT.RawGetInt(n))
		})
	}
}

// TestCompileTableExpr_ExtendedSetlistAtTopLevel covers the simpler case
// where the oversized array literal is returned directly (not nested).
// Only Bug #1 (page-index = 0) hits here; the PropagateKMV cascade
// requires a parent expression. Pre-fix, every element after index
// 25,550 was written to negative array indices and lost.
func TestCompileTableExpr_ExtendedSetlistAtTopLevel(t *testing.T) {
	const n = 30000
	var b strings.Builder
	b.WriteString("return {")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%d,", i+1)
	}
	b.WriteString("}")

	L := NewState()
	defer L.Close()
	if err := L.DoString(b.String()); err != nil {
		t.Fatalf("DoString: %v", err)
	}
	tv := L.Get(-1)
	if tv.Type() != LTTable {
		t.Fatalf("want table, got %s", tv.Type())
	}
	arr := tv.(*LTable)
	errorIfNotEqual(t, n, arr.Len())
	errorIfNotEqual(t, LNumber(n), arr.RawGetInt(n))
}

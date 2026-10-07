package lua

import (
	"fmt"
	"strings"
)

func OpenDebug(L *LState) int {
	dbgmod := L.RegisterModule(DebugLibName, debugFuncs)
	L.Push(dbgmod)
	return 1
}

var debugFuncs = map[string]LGFunction{
	"getfenv":      debugGetFEnv,
	"getinfo":      debugGetInfo,
	"getlocal":     debugGetLocal,
	"getmetatable": debugGetMetatable,
	"getupvalue":   debugGetUpvalue,
	"setfenv":      debugSetFEnv,
	"setlocal":     debugSetLocal,
	"setmetatable": debugSetMetatable,
	"setupvalue":   debugSetUpvalue,
	"traceback":    debugTraceback,
}

func debugGetFEnv(L *LState) int {
	L.Push(L.GetFEnv(L.CheckAny(1)))
	return 1
}

func debugGetInfo(L *LState) int {
	L.CheckTypes(1, LTFunction, LTNumber)
	arg1 := L.Get(1)
	what := L.OptString(2, "Slunf")
	var dbg *Debug
	var fn LValue
	var err error
	var ok bool
	switch lv := arg1.(type) {
	case *LFunction:
		dbg = &Debug{}
		fn, err = L.GetInfo(">"+what, dbg, lv)
	case LNumber:
		dbg, ok = L.GetStack(int(lv))
		if !ok {
			L.Push(LNil)
			return 1
		}
		fn, err = L.GetInfo(what, dbg, LNil)
	}

	if err != nil {
		L.Push(LNil)
		return 1
	}
	tbl := L.NewTable()
	if len(dbg.Name) > 0 {
		tbl.RawSetString("name", LString(dbg.Name))
	} else {
		tbl.RawSetString("name", LNil)
	}
	tbl.RawSetString("what", LString(dbg.What))
	tbl.RawSetString("source", LString(dbg.Source))
	tbl.RawSetString("short_src", LString(chunkID(dbg.Source)))
	tbl.RawSetString("currentline", LNumber(dbg.CurrentLine))
	tbl.RawSetString("nups", LNumber(dbg.NUpvalues))
	tbl.RawSetString("linedefined", LNumber(dbg.LineDefined))
	tbl.RawSetString("lastlinedefined", LNumber(dbg.LastLineDefined))
	tbl.RawSetString("func", fn)
	L.Push(tbl)
	return 1
}

func debugGetLocal(L *LState) int {
	level := L.CheckInt(1)
	idx := L.CheckInt(2)
	dbg, ok := L.GetStack(level)
	if !ok {
		L.ArgError(1, "level out of range")
	}
	name, value := L.GetLocal(dbg, idx)
	if len(name) > 0 {
		L.Push(LString(name))
		L.Push(value)
		return 2
	}
	L.Push(LNil)
	return 1
}

func debugGetMetatable(L *LState) int {
	L.Push(L.GetMetatable(L.CheckAny(1)))
	return 1
}

func debugGetUpvalue(L *LState) int {
	fn := L.CheckFunction(1)
	idx := L.CheckInt(2)
	name, value := L.GetUpvalue(fn, idx)
	if len(name) > 0 {
		L.Push(LString(name))
		L.Push(value)
		return 2
	}
	L.Push(LNil)
	return 1
}

func debugSetFEnv(L *LState) int {
	L.SetFEnv(L.CheckAny(1), L.CheckAny(2))
	return 0
}

func debugSetLocal(L *LState) int {
	level := L.CheckInt(1)
	idx := L.CheckInt(2)
	value := L.CheckAny(3)
	dbg, ok := L.GetStack(level)
	if !ok {
		L.ArgError(1, "level out of range")
	}
	name := L.SetLocal(dbg, idx, value)
	if len(name) > 0 {
		L.Push(LString(name))
	} else {
		L.Push(LNil)
	}
	return 1
}

func debugSetMetatable(L *LState) int {
	L.CheckTypes(2, LTNil, LTTable)
	obj := L.Get(1)
	mt := L.Get(2)
	L.SetMetatable(obj, mt)
	L.SetTop(1)
	return 1
}

func debugSetUpvalue(L *LState) int {
	fn := L.CheckFunction(1)
	idx := L.CheckInt(2)
	value := L.CheckAny(3)
	name := L.SetUpvalue(fn, idx, value)
	if len(name) > 0 {
		L.Push(LString(name))
	} else {
		L.Push(LNil)
	}
	return 1
}

func debugTraceback(L *LState) int {
	msg := ""
	level := L.OptInt(2, 1)
	ls := L
	if L.GetTop() > 0 {
		if s, ok := L.Get(1).(LString); ok {
			msg = string(s)
		}
		if l, ok := L.Get(1).(*LState); ok {
			ls = l
			msg = ""
		}
	}

	traceback := strings.TrimSpace(ls.stackTrace(level))
	if len(msg) > 0 {
		traceback = fmt.Sprintf("%s\n%s", msg, traceback)
	}
	L.Push(LString(traceback))
	return 1
}

// chunkID is the short_src of a source, as luaO_chunkid in Lua 5.1:
// "=name" -> name, "@file" -> file, a string chunk -> [string "first line..."].
func chunkID(source string) string {
	const idSize = 60
	switch {
	case strings.HasPrefix(source, "="):
		s := source[1:]
		if len(s) > idSize-1 {
			s = s[:idSize-1]
		}
		return s
	case strings.HasPrefix(source, "@"):
		s := source[1:]
		if len(s) > idSize-1 {
			s = "..." + s[len(s)-(idSize-1-3):]
		}
		return s
	default:
		s := source
		cut := false
		if i := strings.IndexAny(s, "\r\n"); i >= 0 {
			s, cut = s[:i], true
		}
		if max := idSize - len(`[string "..."]`) - 1; len(s) > max {
			s, cut = s[:max], true
		}
		if cut {
			s += "..."
		}
		return `[string "` + s + `"]`
	}
}

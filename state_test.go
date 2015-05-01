package lua

import (
	"strings"
	"testing"
)

func TestCallStackOverflow(t *testing.T) {
	oldsize := CallStackSize
	CallStackSize = 3
	defer func() {
		CallStackSize = oldsize
	}()
	L := NewState()
	defer L.Close()
	errorIfScriptNotFail(t, L, `
    local function a()
    end
    local function b()
      a()
    end
    local function c()
      print(_printregs())
      b()
    end
    c()
    `, "stack overflow")
}

func TestGetAndReplace(t *testing.T) {
	L := NewState()
	defer L.Close()
	L.Push(LString("a"))
	L.Replace(1, LString("b"))
	L.Replace(0, LString("c"))
	errorIfNotEqual(t, LString("b"), L.Get(1))
	L.Push(LString("c"))
	L.Push(LString("d"))
	L.Replace(-2, LString("e"))
	errorIfNotEqual(t, LString("e"), L.Get(-2))
	registry := L.NewTable()
	L.Replace(RegistryIndex, registry)
	L.G.Registry = registry
	errorIfGFuncNotFail(t, L, func(L *LState) int {
		L.Replace(RegistryIndex, LNil)
		return 0
	}, "registry must be a table")
	errorIfGFuncFail(t, L, func(L *LState) int {
		env := L.NewTable()
		L.Replace(EnvironIndex, env)
		errorIfNotEqual(t, env, L.Get(EnvironIndex))
		return 0
	})
	errorIfGFuncNotFail(t, L, func(L *LState) int {
		L.Replace(EnvironIndex, LNil)
		return 0
	}, "environment must be a table")
	errorIfGFuncFail(t, L, func(L *LState) int {
		gbl := L.NewTable()
		L.Replace(GlobalsIndex, gbl)
		errorIfNotEqual(t, gbl, L.G.Global)
		return 0
	})
	errorIfGFuncNotFail(t, L, func(L *LState) int {
		L.Replace(GlobalsIndex, LNil)
		return 0
	}, "_G must be a table")

	L2 := NewState()
	defer L2.Close()
	clo := L2.NewClosure(func(L2 *LState) int {
		L2.Replace(UpvalueIndex(1), LNumber(3))
		errorIfNotEqual(t, LNumber(3), L2.Get(UpvalueIndex(1)))
		return 0
	}, LNumber(1), LNumber(2))
	L2.SetGlobal("clo", clo)
	errorIfScriptFail(t, L2, `clo()`)
}

func TestObjLen(t *testing.T) {
	L := NewState()
	defer L.Close()
	errorIfNotEqual(t, 3, L.ObjLen(LString("abc")))
	tbl := L.NewTable()
	tbl.Append(LTrue)
	tbl.Append(LTrue)
	errorIfNotEqual(t, 2, L.ObjLen(tbl))
	mt := L.NewTable()
	L.SetField(mt, "__len", L.NewFunction(func(L *LState) int {
		tbl := L.CheckTable(1)
		L.Push(LNumber(tbl.Len() + 1))
		return 1
	}))
	L.SetMetatable(tbl, mt)
	errorIfNotEqual(t, 3, L.ObjLen(tbl))
}

func TestConcat(t *testing.T) {
	L := NewState()
	defer L.Close()
	errorIfNotEqual(t, "a1c", L.Concat(LString("a"), LNumber(1), LString("c")))
}

func TestPCall(t *testing.T) {
	L := NewState()
	defer L.Close()
	L.Register("f1", func(L *LState) int {
		panic("panic!")
		return 0
	})
	errorIfScriptNotFail(t, L, `f1()`, "panic!")
	L.Push(L.GetGlobal("f1"))
	err := L.PCall(0, 0, L.NewFunction(func(L *LState) int {
		L.Push(LString("by handler"))
		return 1
	}))
	errorIfFalse(t, strings.Contains(err.Error(), "by handler"), "")

	err = L.PCall(0, 0, L.NewFunction(func(L *LState) int {
		L.RaiseError("error!")
		return 1
	}))
	errorIfFalse(t, strings.Contains(err.Error(), "error!"), "")

	err = L.PCall(0, 0, L.NewFunction(func(L *LState) int {
		panic("panicc!")
		return 1
	}))
	errorIfFalse(t, strings.Contains(err.Error(), "panicc!"), "")
}

func TestCoroutineApi1(t *testing.T) {
	L := NewState()
	defer L.Close()
	co := L.NewThread()
	errorIfScriptFail(t, L, `
      function coro(v)
        assert(v == 10)
        local ret1, ret2 = coroutine.yield(1,2,3)
        assert(ret1 == 11)
        assert(ret2 == 12)
        coroutine.yield(4)
        return 5
      end
    `)
	fn := L.GetGlobal("coro").(*LFunction)
	st, err, values := L.Resume(co, fn, LNumber(10))
	errorIfNotEqual(t, ResumeYield, st)
	errorIfNotNil(t, err)
	errorIfNotEqual(t, 3, len(values))
	errorIfNotEqual(t, LNumber(1), values[0].(LNumber))
	errorIfNotEqual(t, LNumber(2), values[1].(LNumber))
	errorIfNotEqual(t, LNumber(3), values[2].(LNumber))

	st, err, values = L.Resume(co, fn, LNumber(11), LNumber(12))
	errorIfNotEqual(t, ResumeYield, st)
	errorIfNotNil(t, err)
	errorIfNotEqual(t, 1, len(values))
	errorIfNotEqual(t, LNumber(4), values[0].(LNumber))

	st, err, values = L.Resume(co, fn)
	errorIfNotEqual(t, ResumeOK, st)
	errorIfNotNil(t, err)
	errorIfNotEqual(t, 1, len(values))
	errorIfNotEqual(t, LNumber(5), values[0].(LNumber))

	L.Register("myyield", func(L *LState) int {
		return L.Yield(L.ToNumber(1))
	})
	errorIfScriptFail(t, L, `
      function coro_error()
        coroutine.yield(1,2,3)
        myyield(4)
        assert(false, "--failed--")
      end
    `)
	fn = L.GetGlobal("coro_error").(*LFunction)
	co = L.NewThread()
	st, err, values = L.Resume(co, fn)
	errorIfNotEqual(t, ResumeYield, st)
	errorIfNotNil(t, err)
	errorIfNotEqual(t, 3, len(values))
	errorIfNotEqual(t, LNumber(1), values[0].(LNumber))
	errorIfNotEqual(t, LNumber(2), values[1].(LNumber))
	errorIfNotEqual(t, LNumber(3), values[2].(LNumber))

	st, err, values = L.Resume(co, fn)
	errorIfNotEqual(t, ResumeYield, st)
	errorIfNotNil(t, err)
	errorIfNotEqual(t, 1, len(values))
	errorIfNotEqual(t, LNumber(4), values[0].(LNumber))

	st, err, values = L.Resume(co, fn)
	errorIfNotEqual(t, ResumeError, st)
	errorIfNil(t, err)
	errorIfFalse(t, strings.Contains(err.Error(), "--failed--"), "error message must be '--failed--'")
	st, err, values = L.Resume(co, fn)
	errorIfNotEqual(t, ResumeError, st)
	errorIfNil(t, err)
	errorIfFalse(t, strings.Contains(err.Error(), "can not resume a dead thread"), "can not resume a dead thread")

}

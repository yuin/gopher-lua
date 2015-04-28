package lua

import (
	"strings"
	"testing"
)

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

	L.SetGlobal("myyield", L.NewFunction(func(L *LState) int {
		return L.Yield(L.ToNumber(1))
	}))
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

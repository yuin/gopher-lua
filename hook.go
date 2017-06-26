package lua

import "fmt"

type Hooker interface {
	call(L *LState, cf *callFrame)
	String() string
}

type LHook struct {
	callback *LFunction
	line     int
}

func newLHook(callback *LFunction, line int) *LHook {
	return &LHook{
		callback: callback,
		line:     line,
	}
}

func (lh *LHook) call(L *LState, cf *callFrame) {
	currentline := cf.Fn.Proto.DbgSourcePositions[cf.Pc-1]
	if currentline != 0 && cf.Fn != lh.callback && currentline != L.prevline {
		L.reg.Push(lh.callback)
		L.reg.Push(LString("line"))
		L.reg.Push(LNumber(currentline))
		L.callR(2, 0, -1)
		L.prevline = currentline
	}
}

func (lh *LHook) String() string {
	return fmt.Sprintf("hook: %p", lh)
}

type CHook struct {
	callback *LFunction
	line     int
}

func newCHook(callback *LFunction, line int) *CHook {
	return &CHook{
		callback: callback,
		line:     line,
	}
}

func (ch *CHook) call(L *LState, cf *callFrame) {

}

func (ch *CHook) String() string {
	return fmt.Sprintf("hook: %p", ch)
}

type RHook struct {
	callback *LFunction
	line     int
}

func newRHook(callback *LFunction, line int) *RHook {
	return &RHook{
		callback: callback,
		line:     line,
	}
}

func (rh *RHook) call(L *LState, cf *callFrame) {

}

func (rh *RHook) String() string {
	return fmt.Sprintf("hook: %p", rh)
}

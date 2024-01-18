package lua

import (
	"fmt"
)

type Hooker interface {
	call(L *LState, cf *callFrame)
	String() string
}

type LHook struct {
	callback *LFunction
	line     int
	isCalled bool // prevent recursion
}

func newLHook(callback *LFunction, line int) *LHook {
	return &LHook{
		callback: callback,
		line:     line,
	}
}
func isFunctionInCallFrameChain(cf *callFrame, fn *LFunction) bool {
	// 检查当前callFrame的Fn是否等于目标LFunction
	if cf.Fn == fn {
		return true
	}

	// 递归检查Parent callFrame
	if cf.Parent != nil {
		return isFunctionInCallFrameChain(cf.Parent, fn)
	}

	// 如果递归到达了顶层（Parent为nil），则说明未找到目标LFunction
	return false
}

// cf.Fn != lh.callback
func (lh *LHook) call(L *LState, cf *callFrame) {
	currentline := cf.Fn.Proto.DbgSourcePositions[cf.Pc-1]
	if currentline != 0 && cf.Fn != lh.callback && currentline != L.prevline && !lh.isCalled {
		// if currentline != 0 && cf.Fn != lh.callback && currentline != L.prevline && !isFunctionInCallFrameChain(cf, lh.callback) {
		lh.isCalled = true
		// fmt.Println("lh.line", lh.line)
		// fmt.Println("currentline", currentline)
		// fmt.Println("cf.Fn.Proto.LastLineDefined", cf.Fn.Proto.LastLineDefined)
		// fmt.Println("line call debug hook")
		// fmt.Println("cf.Fn.Proto.SourceName", cf.Fn.Proto.SourceName)
		L.reg.Push(lh.callback)
		L.reg.Push(LString("line"))
		L.reg.Push(LNumber(currentline))
		L.callR(2, 0, -1)
		L.prevline = currentline
		lh.isCalled = false
	}
}

func (lh *LHook) String() string {
	return fmt.Sprintf("hook: %p", lh)
}

type CTHook struct {
	callback     *LFunction
	count        int
	currentCount int
}

func newCTHook(callback *LFunction, count int) *CTHook {
	return &CTHook{
		callback: callback,
		count:    count,
	}
}

func (ct *CTHook) call(L *LState, cf *callFrame) {

	ct.currentCount++
	if ct.currentCount == ct.count {
		// print("ct count call debug hook\n")
		// fmt.Println("ct.Fn.Proto.SourceName", cf.Fn.Proto.SourceName)
		L.reg.Push(ct.callback)
		L.reg.Push(LString("count"))
		L.callR(1, 0, -1)
		ct.currentCount = 0
	}
}

func (ct *CTHook) String() string {
	return fmt.Sprintf("hook: %p", ct)
}

type CHook struct {
	callback *LFunction
}

func newCHook(callback *LFunction) *CHook {
	return &CHook{
		callback: callback,
	}
}

func (ch *CHook) call(L *LState, cf *callFrame) {

	if ch.callback != cf.Fn {
		// fmt.Println("ch.Fn.Proto.SourceName", cf.Fn.Proto.SourceName)
		// print("func call debug hook\n")
		L.reg.Push(ch.callback)
		L.reg.Push(LString("call"))
		L.callR(1, 0, -1)
	}
}

func (ch *CHook) String() string {
	return fmt.Sprintf("hook: %p", ch)
}

type RHook struct {
	callback *LFunction
}

func newRHook(callback *LFunction) *RHook {
	return &RHook{
		callback: callback,
	}
}

func (rh *RHook) call(L *LState, cf *callFrame) {

	// if rh.callback != cf.Fn && !isFunctionInCallFrameChain(cf, rh.callback) {
	if rh.callback != cf.Fn {
		// fmt.Println(cf.TailCall, "rh call debug hook")
		// fmt.Println(".Fn.Proto.SourceName", cf.Fn.Proto.SourceName)
		L.reg.Push(rh.callback)
		L.reg.Push(LString("return"))
		L.callR(1, 0, -1)
	}
}
func (rh *RHook) String() string {
	return fmt.Sprintf("hook: %p", rh)
}

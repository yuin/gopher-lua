package lua

import (
	"fmt"
	"github.com/yuin/gopher-lua/parse"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const MultRet = -1
const RegistryIndex = -10000
const EnvironIndex = -10001
const GlobalsIndex = -10002

/* ApiError {{{ */

type ApiError struct {
	Type   ApiErrorType
	Object LValue
}

func newApiError(code ApiErrorType, message string, object LValue) *ApiError {
	if len(message) > 0 {
		object = LString(message)
	}
	return &ApiError{code, object}
}

func (e *ApiError) Error() string {
	return e.Object.String()
}

type ApiErrorType int

const (
	ApiErrorSyntax ApiErrorType = iota
	ApiErrorFile
	ApiErrorRun
	ApiErrorError
	ApiErrorPanic
)

/* }}} */

/* ResumeState {{{ */

type ResumeState int

const (
	ResumeOK ResumeState = iota
	ResumeYield
	ResumeError
)

/* }}} */

/* P {{{ */

type P struct {
	Fn      LValue
	NRet    int
	Protect bool
	Handler *LFunction
}

/* }}} */

/* Debug {{{ */

type Debug struct {
	frame           *callFrame
	Name            string
	What            string
	Source          string
	CurrentLine     int
	NUpvalues       int
	LineDefined     int
	LastLineDefined int
}

/* }}} */

/* callFrame {{{ */

type callFrame struct {
	Idx        int
	Fn         *LFunction
	Parent     *callFrame
	Pc         int
	Base       int
	LocalBase  int
	ReturnBase int
	NArgs      int
	NRet       int
	TailCall   int
}

type callFrameStack struct {
	array []callFrame
	sp    int
}

func newcallFrameStack(size int) *callFrameStack {
	return &callFrameStack{
		array: make([]callFrame, size),
		sp:    0,
	}
}

func (cs *callFrameStack) IsEmpty() bool { return cs.sp == 0 }

func (cs *callFrameStack) Clear() {
	cs.sp = 0
}

func (cs *callFrameStack) Push(v callFrame) error {
	if cs.sp == CallStackSize {
		return newApiError(ApiErrorRun, "stack overflow", LNil)
	}
	cs.array[cs.sp] = v
	cs.array[cs.sp].Idx = cs.sp
	cs.sp++
	return nil
}

func (cs *callFrameStack) Remove(sp int) {
	psp := sp - 1
	nsp := sp + 1
	var pre *callFrame
	var next *callFrame
	if psp > 0 {
		pre = &cs.array[psp]
	}
	if nsp < cs.sp {
		next = &cs.array[nsp]
	}
	if next != nil {
		next.Parent = pre
	}
	for i := sp; i+1 < cs.sp; i++ {
		cs.array[i] = cs.array[i+1]
		cs.array[i].Idx = i
		cs.sp = i
	}
	cs.sp++
}

func (cs *callFrameStack) Sp() int {
	return cs.sp
}

func (cs *callFrameStack) SetSp(sp int) {
	cs.sp = sp
}

func (cs *callFrameStack) Last() *callFrame {
	if cs.sp == 0 {
		return nil
	}
	return &cs.array[cs.sp-1]
}

func (cs *callFrameStack) At(sp int) *callFrame {
	return &cs.array[sp]
}

func (cs *callFrameStack) Pop() *callFrame {
	cs.sp--
	return &cs.array[cs.sp]
}

/* }}} */

/* registry {{{ */

type registry struct {
	array []LValue
	top   int
}

func newRegistry(size int) *registry {
	return &registry{make([]LValue, size), 0}
}

func (rg *registry) RawSetTop(top int) {
	rg.top = top
}

func (rg *registry) SetTop(top int) {
	oldtop := rg.top
	rg.top = top
	for i := oldtop; i < rg.top; i++ {
		rg.array[i] = LNil
	}
	for i := rg.top; i < oldtop; i++ {
		rg.array[i] = LNil
	}
}

func (rg *registry) Top() int {
	return rg.top
}

func (rg *registry) Push(v LValue) {
	rg.array[rg.top] = v
	rg.top++
}

func (rg *registry) Pop() LValue {
	v := rg.array[rg.top-1]
	rg.array[rg.top-1] = LNil
	rg.top--
	return v
}

func (rg *registry) Get(reg int) LValue {
	return rg.array[reg]
}

func (rg *registry) CopyRange(reg, start, limit, n int) {
	for i := 0; i < n; i++ {
		if tidx := start + i; tidx >= rg.top || limit > -1 && tidx >= limit || tidx < 0 {
			rg.array[reg+i] = LNil
		} else {
			rg.array[reg+i] = rg.array[tidx]
		}
	}
	rg.top = reg + n
}

func (rg *registry) FillNil(reg, n int) {
	for i := 0; i < n; i++ {
		rg.array[reg+i] = LNil
	}
	rg.top = reg + n
}

func (rg *registry) Insert(value LValue, reg int) {
	top := rg.Top()
	if reg >= top {
		rg.Set(reg, value)
		return
	}
	top--
	for ; top >= reg; top-- {
		rg.Set(top+1, rg.Get(top))
	}
	rg.Set(reg, value)
}

func (rg *registry) Set(reg int, val LValue) {
	rg.array[reg] = val
	if reg >= rg.top {
		//rg.FillNil(rg.top, reg-rg.top)
		rg.top = reg + 1
	}
} /* }}} */

/* Global {{{ */

func newGlobal() *Global {
	return &Global{
		MainThread: nil,
		Registry:   newLTable(0, 32),
		Global:     newLTable(0, 64),
		builtinMts: make(map[int]LValue),
		tempFiles:  make([]*os.File, 0, 10),
	}
}

/* }}} */

/* package local methods {{{ */

func newLState() *LState {
	ls := &LState{
		G:      newGlobal(),
		Parent: nil,
		Panic: func(L *LState) {
			panic(L.Get(-1))
		},
		Dead: false,

		stop:         0,
		reg:          newRegistry(RegistrySize),
		stack:        newcallFrameStack(CallStackSize),
		currentFrame: nil,
		wrapped:      false,
		uvcache:      nil,
	}
	ls.Env = ls.G.Global
	return ls
}

func (ls *LState) printReg() {
	println("-------------------------")
	println("thread:", ls)
	println("top:", ls.reg.Top())
	if ls.currentFrame != nil {
		println("function base:", ls.currentFrame.Base)
		println("return base:", ls.currentFrame.ReturnBase)
	} else {
		println("(vm not started)")
	}
	println("local base:", ls.currentLocalBase())
	for i := 0; i < ls.reg.Top(); i++ {
		println(i, ls.reg.Get(i).String())
	}
	println("-------------------------")
}

func (ls *LState) printCallStack() {
	println("-------------------------")
	for i := 0; i < ls.stack.Sp(); i++ {
		print(i)
		print(" ")
		frame := ls.stack.At(i)
		if frame == nil {
			break
		}
		if frame.Fn.IsG {
			println("IsG:", true, "Frame:", frame, "Fn:", frame.Fn)
		} else {
			println("IsG:", false, "Frame:", frame, "Fn:", frame.Fn, "pc:", frame.Pc)
		}
	}
	println("-------------------------")
}

func (ls *LState) closeAllUpvalues() {
	for cf := ls.currentFrame; cf != nil; cf = cf.Parent {
		if !cf.Fn.IsG {
			ls.closeUpvalues(cf.LocalBase)
		}
	}
}

func (ls *LState) raiseError(level int, format string, args ...interface{}) {
	ls.closeAllUpvalues()
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	if level > 0 {
		message = fmt.Sprintf("%v %v", ls.Where(level-1), message)
		message = ls.stackTrace(message, true)
	}
	ls.reg.Push(LString(message))
	ls.Panic(ls)
}

func (ls *LState) findLocal(frame *callFrame, no int) string {
	fn := frame.Fn
	if !fn.IsG {
		if name, ok := fn.LocalName(no, frame.Pc-1); ok {
			return name
		}
	}
	var top int
	if ls.currentFrame == frame {
		top = ls.reg.Top()
	} else if frame.Idx+1 < ls.stack.Sp() {
		top = ls.stack.At(frame.Idx + 1).Base
	} else {
		return ""
	}
	if top-frame.LocalBase >= no {
		return "(*temporary)"
	}
	return ""
}

func (ls *LState) stackTrace(message string, include bool) string {
	buf := []string{}
	if len(message) > 0 {
		buf = append(buf, message)
	}
	buf = append(buf, "stack traceback:")
	if ls.currentFrame != nil {
		i := 1
		if include {
			i = 0
		}
		for dbg, ok := ls.GetStack(i); ok; dbg, ok = ls.GetStack(i) {
			cf := dbg.frame
			buf = append(buf, fmt.Sprintf("\t%v in %v", ls.Where(i), ls.frameFuncName(cf)))
			if !cf.Fn.IsG && cf.TailCall > 0 {
				for tc := cf.TailCall; tc > 0; tc-- {
					buf = append(buf, "\t(tailcall): ?")
					i++
				}
			}
			i++
		}
	}
	buf = append(buf, fmt.Sprintf("\t%v: %v", "[G]", "?"))
	if len(buf) > 10 {
		newbuf := make([]string, 0, 20)
		newbuf = append(newbuf, buf[0:7]...)
		newbuf = append(newbuf, "\t...")
		newbuf = append(newbuf, buf[len(buf)-7:len(buf)-1]...)
		buf = newbuf
	}
	ret := strings.Join(buf, "\n")
	if len(buf) > 0 {
		return "\n" + ret
	}
	return ret
}

func (ls *LState) frameFuncName(fr *callFrame) string {
	frame := fr.Parent
	if frame == nil {
		if ls.Parent == nil {
			return "main chunk"
		} else {
			return "corountine"
		}
	}
	if !frame.Fn.IsG {
		pc := frame.Pc - 1
		for _, call := range frame.Fn.Proto.DbgCalls {
			if call.Pc == pc {
				name := call.Name
				if (name == "?" || fr.TailCall > 0) && !fr.Fn.IsG {
					name = fmt.Sprintf("<%v:%v>", fr.Fn.Proto.SourceName, fr.Fn.Proto.LineDefined)
				}
				return name
			}
		}
	}
	return "anonymous function"
}

func (ls *LState) isStarted() bool {
	return ls.currentFrame != nil
}

func (ls *LState) kill() {
	ls.Dead = true
}

func (ls *LState) indexToReg(idx int) int {
	base := ls.currentLocalBase()
	if idx > 0 {
		return base + idx - 1
	} else if idx == 0 {
		return -1
	} else {
		tidx := ls.reg.Top() + idx
		if tidx < base {
			return -1
		}
		return tidx
	}
}

func (ls *LState) currentLocalBase() int {
	base := 0
	if ls.currentFrame != nil {
		base = ls.currentFrame.LocalBase
	}
	return base
}

func (ls *LState) currentEnv() *LTable {
	return ls.Env
	/*
		if ls.currentFrame == nil {
			return ls.Env
		}
		return ls.currentFrame.Fn.Env
	*/
}

func (ls *LState) rkValue(idx int) LValue {
	/*
		if OpIsK(idx) {
			return ls.currentFrame.Fn.Proto.Constants[opIndexK(idx)]
		}
		return ls.reg.Get(ls.currentFrame.LocalBase + idx)
	*/
	if (idx & opBitRk) != 0 {
		return ls.currentFrame.Fn.Proto.Constants[idx & ^opBitRk]
	}
	return ls.reg.array[ls.currentFrame.LocalBase+idx]
}

func (ls *LState) closeUpvalues(idx int) {
	if ls.uvcache == nil {
		return
	}
	var prev *Upvalue
	for uv := ls.uvcache; uv != nil; uv = uv.next {
		if uv.index >= idx {
			if prev != nil {
				prev.next = nil
			} else {
				ls.uvcache = nil
			}
			uv.Close()
		}
		prev = uv
	}
}

func (ls *LState) findUpvalue(idx int) *Upvalue {
	var prev *Upvalue
	var next *Upvalue
	if ls.uvcache != nil {
		for uv := ls.uvcache; uv != nil; uv = uv.next {
			if uv.index == idx {
				return uv
			}
			if uv.index > idx {
				next = uv
				break
			}
			prev = uv
		}
	}
	uv := &Upvalue{reg: ls.reg, index: idx, closed: false}
	if prev != nil {
		prev.next = uv
	} else {
		ls.uvcache = uv
	}
	if next != nil {
		uv.next = next
	}
	return uv
}

func (ls *LState) metatable(lvalue LValue, rawget bool) LValue {
	var metatable LValue = LNil
	switch obj := lvalue.(type) {
	case *LTable:
		metatable = obj.Metatable
	case *LUserData:
		metatable = obj.Metatable
	default:
		if table, ok := ls.G.builtinMts[int(obj.Type())]; ok {
			metatable = table
		}
	}

	if !rawget && metatable != LNil {
		oldmt := metatable
		if tb, ok := metatable.(*LTable); ok {
			metatable = tb.RawGetH(LString("__metatable"))
			if metatable == LNil {
				metatable = oldmt
			}
		}
	}

	return metatable
}

func (ls *LState) metaOp1(lvalue LValue, event string) LValue {
	if mt := ls.metatable(lvalue, true); mt != LNil {
		if tb, ok := mt.(*LTable); ok {
			return tb.RawGetH(LString(event))
		}
	}
	return LNil
}

func (ls *LState) metaOp2(value1, value2 LValue, event string) LValue {
	if mt := ls.metatable(value1, true); mt != LNil {
		if tb, ok := mt.(*LTable); ok {
			if ret := tb.RawGetH(LString(event)); ret != LNil {
				return ret
			}
		}
	}
	if mt := ls.metatable(value2, true); mt != LNil {
		if tb, ok := mt.(*LTable); ok {
			return tb.RawGetH(LString(event))
		}
	}
	return LNil
}

func (ls *LState) metaCall(lvalue LValue) (*LFunction, bool) {
	if fn, ok := lvalue.(*LFunction); ok {
		return fn, false
	}
	if fn, ok := ls.metaOp1(lvalue, "__call").(*LFunction); ok {
		return fn, true
	}
	return nil, false
}

func (ls *LState) initCallFrame(cf *callFrame) {
	if cf.Fn.IsG {
		ls.reg.SetTop(cf.LocalBase + cf.NArgs)
	} else {
		/* swap vararg positions:
				   closure
				   namedparam1 <- lbase
				   namedparam2
				   vararg1
				   vararg2

		           TO

				   closure
				   nil
				   nil
				   vararg1
				   vararg2
				   namedparam1 <- lbase
				   namedparam2
		*/
		proto := cf.Fn.Proto
		nargs := cf.NArgs
		np := int(proto.NumParameters)
		nvarargs := nargs - np
		if nvarargs < 0 {
			nvarargs = 0
		}
		for i := nargs; i < np; i++ {
			//ls.reg.Set(cf.LocalBase+i, LNil)
			ls.reg.array[cf.LocalBase+i] = LNil
			nargs = np
		}

		if (proto.IsVarArg & VarArgIsVarArg) != 0 {
			ls.reg.SetTop(cf.LocalBase + nargs + np)
			for i := 0; i < np; i++ {
				//ls.reg.Set(cf.LocalBase+nargs+i, ls.reg.Get(cf.LocalBase+i))
				ls.reg.array[cf.LocalBase+nargs+i] = ls.reg.array[cf.LocalBase+i]
				//ls.reg.Set(cf.LocalBase+i, LNil)
				ls.reg.array[cf.LocalBase+i] = LNil
			}

			if CompatVarArg {
				ls.reg.SetTop(cf.LocalBase + nargs + np + 1)
				if (proto.IsVarArg & VarArgNeedsArg) != 0 {
					argtb := newLTable(nvarargs, 0)
					for i := 0; i < nvarargs; i++ {
						argtb.RawSetInt(i+1, ls.reg.Get(cf.LocalBase+np+i))
					}
					argtb.RawSetH(LString("n"), LNumber(nvarargs))
					//ls.reg.Set(cf.LocalBase+nargs+np, argtb)
					ls.reg.array[cf.LocalBase+nargs+np] = argtb
				} else {
					ls.reg.array[cf.LocalBase+nargs+np] = LNil
				}
			}
			cf.LocalBase += nargs
		} else {
			for i := np; i < nargs; i++ {
				ls.reg.array[cf.LocalBase+i] = LNil
			}
			ls.reg.SetTop(cf.LocalBase + np + 1)
		}
		maxreg := cf.LocalBase + int(proto.NumUsedRegisters)
		top := ls.reg.Top()
		ls.reg.SetTop(maxreg)
		for i := top; i < maxreg; i++ {
			//ls.reg.Set(i, LNil)
			ls.reg.array[i] = LNil
		}
	}
}

func (ls *LState) pushCallFrame(cf callFrame, fn LValue, meta bool) {
	if meta {
		cf.NArgs++
		ls.reg.Insert(fn, cf.LocalBase)
	}
	if cf.Fn == nil {
		ls.RaiseError("attempt to call a non-function object")
	}
	err := ls.stack.Push(cf)
	if err != nil {
		ls.RaiseError(err.Error())
	}
	newcf := ls.stack.Last()
	ls.initCallFrame(newcf)
	ls.currentFrame = newcf
}

func (ls *LState) callR(nargs, nret, rbase int) {
	base := ls.reg.Top() - nargs - 1
	if rbase < 0 {
		rbase = base
	}
	lv := ls.reg.Get(base)
	fn, meta := ls.metaCall(lv)
	ls.pushCallFrame(callFrame{
		Fn:         fn,
		Pc:         0,
		Base:       base,
		LocalBase:  base + 1,
		ReturnBase: rbase,
		NArgs:      nargs,
		NRet:       nret,
		Parent:     ls.currentFrame,
		TailCall:   0,
	}, lv, meta)
	if ls.G.MainThread == nil {
		ls.G.MainThread = ls
		ls.G.CurrentThread = ls
		mainLoop(ls, nil)
	} else {
		mainLoop(ls, ls.currentFrame)
	}
	if nret != MultRet {
		ls.reg.SetTop(rbase + nret)
	}
}

func (ls *LState) getField(obj LValue, key LValue) LValue {
	curobj := obj
	for i := 0; i < MaxTableGetLoop; i++ {
		tb, istable := curobj.(*LTable)
		if istable {
			ret := tb.RawGet(key)
			if ret != LNil {
				return ret
			}
		}
		metaindex := ls.metaOp1(curobj, "__index")
		if metaindex == LNil {
			if !istable {
				ls.RaiseError("attempt to index a non-table object(%v)", curobj.Type().String())
			}
			return LNil
		}
		if metaindex.Type() == LTFunction {
			ls.reg.Push(metaindex)
			ls.reg.Push(curobj)
			ls.reg.Push(key)
			ls.Call(2, 1)
			return ls.reg.Pop()
		} else {
			curobj = metaindex
		}
	}
	ls.RaiseError("too many recursions in gettable")
	return nil
}

func (ls *LState) setField(obj LValue, key LValue, value LValue) {
	curobj := obj
	for i := 0; i < MaxTableGetLoop; i++ {
		tb, istable := curobj.(*LTable)
		if istable {
			if tb.RawGet(key) != LNil {
				ls.RawSet(tb, key, value)
				return
			}
		}
		metaindex := ls.metaOp1(curobj, "__newindex")
		if metaindex == LNil {
			if !istable {
				ls.RaiseError("attempt to index a non-table object(%v)", curobj.Type().String())
			}
			ls.RawSet(tb, key, value)
			return
		}
		if metaindex.Type() == LTFunction {
			ls.reg.Push(metaindex)
			ls.reg.Push(curobj)
			ls.reg.Push(key)
			ls.reg.Push(value)
			ls.Call(3, 0)
			return
		} else {
			curobj = metaindex
		}
	}
	ls.RaiseError("too many recursions in settable")
}

/* }}} */

/* api methods {{{ */

func NewState() *LState {
	ls := newLState()
	ls.OpenLibs()
	return ls
}

func (ls *LState) Close() {
	atomic.AddInt32(&ls.stop, 1)
	for _, file := range ls.G.tempFiles {
		// ignore errors in these operations
		file.Close()
		os.Remove(file.Name())
	}
}

/* registry operations {{{ */

func (ls *LState) GetTop() int {
	return ls.reg.Top() - ls.currentLocalBase()
}

func (ls *LState) SetTop(idx int) {
	base := ls.currentLocalBase()
	newtop := ls.indexToReg(idx) + 1
	if newtop < base {
		ls.reg.SetTop(base)
	} else {
		ls.reg.SetTop(newtop)
	}
}

func (ls *LState) Replace(idx int, value LValue) {
	base := ls.currentLocalBase()
	if idx > 0 {
		reg := base + idx - 1
		if reg < ls.reg.Top() {
			ls.reg.Set(reg, value)
		}
	} else if idx == 0 {
	} else if idx > RegistryIndex {
		if tidx := ls.reg.Top() + idx; tidx >= base {
			ls.reg.Set(tidx, value)
		}
	} else {
		switch idx {
		case RegistryIndex:
			if tb, ok := value.(*LTable); ok {
				ls.G.Registry = tb
			} else {
				ls.RaiseError("registry must be a table(%v)", value.Type().String())
			}
		case EnvironIndex:
			if ls.currentFrame == nil {
				ls.RaiseError("no calling environment")
			}
			if tb, ok := value.(*LTable); ok {
				ls.currentFrame.Fn.Env = tb
			} else {
				ls.RaiseError("environment must be a table(%v)", value.Type().String())
			}
		case GlobalsIndex:
			if tb, ok := value.(*LTable); ok {
				ls.G.Global = tb
			} else {
				ls.RaiseError("_G must be a table(%v)", value.Type().String())
			}
		default:
			fn := ls.currentFrame.Fn
			index := GlobalsIndex - idx - 1
			if index < len(fn.Upvalues) {
				fn.Upvalues[index].SetValue(value)
			}
		}
	}
}

func (ls *LState) Get(idx int) LValue {
	base := ls.currentLocalBase()
	if idx > 0 {
		reg := base + idx - 1
		if reg < ls.reg.Top() {
			return ls.reg.Get(reg)
		}
		return LNil
	} else if idx == 0 {
		return LNil
	} else if idx > RegistryIndex {
		tidx := ls.reg.Top() + idx
		if tidx < base {
			return LNil
		}
		return ls.reg.Get(tidx)
	} else {
		switch idx {
		case RegistryIndex:
			return ls.G.Registry
		case EnvironIndex:
			if ls.currentFrame == nil {
				return ls.Env
			}
			return ls.currentFrame.Fn.Env
		case GlobalsIndex:
			return ls.G.Global
		default:
			fn := ls.currentFrame.Fn
			index := GlobalsIndex - idx - 1
			if index < len(fn.Upvalues) {
				return fn.Upvalues[index].Value()
			}
			return LNil
		}
	}
	return LNil
}

func (ls *LState) Push(value LValue) {
	ls.reg.Push(value)
}

func (ls *LState) Pop(n int) {
	for i := 0; i < n; i++ {
		if ls.GetTop() == 0 {
			ls.RaiseError("register underflow")
		}
		ls.reg.Pop()
	}
}

func (ls *LState) Insert(value LValue, index int) {
	reg := ls.indexToReg(index)
	top := ls.reg.Top()
	if reg >= top {
		ls.reg.Set(reg, value)
		return
	}
	if reg <= ls.currentLocalBase() {
		reg = ls.currentLocalBase()
	}
	top--
	for ; top >= reg; top-- {
		ls.reg.Set(top+1, ls.reg.Get(top))
	}
	ls.reg.Set(reg, value)
}

func (ls *LState) Remove(index int) {
	reg := ls.indexToReg(index)
	top := ls.reg.Top()
	switch {
	case reg > top:
		return
	case reg < ls.currentLocalBase():
		return
	case reg == top:
		ls.Pop(1)
		return
	}
	for i := reg; i < top-1; i++ {
		ls.reg.Set(i, ls.reg.Get(i+1))
	}
	ls.reg.SetTop(top - 1)
}

/* }}} */

/* object allocation {{{ */

func (ls *LState) NewTable() *LTable {
	// TODO change size
	return newLTable(32, 32)
}

func (ls *LState) CreateTable(acap, hcap int) *LTable {
	return newLTable(acap, hcap)
}

func (ls *LState) NewThread() *LState {
	thread := newLState()
	thread.G = ls.G
	thread.Env = ls.Env
	return thread
}

func (ls *LState) NewUserData() *LUserData {
	return &LUserData{
		Env:       ls.currentEnv(),
		Metatable: LNil,
	}
}

func (ls *LState) NewFunction(fn LGFunction) *LFunction {
	return newLFunctionG(fn, ls.currentEnv(), 0)
}

func (ls *LState) NewClosure(fn LGFunction, upvalues ...LValue) *LFunction {
	cl := newLFunctionG(fn, ls.currentEnv(), len(upvalues))
	for i, lv := range upvalues {
		cl.Upvalues[i] = &Upvalue{}
		cl.Upvalues[i].Close()
		cl.Upvalues[i].SetValue(lv)
	}
	return cl
}

/* }}} */

/* toType {{{ */

func (ls *LState) ToBool(n int) bool {
	return LVAsBool(ls.Get(n))
}

func (ls *LState) ToInt(n int) int {
	if lv, ok := ls.Get(n).(LNumber); ok {
		return int(lv)
	}
	if lv, ok := ls.Get(n).(LString); ok {
		if num, err := parseNumber(string(lv)); err == nil {
			return int(num)
		}
	}
	return 0
}

func (ls *LState) ToInt64(n int) int64 {
	if lv, ok := ls.Get(n).(LNumber); ok {
		return int64(lv)
	}
	if lv, ok := ls.Get(n).(LString); ok {
		if num, err := parseNumber(string(lv)); err == nil {
			return int64(num)
		}
	}
	return 0
}

func (ls *LState) ToNumber(n int) LNumber {
	return LVAsNumber(ls.Get(n))
}

func (ls *LState) ToString(n int) string {
	return LVAsString(ls.Get(n))
}

func (ls *LState) ToTable(n int) *LTable {
	if lv, ok := ls.Get(n).(*LTable); ok {
		return lv
	}
	return nil
}

func (ls *LState) ToFunction(n int) *LFunction {
	if lv, ok := ls.Get(n).(*LFunction); ok {
		return lv
	}
	return nil
}

func (ls *LState) ToUserData(n int) *LUserData {
	if lv, ok := ls.Get(n).(*LUserData); ok {
		return lv
	}
	return nil
}

func (ls *LState) ToThread(n int) *LState {
	if lv, ok := ls.Get(n).(*LState); ok {
		return lv
	}
	return nil
}

/* }}} */

/* error & debug operations {{{ */

func (ls *LState) RaiseError(format string, args ...interface{}) {
	ls.raiseError(1, format, args...)
}

func (ls *LState) Error(lv LValue, level int) {
	if str, ok := lv.(LString); ok {
		ls.raiseError(level, string(str))
	} else {
		ls.closeAllUpvalues()
		ls.Push(lv)
		ls.Panic(ls)
	}
}

func (ls *LState) GetInfo(what string, dbg *Debug, fn LValue) (LValue, error) {
	if !strings.HasPrefix(what, ">") {
		fn = dbg.frame.Fn
	} else {
		what = what[1:]
	}
	f, ok := fn.(*LFunction)
	if !ok {
		return LNil, newApiError(ApiErrorRun, "can not get debug info(an object in not a function)", LNil)
	}

	retfn := false
	for _, c := range what {
		switch c {
		case 'f':
			retfn = true
		case 'S':
			if dbg.frame != nil && dbg.frame.Parent == nil {
				dbg.What = "main"
			} else if f.IsG {
				dbg.What = "G"
			} else if dbg.frame != nil && dbg.frame.TailCall > 0 {
				dbg.What = "tail"
			} else {
				dbg.What = "Lua"
			}
			if !f.IsG {
				dbg.Source = f.Proto.SourceName
				dbg.LineDefined = f.Proto.LineDefined
				dbg.LastLineDefined = f.Proto.LastLineDefined
			}
		case 'l':
			if !f.IsG && dbg.frame != nil {
				if dbg.frame.Pc > 0 {
					dbg.CurrentLine = f.Proto.DbgSourcePositions[dbg.frame.Pc-1]
				}
			}
		case 'u':
			dbg.NUpvalues = len(f.Upvalues)
		case 'n':
			if dbg.frame != nil {
				dbg.Name = ls.frameFuncName(dbg.frame)
			}
		default:
			return LNil, newApiError(ApiErrorRun, "invalid what: "+string(c), LNil)
		}
	}

	if retfn {
		return f, nil
	}
	return LNil, nil

}

func (ls *LState) GetStack(level int) (*Debug, bool) {
	frame := ls.currentFrame
	for ; level > 0 && frame != nil; frame = frame.Parent {
		level--
		if !frame.Fn.IsG {
			level -= frame.TailCall
		}
	}

	if level == 0 && frame != nil {
		return &Debug{frame: frame}, true
	} else if level < 0 && ls.stack.Sp() > 0 {
		return &Debug{frame: ls.stack.At(0)}, true
	}
	return &Debug{}, false
}

func (ls *LState) GetLocal(dbg *Debug, no int) (string, LValue) {
	frame := dbg.frame
	if name := ls.findLocal(frame, no); len(name) > 0 {
		return name, ls.reg.Get(frame.LocalBase + no - 1)
	}
	return "", LNil
}

func (ls *LState) SetLocal(dbg *Debug, no int, lv LValue) string {
	frame := dbg.frame
	if name := ls.findLocal(frame, no); len(name) > 0 {
		ls.reg.Set(frame.LocalBase+no-1, lv)
		return name
	}
	return ""
}

func (ls *LState) GetUpvalue(fn *LFunction, no int) (string, LValue) {
	if fn.IsG {
		return "", LNil
	}

	no--
	if no >= 0 && no < len(fn.Upvalues) {
		return fn.Proto.DbgUpvalues[no], fn.Upvalues[no].Value()
	}
	return "", LNil
}

func (ls *LState) SetUpvalue(fn *LFunction, no int, lv LValue) string {
	if fn.IsG {
		return ""
	}

	no--
	if no >= 0 && no < len(fn.Upvalues) {
		fn.Upvalues[no].SetValue(lv)
		return fn.Proto.DbgUpvalues[no]
	}
	return ""
}

/* }}} */

/* env operations {{{ */

func (ls *LState) GetFEnv(obj LValue) LValue {
	switch lv := obj.(type) {
	case *LFunction:
		return lv.Env
	case *LUserData:
		return lv.Env
	case *LState:
		return lv.Env
	}
	return LNil
}

func (ls *LState) SetFEnv(obj LValue, env LValue) {
	tb, ok := env.(*LTable)
	if !ok {
		ls.RaiseError("cannot use %v as an environment", env.Type().String())
	}

	switch lv := obj.(type) {
	case *LFunction:
		lv.Env = tb
	case *LUserData:
		lv.Env = tb
	case *LState:
		lv.Env = tb
	}
	/* do nothing */
}

/* }}} */

/* table operations {{{ */

func (ls *LState) RawGet(tb *LTable, key LValue) LValue {
	return tb.RawGet(key)
}

func (ls *LState) RawGetInt(tb *LTable, key int) LValue {
	return tb.RawGetInt(key)
}

func (ls *LState) GetField(obj LValue, skey string) LValue {
	return ls.getField(obj, LString(skey))
}

func (ls *LState) RawSet(tb *LTable, key LValue, value LValue) {
	if n, ok := key.(LNumber); ok && math.IsNaN(float64(n)) {
		ls.RaiseError("table index is NaN")
	} else if key == LNil {
		ls.RaiseError("table index is nil")
	}
	tb.RawSet(key, value)
}

func (ls *LState) RawSetInt(tb *LTable, key int, value LValue) {
	tb.RawSetInt(key, value)
}

func (ls *LState) SetField(obj LValue, key string, value LValue) {
	ls.setField(obj, LString(key), value)
}

func (ls *LState) ForEach(tb *LTable, cb func(LValue, LValue)) {
	tb.ForEach(cb)
}

func (ls *LState) GetGlobal(name string) LValue {
	return ls.GetField(ls.Get(GlobalsIndex), name)
}

func (ls *LState) SetGlobal(name string, value LValue) {
	ls.SetField(ls.Get(GlobalsIndex), name, value)
}

func (ls *LState) Next(tb *LTable, key LValue) (LValue, LValue) {
	return tb.Next(key)
}

/* }}} */

/* unary operations {{{ */

func (ls *LState) ObjLen(v1 LValue) int {
	if v1.Type() == LTString {
		return len(string(v1.(LString)))
	}
	op := ls.metaOp1(v1, "__len")
	if op.Type() == LTFunction {
		ls.Push(op)
		ls.Push(v1)
		ls.Call(1, 1)
		ret := ls.reg.Pop()
		if ret.Type() == LTNumber {
			return int(ret.(LNumber))
		}
	} else if v1.Type() == LTTable {
		return v1.(*LTable).Len()
	}
	return 0
}

/* }}} */

/* binary operations {{{ */

func (ls *LState) Concat(values ...LValue) string {
	top := ls.reg.Top()
	for _, value := range values {
		ls.reg.Push(value)
	}
	ret := stringConcat(ls, len(values), ls.reg.Top()-1)
	ls.reg.SetTop(top)
	return LVAsString(ret)
}

func (ls *LState) LessThan(lhs, rhs LValue) bool {
	return lessThan(ls, lhs, rhs)
}

func (ls *LState) Equal(lhs, rhs LValue) bool {
	return equals(ls, lhs, rhs, false)
}

func (ls *LState) RawEqual(lhs, rhs LValue) bool {
	return equals(ls, lhs, rhs, true)
}

/* }}} */

/* register operations {{{ */

func (ls *LState) Register(name string, fn LGFunction) {
	ls.SetGlobal(name, ls.NewFunction(fn))
}

/* }}} */

/* load and function call operations {{{ */

func (ls *LState) Load(reader io.Reader, name string) (*LFunction, error) {
	chunk, err := parse.Parse(reader, name)
	if err != nil {
		return nil, newApiError(ApiErrorSyntax, err.Error(), LNil)
	}
	proto, err := Compile(chunk, name)
	if err != nil {
		return nil, newApiError(ApiErrorSyntax, err.Error(), LNil)
	}
	return newLFunctionL(proto, ls.currentEnv(), 0), nil
}

func (ls *LState) Call(nargs, nret int) {
	ls.callR(nargs, nret, -1)
}

func (ls *LState) PCall(nargs, nret int, errfunc *LFunction) (err error) {
	err = nil
	sp := ls.stack.Sp()
	base := ls.reg.Top() - nargs - 1
	oldpanic := ls.Panic
	ls.Panic = func(L *LState) {
		panic(newApiError(ApiErrorRun, "", L.Get(-1)))
	}
	defer func() {
		ls.Panic = oldpanic
		rcv := recover()
		if rcv != nil {
			if _, ok := rcv.(*ApiError); !ok {
				buf := make([]byte, 4096)
				runtime.Stack(buf, false)
				err = newApiError(ApiErrorPanic, fmt.Sprintf("%v\n%v", rcv, strings.Trim(string(buf), "\000")), LNil)
			} else {
				err = rcv.(*ApiError)
			}
			if errfunc != nil {
				ls.Push(errfunc)
				ls.Push(err.(*ApiError).Object)
				ls.Panic = func(L *LState) {
					panic(newApiError(ApiErrorError, "", L.Get(-1)))
				}
				defer func() {
					ls.Panic = oldpanic
					rcv := recover()
					if rcv != nil {
						if _, ok := rcv.(*ApiError); !ok {
							buf := make([]byte, 4096)
							runtime.Stack(buf, false)
							err = newApiError(ApiErrorPanic, fmt.Sprintf("%v\n%v", rcv, strings.Trim(string(buf), "\000")), LNil)
						} else {
							err = rcv.(*ApiError)
						}
					}
				}()
				ls.Call(1, 1)
			}
			ls.reg.SetTop(base)
		}
		ls.stack.SetSp(sp)
	}()

	ls.Call(nargs, nret)

	return
}

func (ls *LState) GPCall(fn LGFunction, data LValue) error {
	ls.Push(newLFunctionG(fn, ls.currentEnv(), 0))
	ls.Push(data)
	return ls.PCall(1, MultRet, nil)
}

func (ls *LState) CallByParam(cp P, args ...LValue) error {
	ls.Push(cp.Fn)
	for _, arg := range args {
		ls.Push(arg)
	}

	if cp.Protect {
		return ls.PCall(len(args), cp.NRet, cp.Handler)
	}
	ls.Call(len(args), cp.NRet)
	return nil
}

/* }}} */

/* metatable operations {{{ */

func (ls *LState) GetMetatable(obj LValue) LValue {
	return ls.metatable(obj, false)
}

func (ls *LState) SetMetatable(obj LValue, mt LValue) {
	switch mt.(type) {
	case *LNilType, *LTable:
	default:
		ls.RaiseError("metatable must be a table or nil, but got %v", mt.Type().String())
	}

	switch v := obj.(type) {
	case *LTable:
		v.Metatable = mt
	case *LUserData:
		v.Metatable = mt
	default:
		ls.G.builtinMts[int(obj.Type())] = mt
	}
}

/* }}} */

/* coroutine operations {{{ */

func (ls *LState) Status(th *LState) string {
	status := "suspended"
	if th.Dead {
		status = "dead"
	} else if ls.G.CurrentThread == th {
		status = "running"
	} else if ls.Parent == th {
		status = "normal"
	}
	return status
}

func (ls *LState) Resume(th *LState, fn *LFunction, args ...LValue) (ResumeState, error, []LValue) {
	isstarted := th.isStarted()
	if !isstarted {
		base := 0
		err := th.stack.Push(callFrame{
			Fn:         fn,
			Pc:         0,
			Base:       base,
			LocalBase:  base + 1,
			ReturnBase: base,
			NArgs:      0,
			NRet:       MultRet,
			Parent:     nil,
			TailCall:   0,
		})
		if err != nil {
			ls.RaiseError(err.Error())
		}
	}

	if ls.G.CurrentThread == th {
		return ResumeError, newApiError(ApiErrorRun, "can not resume a running thread", LNil), nil
	}
	if th.Dead {
		return ResumeError, newApiError(ApiErrorRun, "can not resume a dead thread", LNil), nil
	}
	th.Parent = ls
	ls.G.CurrentThread = th
	if !isstarted {
		cf := th.stack.Last()
		th.currentFrame = cf
		th.SetTop(0)
		for _, arg := range args {
			th.Push(arg)
		}
		cf.NArgs = len(args)
		th.initCallFrame(cf)
		th.Panic = func(L *LState) {
			panic(L.Get(-1))
		}
	} else {
		for _, arg := range args {
			th.Push(arg)
		}
	}
	top := ls.GetTop()
	threadRun(th)
	haserror := LVIsFalse(ls.Get(top + 1))
	ret := make([]LValue, 0, ls.GetTop())
	for idx := top + 2; idx <= ls.GetTop(); idx++ {
		ret = append(ret, ls.Get(idx))
	}
	ls.SetTop(top)

	if haserror {
		return ResumeError, newApiError(ApiErrorRun, fmt.Sprint(ret[0]), LNil), nil
	} else if th.stack.IsEmpty() {
		return ResumeOK, nil, ret
	}
	return ResumeYield, nil, ret
}

func (ls *LState) Yield(values ...LValue) int {
	ls.SetTop(0)
	for _, lv := range values {
		ls.Push(lv)
	}
	return -1
}

func (ls *LState) XMoveTo(other *LState, n int) {
	if ls == other {
		return
	}
	top := ls.GetTop()
	n = intMin(n, top)
	for i := n; i > 0; i-- {
		other.Push(ls.Get(top - i + 1))
	}
	ls.SetTop(top - n)
}

/* }}} */

/* GopherLua original APIs {{{ */

// Set maximum memory size. This function can only be called from the main thread.
func (ls *LState) SetMx(mx int) {
	if ls.Parent != nil {
		ls.RaiseError("sub threads are not allowed to set a memory limit")
	}
	go func() {
		limit := uint64(mx * 1024 * 1024) //MB
		var s runtime.MemStats
		for ls.stop == 0 {
			runtime.ReadMemStats(&s)
			if s.Alloc >= limit {
				fmt.Println("out of memory")
				os.Exit(3)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

// Converts the Lua value at the given acceptable index to the chan LValue.
func (ls *LState) ToChannel(n int) chan LValue {
	if lv, ok := ls.Get(n).(LChannel); ok {
		return (chan LValue)(lv)
	}
	return nil
}

/* }}} */

/* }}} */

//

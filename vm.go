package lua

import (
	"fmt"
	"math"
	"strings"
)

func copyReturnValues(L *LState, reg, start, n, b int) {
	if b == 1 {
		L.reg.FillNil(reg, n)
	} else {
		L.reg.CopyRange(reg, start, -1, n)
	}
}

func switchToParentThread(L *LState, nargs int, haserror bool, kill bool) {
	parent := L.Parent
	if parent == nil {
		L.RaiseError("can not yield from outside of a coroutine")
	}
	L.G.CurrentThread = parent
	L.Parent = nil
	if !L.wrapped {
		if haserror {
			parent.Push(LFalse)
		} else {
			parent.Push(LTrue)
		}
	}
	L.XMoveTo(parent, nargs)
	L.stack.Pop()
	offset := L.currentFrame.LocalBase - L.currentFrame.ReturnBase
	L.currentFrame = L.stack.Last()
	L.reg.SetTop(L.reg.Top() - offset) // remove 'yield' function(including tailcalled functions)
	if kill {
		L.kill()
	}
}

func callGFunction(L *LState, tailcall bool) bool {
	frame := L.currentFrame
	gfnret := frame.Fn.GFunction(L)
	if tailcall {
		L.stack.Remove(L.stack.Sp() - 2) // remove caller lua function frame
		L.currentFrame = L.stack.Last()
	}

	if gfnret < 0 {
		switchToParentThread(L, L.GetTop(), false, false)
		return true
	}

	wantret := frame.NRet
	if wantret == MultRet {
		wantret = gfnret
	}

	if tailcall && L.Parent != nil && L.stack.Sp() == 1 {
		switchToParentThread(L, wantret, false, true)
		return true
	}

	L.reg.CopyRange(frame.ReturnBase, L.reg.Top()-gfnret, -1, wantret)
	L.stack.Pop()
	L.currentFrame = L.stack.Last()
	return false
}

func threadRun(L *LState) {
	if L.stack.IsEmpty() {
		return
	}

	defer func() {
		if rcv := recover(); rcv != nil {
			var lv LValue
			if v, ok := rcv.(LValue); ok {
				lv = v
			}
			if lv == nil {
				panic(rcv)
			}
			if parent := L.Parent; parent != nil {
				if L.wrapped {
					L.Push(lv)
					parent.Panic(L)
				} else {
					L.SetTop(0)
					L.Push(lv)
					switchToParentThread(L, 1, true, true)
				}
			} else {
				panic(rcv)
			}
		}
	}()
	mainLoop(L, nil)
}

func mainLoop(L *LState, startframe *callFrame) {
	var inst uint32
	var lbase, A, RA, B, C, Bx, Sbx int
	var cf *callFrame

	if L.stack.IsEmpty() {
		return
	}

	L.currentFrame = L.stack.Last()
	if L.currentFrame != nil && L.currentFrame.Fn.IsG {
		callGFunction(L, false)
		return
	}

	reg := L.reg
	for {
		cf = L.currentFrame
		inst = cf.Fn.Proto.Code[cf.Pc]
		cf.Pc++
		lbase = cf.LocalBase
		opcode := int(inst >> 26) //GETOPCODE
		A = int(inst>>18) & 0xff  //GETA
		RA = lbase + A

		switch opcode {
		case OP_MOVE:
			B = int(inst & 0x1ff) //GETB
			reg.Set(RA, reg.Get(lbase+B))
		case OP_LOADK:
			Bx = int(inst & 0x3ffff) //GETBX
			reg.Set(RA, cf.Fn.Proto.Constants[Bx])
		case OP_LOADBOOL:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			if B != 0 {
				reg.Set(RA, LTrue)
			} else {
				reg.Set(RA, LFalse)
			}
			if C != 0 {
				cf.Pc++
			}
		case OP_LOADNIL:
			B = int(inst & 0x1ff) //GETB
			for i := RA; i <= lbase+B; i++ {
				reg.Set(i, LNil)
			}
		case OP_GETUPVAL:
			B = int(inst & 0x1ff) //GETB
			reg.Set(RA, cf.Fn.Upvalues[B].Value())
		case OP_GETGLOBAL:
			Bx = int(inst & 0x3ffff) //GETBX
			reg.Set(RA, L.getField(cf.Fn.Env, cf.Fn.Proto.Constants[Bx]))
		case OP_GETTABLE:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			reg.Set(RA, L.getField(reg.Get(lbase+B), L.rkValue(C)))
		case OP_SETGLOBAL:
			Bx = int(inst & 0x3ffff) //GETBX
			L.setField(cf.Fn.Env, cf.Fn.Proto.Constants[Bx], reg.Get(RA))
		case OP_SETUPVAL:
			B = int(inst & 0x1ff) //GETB
			cf.Fn.Upvalues[B].SetValue(reg.Get(RA))
		case OP_SETTABLE:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			L.setField(reg.Get(RA), L.rkValue(B), L.rkValue(C))
		case OP_NEWTABLE:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			reg.Set(RA, newLTable(B, C))
		case OP_SELF:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			selfobj := reg.Get(lbase + B)
			reg.Set(RA, L.getField(selfobj, L.rkValue(C)))
			reg.Set(RA+1, selfobj)
		case OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			lhs := L.rkValue(B)
			rhs := L.rkValue(C)
			var ret LValue
			v1, ok1 := lhs.(LNumber)
			v2, ok2 := rhs.(LNumber)
			if ok1 && ok2 {
				ret = numberArith(L, opcode, v1, v2)
			} else {
				ret = objectArith(L, opcode, lhs, rhs)
			}
			reg.Set(RA, ret)
		case OP_UNM:
			B = int(inst & 0x1ff) //GETB
			unaryv := L.rkValue(B)
			if nm, ok := unaryv.(LNumber); ok {
				reg.Set(RA, LNumber(-nm))
			} else {
				op := L.metaOp1(unaryv, "__unm")
				if op.Type() == LTFunction {
					reg.Push(op)
					reg.Push(unaryv)
					L.Call(1, 1)
					reg.Set(RA, reg.Pop())
				} else if str, ok1 := unaryv.(LString); ok1 {
					if num, err := parseNumber(string(str)); err == nil {
						reg.Set(RA, -num)
					} else {
						L.RaiseError("__unm undefined")
					}
				} else {
					L.RaiseError("__unm undefined")
				}
			}
		case OP_NOT:
			B = int(inst & 0x1ff) //GETB
			if LVIsFalse(reg.Get(lbase + B)) {
				reg.Set(RA, LTrue)
			} else {
				reg.Set(RA, LFalse)
			}
		case OP_LEN:
			B = int(inst & 0x1ff) //GETB
			switch lv := L.rkValue(B).(type) {
			case LString:
				reg.Set(RA, LNumber(len(lv)))
			case *LTable:
				reg.Set(RA, LNumber(lv.Len()))
			default:
				op := L.metaOp1(lv, "__len")
				if op.Type() == LTFunction {
					reg.Push(op)
					reg.Push(lv)
					L.Call(1, 1)
					reg.Set(RA, reg.Pop())
				} else {
					L.RaiseError("__len undefined")
				}
			}
		case OP_CONCAT:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			RC := lbase + C
			RB := lbase + B
			reg.Set(RA, stringConcat(L, RC-RB+1, RC))
		case OP_JMP:
			Sbx = int(inst&0x3ffff) - opMaxArgSbx //GETSBX
			cf.Pc += Sbx
		case OP_EQ:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			ret := equals(L, L.rkValue(B), L.rkValue(C), false)
			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
		case OP_LT:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			ret := lessThan(L, L.rkValue(B), L.rkValue(C))
			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
		case OP_LE:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			lhs := L.rkValue(B)
			rhs := L.rkValue(C)
			ret := false

			if v1, ok1 := lhs.(LNumber); ok1 {
				if v2, ok2 := rhs.(LNumber); ok2 {
					ret = v1 <= v2
				} else {
					L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
				}
			} else {
				if lhs.Type() != rhs.Type() {
					L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
				}
				switch lhs.Type() {
				case LTString:
					ret = strCmp(string(lhs.(LString)), string(rhs.(LString))) <= 0
				default:
					switch objectRational(L, lhs, rhs, "__le") {
					case 1:
						ret = true
					case 0:
						ret = false
					default:
						ret = !objectRationalWithError(L, rhs, lhs, "__lt")
					}
				}
			}

			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
		case OP_TEST:
			C = int(inst>>9) & 0x1ff //GETC
			if LVAsBool(reg.Get(RA)) == (C == 0) {
				cf.Pc++
			}
		case OP_TESTSET:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			if value := reg.Get(lbase + B); LVAsBool(value) != (C == 0) {
				reg.Set(RA, value)
			} else {
				cf.Pc++
			}
		case OP_CALL:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			nargs := B - 1
			if B == 0 {
				nargs = reg.Top() - (RA + 1)
			}
			fn := reg.Get(RA)
			nret := C - 1
			callable, meta := L.metaCall(fn)
			L.pushCallFrame(callFrame{
				Fn:         callable,
				Pc:         0,
				Base:       RA,
				LocalBase:  RA + 1,
				ReturnBase: RA,
				NArgs:      nargs,
				NRet:       nret,
				Parent:     cf,
				TailCall:   0,
			}, fn, meta)
			if callable.IsG && callGFunction(L, false) {
				return
			}
			if L.currentFrame == nil || L.currentFrame.Fn.IsG {
				return
			}
		case OP_TAILCALL:
			B = int(inst & 0x1ff) //GETB
			nargs := B - 1
			if B == 0 {
				nargs = reg.Top() - (RA + 1)
			}
			fn := reg.Get(RA)
			callable, meta := L.metaCall(fn)
			L.closeUpvalues(lbase)
			if callable.IsG {
				luaframe := cf
				L.pushCallFrame(callFrame{
					Fn:         callable,
					Pc:         0,
					Base:       RA,
					LocalBase:  RA + 1,
					ReturnBase: cf.ReturnBase,
					NArgs:      nargs,
					NRet:       cf.NRet,
					Parent:     cf,
					TailCall:   0,
				}, fn, meta)
				if callGFunction(L, true) {
					return
				}
				if L.currentFrame == nil || L.currentFrame.Fn.IsG || luaframe == startframe {
					return
				}
			} else {
				base := cf.Base
				cf.Fn = callable
				cf.Pc = 0
				cf.Base = RA
				cf.LocalBase = RA + 1
				cf.ReturnBase = cf.ReturnBase
				cf.NArgs = nargs
				cf.NRet = cf.NRet
				cf.TailCall++
				lbase := cf.LocalBase
				if meta {
					cf.NArgs++
					L.reg.Insert(fn, cf.LocalBase)
				}
				L.initCallFrame(cf)
				L.reg.CopyRange(base, RA, -1, reg.Top()-RA-1)
				cf.Base = base
				cf.LocalBase = base + (cf.LocalBase - lbase + 1)
			}
		case OP_RETURN:
			B = int(inst & 0x1ff) //GETB
			L.closeUpvalues(lbase)
			nret := B - 1
			if B == 0 {
				nret = reg.Top() - RA
			}
			n := cf.NRet
			if cf.NRet == MultRet {
				n = nret
			}

			if L.Parent != nil && (startframe == cf || L.stack.Sp() == 1) {
				copyReturnValues(L, reg.Top(), RA, n, B)
				switchToParentThread(L, n, false, true)
				return
			}
			islast := startframe == L.stack.Pop() || L.stack.IsEmpty()
			copyReturnValues(L, cf.ReturnBase, RA, n, B)
			L.currentFrame = L.stack.Last()
			if islast || L.currentFrame == nil || L.currentFrame.Fn.IsG {
				return
			}
		case OP_FORPREP:
			Sbx = int(inst&0x3ffff) - opMaxArgSbx //GETSBX
			if init, ok1 := reg.Get(RA).(LNumber); ok1 {
				if step, ok2 := reg.Get(RA + 2).(LNumber); ok2 {
					reg.Set(RA, LNumber(init-step))
				} else {
					L.RaiseError("for statement step must be a number")
				}
			} else {
				L.RaiseError("for statement init must be a number")
			}
			cf.Pc += Sbx
		case OP_FORLOOP:
			if init, ok1 := reg.Get(RA).(LNumber); ok1 {
				if limit, ok2 := reg.Get(RA + 1).(LNumber); ok2 {
					if step, ok3 := reg.Get(RA + 2).(LNumber); ok3 {
						init += step
						reg.Set(RA, LNumber(init))
						if (step > 0 && init <= limit) || (step <= 0 && init >= limit) {
							Sbx = int(inst&0x3ffff) - opMaxArgSbx //GETSBX
							cf.Pc += Sbx
							reg.Set(RA+3, LNumber(init))
						} else {
							reg.SetTop(RA + 1)
						}
					} else {
						L.RaiseError("for statement step must be a number")
					}
				} else {
					L.RaiseError("for statement limit must be a number")
				}
			} else {
				L.RaiseError("for statement init must be a number")
			}
		case OP_TFORLOOP:
			C = int(inst>>9) & 0x1ff //GETC
			nret := C
			reg.SetTop(RA + 3)
			L.callR(2, nret, RA+3)
			reg.SetTop(RA + 2 + C + 1)
			if value := reg.Get(RA + 3); value != LNil {
				reg.Set(RA+2, value)
			} else {
				cf.Pc++
			}
		case OP_SETLIST:
			B = int(inst & 0x1ff)    //GETB
			C = int(inst>>9) & 0x1ff //GETC
			if C == 0 {
				C = int(cf.Fn.Proto.Code[cf.Pc])
				cf.Pc++
			}
			offset := (C - 1) * FieldsPerFlush
			table := reg.Get(RA).(*LTable)
			nelem := B
			if B == 0 {
				nelem = reg.Top() - RA - 1
			}
			for i := 1; i <= nelem; i++ {
				table.RawSetInt(offset+i, reg.Get(RA+i))
			}
		case OP_CLOSE:
			L.closeUpvalues(RA)
		case OP_CLOSURE:
			Bx = int(inst & 0x3ffff) //GETBX
			proto := cf.Fn.Proto.FunctionPrototypes[Bx]
			closure := newLFunctionL(proto, cf.Fn.Env, int(proto.NumUpvalues))
			reg.Set(RA, closure)
			for i := 0; i < int(proto.NumUpvalues); i++ {
				inst = cf.Fn.Proto.Code[cf.Pc]
				cf.Pc++
				B = opGetArgB(inst)
				switch opGetOpCode(inst) {
				case OP_MOVE:
					closure.Upvalues[i] = L.findUpvalue(lbase + B)
				case OP_GETUPVAL:
					closure.Upvalues[i] = cf.Fn.Upvalues[B]
				}
			}
		case OP_VARARG:
			B = int(inst & 0x1ff) //GETB
			nparams := int(cf.Fn.Proto.NumParameters)
			nvarargs := cf.NArgs - nparams
			if nvarargs < 0 {
				nvarargs = 0
			}
			nwant := B - 1
			if B == 0 {
				nwant = nvarargs
			}
			reg.CopyRange(RA, cf.Base+nparams+1, cf.LocalBase, nwant)
		case OP_NOP:
			/* nothing to do */
		default:
			panic(fmt.Sprintf("not implemented %v", opToString(inst)))
		}
	}
}

func luaModulo(lhs, rhs LNumber) LNumber {
	flhs := float64(lhs)
	frhs := float64(rhs)
	v := math.Mod(flhs, frhs)
	if flhs < 0 || frhs < 0 && !(flhs < 0 && frhs < 0) {
		v += frhs
	}
	return LNumber(v)
}

func numberArith(L *LState, opcode int, lhs, rhs LNumber) LNumber {
	switch opcode {
	case OP_ADD:
		return lhs + rhs
	case OP_SUB:
		return lhs - rhs
	case OP_MUL:
		return lhs * rhs
	case OP_DIV:
		return lhs / rhs
	case OP_MOD:
		return luaModulo(lhs, rhs)
	case OP_POW:
		flhs := float64(lhs)
		frhs := float64(rhs)
		return LNumber(math.Pow(flhs, frhs))
	}
	panic("should not reach here")
	return LNumber(0)
}

func objectArith(L *LState, opcode int, lhs, rhs LValue) LValue {
	event := ""
	switch opcode {
	case OP_ADD:
		event = "__add"
	case OP_SUB:
		event = "__sub"
	case OP_MUL:
		event = "__mul"
	case OP_DIV:
		event = "__div"
	case OP_MOD:
		event = "__mod"
	case OP_POW:
		event = "__pow"
	}
	op := L.metaOp2(lhs, rhs, event)
	if op.Type() == LTFunction {
		L.reg.Push(op)
		L.reg.Push(lhs)
		L.reg.Push(rhs)
		L.Call(2, 1)
		return L.reg.Pop()
	}
	if str, ok := lhs.(LString); ok {
		if lnum, err := parseNumber(string(str)); err == nil {
			lhs = lnum
		}
	}
	if str, ok := rhs.(LString); ok {
		if rnum, err := parseNumber(string(str)); err == nil {
			rhs = rnum
		}
	}
	if lhs.Type() == LTNumber && rhs.Type() == LTNumber {
		return numberArith(L, opcode, lhs.(LNumber), rhs.(LNumber))
	}
	L.RaiseError(fmt.Sprintf("cannot performs %v operation between %v and %v",
		strings.TrimLeft(event, "_"), lhs.Type().String(), rhs.Type().String()))

	return LNil
}

func stringConcat(L *LState, total, last int) LValue {
	rhs := L.reg.Get(last)
	total--
	for i := last - 1; total > 0; {
		lhs := L.reg.Get(i)
		if !(LVCanConvToString(lhs) && LVCanConvToString(rhs)) {
			op := L.metaOp2(lhs, rhs, "__concat")
			if op.Type() == LTFunction {
				L.reg.Push(op)
				L.reg.Push(lhs)
				L.reg.Push(rhs)
				L.Call(2, 1)
				rhs = L.reg.Pop()
				total--
				i--
			} else {
				L.RaiseError("cannot perform concat operation between %v and %v", lhs.Type().String(), rhs.Type().String())
				return LNil
			}
		} else {
			buf := make([]string, total+1)
			buf[total] = LVAsString(rhs)
			for total > 0 {
				lhs = L.reg.Get(i)
				if !LVCanConvToString(lhs) {
					break
				}
				buf[total-1] = LVAsString(lhs)
				i--
				total--
			}
			rhs = LString(strings.Join(buf, ""))
		}
	}
	return rhs
}

func lessThan(L *LState, lhs, rhs LValue) bool {
	// optimization for numbers
	if v1, ok1 := lhs.(LNumber); ok1 {
		if v2, ok2 := rhs.(LNumber); ok2 {
			return v1 < v2
		}
		L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
	}
	if lhs.Type() != rhs.Type() {
		L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
		return false
	}
	ret := false
	switch lhs.Type() {
	case LTString:
		ret = strCmp(string(lhs.(LString)), string(rhs.(LString))) < 0
	default:
		ret = objectRationalWithError(L, lhs, rhs, "__lt")
	}
	return ret
}

func equals(L *LState, lhs, rhs LValue, raw bool) bool {
	if lhs.Type() != rhs.Type() {
		return false
	}

	ret := false
	switch lhs.Type() {
	case LTNil:
		ret = true
	case LTNumber:
		ret = float64(lhs.(LNumber)) == float64(rhs.(LNumber))
	case LTBool:
		ret = bool(lhs.(LBool)) == bool(rhs.(LBool))
	case LTString:
		ret = string(lhs.(LString)) == string(rhs.(LString))
	case LTUserData, LTTable:
		if lhs == rhs {
			ret = true
		} else if !raw {
			switch objectRational(L, lhs, rhs, "__eq") {
			case 1:
				ret = true
			default:
				ret = false
			}
		}
	default:
		ret = lhs == rhs
	}
	return ret
}

func tostring(L *LState, lv LValue) LValue {
	if fn, ok := L.metaOp1(lv, "__tostring").(*LFunction); ok {
		L.Push(fn)
		L.Push(lv)
		L.Call(1, 1)
		return L.reg.Pop()
	} else {
		return LString(lv.String())
	}
}

func objectRationalWithError(L *LState, lhs, rhs LValue, event string) bool {
	switch objectRational(L, lhs, rhs, event) {
	case 1:
		return true
	case 0:
		return false
	default:
		L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
		return false
	}
	return false
}

func objectRational(L *LState, lhs, rhs LValue, event string) int {
	m1 := L.metaOp1(lhs, event)
	m2 := L.metaOp1(rhs, event)
	if m1.Type() == LTFunction && m1 == m2 {
		L.reg.Push(m1)
		L.reg.Push(lhs)
		L.reg.Push(rhs)
		L.Call(2, 1)
		if LVAsBool(L.reg.Pop()) {
			return 1
		}
		return 0
	}
	return -1
}

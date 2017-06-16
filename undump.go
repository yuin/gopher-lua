package lua

import (
	// "bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type undumpState struct {
	l     *LState
	in    io.Reader
	order binary.ByteOrder
	err   error
	name  string
}

func (ud *undumpState) read(data interface{}) error {
	return binary.Read(ud.in, ud.order, data)
}

func (ud *undumpState) readNumber() (f float64, err error) {
	err = ud.read(&f)
	return
}

func (ud *undumpState) readInt() (i uint32, err error) {
	err = ud.read(&i)
	return
}

func (ud *undumpState) readByte() (b byte, err error) {
	err = ud.read(&b)
	return
}

func (ud *undumpState) readBool() (bool, error) {
	b, err := ud.readByte()
	return b != 0, err
}

func (ud *undumpState) readString() (s string, err error) {
	var size uint64
	err = ud.read(&size)

	if err != nil || size == 0 {
		return
	}
	ba := make([]byte, size)
	if err = ud.read(ba); err == nil {
		s = string(ba[:len(ba)-1])
	}
	return
}

func (ud *undumpState) readCode() (code []uint32, err error) {
	n, err := ud.readInt()
	if err != nil || n == 0 {
		return
	}
	code = make([]uint32, n)
	err = ud.read(code)
	return
}

func (ud *undumpState) readConstants() (constants []LValue, err error) {
	n, err := ud.readInt()
	if err != nil || n == 0 {
		return
	}

	constants = make([]LValue, n)

	for i := 0; i < int(n); i++ {
		if b, berr := ud.readByte(); berr != nil {
			err = berr
			return
		} else {
			switch b {
			case byte(LTNil):
				constants[i] = LNil
			case byte(LTBool):
				if bval, berr := ud.readBool(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = LBool(bval)
				}
			case byte(LTNumber):
				if nval, berr := ud.readNumber(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = LNumber(nval)
				}
			case byte(LTString):
				if sval, berr := ud.readString(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = LString(sval)
				}
			default:
				return
			}
		}
	}

	return
}

func (ud *undumpState) readHeader() error {
	hdr := header

	if err := ud.read(&hdr); err != nil {
		return err
	} else if hdr == header {
		return nil
	} else if string(hdr.Signature[:]) != Signature {
		return errors.New("input is not a precompiled chunk")
	} else if hdr.Version != header.Version || hdr.Format != header.Format {
		return errors.New("version mismatch")
	}

	return errors.New("incompatible input")
}

func (ud *undumpState) readFunction() (p *FunctionProto, errs error) {
	p = newFunctionProto("")

	if p.SourceName, errs = ud.readString(); errs != nil {
		return
	}

	if n, err := ud.readInt(); err != nil {
		errs = err
		return
	} else {
		p.LineDefined = int(n)
	}

	if n, err := ud.readInt(); err != nil {
		errs = err
		return
	} else {
		p.LastLineDefined = int(n)
	}

	if n, err := ud.readByte(); err != nil {
		errs = err
		return
	} else {
		p.NumUpvalues = n
	}

	if n, err := ud.readByte(); err != nil {
		errs = err
		return
	} else {
		p.NumParameters = n
	}

	if n, err := ud.readByte(); err != nil {
		errs = err
		return
	} else {
		p.IsVarArg = n
	}

	if n, err := ud.readByte(); err != nil {
		errs = err
		return
	} else {
		p.NumUsedRegisters = n
	}

	if code, err := ud.readCode(); err != nil {
		errs = err
		return
	} else {
		p.Code = code
	}

	if constants, err := ud.readConstants(); err != nil {
		errs = err
		return
	} else {
		p.Constants = constants
		for _, clv := range p.Constants {
			sv := ""
			if slv, ok := clv.(LString); ok {
				sv = string(slv)
			}
			p.stringConstants = append(p.stringConstants, sv)
		}
	}

	numFunctions, err := ud.readInt()
	if err != nil {
		return
	}

	p.FunctionPrototypes = make([]*FunctionProto, numFunctions)

	for i := 0; i < int(numFunctions); i++ {
		if f, err := ud.readFunction(); err == nil {
			p.FunctionPrototypes[i] = f
		} else {
			fmt.Println("ERRRR", err)
		}
	}

	// Read Debug Info

	if numDebug, err := ud.readInt(); err != nil {
		return
	} else {
		p.DbgSourcePositions = make([]int, numDebug)
		for i := 0; i < int(numDebug); i++ {
			if dp, err := ud.readInt(); err == nil {
				p.DbgSourcePositions[i] = int(dp)
			}
		}
	}

	if numDebugLocals, err := ud.readInt(); err != nil {
		return
	} else {
		p.DbgLocals = make([]*DbgLocalInfo, numDebugLocals)
		for i := 0; i < int(numDebugLocals); i++ {
			name, _ := ud.readString()
			startpc, _ := ud.readInt()
			endpc, _ := ud.readInt()

			p.DbgLocals[i] = &DbgLocalInfo{
				Name:    name,
				StartPc: int(startpc),
				EndPc:   int(endpc),
			}
		}
	}

	if numDbgUpvals, err := ud.readInt(); err != nil {
		return
	} else {
		p.DbgUpvalues = make([]string, numDbgUpvals)
		for i := 0; i < int(numDbgUpvals); i++ {
			if uval, err := ud.readString(); err != nil {
				return
			} else {
				p.DbgUpvalues[i] = uval
			}
		}

	}

	/* GopherLua specific */
	if numDebugCalls, err := ud.readInt(); err != nil {
		return
	} else {
		p.DbgCalls = make([]DbgCall, numDebugCalls)
		for i := 0; i < int(numDebugCalls); i++ {
			name, _ := ud.readString()
			pc, _ := ud.readInt()

			p.DbgCalls[i] = DbgCall{
				Name: name,
				Pc:   int(pc),
			}
		}
	}

	return
}

func (l *LState) Undump(in io.Reader, name string) (fp *FunctionProto, err error) {
	undumpState := undumpState{l: l, in: in, order: binary.LittleEndian}

	if err = undumpState.readHeader(); err != nil {
		return nil, err
	}
	if fp, err = undumpState.readFunction(); err != nil {
		return nil, err
	}

	return fp, nil
}

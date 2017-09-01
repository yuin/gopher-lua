package goldump

import (
	"encoding/binary"
	"errors"
	"github.com/yuin/gopher-lua"
	"io"
)

type undumpState struct {
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

func (ud *undumpState) readConstants() (constants []lua.LValue, err error) {
	n, err := ud.readInt()
	if err != nil || n == 0 {
		return
	}

	constants = make([]lua.LValue, n)

	for i := 0; i < int(n); i++ {
		if b, berr := ud.readByte(); berr != nil {
			err = berr
			return
		} else {
			switch b {
			case byte(lua.LTNil):
				constants[i] = lua.LNil
			case byte(lua.LTBool):
				if bval, berr := ud.readBool(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = lua.LBool(bval)
				}
			case byte(lua.LTNumber):
				if nval, berr := ud.readNumber(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = lua.LNumber(nval)
				}
			case byte(lua.LTString):
				if sval, berr := ud.readString(); berr != nil {
					err = berr
					return
				} else {
					constants[i] = lua.LString(sval)
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

func (ud *undumpState) readFunction(p *lua.FunctionProto) (errs error) {
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
			if slv, ok := clv.(lua.LString); ok {
				sv = string(slv)
			}
			p.StringConstants = append(p.StringConstants, sv)
		}
	}

	numFunctions, err := ud.readInt()
	if err != nil {
		return
	}

	p.FunctionPrototypes = make([]*lua.FunctionProto, numFunctions)

	for i := 0; i < int(numFunctions); i++ {
		f := lua.NewFunctionProto("")
		if err := ud.readFunction(f); err == nil {
			p.FunctionPrototypes[i] = f
		} else {
			errs = err
			return
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
			} else {
				errs = err
				return
			}
		}
	}

	if numDebugLocals, err := ud.readInt(); err != nil {
		return
	} else {
		p.DbgLocals = make([]*lua.DbgLocalInfo, numDebugLocals)
		for i := 0; i < int(numDebugLocals); i++ {
			name, _ := ud.readString()
			startpc, _ := ud.readInt()
			endpc, _ := ud.readInt()

			p.DbgLocals[i] = &lua.DbgLocalInfo{
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
		p.DbgCalls = make([]lua.DbgCall, numDebugCalls)
		for i := 0; i < int(numDebugCalls); i++ {
			name, _ := ud.readString()
			pc, _ := ud.readInt()

			p.DbgCalls[i] = lua.DbgCall{
				Name: name,
				Pc:   int(pc),
			}
		}
	}

	return
}

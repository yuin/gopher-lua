package goldump

import (
	"encoding/binary"
	"errors"
	"github.com/yuin/gopher-lua"
	"io"
)

type dumpState struct {
	out   io.Writer
	order binary.ByteOrder
	strip bool
	err   error
}

func (d *dumpState) write(data interface{}) {
	if d.err == nil {
		d.err = binary.Write(d.out, d.order, data)
	}
}

func (d *dumpState) writeInt(i uint32) {
	d.write(uint32(i))
}

func (d *dumpState) writeCode(p *lua.FunctionProto) {
	d.writeInt(uint32(len(p.Code)))
	for _, code := range p.Code {
		d.writeInt(code)
	}
}

func (d *dumpState) writeByte(b byte) {
	d.write(b)
}

func (d *dumpState) writeBool(b bool) {
	if b {
		d.writeByte(1)
	} else {
		d.writeByte(0)
	}
}

func (d *dumpState) writeNumber(f float64) {
	d.write(f)
}

func (d *dumpState) writeConstants(p *lua.FunctionProto) {
	d.writeInt(uint32(len(p.Constants)))

	for _, constant := range p.Constants {
		cst := byte(constant.Type())
		d.writeByte(cst)

		switch constant.Type() {
		case lua.LTNil:
		case lua.LTBool:
			d.writeBool(lua.LVAsBool(constant))
		case lua.LTNumber:
			d.writeNumber(float64(lua.LVAsNumber(constant)))
		case lua.LTString:
			d.writeString(lua.LVAsString(constant))
		default:
			d.err = errors.New("invalid constat in encoder")
		}
	}

	d.writeInt(uint32(len(p.FunctionPrototypes)))

	for _, fproto := range p.FunctionPrototypes {
		d.dumpFunction(fproto)
	}
}

func (d *dumpState) writeString(s string) {
	ba := []byte(s)
	size := len(s)
	if size > 0 {
		size++ // Accounts for 0 byte at the end
	}

	switch header.PointerSize {
	case 8:
		d.write(uint64(size))
		break
	case 4:
		d.write(uint32(size))
		break
	default:
		d.err = errors.New("unsupported pointer size")
	}

	if size > 0 {
		d.write(ba)
		d.writeByte(0) // Write the NULL byte at the end
	}
}

func (d *dumpState) writeDebug(p *lua.FunctionProto) {
	var length uint32
	if length = uint32(len(p.DbgSourcePositions)); d.strip {
		length = 0
	}
	d.writeInt(length)

	if d.strip == false {
		for _, sp := range p.DbgSourcePositions {
			d.writeInt(uint32(sp))
		}
	}

	if length = uint32(len(p.DbgLocals)); d.strip {
		length = 0
	}
	d.writeInt(length)

	if d.strip == false {
		for _, lvar := range p.DbgLocals {
			d.writeString(lvar.Name)
			d.writeInt(uint32(lvar.StartPc))
			d.writeInt(uint32(lvar.EndPc))
		}
	}

	if length = uint32(len(p.DbgUpvalues)); d.strip {
		length = 0
	}
	d.writeInt(length)

	if d.strip == false {
		for _, upval := range p.DbgUpvalues {
			d.writeString(upval)
		}
	}

	if length = uint32(len(p.DbgCalls)); d.strip {
		length = 0
	}
	d.writeInt(length)

	if d.strip == false {
		for _, dc := range p.DbgCalls {
			d.writeString(dc.Name)
			d.writeInt(uint32(dc.Pc))
		}
	}
}

func (d *dumpState) dumpFunction(p *lua.FunctionProto) {
	if d.strip == false {
		d.writeString(p.SourceName)
	} else {
		d.writeString("")
	}

	d.writeInt(uint32(p.LineDefined))
	d.writeInt(uint32(p.LastLineDefined))
	d.writeByte(p.NumUpvalues)
	d.writeByte(p.NumParameters)
	d.writeByte(p.IsVarArg)
	d.writeByte(p.NumUsedRegisters)
	d.writeCode(p)
	d.writeConstants(p)
	d.writeDebug(p)
}

func (d *dumpState) dumpHeader() {
	d.err = binary.Write(d.out, d.order, header)
}

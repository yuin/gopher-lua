package lua

import (
	"encoding/binary"
	"fmt"
	"io"
)

const Signature = "\033GoL"
const (
	VersionMajor = 5
	VersionMinor = 1
)

var header struct {
	Signature                            [4]byte
	Version, Format, Endianness, IntSize byte
	PointerSize, InstructionSize         byte
	NumberSize, IntegralNumber           byte
}

type dumpState struct {
	l     *LState
	out   io.Writer
	order binary.ByteOrder
	strip bool
	err   error
}

func init() {
	copy(header.Signature[:], Signature)
	header.Version = VersionMajor<<4 | VersionMinor
	header.Format = 0
	header.Endianness = 1 // binary.LittleEndian
	header.IntSize = 4
	header.PointerSize = byte(1+^uintptr(0)>>32&1) * 4
	header.InstructionSize = byte(1+^uint32(0)>>32&1) * 4
	header.NumberSize = 8
	header.IntegralNumber = 0
}

func (d *dumpState) write(data interface{}) {
	if d.err == nil {
		d.err = binary.Write(d.out, d.order, data)
	}
}

func (d *dumpState) writeInt(i uint32) {
	d.write(uint32(i))
}

func (d *dumpState) writeCode(p *FunctionProto) {
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

func (d *dumpState) writeConstants(p *FunctionProto) {
	d.writeInt(uint32(len(p.Constants)))

	for _, constant := range p.Constants {
		cst := byte(constant.Type())
		d.writeByte(cst)

		switch constant.Type() {
		case LTNil:
		case LTBool:
			d.writeBool(LVAsBool(constant))
		case LTNumber:
			f, _ := constant.assertFloat64()
			d.writeNumber(f)
		case LTString:
			s, _ := constant.assertString()
			d.writeString(s)
		default:
			d.l.Panic(d.l)
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
		panic(fmt.Sprintf("unsupported pointer size (%d)"))
	}

	if size > 0 {
		d.write(ba)
		d.writeByte(0) // Write the NULL byte at the end
	}
}

func (d *dumpState) writeDebug(p *FunctionProto) {
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
}

func (d *dumpState) dumpFunction(p *FunctionProto) {
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

func (l *LState) dump(p *FunctionProto, w io.Writer) error {
	d := dumpState{l: l, out: w, order: binary.LittleEndian, strip: false}
	d.dumpHeader()
	d.dumpFunction(p)

	return d.err
}

func (l *LState) Dump(w io.Writer) error {
	fn := l.CheckFunction(1)
	fp := fn.Proto

	if err := l.dump(fp, w); err != nil {
		fmt.Println("An error occured.", err)
		return err
	}

	return nil
}

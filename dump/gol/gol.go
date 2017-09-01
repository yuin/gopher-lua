package goldump

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/yuin/gopher-lua"
)

const (
	VersionMajor = 5
	VersionMinor = 1
	Signature    = "\033GoL"
)

var header struct {
	Signature                            [4]byte
	Version, Format, Endianness, IntSize byte
	PointerSize, InstructionSize         byte
	NumberSize, IntegralNumber           byte
}

type Codec struct {
	header struct {
		Signature                            [4]byte
		Version, Format, Endianness, IntSize byte
		PointerSize, InstructionSize         byte
		NumberSize, IntegralNumber           byte
	}
	ds dumpState
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

func (c *Codec) Signature() string {
	return Signature
}

func (c *Codec) Encode(fp *lua.FunctionProto) ([]byte, error) {
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)

	d := dumpState{
		out:   writer,
		order: binary.LittleEndian,
		strip: false, // FIXME
	}
	d.dumpHeader()
	d.dumpFunction(fp)

	writer.Flush()

	return b.Bytes(), d.err
}

func (c *Codec) Decode(data []byte, fp *lua.FunctionProto) error {
	reader := bytes.NewReader(data)

	undumpState := undumpState{in: reader, order: binary.LittleEndian}

	if err := undumpState.readHeader(); err != nil {
		return err
	}

	if err := undumpState.readFunction(fp); err != nil {
		return err
	}

	return nil
}

func NewCodec() *Codec {
	return &Codec{}
}

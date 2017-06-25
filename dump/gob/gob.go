package gobdump

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"github.com/yuin/gopher-lua"
)

func init() {
	gob.Register(lua.LString(""))
	gob.Register(lua.LNumber(0))
}

type Codec struct {
}

func (c *Codec) Signature() string {
	return "\033GoB"
}

func (c *Codec) Encode(fp *lua.FunctionProto) ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Write([]byte(c.Signature()))

	enc := gob.NewEncoder(buf)
	if err := enc.Encode(fp); err != nil {
		return []byte(""), err
	}

	return buf.Bytes(), nil
}

func (c *Codec) Decode(data []byte, fp *lua.FunctionProto) error {
	b := bufio.NewReader(bytes.NewBuffer(data))

	if sbuf, err := b.Peek(4); err == nil {
		if string(sbuf) == c.Signature() {
			b.Discard(4)
			dec := gob.NewDecoder(b)
			if err := dec.Decode(fp); err != nil {
				return err
			}
		}
	}

	return nil
}

func NewCodec() *Codec {
	return &Codec{}
}

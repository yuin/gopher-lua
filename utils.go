package lua

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

func intMin(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func defaultFormat(v interface{}, f fmt.State, c rune) {
	buf := make([]string, 0, 10)
	buf = append(buf, "%")
	for i := 0; i < 128; i++ {
		if f.Flag(i) {
			buf = append(buf, string(i))
		}
	}

	if w, ok := f.Width(); ok {
		buf = append(buf, strconv.Itoa(w))
	}
	if p, ok := f.Precision(); ok {
		buf = append(buf, "."+strconv.Itoa(p))
	}
	buf = append(buf, string(c))
	format := strings.Join(buf, "")
	fmt.Fprintf(f, format, v)
}

type flagScanner struct {
	flag       byte
	start      string
	end        string
	buf        []byte
	str        string
	length     int
	pos        int
	hasFlag    bool
	changeFlag bool
}

func newFlagScanner(flag byte, start, end, str string) *flagScanner {
	return &flagScanner{flag, start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false}
}

func (fs *flagScanner) AppendString(str string) { fs.buf = append(fs.buf, str...) }

func (fs *flagScanner) AppendChar(ch byte) { fs.buf = append(fs.buf, ch) }

func (fs *flagScanner) String() string { return string(fs.buf) }

func (fs *flagScanner) Next() (byte, bool) {
	c := byte('\000')
	fs.changeFlag = false
	if fs.pos == fs.length {
		if fs.hasFlag {
			fs.AppendString(fs.end)
		}
		return c, true
	}
	c = fs.str[fs.pos]
	if c == fs.flag {
		if fs.pos < (fs.length-1) && fs.str[fs.pos+1] == fs.flag {
			fs.hasFlag = false
			fs.AppendChar(fs.flag)
			fs.pos += 2
			return fs.Next()
		} else if fs.pos != fs.length-1 {
			if fs.hasFlag {
				fs.AppendString(fs.end)
			}
			fs.AppendString(fs.start)
			fs.changeFlag = true
			fs.hasFlag = true
		}
	}
	fs.pos++
	return c, false
}

var cDateFlagToGo = map[byte]string{
	'a': "mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'c': "02 Jan 06 15:04 MST", 'd': "02",
	'F': "2006-01-02", 'H': "15", 'I': "03", 'm': "01", 'M': "04", 'p': "PM", 'P': "pm", 'S': "05",
	'x': "15/04/05", 'X': "15:04:05", 'y': "06", 'Y': "2006", 'z': "-0700", 'Z': "MST"}

func strftime(t time.Time, cfmt string) string {
	sc := newFlagScanner('%', "", "", cfmt)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.changeFlag {
			if sc.hasFlag {
				if v, ok := cDateFlagToGo[c]; ok {
					sc.AppendString(t.Format(v))
				} else {
					switch c {
					case 'w':
						sc.AppendString(fmt.Sprint(int(t.Weekday())))
					default:
						sc.AppendChar('%')
						sc.AppendChar(c)
					}
				}
				sc.hasFlag = false
			} else {
				sc.AppendChar(c)
			}
		}
	}

	return sc.String()
}

func isInteger(v LNumber) bool {
	return float64(v) == float64(int64(v))
	//_, frac := math.Modf(float64(v))
	//return frac == 0.0
}

func isArrayKey(v LNumber) bool {
	return isInteger(v) && v < LNumber(int((^uint(0))>>1)) && v > LNumber(0) && v < LNumber(MaxArrayIndex)
}

func parseNumber(number string) (LNumber, error) {
	number = strings.Trim(number, " \t\n")
	vi, err := strconv.ParseInt(number, 0, LNumberBit)
	if err == nil {
		return LNumber(vi), nil
	}
	vf, err := strconv.ParseFloat(number, LNumberBit)
	if err == nil {
		return LNumber(vf), nil
	}
	return LNumber(0), err
}

func popenArgs(arg string) (string, []string) {
	cmd := "/bin/sh"
	args := []string{"-c"}
	if LuaOS == "windows" {
		cmd = "C:\\Windows\\system32\\cmd.exe"
		args = []string{"/c"}
	}
	args = append(args, arg)
	return cmd, args
}

func isGoroutineSafe(lv LValue) bool {
	switch v := lv.(type) {
	case *LFunction, *LUserData, *LState:
		return false
	case *LTable:
		return v.Metatable == LNil
	default:
		return true
	}
}

func readBufioSize(reader *bufio.Reader, size int64) ([]byte, bool, error) {
	result := []byte{}
	read := int64(0)
	var err error
	var n int
	for read != size {
		buf := make([]byte, size-read)
		n, err = reader.Read(buf)
		if err != nil {
			break
		}
		read += int64(n)
		result = append(result, buf[:n]...)
	}
	e := err
	if e == io.EOF {
		e = nil
	}

	return result, len(result) == 0 && err == io.EOF, e
}

func readBufioLine(reader *bufio.Reader) ([]byte, bool, error) {
	result := []byte{}
	var buf []byte
	var err error
	var isprefix bool = true
	for isprefix {
		buf, isprefix, err = reader.ReadLine()
		if err != nil {
			break
		}
		result = append(result, buf...)
	}
	e := err
	if e == io.EOF {
		e = nil
	}

	return result, len(result) == 0 && err == io.EOF, e
}

func int2Fb(val int) int {
	e := 0
	x := val
	for x >= 16 {
		x = (x + 1) >> 1
		e++
	}
	if x < 8 {
		return x
	}
	return ((e + 1) << 3) | (x - 8)
}

func strCmp(s1, s2 string) int {
	len1 := len(s1)
	len2 := len(s2)
	for i := 0; ; i++ {
		c1 := -1
		if i < len1 {
			c1 = int(s1[i])
		}
		c2 := -1
		if i != len2 {
			c2 = int(s2[i])
		}
		switch {
		case c1 < c2:
			return -1
		case c1 > c2:
			return +1
		case c1 < 0:
			return 0
		}
	}
}

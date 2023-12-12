package lua

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
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
			buf = append(buf, string(rune(i)))
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
	flag        byte
	modifiers   []byte
	start       string
	end         string
	buf         []byte
	str         string
	Length      int
	Pos         int
	HasFlag     bool
	ChangeFlag  bool
	HasModifier bool
	Modifier    byte
}

func newFlagScanner(flag byte, modifiers, start, end, str string) *flagScanner {
	return &flagScanner{flag, []byte(modifiers), start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false, false, 0}
}

func (fs *flagScanner) AppendString(str string) { fs.buf = append(fs.buf, str...) }

func (fs *flagScanner) AppendChar(ch byte) { fs.buf = append(fs.buf, ch) }

func (fs *flagScanner) String() string { return string(fs.buf) }

func (fs *flagScanner) Next() (byte, bool) {
	c := byte('\000')
	fs.ChangeFlag = false
	if fs.Pos == fs.Length {
		if fs.HasFlag {
			fs.AppendString(fs.end)
		}
		return c, true
	} else {
		c = fs.str[fs.Pos]
		if c == fs.flag {
			if fs.Pos < (fs.Length-1) && fs.str[fs.Pos+1] == fs.flag {
				fs.HasFlag = false
				fs.AppendChar(fs.flag)
				fs.Pos += 2
				return fs.Next()
			} else if fs.Pos < fs.Length-1 {
				if fs.HasFlag {
					fs.AppendString(fs.end)
				}
				fs.AppendString(fs.start)
				fs.ChangeFlag = true
				fs.HasFlag = true
				fs.HasModifier = false
				fs.Modifier = 0
				if fs.Pos < fs.Length-2 {
					for _, modifier := range fs.modifiers {
						if fs.str[fs.Pos+1] == modifier {
							fs.HasModifier = true
							fs.Modifier = modifier
							fs.Pos += 1
						}
					}
				}
			}
		}
	}
	fs.Pos++
	return c, false
}

var cDateFlagToGo = map[string]string{
	// Formatting
	"n": "\n",
	"t": "\t",

	// Year
	"Y": "2006", "y": "06",

	// Month
	"b": "Jan", "B": "January",
	"m": "01", "-m": "1",

	// Day of the year/month
	"j": "002",
	"d": "02", "-d": "2", "e": "_2",

	// Day of the week
	"a": "Mon", "A": "Monday",

	// Hour, minute, second
	"H": "15",
	"I": "03", "l": "3",
	"M": "04",
	"S": "05",

	// Other
	"c": "02 Jan 06 15:04 MST",
	"x": "01/02/06", "X": "15:04:05",
	"D": "01/02/06",
	"F": "2006-01-02",
	"r": "03:04:05 PM", "R": "15:04",
	"T": "15:04:05",
	"p": "PM", "P": "pm",
	"z": "-0700", "Z": "MST",

	// Many other flags are handled in the body of strftime since they cannot
	// be represented in Go format strings.
}

// This implementation of strftime is inspired by both the C spec and Ruby's
// extensions. This allows for flags like %-d, which provides the day of the
// month without padding (1..31 instead of 01..31).
func strftime(t time.Time, cfmt string) string {
	sc := newFlagScanner('%', "-", "", "", cfmt)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.ChangeFlag {
			if sc.HasFlag {
				flag := string(c)
				if sc.HasModifier {
					flag = string(sc.Modifier) + flag
				}

				if v, ok := cDateFlagToGo[flag]; ok {
					sc.AppendString(t.Format(v))
				} else {
					switch c {
					case 'G':
						isoYear, _ := t.ISOWeek()
						sc.AppendString(fmt.Sprint(isoYear))
					case 'g':
						isoYear, _ := t.ISOWeek()
						sc.AppendString(fmt.Sprint(isoYear)[2:])
					case 'V':
						_, isoWeek := t.ISOWeek()
						sc.AppendString(fmt.Sprint(isoWeek))
					case 'w':
						sc.AppendString(fmt.Sprint(int(t.Weekday())))
					default:
						sc.AppendChar('%')
						if sc.HasModifier {
							sc.AppendChar(sc.Modifier)
						}
						sc.AppendChar(c)
					}
				}
				sc.HasFlag = false
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
	var value LNumber
	number = strings.Trim(number, " \t\n")
	if v, err := strconv.ParseInt(number, 0, LNumberBit); err != nil {
		if v2, err2 := strconv.ParseFloat(number, LNumberBit); err2 != nil {
			return LNumber(0), err2
		} else {
			value = LNumber(v2)
		}
	} else {
		value = LNumber(v)
	}
	return value, nil
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

func readBufioSize(reader *bufio.Reader, size int64) ([]byte, error, bool) {
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
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
}

func readBufioLine(reader *bufio.Reader) ([]byte, error, bool) {
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
	if e != nil && e == io.EOF {
		e = nil
	}

	return result, e, len(result) == 0 && err == io.EOF
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

func unsafeFastStringToReadOnlyBytes(s string) (bs []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return
}

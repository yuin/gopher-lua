package lua

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
	Length     int
	Pos        int
	HasFlag    bool
	ChangeFlag bool
}

func newFlagScanner(flag byte, start, end, str string) *flagScanner {
	return &flagScanner{flag, start, end, make([]byte, 0, len(str)), str, len(str), 0, false, false}
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
			} else if fs.Pos != fs.Length-1 {
				if fs.HasFlag {
					fs.AppendString(fs.end)
				}
				fs.AppendString(fs.start)
				fs.ChangeFlag = true
				fs.HasFlag = true
			}
		}
	}
	fs.Pos++
	return c, false
}

var cDateFlagToGo = map[byte]string{
	'a': "mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'c': "02 Jan 06 15:04 MST", 'd': "02",
	'F': "2006-01-02", 'H': "15", 'I': "03", 'm': "01", 'M': "04", 'p': "PM", 'P': "pm", 'S': "05",
	'y': "06", 'Y': "2006", 'z': "-0700", 'Z': "MST"}

func strftime(t time.Time, cfmt string) string {
	sc := newFlagScanner('%', "", "", cfmt)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.ChangeFlag {
			if sc.HasFlag {
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
				sc.HasFlag = false
			} else {
				sc.AppendChar(c)
			}
		}
	}

	return sc.String()
}

func compileLuaTemplate(repl string) string {
	if !LuaRegex {
		return repl
	}
	sc := newFlagScanner('%', "${", "}", repl)
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
		if !sc.ChangeFlag {
			if c >= '0' && c <= '9' {
				sc.AppendChar(c)
				continue
			}
			if sc.HasFlag {
				sc.AppendChar('}')
				sc.HasFlag = false
			}
			sc.AppendChar(c)
		}
	}
	return string(sc.String())
}

func rRange(start, end byte, isnot bool) string {
	if isnot {
		return fmt.Sprintf(`\x00-\x%02x\x%02x-\xFF`, start-1, end+1)
	}
	return fmt.Sprintf(`\x%02x-\x%02x`, start, end)
}

func compileLuaRegex(pattern string) (*regexp.Regexp, error) {
	if !LuaRegex {
		return regexp.Compile(pattern)
	}

	b := make([]byte, 1)
	sc := newFlagScanner('%', "", "", pattern)
	inset := false
	for c, eos := sc.Next(); !eos; c, eos = sc.Next() {
	retry:
		if !sc.ChangeFlag {
			if sc.HasFlag {
				if !unicode.IsLetter(rune(c)) {
					sc.AppendChar('\\')
					sc.AppendChar(c)
				} else {
					if !inset {
						sc.AppendChar('[')
					}

					switch c {
					case 'a':
						sc.AppendString(`\x61-\x7a\x41-\x5a`)
					case 'A':
						sc.AppendString(`\x00-\x60\x7b-\xFF\x00-\x40\x5b-\xFF`)
					case 'c':
						sc.AppendString(`\x00-\x1f\x7f-\x7f`)
					case 'C':
						sc.AppendString(`\x00-\xff\x20-\xFF\x00-\x7e\x80-\xFF`)
					case 'd':
						sc.AppendString(`\d`)
					case 'D':
						sc.AppendString(`\D`)
					case 'l':
						sc.AppendString(`\x61-\x7a`)
					case 'L':
						sc.AppendString(`\x00-\x60\x7b-\xFF`)
					case 'p':
						sc.AppendString(`\x21-\x2f\x3a-\x40\x5b-\x60\x7b-\x7d`)
					case 'P':
						sc.AppendString(`\x00-\x20\x30-\xFF\x00-\x39\x41-\xFF\x00-\x5a\x61-\xFF\x00-\x7a\x7e-\xFF`)
					case 's':
						sc.AppendString(`\s`)
					case 'S':
						sc.AppendString(`\S`)
					case 'u':
						sc.AppendString(`\x41-\x5a`)
					case 'U':
						sc.AppendString(`\x00-\x40\x5b-\xFF`)
					case 'w':
						sc.AppendString(`\w`)
					case 'W':
						sc.AppendString(`\W`)
					case 'x':
						sc.AppendString(`\x30-\x39\x61-\x66\x41-\x46`)
					case 'X':
						sc.AppendString(`\x00-\x2f\x3a-\xFF\x00-\x60\x67-\xFF\x00-\x40\x47-\xFF`)
					case 'z':
						sc.AppendString(`\x00`)
					case 'Z':
						sc.AppendString(`\x01-\xff`)
					default:
						return nil, errors.New("invalid character class:" + string(c))
					}
					if !inset {
						sc.AppendChar(']')
					}
				}
				sc.HasFlag = false
			} else {
				if c == '[' {
					inset = true
					sc.AppendChar(c)
					c, eos = sc.Next()
					if eos {
						break
					}
					if c == '^' {
						sc.AppendChar(c)
					} else {
						goto retry
					}
				} else if c == ']' {
					inset = false
					sc.AppendChar(c)
				} else if c == '-' && !inset {
					sc.AppendString("*?")
				} else if c == '$' && sc.Pos != sc.Length {
					sc.AppendString("\\$")
				} else if c == '^' && sc.Pos != 1 && !inset {
					sc.AppendString("\\^")
				} else if c == '\\' {
					sc.AppendString("\\\\")
				} else {
					b[0] = c
					if utf8.Valid(b) {
						sc.AppendChar(c)
					} else {
						sc.AppendString(fmt.Sprintf(`\x%02x`, c))
					}
				}
			}
		}
	}

	gopattern := sc.String()
	return regexp.Compile(gopattern)
}

func isInteger(v LNumber) bool {
	_, frac := math.Modf(float64(v))
	return frac == 0.0
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
	} else {
		envsh := os.Getenv("SHELL")
		if len(envsh) > 0 {
			cmd = envsh
		}
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

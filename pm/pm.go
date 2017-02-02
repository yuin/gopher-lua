// Lua pattern match functions for Go
package pm

import (
	"fmt"
)

const EOS = -1
const _UNKNOWN = -2

/* Error {{{ */

type Error struct {
	Pos     int
	Message string
}

func newError(pos int, message string, args ...interface{}) *Error {
	if len(args) == 0 {
		return &Error{pos, message}
	}
	return &Error{pos, fmt.Sprintf(message, args...)}
}

func (e *Error) Error() string {
	switch e.Pos {
	case EOS:
		return fmt.Sprintf("%s at EOS", e.Message)
	case _UNKNOWN:
		return fmt.Sprintf("%s", e.Message)
	default:
		return fmt.Sprintf("%s at %d", e.Message, e.Pos)
	}
}

/* }}} */

/* MatchData {{{ */

type MatchData struct {
	// captured positions
	// layout
	// xxxx xxxx xxxx xxx0 : caputured positions
	// xxxx xxxx xxxx xxx1 : position captured positions
	captures []uint32
}

func newMatchState() *MatchData { return &MatchData{[]uint32{}} }

func (st *MatchData) addPosCapture(s, pos int) {
	for s+1 >= len(st.captures) {
		st.captures = append(st.captures, 0)
	}
	st.captures[s] = (uint32(pos) << 1) | 1
	st.captures[s+1] = (uint32(pos) << 1) | 1
}

func (st *MatchData) setCapture(s, pos int) uint32 {
	for s >= len(st.captures) {
		st.captures = append(st.captures, 0)
	}
	v := st.captures[s]
	st.captures[s] = (uint32(pos) << 1)
	return v
}

func (st *MatchData) restoreCapture(s int, pos uint32) { st.captures[s] = pos }

func (st *MatchData) CaptureLength() int { return len(st.captures) }

func (st *MatchData) IsPosCapture(idx int) bool { return (st.captures[idx] & 1) == 1 }

func (st *MatchData) Capture(idx int) int { return int(st.captures[idx] >> 1) }

/* }}} */

/* scanner {{{ */

type scannerState struct {
	pos     int
	started bool
}

type scanner struct {
	src   []byte
	state scannerState
	saved scannerState
}

func newScanner(src []byte) *scanner {
	return &scanner{
		src: src,
		state: scannerState{
			pos:     0,
			started: false,
		},
		saved: scannerState{},
	}
}

func (sc *scanner) Length() int { return len(sc.src) }

func (sc *scanner) Next() int {
	if !sc.state.started {
		sc.state.started = true
		if len(sc.src) == 0 {
			sc.state.pos = EOS
		}
	} else {
		sc.state.pos = sc.NextPos()
	}
	if sc.state.pos == EOS {
		return EOS
	}
	return int(sc.src[sc.state.pos])
}

func (sc *scanner) CurrentPos() int {
	return sc.state.pos
}

func (sc *scanner) NextPos() int {
	if sc.state.pos == EOS || sc.state.pos >= len(sc.src)-1 {
		return EOS
	}
	if !sc.state.started {
		return 0
	} else {
		return sc.state.pos + 1
	}
}

func (sc *scanner) Peek() int {
	cureof := sc.state.pos == EOS
	ch := sc.Next()
	if !cureof {
		if sc.state.pos == EOS {
			sc.state.pos = len(sc.src) - 1
		} else {
			sc.state.pos--
			if sc.state.pos < 0 {
				sc.state.pos = 0
				sc.state.started = false
			}
		}
	}
	return ch
}

func (sc *scanner) Save() { sc.saved = sc.state }

func (sc *scanner) Restore() { sc.state = sc.saved }

/* }}} */

/* bytecode {{{ */

type opCode int

const (
	opChar opCode = iota
	opMatch
	opTailMatch
	opJmp
	opSplit
	opSave
	opPSave
	opBrace
	opNumber
)

type inst struct {
	code     opCode
	cls      class
	operand1 int
	operand2 int
}

/* }}} */

/* classes {{{ */

type class interface {
	Matches(ch int) bool
}

type dotClass struct{}

func (pn *dotClass) Matches(ch int) bool { return true }

type charClass struct {
	ch int
}

func (pn *charClass) Matches(ch int) bool { return pn.ch == ch }

type singleClass struct {
	class int
}

func (pn *singleClass) Matches(ch int) bool {
	ret := false
	switch pn.class {
	case 'a', 'A':
		ret = 'A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z'
	case 'c', 'C':
		ret = (0x00 <= ch && ch <= 0x1F) || ch == 0x7F
	case 'd', 'D':
		ret = '0' <= ch && ch <= '9'
	case 'l', 'L':
		ret = 'a' <= ch && ch <= 'z'
	case 'p', 'P':
		ret = (0x21 <= ch && ch <= 0x2f) || (0x30 <= ch && ch <= 0x40) || (0x5b <= ch && ch <= 0x60) || (0x7b <= ch && ch <= 0x7e)
	case 's', 'S':
		switch ch {
		case ' ', '\f', '\n', '\r', '\t', '\v':
			ret = true
		}
	case 'u', 'U':
		ret = 'A' <= ch && ch <= 'Z'
	case 'w', 'W':
		ret = '0' <= ch && ch <= '9' || 'A' <= ch && ch <= 'Z' || 'a' <= ch && ch <= 'z'
	case 'x', 'X':
		ret = '0' <= ch && ch <= '9' || 'a' <= ch && ch <= 'f' || 'A' <= ch && ch <= 'F'
	case 'z', 'Z':
		ret = ch == 0
	default:
		return ch == pn.class
	}
	if 'A' <= pn.class && pn.class <= 'Z' {
		return !ret
	}
	return ret
}

type setClass struct {
	isNot   bool
	classes []class
}

func (pn *setClass) Matches(ch int) bool {
	for _, class := range pn.classes {
		if class.Matches(ch) {
			return !pn.isNot
		}
	}
	return pn.isNot
}

type rangeClass struct {
	begin class
	end   class
}

func (pn *rangeClass) Matches(ch int) bool {
	switch begin := pn.begin.(type) {
	case *charClass:
		end, ok := pn.end.(*charClass)
		if !ok {
			return false
		}
		return begin.ch <= ch && ch <= end.ch
	}
	return false
}

// }}}

// patterns {{{

type pattern interface{}

type singlePattern struct {
	cls class
}

type seqPattern struct {
	mustHead bool
	mustTail bool
	patterns []pattern
}

type repeatPattern struct {
	t   int
	cls class
}

type posCapPattern struct{}

type capPattern struct {
	pattern pattern
}

type numberPattern struct {
	n int
}

type bracePattern struct {
	begin int
	end   int
}

// }}}

/* parse {{{ */

func parseClass(sc *scanner, allowset bool) class {
	ch := sc.Next()
	switch ch {
	case '%':
		return &singleClass{sc.Next()}
	case '.':
		if allowset {
			return &dotClass{}
		} else {
			return &charClass{ch}
		}
	case '[':
		if !allowset {
			panic(newError(sc.CurrentPos(), "invalid '['"))
		}
		return parseClassSet(sc)
	//case '^' '$', '(', ')', ']', '*', '+', '-', '?':
	//	panic(newError(sc.CurrentPos(), "invalid %c", ch))
	case EOS:
		panic(newError(sc.CurrentPos(), "unexpected EOS"))
	default:
		return &charClass{ch}
	}
}

func parseClassSet(sc *scanner) class {
	set := &setClass{false, []class{}}
	if sc.Peek() == '^' {
		set.isNot = true
		sc.Next()
	}
	isrange := false
	for {
		ch := sc.Peek()
		switch ch {
		case '[':
			panic(newError(sc.CurrentPos(), "'[' can not be nested"))
		case ']':
			sc.Next()
			goto exit
		case EOS:
			panic(newError(sc.CurrentPos(), "unexpected EOS"))
		case '-':
			if len(set.classes) > 0 {
				sc.Next()
				isrange = true
				continue
			}
			fallthrough
		default:
			set.classes = append(set.classes, parseClass(sc, false))
		}
		if isrange {
			begin := set.classes[len(set.classes)-2]
			end := set.classes[len(set.classes)-1]
			set.classes = set.classes[0 : len(set.classes)-2]
			set.classes = append(set.classes, &rangeClass{begin, end})
			isrange = false
		}
	}
exit:
	if isrange {
		set.classes = append(set.classes, &charClass{'-'})
	}

	return set
}

func parsePattern(sc *scanner, toplevel bool) *seqPattern {
	pat := &seqPattern{}
	if toplevel {
		if sc.Peek() == '^' {
			sc.Next()
			pat.mustHead = true
		}
	}
	for {
		ch := sc.Peek()
		switch ch {
		case '%':
			sc.Save()
			sc.Next()
			switch sc.Peek() {
			case '0':
				panic(newError(sc.CurrentPos(), "invalid capture index"))
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				pat.patterns = append(pat.patterns, &numberPattern{sc.Next() - 48})
			case 'b':
				sc.Next()
				pat.patterns = append(pat.patterns, &bracePattern{sc.Next(), sc.Next()})
			default:
				sc.Restore()
				pat.patterns = append(pat.patterns, &singlePattern{parseClass(sc, true)})
			}
		case '.', '[':
			pat.patterns = append(pat.patterns, &singlePattern{parseClass(sc, true)})
		case ']':
			panic(newError(sc.CurrentPos(), "invalid ']'"))
		case ')':
			if toplevel {
				panic(newError(sc.CurrentPos(), "invalid ')'"))
			}
			return pat
		case '(':
			sc.Next()
			if sc.Peek() == ')' {
				sc.Next()
				pat.patterns = append(pat.patterns, &posCapPattern{})
			} else {
				ret := &capPattern{parsePattern(sc, false)}
				if sc.Peek() != ')' {
					panic(newError(sc.CurrentPos(), "unfinished capture"))
				}
				sc.Next()
				pat.patterns = append(pat.patterns, ret)
			}
		case '*', '+', '-', '?':
			sc.Next()
			if len(pat.patterns) > 0 {
				spat, ok := pat.patterns[len(pat.patterns)-1].(*singlePattern)
				if ok {
					pat.patterns = pat.patterns[0 : len(pat.patterns)-1]
					pat.patterns = append(pat.patterns, &repeatPattern{ch, spat.cls})
					continue
				}
			}
			pat.patterns = append(pat.patterns, &singlePattern{&charClass{ch}})
		case '$':
			if toplevel && (sc.NextPos() == sc.Length()-1 || sc.NextPos() == EOS) {
				pat.mustTail = true
			} else {
				pat.patterns = append(pat.patterns, &singlePattern{&charClass{ch}})
			}
			sc.Next()
		case EOS:
			sc.Next()
			goto exit
		default:
			sc.Next()
			pat.patterns = append(pat.patterns, &singlePattern{&charClass{ch}})
		}
	}
exit:
	return pat
}

type iptr struct {
	insts   []inst
	capture int
}

func compilePattern(p pattern, ps ...*iptr) []inst {
	var ptr *iptr
	toplevel := false
	if len(ps) == 0 {
		toplevel = true
		ptr = &iptr{[]inst{inst{opSave, nil, 0, -1}}, 2}
	} else {
		ptr = ps[0]
	}
	switch pat := p.(type) {
	case *singlePattern:
		ptr.insts = append(ptr.insts, inst{opChar, pat.cls, -1, -1})
	case *seqPattern:
		for _, cp := range pat.patterns {
			compilePattern(cp, ptr)
		}
	case *repeatPattern:
		idx := len(ptr.insts)
		switch pat.t {
		case '*':
			ptr.insts = append(ptr.insts,
				inst{opSplit, nil, idx + 1, idx + 3},
				inst{opChar, pat.cls, -1, -1},
				inst{opJmp, nil, idx, -1})
		case '+':
			ptr.insts = append(ptr.insts,
				inst{opChar, pat.cls, -1, -1},
				inst{opSplit, nil, idx, idx + 2})
		case '-':
			ptr.insts = append(ptr.insts,
				inst{opSplit, nil, idx + 3, idx + 1},
				inst{opChar, pat.cls, -1, -1},
				inst{opJmp, nil, idx, -1})
		case '?':
			ptr.insts = append(ptr.insts,
				inst{opSplit, nil, idx + 1, idx + 2},
				inst{opChar, pat.cls, -1, -1})
		}
	case *posCapPattern:
		ptr.insts = append(ptr.insts, inst{opPSave, nil, ptr.capture, -1})
		ptr.capture += 2
	case *capPattern:
		c0, c1 := ptr.capture, ptr.capture+1
		ptr.capture += 2
		ptr.insts = append(ptr.insts, inst{opSave, nil, c0, -1})
		compilePattern(pat.pattern, ptr)
		ptr.insts = append(ptr.insts, inst{opSave, nil, c1, -1})
	case *bracePattern:
		ptr.insts = append(ptr.insts, inst{opBrace, nil, pat.begin, pat.end})
	case *numberPattern:
		ptr.insts = append(ptr.insts, inst{opNumber, nil, pat.n, -1})
	}
	if toplevel {
		if p.(*seqPattern).mustTail {
			ptr.insts = append(ptr.insts, inst{opSave, nil, 1, -1}, inst{opTailMatch, nil, -1, -1})
		}
		ptr.insts = append(ptr.insts, inst{opSave, nil, 1, -1}, inst{opMatch, nil, -1, -1})
	}
	return ptr.insts
}

/* }}} parse */

/* VM {{{ */

// Simple recursive virtual machine based on the
// "Regular Expression Matching: the Virtual Machine Approach" (https://swtch.com/~rsc/regexp/regexp2.html)
func recursiveVM(src []byte, insts []inst, pc, sp int, ms ...*MatchData) (bool, int, *MatchData) {
	var m *MatchData
	if len(ms) == 0 {
		m = newMatchState()
	} else {
		m = ms[0]
	}
redo:
	inst := insts[pc]
	switch inst.code {
	case opChar:
		if sp >= len(src) || !inst.cls.Matches(int(src[sp])) {
			return false, sp, m
		}
		pc++
		sp++
		goto redo
	case opMatch:
		return true, sp, m
	case opTailMatch:
		return sp >= len(src), sp, m
	case opJmp:
		pc = inst.operand1
		goto redo
	case opSplit:
		if ok, nsp, _ := recursiveVM(src, insts, inst.operand1, sp, m); ok {
			return true, nsp, m
		}
		pc = inst.operand2
		goto redo
	case opSave:
		s := m.setCapture(inst.operand1, sp)
		if ok, nsp, _ := recursiveVM(src, insts, pc+1, sp, m); ok {
			return true, nsp, m
		}
		m.restoreCapture(inst.operand1, s)
		return false, sp, m
	case opPSave:
		m.addPosCapture(inst.operand1, sp+1)
		pc++
		goto redo
	case opBrace:
		if sp >= len(src) || int(src[sp]) != inst.operand1 {
			return false, sp, m
		}
		count := 1
		for sp = sp + 1; sp < len(src); sp++ {
			if int(src[sp]) == inst.operand2 {
				count--
			}
			if count == 0 {
				pc++
				sp++
				goto redo
			}
			if int(src[sp]) == inst.operand1 {
				count++
			}
		}
		return false, sp, m
	case opNumber:
		idx := inst.operand1 * 2
		if idx >= m.CaptureLength()-1 {
			panic(newError(_UNKNOWN, "invalid capture index"))
		}
		capture := src[m.Capture(idx):m.Capture(idx+1)]
		for i := 0; i < len(capture); i++ {
			if i+sp >= len(src) || capture[i] != src[i+sp] {
				return false, sp, m
			}
		}
		pc++
		sp += len(capture)
		goto redo
	}
	panic("should not reach here")
	return false, sp, m
}

/* }}} */

/* API {{{ */

func Find(p string, src []byte, offset, limit int) (matches []*MatchData, err error) {
	defer func() {
		if v := recover(); v != nil {
			if perr, ok := v.(*Error); ok {
				err = perr
			} else {
				panic(v)
			}
		}
	}()
	pat := parsePattern(newScanner([]byte(p)), true)
	insts := compilePattern(pat)
	matches = []*MatchData{}
	for sp := offset; sp <= len(src); {
		ok, nsp, ms := recursiveVM(src, insts, 0, sp)
		sp++
		if ok {
			if sp < nsp {
				sp = nsp
			}
			matches = append(matches, ms)
		}
		if len(matches) == limit || pat.mustHead {
			break
		}
	}
	return
}

/* }}} */

package lua

import "math"

const bitCount = 32

func OpenBit(l *LState) int {
	mod := l.RegisterModule(Bit32LibName, bitFuncs).(*LTable)
	l.Push(mod)
	return 1
}

var bitFuncs = map[string]LGFunction{
	"arshift": bitArshift,
	"band":    bitBand,
	"bnot":    bitBnot,
	"bor":     bitBor,
	"bxor":    bitBxor,
	"btest":   bitBtest,
	"extract": bitExtract,
	"lrotate": bitLrotate,
	"lshift":  bitLshift,
	"replace": bitReplace,
	"rrotate": bitRrotate,
	"rshift":  bitRshift,
}

func bitArshift(l *LState) int {
	if r, i := l.CheckUnsignedNumber(1), int(l.CheckNumber(2)); i < 0 || 0 == (r&(1<<(bitCount-1))) {
		return shift(l, r, -i)
	} else {
		if i >= bitCount {
			r = math.MaxUint32
		} else {
			r = trim((r >> uint(i)) | ^(math.MaxUint32 >> uint(i)))
		}
		l.Push(LNumber(r))
	}
	return 1
}

func bitBand(l *LState) int {
	l.Push(LNumber(andHelper(l)))
	return 1
}

func bitBnot(l *LState) int {
	l.Push(LNumber(trim(^l.CheckUnsignedNumber(1))))
	return 1
}

func bitBor(l *LState) int {
	l.Push(LNumber(bitOp(l, 0, func(a, b uint) uint { return a | b })))
	return 1
}

func bitBxor(l *LState) int {
	l.Push(LNumber(bitOp(l, 0, func(a, b uint) uint { return a ^ b })))
	return 1
}

func bitBtest(l *LState) int {
	l.Push(LBool(andHelper(l) != 0))
	return 1
}

func bitExtract(l *LState) int {
	r := l.CheckUnsignedNumber(1)
	f, w := fieldArguments(l, 2)
	l.Push(LNumber((r >> f) & mask(w)))
	return 1
}

func bitLrotate(l *LState) int {
	return rotate(l, int(l.CheckNumber(2)))
}

func bitLshift(l *LState) int {
	return shift(l, l.CheckUnsignedNumber(1), int(l.CheckNumber(2)))
}

func bitReplace(l *LState) int {
	r, v := l.CheckUnsignedNumber(1), l.CheckUnsignedNumber(2)
	f, w := fieldArguments(l, 3)
	m := mask(w)
	v &= m
	l.Push(LNumber((r & ^(m << f)) | (v << f)))
	return 1
}

func bitRrotate(l *LState) int {
	return rotate(l, -int(l.CheckNumber(2)))
}

func bitRshift(l *LState) int {
	return shift(l, l.CheckUnsignedNumber(1), -int(l.CheckNumber(2)))
}

func trim(x uint) uint { return x & math.MaxUint32 }
func mask(n uint) uint { return ^(math.MaxUint32 << n) }

func shift(l *LState, r uint, i int) int {
	if i < 0 {
		if i, r = -i, trim(r); i >= bitCount {
			r = 0
		} else {
			r >>= uint(i)
		}
	} else {
		if i >= bitCount {
			r = 0
		} else {
			r <<= uint(i)
		}
		r = trim(r)
	}
	l.Push(LNumber(r))
	return 1
}

func rotate(l *LState, i int) int {
	r := trim(l.CheckUnsignedNumber(1))
	if i &= bitCount - 1; i != 0 {
		r = trim((r << uint(i)) | (r >> uint(bitCount-i)))
	}
	l.Push(LNumber(r))
	return 1
}

func bitOp(l *LState, init uint, f func(a, b uint) uint) uint {
	r := init
	for i, n := 1, l.GetTop(); i <= n; i++ {
		r = f(r, l.CheckUnsignedNumber(i))
	}
	return trim(r)
}

func andHelper(l *LState) uint {
	x := bitOp(l, ^uint(0), func(a, b uint) uint { return a & b })
	return x
}

func fieldArguments(l *LState, fieldIndex int) (uint, uint) {
	f, w := l.CheckNumber(fieldIndex), LNumber(l.OptInt(fieldIndex+1, 1))
	ArgumentCheck(l, 0 <= f, fieldIndex, "field cannot be negative")
	ArgumentCheck(l, 0 < w, fieldIndex+1, "width must be positive")
	if f+w > bitCount {
		l.RaiseError("trying to access non-existent bits")
	}
	return uint(f), uint(w)
}

// ArgumentCheck checks whether cond is true. If not, raises an error with a standard message.
func ArgumentCheck(l *LState, cond bool, index int, extraMessage string) {
	if !cond {
		l.ArgError(index, extraMessage)
	}
}

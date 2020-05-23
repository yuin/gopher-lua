package lua

import (
	"errors"
	"math"
	"unsafe"
)

// Adapt from lua 5.3 implementation.
// https://www.lua.org/source/5.3/ltable.c.html

const (
	/*
	** Maximum size of array part (MAXASIZE) is 2^MAXABITS. MAXABITS is
	** the largest integer such that MAXASIZE fits in an unsigned int.
	 */
	MAXABITS = 31
	MAXASIZE = 1 << MAXABITS

	/*
	** Maximum size of hash part is 2^MAXHBITS. MAXHBITS is the largest
	** integer such that 2^MAXHBITS fits in a signed int. (Note that the
	** maximum number of elements in a table, 2^MAXABITS + 2^MAXHBITS, still
	** fits comfortably in an unsigned int.)
	 */
	MAXHBITS = MAXABITS - 1
)

var log_2 [256]byte = [256]byte{ /* log_2[i] = ceil(log2(i - 1)) */
	0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

func luaO_ceillog2(x uint32) int {
	l := 0
	x--
	for x >= 256 {
		l += 8
		x >>= 8
	}
	return l + int(log_2[x])
}

// https://www.lua.org/source/5.3/lstring.c.html#luaS_hash
// use FNV1a instead
func hashString32(s string) uint32 {
	const (
		prime32 = uint32(16777619)
		seed    = uint32(2166136261)
	)

	h := uint32(seed)
	l := len(s)
	h = h ^ uint32(l)
	step := (l >> 5) + 1
	for ; l >= step; l -= step {
		// h = h ^ ((h << 5) + (h >> 2) + uint32(s[l-1]))
		h = (h ^ uint32(s[0])) * prime32
	}
	return h
}

func isIntegerKey(v LNumber) bool {
	t := int64(v)
	return LNumber(t) == v && t >= math.MinInt64 && t <= math.MaxInt64
}

type tkey struct {
	tvk  LValue
	next int32 /* for chaining (offset for next node) */
}

type tnode struct {
	val LValue
	key tkey
}

type ltable struct {
	lsizenode   byte /* log2 of size of 'node' array */
	sizearray   uint32
	array       []LValue
	node        []tnode
	lastfreeIdx int32
}

var dummynodes = []tnode{
	{val: LNil, key: tkey{LNil, 0}},
}

const ltableBadPosIdx = -1

var (
	errTabInvalidKeyNext = errors.New("invalid key to 'next'")
)

// for test only
func newltable(nhsize int) (*ltable, error) {
	tab := &ltable{}
	initltable(tab, 0, nhsize)
	return tab, nil
}

func initltable(tab *ltable, nasize, nhsize int) {
	if nasize < 0 {
		nasize = 0
	}
	if nhsize < 0 {
		nhsize = 0
	}
	tab.setnodevector(uint32(nhsize))
	// XXX: removed on lua 5.3
	if nasize > 0 {
		tab.setarrayvector(uint32(nasize))
	}
}

func (t *ltable) sizenode() uint32 {
	return 1 << uint32(t.lsizenode)
}

func (t *ltable) hashpow2i(n uint64) uint32 {
	mod := uint32(n) & (t.sizenode() - 1)
	return mod
}

/*
** for some types, it is better to avoid modulus by power of 2, as
** they tend to have many 2 factors.
 */

func (t *ltable) hashmodi(n uint32) uint32 {
	mod := n % ((t.sizenode() - 1) | 1)
	return mod
}

func (t *ltable) hashstri(s string) uint32 {
	return hashString32(s) & (t.sizenode() - 1)
}

func (t *ltable) hashpointer(n LValue) uint32 {
	v := *(*[2]uintptr)(unsafe.Pointer(&n))
	return t.hashmodi(uint32(v[1] & math.MaxUint32))
}

func (t *ltable) setnodevector(size uint32) {
	if size == 0 {
		t.node = dummynodes
		t.lsizenode = 0
		t.lastfreeIdx = ltableBadPosIdx
	} else {
		lsize := luaO_ceillog2(size)
		if lsize > MAXHBITS {
			panic("table overflow")
		}
		size := 1 << uint(lsize)
		t.node = make([]tnode, size)
		for i := 0; i < int(size); i++ {
			t.node[i].val = LNil
			t.node[i].key.tvk = LNil
		}
		t.lsizenode = byte(lsize)
		t.lastfreeIdx = int32(size)
	}
}

func (t *ltable) isdummy() bool {
	return t.lastfreeIdx == ltableBadPosIdx
}

func (t *ltable) newkey(key LValue) *LValue {
	if key.Type() == LTNil {
		panic("table index is nil")
	}
	if key.Type() == LTChannel {
		panic("table index is channel")
	}
	mpi := int32(t.mainposition(key))
	if t.node[mpi].val != LNil || t.isdummy() { // main position is taken?
		fi := t.getfreepos()
		if fi == ltableBadPosIdx {
			t.rehash(key)
			return t.set(key)
		}
		if t.isdummy() {
			panic("still dummy hash")
		}
		otherni := int32(t.mainposition(t.node[mpi].key.tvk))
		if otherni != mpi { // is colliding node out of its main position?
			// yes; move colliding node into free position
			for otherni+t.node[otherni].key.next != mpi { // find previous
				otherni += t.node[otherni].key.next
			}
			t.node[otherni].key.next = fi - otherni // rechain to point to 'f'
			t.node[fi] = t.node[mpi]                // copy colliding node into free pos. (mp->next also goes)
			if t.node[mpi].key.next != 0 {
				t.node[fi].key.next += mpi - fi // correct 'next'
				t.node[mpi].key.next = 0        // now 'mp' is free
			}
			t.node[mpi].val = LNil
		} else { // colliding node is in its own main position
			// new node will go into free position
			if t.node[mpi].key.next != 0 {
				t.node[fi].key.next = mpi + t.node[mpi].key.next - fi
			} else {
				if t.node[fi].key.next != 0 {
					panic("fi next != 0")
				}
			}
			t.node[mpi].key.next = fi - mpi
			mpi = fi
		}
	}
	t.node[mpi].key.tvk = key
	return &t.node[mpi].val
}

func hashfloat(f float64) int32 {
	n, i := math.Frexp(f)
	n = n * -math.MinInt32
	if !(n >= math.MinInt64 && n <= math.MaxInt64) {
		// is 'n' inf/-inf/NaN?
		return 0
	}
	u := uint32(i) + uint32(int64(n))
	if u <= uint32(math.MaxInt32) {
		return int32(u)
	} else {
		return int32(^u)
	}
}

func (t *ltable) mainposition(key LValue) uint32 {
	switch key.Type() {
	case LTNumber:
		tv := key.(LNumber)
		if isIntegerKey(tv) {
			return t.hashpow2i(uint64(tv))
		}
		return t.hashmodi(uint32(hashfloat(float64(tv))))
	case LTBool:
		if key == LTrue {
			return t.hashpow2i(1)
		} else {
			return t.hashpow2i(0)
		}
	case LTString:
		tv := key.(LString)
		return t.hashstri(string(tv))
	case LTChannel:
		panic("using channel fo key is not unsupported")
	default:
		return t.hashpointer(key)
	}
}

func (t *ltable) getfreepos() int32 {
	if !t.isdummy() {
		for t.lastfreeIdx > 0 {
			t.lastfreeIdx--
			if t.node[t.lastfreeIdx].key.tvk == LNil {
				return t.lastfreeIdx
			}
		}
	}
	return ltableBadPosIdx
}

func keyRawEquals(lhs, rhs LValue) bool {
	if lhs.Type() != rhs.Type() {
		return false
	}

	ret := false
	switch lhs.Type() {
	case LTNil:
		ret = true
	case LTNumber:
		v1, _ := lhs.assertFloat64()
		v2, _ := rhs.assertFloat64()
		ret = v1 == v2
	case LTBool:
		ret = bool(lhs.(LBool)) == bool(rhs.(LBool))
	case LTString:
		ret = lhs.(LString) == rhs.(LString)
	case LTUserData, LTTable:
		if lhs == rhs {
			ret = true
		}
	default:
		ret = lhs == rhs
	}
	return ret
}

/*
** returns the index for 'key' if 'key' is an appropriate key to live in
** the array part of the table, 0 otherwise.
 */
func arrayindex(key LValue) uint32 {
	if tv, ok := key.(LNumber); ok && isIntegerKey(tv) {
		k := int64(tv)
		if k > 0 && k <= MAXASIZE {
			return uint32(k)
		}
	}
	return 0
}

/*
** returns the index of a 'key' for table traversals. First goes all
** elements in the array part, then elements in the hash part. The
** beginning of a traversal is signaled by 0.
 */
func (t *ltable) findindex(key LValue) (uint32, error) {
	if key == LNil {
		return 0, nil
	}
	i := arrayindex(key)
	if i != 0 && i <= t.sizearray {
		return i, nil
	} else {
		ni := int32(t.mainposition(key))
		var nx int32
		for {
			if keyRawEquals(t.node[ni].key.tvk, key) {
				i = uint32(ni)
				// hash elements are numbered after array ones
				return (i + 1) + t.sizearray, nil
			}
			nx = t.node[ni].key.next
			if nx == 0 {
				return 0, errTabInvalidKeyNext
			} else {
				ni += nx
			}
		}
	}
}

func (t *ltable) Next(key LValue) (LValue, LValue, bool) {
	i, err := t.findindex(key)
	if err != nil {
		return LNil, LNil, false
	}
	// try first array part
	for ; i < t.sizearray; i++ {
		if t.array[i] != LNil {
			return LNumber(i + 1), t.array[i], true
		}
	}
	// hash part
	for i -= t.sizearray; int32(i) < int32(t.sizenode()); i++ {
		gv := t.node[i].val
		if gv != LNil {
			return t.node[i].key.tvk, gv, true
		}
	}
	return LNil, LNil, false
}

/*
** Compute the optimal size for the array part of table 't'. 'nums' is a
** "count array" where 'nums[i]' is the number of integers in the table
** between 2^(i - 1) + 1 and 2^i. 'pna' enters with the total number of
** integer keys in the table and leaves with the number of keys that
** will go to the array part; return the optimal size.
 */
func computesizes(nums []uint32, pna *uint32) uint32 {
	var i int32
	var twotoi, a, na, optimal uint32
	twotoi = 1
	for ; twotoi > 0 && *pna > twotoi/2; i++ {
		if nums[i] > 0 {
			a += nums[i]
			if a > twotoi/2 { /* more than half elements present? */
				optimal = twotoi
				na = a
			}
		}
		twotoi *= 2
	}
	*pna = na
	return optimal
}

func countint(key LValue, nums []uint32) int32 {
	k := arrayindex(key)
	if k != 0 {
		nums[luaO_ceillog2(k)]++
		return 1
	} else {
		return 0
	}
}

func (t *ltable) numusearray(nums []uint32) uint32 {
	var lg int32
	var ttlg, ause uint32
	i := uint32(1)
	ttlg = 1
	/* traverse each slice */
	for ; lg <= MAXABITS; lg++ {
		lim := ttlg
		var lc uint32
		if lim > t.sizearray {
			lim = t.sizearray
			if i > lim {
				break // no more elements to count
			}
		}
		// count elements in range (2^(lg - 1), 2^lg]
		for ; i <= lim; i++ {
			if t.array[i-1] != LNil {
				lc++
			}
		}
		nums[lg] += lc
		ause += lc
		ttlg *= 2
	}
	return ause
}

func (t *ltable) numusehash(nums []uint32, pna *uint32) int32 {
	var totaluse int32
	var ause int32
	i := int32(t.sizenode())
	for i > 0 {
		i--
		if t.node[i].val != LNil {
			ause += countint(t.node[i].key.tvk, nums)
			totaluse++
		}
	}
	*pna += uint32(ause)
	return totaluse
}

func (t *ltable) setarrayvector(size uint32) {
	s := make([]LValue, size)
	copy(s, t.array)
	t.array = s
	for i := t.sizearray; i < size; i++ {
		t.array[i] = LNil
	}
	t.sizearray = size
}

func (t *ltable) allocsizenode() uint32 {
	if t.isdummy() {
		return 0
	} else {
		return t.sizenode()
	}
}

func (t *ltable) resize(nasize, nhsize uint32) {
	oldasize := t.sizearray
	ohsize := t.allocsizenode()
	nold := t.node
	// create new hash part with appropriate size
	t.setnodevector(nhsize)
	// println("ltable resize", nasize, nhsize, len(t.node))
	if nasize > oldasize {
		// array part must grow?
		t.setarrayvector(nasize)
	}
	if nasize < oldasize {
		t.sizearray = nasize
		// re-insert elements from vanishing slice
		for i := nasize; i < oldasize; i++ {
			if t.array[i] != LNil {
				t.SetInt(int64(i+1), t.array[i])
			}
		}
		s := make([]LValue, nasize)
		copy(s, t.array)
		t.array = s
	}
	// re-insert elements from hash part
	for j := int32(ohsize) - 1; j >= 0; j-- {
		if nold[j].val != LNil {
			*t.set(nold[j].key.tvk) = nold[j].val
		}
	}
	return
}

func (t *ltable) getInt(key int64) *LValue {
	if uint64(key)-1 < uint64(t.sizearray) {
		return &t.array[key-1]
	} else {
		ni := int32(t.hashpow2i(uint64(key)))
		for {
			if kv, ok := t.node[ni].key.tvk.(LNumber); ok && isIntegerKey(kv) && int64(kv) == key {
				return &t.node[ni].val
			} else {
				nx := t.node[ni].key.next
				if nx == 0 {
					break
				}
				ni += nx
			}
		}
		return &LNil
	}
}

func (t *ltable) GetInt(key int64) LValue {
	return *t.getInt(key)
}

func (t *ltable) getgeneric(key LValue) *LValue {
	ni := int32(t.mainposition(key))
	for {
		if keyRawEquals(t.node[ni].key.tvk, key) {
			return &t.node[ni].val
		} else {
			nx := t.node[ni].key.next
			if nx == 0 {
				return &LNil
			}
			ni += nx
		}
	}
}

func (t *ltable) getStr(key string) *LValue {
	ni := int32(t.hashstri(key)) // mainposition
	for {
		tvk := t.node[ni].key.tvk
		if tvk.Type() == LTString && string(tvk.(LString)) == key {
			return &t.node[ni].val
		} else {
			nx := t.node[ni].key.next
			if nx == 0 {
				return &LNil
			}
			ni += nx
		}
	}
}

func (t *ltable) get(key LValue) *LValue {
	switch key.Type() {
	case LTNil:
		return &LNil
	case LTChannel:
		return &LNil
	case LTNumber:
		tv := key.(LNumber)
		if isIntegerKey(tv) {
			return t.getInt(int64(tv))
		}
		return t.getgeneric(key)
	case LTString:
		return t.getStr(string(key.(LString)))
	default:
		return t.getgeneric(key)
	}
}

func (t *ltable) Get(key LValue) LValue {
	return *t.get(key)
}

func (t *ltable) GetString(key string) LValue {
	return *t.getStr(key)
}

func (t *ltable) SetInt(key int64, value LValue) {
	p := t.setInt(key)
	*p = value
}

func (t *ltable) Set(key LValue, value LValue) {
	p := t.set(key)
	*p = value
	// t.set(key)
}

func (t *ltable) setInt(key int64) *LValue {
	p := t.getInt(key)
	if p != &LNil {
		return p
	}
	return t.newkey(LNumber(key))
}

func (t *ltable) set(key LValue) *LValue {
	p := t.get(key)
	if p != &LNil {
		return p
	}
	return t.newkey(key)
}

func (t *ltable) rehash(key LValue) {
	var asize, na uint32
	var nums [MAXABITS + 1]uint32
	na = t.numusearray(nums[:])
	totaluse := na
	totaluse += uint32(t.numusehash(nums[:], &na))
	na += uint32(countint(key, nums[:]))
	totaluse++
	asize = computesizes(nums[:], &na)
	t.resize(asize, totaluse-na)
}

/*
** Try to find a boundary in table 't'. A 'boundary' is an integer index
** such that t[i] is non-nil and t[i+1] is nil (and 0 if t[1] is nil).
 */
func (t *ltable) GetN() uint64 {
	j := t.sizearray
	if j > 0 && t.array[j-1] == LNil {
		// there is a boundary in the array part: (binary) search for it
		i := uint32(0)
		for j-i > 1 {
			m := (i + j) / 2
			if t.array[m-1] == LNil {
				j = m
			} else {
				i = m
			}
		}
		return uint64(i)
	} else if t.isdummy() {
		return uint64(j)
	} else {
		return t.unboundSearch(uint64(j))
	}
}

func (t *ltable) unboundSearch(j uint64) uint64 {
	i := j
	j++
	// find 'i' and 'j' such that i is present and j is not
	for *t.getInt(int64(j)) != LNil {
		i = j
		if j > uint64(math.MaxInt64)/2 {
			// overflow?
			// table was built with bad purposes: resort to linear search
			i = 1
			for *t.getInt(int64(i)) != LNil {
				i++
			}
			return i - 1
		}
		j *= 2
	}
	// now do a binary search between them
	for j-i > 1 {
		m := (i + j) / 2
		if *t.getInt(int64(m)) == LNil {
			j = m
		} else {
			i = m
		}
	}
	return i
}

func (t *ltable) Swap(i, j int64) {
	pi := t.setInt(i)
	pj := t.setInt(j)
	*pi, *pj = *pj, *pi
}

func (t *ltable) ResizeArray(nasize int) {
	if nasize < 0 {
		nasize = 0
	}
	nsize := t.allocsizenode()
	t.resize(uint32(nasize), nsize)
}

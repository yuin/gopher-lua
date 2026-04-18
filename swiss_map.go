// This is a port of the swiss_map.go file from the dolthub/swiss repository.
// The original source code is licensed under the Apache License, Version 2.0.
// The original source code can be found at:
// https://github.com/dolthub/swiss
// remove generic functions and types to adapt to the project
// remove unused functions and types

package lua

import (
	"math/rand"
	"unsafe"
)

type keyKind int

// currently only two key kinds are supported
const (
	KeyKindStr keyKind = iota
	keyKindIntr
)

type hashFunc func(unsafe.Pointer, uintptr) uintptr

//go:linkname strhash runtime.strhash
func strhash(p unsafe.Pointer, s uintptr) uintptr

//go:linkname interhash runtime.interhash
func interhash(p unsafe.Pointer, s uintptr) uintptr

// baseMap is an open-addressing hash map
// based on Abseil's flat_hash_map.
type baseMap struct {
	kind     keyKind
	ctrl     []metadata
	groups   []group
	randSeed uint64
	hashFn   hashFunc
	resident uint32
	dead     uint32
	limit    uint32
}

// metadata is the h2 metadata array for a group.
// find operations first probe the controls bytes
// to filter candidates before matching keys
type metadata [groupSize]int8

type groupKey [2]uintptr

// group is a group of 16 key-value pairs
type group struct {
	keys   [groupSize]groupKey
	values [groupSize]LValue
}

const (
	h1Mask    uint64 = 0xffff_ffff_ffff_ff80
	h2Mask    uint64 = 0x0000_0000_0000_007f
	empty     int8   = -128 // 0b1000_0000
	tombstone int8   = -2   // 0b1111_1110
)

// h1 is a 57 bit hash prefix
type h1 uint64

// h2 is a 7 bit hash suffix
type h2 int8

// newMap constructs a Map.
func newMap(kind keyKind, sz uint32) (m *baseMap) {
	var hashFn hashFunc
	if kind == KeyKindStr {
		hashFn = strhash
	} else {
		hashFn = interhash
	}
	groups := numGroups(sz)
	m = &baseMap{
		kind:     kind,
		ctrl:     make([]metadata, groups),
		groups:   make([]group, groups),
		randSeed: rand.Uint64(),
		hashFn:   hashFn,
		limit:    groups * maxAvgGroupLoad,
	}
	for i := range m.ctrl {
		m.ctrl[i] = newEmptyMetadata()
	}
	return
}

func (m *baseMap) hash(key unsafe.Pointer) uint64 {
	return uint64(m.hashFn(key, uintptr(m.randSeed)))
}

func (m *baseMap) keyEqualGroupKey(key unsafe.Pointer, k *groupKey) bool {
	if m.kind == KeyKindStr {
		keyStrPtr := (*string)(key)
		kStrPtr := (*string)(unsafe.Pointer(k))
		return *keyStrPtr == *kStrPtr
	}
	keyIntPtr := (*LValue)(key)
	kIntPtr := (*LValue)(unsafe.Pointer(k))
	return *keyIntPtr == *kIntPtr
}

// get returns the |value| mapped by |key| if one exists.
func (m *baseMap) get(key unsafe.Pointer) (value LValue, ok bool) {
	if m == nil {
		return
	}
	hi, lo := splitHash(m.hash(key))
	g := probeStart(hi, len(m.groups))
	for { // inlined find loop
		matches := metaMatchH2(&m.ctrl[g], lo)
		for matches != 0 {
			s := nextMatch(&matches)
			if m.keyEqualGroupKey(key, &m.groups[g].keys[s]) {
				value, ok = m.groups[g].values[s], true
				return
			}
		}
		// |key| is not in group |g|,
		// stop probing if we see an empty slot
		matches = metaMatchEmpty(&m.ctrl[g])
		if matches != 0 {
			ok = false
			return
		}
		g += 1 // linear probing
		if g >= uint32(len(m.groups)) {
			g = 0
		}
	}
}

// first returns the first key-value pair in the Map.
func (m *baseMap) first() (key unsafe.Pointer, value LValue, ok bool) {
	for g, c := range m.ctrl {
		for s := range c {
			if c[s] != empty && c[s] != tombstone {
				return (unsafe.Pointer)(&m.groups[g].keys[s]), m.groups[g].values[s], true
			}
		}
	}
	return
}

// put attempts to insert |key| and |value|
func (m *baseMap) put(key unsafe.Pointer, value LValue) {
	if m.resident >= m.limit {
		m.rehash(m.nextSize())
	}

	hi, lo := splitHash(m.hash(key))
	g := probeStart(hi, len(m.groups))
	for { // inlined find loop
		matches := metaMatchH2(&m.ctrl[g], lo)
		for matches != 0 {
			s := nextMatch(&matches)
			if m.keyEqualGroupKey(key, &m.groups[g].keys[s]) { // update
				m.groups[g].keys[s] = *(*groupKey)(key)
				m.groups[g].values[s] = value
				return
			}
		}
		// |key| is not in group |g|,
		// stop probing if we see an empty slot
		matches = metaMatchEmpty(&m.ctrl[g])
		if matches != 0 { // insert
			s := nextMatch(&matches)
			m.groups[g].keys[s] = *(*groupKey)(key)
			m.groups[g].values[s] = value
			m.ctrl[g][s] = int8(lo)
			m.resident++
			return
		}
		g += 1 // linear probing
		if g >= uint32(len(m.groups)) {
			g = 0
		}
	}
}

// delete attempts to remove |key|, returns true successful.
func (m *baseMap) delete(key unsafe.Pointer) (ok bool) {
	hi, lo := splitHash(m.hash(key))
	g := probeStart(hi, len(m.groups))
	for {
		matches := metaMatchH2(&m.ctrl[g], lo)
		for matches != 0 {
			s := nextMatch(&matches)
			if m.keyEqualGroupKey(key, &m.groups[g].keys[s]) {
				ok = true
				// optimization: if |m.ctrl[g]| contains any empty
				// metadata bytes, we can physically delete |key|
				// rather than placing a tombstone.
				// The observation is that any probes into group |g|
				// would already be terminated by the existing empty
				// slot, and therefore reclaiming slot |s| will not
				// cause premature termination of probes into |g|.
				if metaMatchEmpty(&m.ctrl[g]) != 0 {
					m.ctrl[g][s] = empty
					m.resident--
				} else {
					m.ctrl[g][s] = tombstone
					m.dead++
				}
				var k groupKey
				var v LValue
				m.groups[g].keys[s] = k
				m.groups[g].values[s] = v
				return
			}
		}
		// |key| is not in group |g|,
		// stop probing if we see an empty slot
		matches = metaMatchEmpty(&m.ctrl[g])
		if matches != 0 { // |key| absent
			ok = false
			return
		}
		g += 1 // linear probing
		if g >= uint32(len(m.groups)) {
			g = 0
		}
	}
}

// iter iterates the elements of the Map, passing them to the callback.
// It guarantees that any key in the Map will be visited only once, and
// for un-mutated Maps, every key will be visited once. If the Map is
// Mutated during iteration, mutations will be reflected on return from
// iter, but the set of keys visited by iter is non-deterministic.
func (m *baseMap) iter(cb func(k unsafe.Pointer, v LValue) (stop bool)) {
	// take a consistent view of the table in case
	// we rehash during iteration
	ctrl, groups := m.ctrl, m.groups
	// pick a random starting group
	g := rand.Intn(len(groups))
	for n := 0; n < len(groups); n++ {
		for s, c := range ctrl[g] {
			if c == empty || c == tombstone {
				continue
			}
			k, v := groups[g].keys[s], groups[g].values[s]
			if stop := cb((unsafe.Pointer)(&k), v); stop {
				return
			}
		}
		g++
		if g >= len(groups) {
			g = 0
		}
	}
}

// count returns the number of elements in the Map.
func (m *baseMap) count() int {
	return int(m.resident - m.dead)
}

// // capacity returns the number of additional elements
// // the can be added to the Map before resizing.
// func (m *baseMap) capacity() int {
// 	return int(m.limit - m.resident)
// }

// findNext returns the next key-value pair in the Map after |key|.
func (m *baseMap) findNext(key unsafe.Pointer) (retKey unsafe.Pointer, retValue LValue, ok bool) {
	hi, lo := splitHash(m.hash(key))
	startG := probeStart(hi, len(m.groups))
	g := startG
	for { // inlined find loop
		matches := metaMatchH2(&m.ctrl[g], lo)
		for matches != 0 {
			s := nextMatch(&matches)
			if m.keyEqualGroupKey(key, &m.groups[g].keys[s]) {
				// move to the next key
				for {
					s++
					if s >= groupSize {
						s = 0
						g++
						if g >= uint32(len(m.groups)) {
							// end of the table
							ok = false
							return
						}
					}
					if m.ctrl[g][s] != empty && m.ctrl[g][s] != tombstone {
						retKey = (unsafe.Pointer)(&m.groups[g].keys[s])
						retValue = m.groups[g].values[s]
						ok = true
						return
					}
				}
			}
		}
		// |key| is not in group |g|,
		// stop probing if we see an empty slot
		matches = metaMatchEmpty(&m.ctrl[g])
		if matches != 0 {
			ok = false
			return
		}
		g += 1 // linear probing
		if g >= uint32(len(m.groups)) {
			g = 0
		}
	}
}

func (m *baseMap) nextSize() (n uint32) {
	n = uint32(len(m.groups)) * 2
	if m.dead >= (m.resident / 2) {
		n = uint32(len(m.groups))
	}
	return
}

func (m *baseMap) rehash(n uint32) {
	groups, ctrl := m.groups, m.ctrl
	m.groups = make([]group, n)
	m.ctrl = make([]metadata, n)
	for i := range m.ctrl {
		m.ctrl[i] = newEmptyMetadata()
	}
	m.randSeed = rand.Uint64()
	m.limit = n * maxAvgGroupLoad
	m.resident, m.dead = 0, 0
	for g := range ctrl {
		for s := range ctrl[g] {
			c := ctrl[g][s]
			if c == empty || c == tombstone {
				continue
			}
			m.put((unsafe.Pointer)(&groups[g].keys[s]), groups[g].values[s])
		}
	}
}

// numGroups returns the minimum number of groups needed to store |n| elems.
func numGroups(n uint32) (groups uint32) {
	groups = (n + maxAvgGroupLoad - 1) / maxAvgGroupLoad
	if groups == 0 {
		groups = 1
	}
	return
}

func newEmptyMetadata() (meta metadata) {
	for i := range meta {
		meta[i] = empty
	}
	return
}

func splitHash(h uint64) (h1, h2) {
	return h1((h & h1Mask) >> 7), h2(h & h2Mask)
}

func probeStart(hi h1, groups int) uint32 {
	return fastModN(uint32(hi), uint32(groups))
}

// lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
func fastModN(x, n uint32) uint32 {
	return uint32((uint64(x) * uint64(n)) >> 32)
}

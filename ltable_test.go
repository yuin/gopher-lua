package lua

import (
	"testing"
)

func TestLTable(t *testing.T) {
	tb, err := newltable(5)
	if err != nil {
		t.Fatal(err)
	}
	for i := -10; i <= 10; i++ {
		tb.setint(int64(i), LNumber(i))
		// fmt.Println(i, tb.array, tb.node)
	}
	t.Log(tb.node)
	// p, _ := tb.set(LString("XXX"))
	// *p = LString("XXXVAL")
	// p, _ = tb.set(LString("YYY"))
	// *p = LString("YYYVAL1")
	// p, _ = tb.set(LString("YYY"))
	// *p = LString("YYYVAL2")

	print("HERE")
	nk, nv, _ := tb.next(LNil)
	for nk != LNil {
		t.Log(nk, nv)
		nk, nv, _ = tb.next(nk)
	}

	t.Log("getn: ", tb.getn())
}

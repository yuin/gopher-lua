package lua

import (
	"testing"
)

func TestLTableBasic(t *testing.T) {
	tb, err := newltable(0)
	if err != nil {
		t.Fatal(err)
	}
	if tb.GetN() != 0 {
		t.Error("not 0")
	}
	for i := -10; i <= 10; i++ {
		tb.SetInt(int64(i), LNumber(i))
		// fmt.Println(i, tb.array, tb.node)
	}
	for i := -10; i <= 10; i++ {
		p := tb.GetInt(int64(i))
		if (*p).(LNumber) != LNumber(i) {
			t.Error("bad: ", i, *p)
		}
	}

	t.Log(tb.node)
	p, _ := tb.Set(LString("XXX"))
	*p = LString("XXXVAL")
	p, _ = tb.Set(LString("YYY"))
	*p = LString("YYYVAL1")
	p, _ = tb.Set(LString("YYY"))
	*p = LString("YYYVAL2")

	p, _ = tb.Set(LBool(true))
	*p = LString("b1")
	p, _ = tb.Set(LBool(false))
	*p = LString("b0")

	if *tb.Get(LString("YYY")) != LString("YYYVAL2") {
		t.Error("bad string key")
	}

	if *tb.Get(LBool(true)) != LString("b1") {
		t.Error("bad bool key")
	}

	if *tb.Get(LBool(false)) != LString("b0") {
		t.Error("bad bool key")
	}

	if *tb.Get(LString("NOTFOUNDKEY")) != LNil {
		t.Error("bad key is not &nil")
	}

	count := 0
	nk, nv, _ := tb.Next(LNil)
	for nk != LNil {
		count++
		t.Log(nk, nv)
		nk, nv, _ = tb.Next(nk)
	}
	if count != 21+4 {
		t.Error("not 25")
	}

	t.Log("getn: ", tb.GetN())
	if tb.GetN() != 10 {
		t.Error("not 10")
	}

	ud := &LUserData{}

	p, _ = tb.Set(ud)
	*p = LString("UD")
	if *tb.Get(ud) != LString("UD") {
		t.Error("bad userdata key")
	}

	fl := LNumber(34.232)
	p, _ = tb.Set(fl)
	*p = LString("float")
	if *tb.Get(fl) != LString("float") {
		t.Error("bad float64 key")
	}
	if *tb.Get(LNumber(1243243211232432.23)) != LNil {
		t.Error("bad float64 key")
	}
}

func TestLTableDense(t *testing.T) {
	tb, _ := newltable(5)
	for i := 1; i <= 1000; i++ {
		tb.SetInt(int64(i), LNumber(i))
	}
	if len(tb.array) != 1024 {
		t.Error("array size should be 1024")
	}
	if len(tb.node) != 1 && !tb.isdummy() && tb.lsizenode != 0 {
		t.Error("hash size should be 0")
	}

	tb1, _ := newltable(5)
	for i := 1000; i >= 1; i-- {
		tb1.SetInt(int64(i), LNumber(i))
	}

	if len(tb1.array) != 1024 {
		t.Error("array size should be 1024")
	}

	if len(tb1.node) != 1 {
		t.Error("hash size should be 0")
	}

	// shrink

	for i := 200; i <= 1000; i++ {
		p, _ := tb.Set(LNumber(i))
		*p = LNil
	}
	{
		// trigger shrink
		p, _ := tb.Set(LNumber(32.2))
		*p = LNumber(32.2)
	}
	if len(tb.array) != 256 {
		t.Error("array size should be 256")
	}

	// rotate the table (queue)
	tb2, _ := newltable(0)

	for i := 1; i <= 1000; i++ {
		tb2.SetInt(int64(i), LNumber(i))
	}
	if tb2.GetN() != 1000 {
		t.Error("getn not 1000")
	}
	for i := 1; i <= 800; i++ {
		p, _ := tb2.Set(LNumber(i))
		*p = LNil
		tb2.SetInt(int64(1000+i), LNumber(i))
	}
	if len(tb2.array) != 0 {
		t.Error("array size should be 0")
	}

}

func TestLTableSparse(t *testing.T) {
	tb, err := newltable(5)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		_ = tb.SetInt(int64(i*1000), LNumber(i*1000))
	}
	for i := 0; i < 1000; i++ {
		p := tb.Get(LNumber(i * 1000))
		if *p != LNumber(i*1000) {
			t.Error("bad: ", i)
		}
	}

	tb1, _ := newltable(0)
	tb1.SetInt(1, LNumber(1))
	tb1.SetInt(2, LNumber(2))
	tb1.SetInt(3, LNumber(3))
	tb1.SetInt(4, LNumber(4))
	if tb1.GetN() != 4 {
		t.Error("bad: ", tb1.GetN())
	}
	p, _ := tb1.Set(LNumber(42.2))
	*p = LNumber(42.2)
	// tb1.SetInt(5, LNumber(5))
	if tb1.GetN() != 4 {
		t.Error("bad: ", tb1.GetN())
	}
}

func TestLTableGetN(t *testing.T) {
	tb, _ := newltable(5)
	for i := 1; i <= 5; i++ {
		_ = tb.SetInt(int64(i), LNumber(i))
	}
	if tb.GetN() != 5 {
		t.Error("not 5")
	}
	tb.SetInt(int64(5), LNil)
	if tb.GetN() != 4 {
		t.Error("not 4")
	}
}

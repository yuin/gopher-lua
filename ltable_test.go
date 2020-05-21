package lua

import (
	"testing"
)

func TestLTableBasic(t *testing.T) {
	tb, err := newltable(0)
	if err != nil {
		t.Fatal(err)
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

}

func TestLTableSparse(t *testing.T) {
	tb, err := newltable(5)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 1000; i++ {
		_ = tb.SetInt(int64(i*1000), LNumber(i*1000))
	}
	for i := 1; i < 1000; i++ {
		p := tb.Get(LNumber(i * 1000))
		if *p != LNumber(i*1000) {
			t.Error("bad: ", i)
		}
	}
}

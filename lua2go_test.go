package lua_test

import (
	"github.com/yuin/gopher-lua"
	"testing"
)

func Test_Lua2Go_String(t *testing.T) {
	gv := lua.LuaToGo(lua.LString("string"))

	if gv != "string" {
		t.Fatal("Expected \"string\"")
		return
	}
}

func Test_Lua2Go_Nil(t *testing.T) {
	gv := lua.LuaToGo(lua.LNil)

	if gv != nil {
		t.Fatal("Expected nil")
		return
	}
}

func Test_Lua2Go_Bool(t *testing.T) {
	gvt := lua.LuaToGo(lua.LTrue).(bool)
	if !gvt {
		t.Fatal("Expected true")
	}
	gvf := lua.LuaToGo(lua.LFalse).(bool)
	if gvf {
		t.Fatal("Expected false")
	}
}

func Test_Lua2Go_Number(t *testing.T) {
	gvi := lua.LuaToGo(lua.LNumber(2048))
	if gvi != int64(2048) {
		t.Fatal("Expected int == 2048")
	}
	gvf := lua.LuaToGo(lua.LNumber(200000.00001))
	if gvf != float64(200000.00001) {
		t.Fatal("Expected float == 200000.00001")
	}
}

func Test_Lua2Go_Table_SimpleMap(t *testing.T) {
	b := lua.LTable{}
	b.RawSet(lua.LString("key"), lua.LString("value"))

	gv := lua.LuaToGo(b).(map[interface{}]interface{})
	if len(gv) != 1 {
		t.Fatal("Expected one element")
		return
	}
	if gv["key"] != "value" {
		t.Fatal("Expected key => value")
		return
	}
}

func Test_Lua2Go_Table_Array(t *testing.T) {
	r := lua.LTable{}
	r.Append(lua.LString("hi"))

	b := lua.LTable{}
	b.Append(lua.LNil)
	b.Append(lua.LTrue)
	b.Append(lua.LNumber(2048))
	b.Append(lua.LString("other"))
	b.Append(r)

	gv := lua.LuaToGo(b).(map[interface{}]interface{})
	ln := len(gv)
	if ln != 4 {
		t.Fatalf("Expected 5 elements, got %d", ln)
		return
	}

	i1 := gv[int64(1)]
	if i1 != nil {
		t.Fatal("Expected map not to contain index 1")
	}

	i2 := gv[int64(2)].(bool)
	if !i2 {
		t.Fatal("Expected index 2 to be true")
	}

	i3 := gv[int64(3)].(int64)
	if i3 != 2048 {
		t.Fatal("Expected index 3 to be 2048")
	}

	i4 := gv[int64(4)].(string)
	if i4 != "other" {
		t.Fatal("Expected index 4 to be \"other\"")
	}

	i5 := gv[int64(5)].(map[interface{}]interface{})
	if len(i5) != 1 {
		t.Fatal("Expected len of index 5 to be 1")
	}

	i5_1 := i5[int64(1)].(string)
	if i5_1 != "hi" {
		t.Fatal("Expected gv[5][1] to be \"hi\"")
	}
}

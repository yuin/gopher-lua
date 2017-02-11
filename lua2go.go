package lua

func LuaToGo(value LValue) interface{} {
	switch value.Type() {
	case LTNil:
		return nil
	case LTBool:
		return value == LTrue
	case LTNumber:
		nm := value.(LNumber)
		if nm.IsInteger() {
			return nm.Integer()
		}
		return nm.Float()
	case LTString:
		return value.(LString).String()
	case LTFunction:
		return nil
	case LTTable:
		return LuaTableToGo(value.(LTable))
	}
	return ""
}

func LuaTableToGo(t LTable) interface{} {
	r := make(map[interface{}]interface{})

	t.ForEach(func(k, v LValue) {
		kg := LuaToGo(k)
		vg := LuaToGo(v)

		r[kg] = vg
	})

	return r

}

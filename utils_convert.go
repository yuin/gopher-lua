package lua

import "time"

// ConvertMapStringToLTable would conver a map to lua table
func ConvertMapStringToLTable(data map[string]string) *LTable {
	lt := newLTable(0, len(data))
	for k, v := range data {
		lt.RawSetString(k, LString(v))
	}
	return lt
}

// ConvertMapToLTable would conver a map to lua table
func ConvertMapToLTable(data map[string]interface{}) *LTable {
	lt := newLTable(0, len(data))
	for k, v := range data {
		lt.RawSetString(k, ConvertToLValue(v))
	}
	return lt
}

// ConvertToLValue would conver an interface value to lua value
//   - bool: boolean
//   - string, []byte: string
//   - float32/64, int/32/64, uint/32/64: number
//   - map[string][]string: table{key: table{...}, ...}
//   - map[string]string, map[string]interface{}: table
//   - []string, []interface{}: table
//   - time.Time: number (UTC Unix timestamp)
//   - nil: nil
func ConvertToLValue(val interface{}) LValue {
	if val == nil {
		return LNil
	}
	switch v := val.(type) {
	case bool:
		return LBool(v)
	case string:
		return LString(v)
	case []byte:
		return LString(v)
	case float32:
		return LNumber(v)
	case float64:
		return LNumber(v)
	case int:
		return LNumber(v)
	case int32:
		return LNumber(v)
	case int64:
		return LNumber(v)
	case uint:
		return LNumber(v)
	case uint32:
		return LNumber(v)
	case uint64:
		return LNumber(v)
	case map[string][]string:
		lt := newLTable(0, len(v))
		for k, v := range v {
			lt.RawSetString(k, ConvertToLValue(v))
		}
		return lt
	case map[string]string:
		return ConvertMapStringToLTable(v)
	case map[string]interface{}:
		return ConvertMapToLTable(v)
	case map[interface{}]interface{}:
		lt := newLTable(0, len(v))
		for k, v := range v {
			lt.RawSet(ConvertToLValue(k), ConvertToLValue(v))
		}
		return lt
	case []string:
		lt := newLTable(len(v), 0)
		for i := range v {
			lt.RawSetInt(i+1, LString(v[i]))
		}
		return lt
	case []interface{}:
		lt := newLTable(len(v), 0)
		for i := range v {
			lt.RawSetInt(i+1, ConvertToLValue(v[i]))
		}
		return lt
	case time.Time:
		return LNumber(v.UTC().Unix())
	default:
		return LNil
	}
}

// ConvertFromLValue would conver lua value to an interface value
//  - boolean: bool
//  - string: string
//  - number: float64 or int64
//  - table: map[string]interface{} or []interface{} (by MaxN)
//  - nil: nil
func ConvertFromLValue(lv LValue) interface{} {
	switch v := lv.(type) {
	case *LNilType:
		return nil
	case LBool:
		return bool(v)
	case LString:
		return string(v)
	case LNumber:
		vf := float64(v)
		vi := int64(v)
		if vf == float64(vi) {
			return vi
		}
		return vf
	case *LTable:
		maxn := v.MaxN()
		if maxn == 0 {
			// as table
			ret := make(map[string]interface{}, v.DictionaryLen())
			v.ForEach(func(key, value LValue) {
				keyStr := key.String()
				ret[keyStr] = ConvertFromLValue(value)
			})
			return ret
		}
		// as array
		ret := make([]interface{}, maxn)
		for i := 1; i <= maxn; i++ {
			ret[i-1] = ConvertFromLValue(v.RawGetInt(i))
		}
		return ret
	default:
		return v
	}
}

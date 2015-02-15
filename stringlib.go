package lua

import (
	"fmt"
	"regexp"
	"strings"
)

func stringOpen(L *LState) {
	_, ok := L.G.builtinMts[int(LTString)]
	if !ok {
		mod := L.RegisterModule("string", strFuncs).(*LTable)
		gmatch := L.NewClosure(strGmatch, L.NewFunction(strGmatchIter))
		mod.RawSetH(LString("gmatch"), gmatch)
		mod.RawSetH(LString("gfind"), gmatch)
		mod.RawSetH(LString("__index"), mod)
		L.G.builtinMts[int(LTString)] = mod
	}
}

var strFuncs = map[string]LGFunction{
	"byte":    strByte,
	"char":    strChar,
	"dump":    strDump,
	"find":    strFind,
	"format":  strFormat,
	"gsub":    strGsub,
	"len":     strLen,
	"lower":   strLower,
	"match":   strMatch,
	"rep":     strRep,
	"reverse": strReverse,
	"sub":     strSub,
	"upper":   strUpper,
}

func strByte(L *LState) int {
	str := L.CheckString(1)
	start := L.OptInt(2, 1) - 1
	end := L.OptInt(3, -1)
	l := len(str)
	if start < 0 {
		start = l + start + 1
	}
	if end < 0 {
		end = l + end + 1
	}

	if L.GetTop() == 2 {
		if start < 0 || start >= l {
			return 0
		}
		L.Push(LNumber(str[start]))
		return 1
	}

	start = intMax(start, 0)
	end = intMin(end, l)
	if end < 0 || end <= start || start >= l {
		return 0
	}

	for i := start; i < end; i++ {
		L.Push(LNumber(str[i]))
	}
	return end - start
}

func strChar(L *LState) int {
	top := L.GetTop()
	bytes := make([]byte, L.GetTop())
	for i := 1; i <= top; i++ {
		bytes[i-1] = uint8(L.CheckInt(i))
	}
	L.Push(LString(string(bytes)))
	return 1
}

func strDump(L *LState) int {
	L.RaiseError("GopherLua does not support the string.dump")
	return 0
}

func strFind(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	if len(str) == 0 && len(pattern) == 0 {
		L.Push(LNumber(1))
		L.Push(LNumber(0))
		return 2
	}
	init := luaIndex2StringIndex(str, L.OptInt(3, 1), true)
	plain := false
	if L.GetTop() == 4 {
		plain = LVAsBool(L.Get(4))
	}
	if len(str) == 0 && len(pattern) == 0 {
		L.Push(LNumber(1))
		return 1
	}

	if plain {
		pos := strings.Index(str[init:], pattern)
		if pos < 0 {
			L.Push(LNil)
			return 1
		}
		L.Push(LNumber(init+pos) + 1)
		L.Push(LNumber(init + pos + len(pattern)))
		return 2
	}

	re, err := compileLuaRegex(pattern)
	if err != nil {
		L.RaiseError(err.Error())
	}
	stroffset := str[init:]
	positions := re.FindStringSubmatchIndex(stroffset)
	if positions == nil {
		L.Push(LNil)
		return 1
	}
	npos := len(positions)
	L.Push(LNumber(init+positions[0]) + 1)
	L.Push(LNumber(init + positions[npos-1]))
	for i := 2; i < npos; i += 2 {
		L.Push(LString(stroffset[positions[i]:positions[i+1]]))
	}
	return npos/2 + 1
}

func strFormat(L *LState) int {
	str := L.CheckString(1)
	args := make([]interface{}, L.GetTop()-1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		args[i-2] = L.Get(i)
	}
	npat := strings.Count(str, "%") - strings.Count(str, "%%")
	L.Push(LString(fmt.Sprintf(str, args[:intMin(npat, len(args))]...)))
	return 1
}

func strGsub(L *LState) int {
	str := L.CheckString(1)
	pat := L.CheckString(2)
	L.CheckTypes(3, LTString, LTTable, LTFunction)
	repl := L.CheckAny(3)
	limit := L.OptInt(4, -1)

	re, err := compileLuaRegex(pat)
	if err != nil {
		L.RaiseError(err.Error())
	}
	matches := re.FindAllStringSubmatchIndex(str, limit)
	if matches == nil || len(matches) == 0 {
		L.SetTop(1)
		L.Push(LNumber(0))
		return 2
	}
	switch lv := repl.(type) {
	case LString:
		L.Push(LString(strGsubStr(str, re, string(lv), matches)))
	case *LTable:
		L.Push(LString(strGsubTable(L, str, lv, matches)))
	case *LFunction:
		L.Push(LString(strGsubFunc(L, str, lv, matches)))
	}
	L.Push(LNumber(len(matches)))
	return 2
}

type replaceInfo struct {
	Indicies []int
	String   string
}

func strGsubDoReplace(str string, info []replaceInfo) string {
	offset := 0
	buf := []byte(str)
	for _, replace := range info {
		oldlen := len(buf)
		b1 := append([]byte(""), buf[0:offset+replace.Indicies[0]]...)
		b2 := []byte("")
		index2 := offset + replace.Indicies[1]
		if index2 <= len(buf) {
			b2 = append(b2, buf[index2:len(buf)]...)
		}
		buf = append(b1, replace.String...)
		buf = append(buf, b2...)
		offset += len(buf) - oldlen
	}
	return string(buf)
}

func strGsubStr(str string, re *regexp.Regexp, repl string, matches [][]int) string {
	infoList := make([]replaceInfo, 0, len(matches))
	repl = compileLuaTemplate(repl)
	for _, match := range matches {
		start, end := match[0], match[1]
		if end < 0 {
			continue
		}
		buf := make([]byte, 0, end-start)
		buf = re.ExpandString(buf, repl, str, match)
		infoList = append(infoList, replaceInfo{[]int{start, end}, string(buf)})
	}

	return strGsubDoReplace(str, infoList)
}

func strGsubTable(L *LState, str string, repl *LTable, matches [][]int) string {
	infoList := make([]replaceInfo, 0, len(matches))
	for _, match := range matches {
		var key string
		start, end := match[0], match[1]
		if end < 0 {
			continue
		}
		if len(match) > 2 { // has captures
			key = str[match[2]:match[3]]
		} else {
			key = str[match[0]:match[1]]
		}
		value := L.GetField(repl, key)
		if !LVIsFalse(value) {
			infoList = append(infoList, replaceInfo{[]int{start, end}, LVAsString(value)})
		}
	}
	return strGsubDoReplace(str, infoList)
}

func strGsubFunc(L *LState, str string, repl *LFunction, matches [][]int) string {
	infoList := make([]replaceInfo, 0, len(matches))
	for _, match := range matches {
		start, end := match[0], match[1]
		if end < 0 {
			continue
		}
		L.Push(repl)
		nargs := 0
		if len(match) > 2 { // has captures
			for i := 2; i < len(match); i += 2 {
				L.Push(LString(str[match[i]:match[i+1]]))
				nargs++
			}
		} else {
			L.Push(LString(str[start:end]))
			nargs++
		}
		L.Call(nargs, 1)
		ret := L.reg.Pop()
		if !LVIsFalse(ret) {
			infoList = append(infoList, replaceInfo{[]int{start, end}, LVAsString(ret)})
		}
	}
	return strGsubDoReplace(str, infoList)
}

type strMatchData struct {
	str     string
	pos     int
	matches [][]int
}

func strGmatchIter(L *LState) int {
	md := L.CheckUserData(1).Value.(*strMatchData)
	str := md.str
	matches := md.matches
	idx := md.pos
	md.pos += 1
	if idx == len(matches) {
		return 0
	}
	L.Push(L.Get(1))
	match := matches[idx]
	if len(match) == 2 {
		L.Push(LString(str[match[0]:match[1]]))
		return 1
	}

	for i := 2; i < len(match); i += 2 {
		L.Push(LString(str[match[i]:match[i+1]]))
	}
	return len(match)/2 - 1
}

func strGmatch(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	re, err := compileLuaRegex(pattern)
	if err != nil {
		L.RaiseError(err.Error())
	}
	L.Push(L.Get(UpvalueIndex(1)))
	ud := L.NewUserData()
	ud.Value = &strMatchData{str, 0, re.FindAllStringSubmatchIndex(str, -1)}
	L.Push(ud)
	return 2
}

func strLen(L *LState) int {
	str := L.CheckString(1)
	L.Push(LNumber(len(str)))
	return 1
}

func strLower(L *LState) int {
	str := L.CheckString(1)
	L.Push(LString(strings.ToLower(str)))
	return 1
}

func strMatch(L *LState) int {
	str := L.CheckString(1)
	pattern := L.CheckString(2)
	offset := L.OptInt(3, 1)
	l := len(str)
	if offset < 0 {
		offset = l + offset + 1
	}
	offset--
	if offset < 0 {
		offset = 0
	}

	re, err := compileLuaRegex(pattern)
	if err != nil {
		L.RaiseError(err.Error())
	}
	subs := re.FindStringSubmatchIndex(str[offset:])
	nsubs := len(subs) / 2
	switch nsubs {
	case 0:
		L.Push(LNil)
		return 1
	case 1:
		L.Push(LString(str[subs[0]:subs[1]]))
		return 1
	default:
		for i := 2; i < len(subs); i += 2 {
			L.Push(LString(str[subs[i]:subs[i+1]]))
		}
		return nsubs - 1
	}

}

func strRep(L *LState) int {
	str := L.CheckString(1)
	n := L.CheckInt(2)
	L.Push(LString(strings.Repeat(str, n)))
	return 1
}

func strReverse(L *LState) int {
	str := L.CheckString(1)
	bts := []byte(str)
	out := make([]byte, len(bts))
	for i, j := 0, len(bts)-1; j >= 0; i, j = i+1, j-1 {
		out[i] = bts[j]
	}
	L.Push(LString(string(out)))
	return 1
}

func strSub(L *LState) int {
	str := L.CheckString(1)
	start := luaIndex2StringIndex(str, L.CheckInt(2), true)
	end := luaIndex2StringIndex(str, L.OptInt(3, -1), false)
	l := len(str)
	if start >= l || end < start {
		L.Push(LString(""))
	} else {
		L.Push(LString(str[start:end]))
	}
	return 1
}

func strUpper(L *LState) int {
	str := L.CheckString(1)
	L.Push(LString(strings.ToUpper(str)))
	return 1
}

func luaIndex2StringIndex(str string, i int, start bool) int {
	if start {
		i -= 1
	}
	l := len(str)
	if i < 0 {
		i = l + i + 1
	}
	i = intMax(0, i)
	if !start && i > l {
		i = l
	}
	return i
}

//

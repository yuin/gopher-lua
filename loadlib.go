package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"regexp"
	"strings"
)

/* load lib {{{ */

const loRegPrefix = "LOADLIB: "

var loLoaders = []LGFunction{loLoaderPreload, loLoaderLua, loLoaderC}

func loGetPath(env string, defpath string) string {
	path := os.Getenv(env)
	if len(path) == 0 {
		path = defpath
	}
	path = strings.Replace(path, ";;", ";"+defpath+";", -1)
	if os.PathSeparator != '/' {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			panic(err)
		}
		path = strings.Replace(path, "!", dir, -1)
	}
	return path
}

func loFindFile(L *LState, name, pname string) (string, string) {
	name = strings.Replace(name, ".", string(os.PathSeparator), -1)
	lv := L.GetField(L.GetField(L.Get(EnvironIndex), "package"), pname)
	path, ok := lv.(LString)
	if !ok {
		L.RaiseError("package.%s must be a string", pname)
	}
	messages := []string{}
	for _, pattern := range strings.Split(string(path), ";") {
		luapath := strings.Replace(pattern, "?", name, -1)
		if _, err := os.Stat(luapath); err == nil {
			return luapath, ""
		} else {
			messages = append(messages, err.Error())
		}
	}
	return "", strings.Join(messages, "\n\t")
}

func loRegister(L *LState, path string) *LUserData {
	rname := loRegPrefix + path
	reg := L.Get(RegistryIndex).(*LTable)
	r := reg.RawGetString(rname)
	if r != LNil {
		return r.(*LUserData)
	}
	plib := L.NewUserData()
	plib.Value = nil
	L.SetMetatable(plib, L.GetMetatable(reg))
	reg.RawSetString(rname, plib)
	return plib
}

func loSym(L *LState, ud *LUserData, sym string) (*LFunction, error) {
	p := ud.Value.(*plugin.Plugin)
	f, err := p.Lookup(sym)
	if err != nil {
		return nil, err
	}
	return L.NewFunction(f.(func(*LState) int)), nil
}

func loLoadFunc(L *LState, path, sym string) (*LFunction, error) {
	ud := loRegister(L, path)
	if ud.Value == nil {
		p, err := plugin.Open(path)
		if err != nil {
			return nil, err
		}
		ud.Value = p
	}

	f, err := loSym(L, ud, sym)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func loMakeFuncName(s string) string {
	re := regexp.MustCompile("^[^\\-]*\\-")
	name := re.ReplaceAllString(s, "")

	re = regexp.MustCompile("\\.(\\w)")

	funcname := re.ReplaceAllStringFunc(name, func(s string) string {
		return strings.ToUpper(re.FindStringSubmatch(s)[1])
	})
	return "LuaOpen" + strings.Title(funcname)
}

func OpenPackage(L *LState) int {
	packagemod := L.RegisterModule(LoadLibName, loFuncs)

	L.SetField(packagemod, "preload", L.NewTable())

	loaders := L.CreateTable(len(loLoaders), 0)
	for i, loader := range loLoaders {
		L.RawSetInt(loaders, i+1, L.NewFunction(loader))
	}
	L.SetField(packagemod, "loaders", loaders)
	L.SetField(L.Get(RegistryIndex), "_LOADERS", loaders)

	loaded := L.NewTable()
	L.SetField(packagemod, "loaded", loaded)
	L.SetField(L.Get(RegistryIndex), "_LOADED", loaded)

	L.SetField(packagemod, "path", LString(loGetPath(LuaPath, LuaPathDefault)))
	L.SetField(packagemod, "cpath", LString(""))

	L.Push(packagemod)
	return 1
}

var loFuncs = map[string]LGFunction{
	"loadlib": loLoadLib,
	"seeall":  loSeeAll,
}

func loLoaderPreload(L *LState) int {
	name := L.CheckString(1)
	preload := L.GetField(L.GetField(L.Get(EnvironIndex), "package"), "preload")
	if _, ok := preload.(*LTable); !ok {
		L.RaiseError("package.preload must be a table")
	}
	lv := L.GetField(preload, name)
	if lv == LNil {
		L.Push(LString(fmt.Sprintf("no field package.preload['%s']", name)))
		return 1
	}
	L.Push(lv)
	return 1
}

func loLoaderLua(L *LState) int {
	name := L.CheckString(1)
	path, msg := loFindFile(L, name, "path")
	if len(path) == 0 {
		L.Push(LString(msg))
		return 1
	}
	fn, err1 := L.LoadFile(path)
	if err1 != nil {
		L.RaiseError(err1.Error())
	}
	L.Push(fn)
	return 1
}

func loLoaderC(L *LState) int {
	name := L.CheckString(1)
	path, msg := loFindFile(L, name, "cpath")
	if len(path) == 0 {
		L.Push(LString(msg))
		return 1
	}
	f, err := loLoadFunc(L, path, loMakeFuncName(name))
	if err != nil {
		L.RaiseError(err.Error())
	}
	L.Push(f)
	return 1
}

func loLoadLib(L *LState) int {
	path := L.CheckString(1)
	init := L.CheckString(2)
	f, err := loLoadFunc(L, path, init)
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		if strings.Contains(err.Error(), "plugin.Open") {
			L.Push(LString("open"))
		} else {
			L.Push(LString("init"))
		}
		return 3
	}
	L.Push(f)
	return 1
}

func loSeeAll(L *LState) int {
	mod := L.CheckTable(1)
	mt := L.GetMetatable(mod)
	if mt == LNil {
		mt = L.CreateTable(0, 1)
		L.SetMetatable(mod, mt)
	}
	L.SetField(mt, "__index", L.Get(GlobalsIndex))
	return 0
}

/* }}} */

//

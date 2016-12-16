package lua

import (
	"os"
)

var CompatVarArg = true
var FieldsPerFlush = 50
var RegistrySize = 256 * 20
var CallStackSize = 256
var MaxTableGetLoop = 100
var MaxArrayIndex = 67108864

type LNumber float64

const LNumberBit = 64
const LNumberScanFormat = "%f"

var LuaPath = "LUA_PATH"
var LuaLDir string
var LuaPathDefault string
var LuaOS string
var LuaCDir string
var LuaCPathDefault string

func init() {
	if os.PathSeparator == '/' { // unix-like
		LuaOS = "unix"
		LuaLDir = "/usr/local/share/lua/5.1"
		LuaPathDefault = "./?.lua;" + LuaLDir + "/?.lua;" + LuaLDir + "/?/init.lua"
		LuaCDir = "/usr/local/lib/glua/5.1"
		LuaCPathDefault = "./?.so;" + LuaCDir + "/?.so;" + LuaCDir + "/loadall.so;"
	} else { // windows
		LuaOS = "windows"
		LuaLDir = "!\\lua"
		LuaPathDefault = ".\\?.lua;" + LuaLDir + "\\?.lua;" + LuaLDir + "\\?\\init.lua"
		LuaCDir = "!\\"
		LuaPathDefault = ".\\?.dll;" + LuaCDir + "\\?.dll;" + LuaCDir + "\\?\\loadall.dll"
	}
}

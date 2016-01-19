package lua

import "fmt"

const (
	// TabLibName is the name of the table Library.
	TabLibName = "table"
	// IoLibName is the name of the io Library.
	IoLibName = "io"
	// OsLibName is the name of the os Library.
	OsLibName = "os"
	// StringLibName is the name of the string Library.
	StringLibName = "string"
	// MathLibName is the name of the math Library.
	MathLibName = "math"
	// DebugLibName is the name of the debug Library.
	DebugLibName = "debug"
	// ChannelLibName is the name of the channel Library.
	ChannelLibName = "channel"
	// CoroutineLibName is the name of the coroutine Library.
	CoroutineLibName = "coroutine"
	// BaseLibName is here for consistency; the base functions have no namespace/library.
	BaseLibName = "_baseLib"
	// LoadLibName is here for consistency; the loading system has no namespace/library.
	LoadLibName = "_loadLib"
)

// LuaLibs are the built-in Gopher-lua libraries as opened by LState.OpenLibs(),
// including Base/Load.
var LuaLibs = map[string]LGFunction{
	TabLibName:       OpenTable,
	IoLibName:        OpenIo,
	OsLibName:        OpenOs,
	StringLibName:    OpenString,
	MathLibName:      OpenMath,
	DebugLibName:     OpenDebug,
	ChannelLibName:   OpenChannel,
	CoroutineLibName: OpenCoroutine,
	BaseLibName:      OpenBase,
	LoadLibName:      OpenLoad,
}

// OpenLibs loads the built-in libraries. It is equivalent to running OpenLoad,
// then OpenBase, then iterating over the other OpenXXX functions in any order.
func (ls *LState) OpenLibs() {
	// NB: Map iteration order in Go is deliberately randomised, so must open Load/Base
	// prior to iterating.
	OpenLoad(ls)
	OpenBase(ls)
	for name, loader := range LuaLibs {
		if name == BaseLibName || name == LoadLibName {
			continue
		}
		ls.PreloadModule(name, loader)
		// TODO: Are all built-ins normally "required"
		ls.DoString(fmt.Sprintf(`%s = require "%s"`, name, name))
	}
}

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
)

// LuaLibs are the built-in Gopher-lua libraries as opened by LState.OpenLibs().
var LuaLibs = map[string]LGFunction{
	//	TabLibName:     OpenTable,
	IoLibName: OpenIo,
	//	OsLibName:      OpenOs,
	//	StringLibName:  OpenString,
	//	MathLibName:    OpenMath,
	//	DebugLibName:   OpenDebug,
	//	ChannelLibName: OpenChannel,
}

// OpenLibs loads the built-in libraries. It is equivalent to iterating over
// LuaLibs, preloading each, and requiring each to its own name.
func (ls *LState) OpenLibs() {
	// TODO: Remove when ready.
	ls.oldOpenLibs()

	for name, loader := range LuaLibs {
		ls.PreloadModule(name, loader)
		ls.DoString(fmt.Sprintf(`%s = require "%s"`, name, name))
	}
	/*/ loadlib must be loaded 1st
	loadOpen(ls)
	baseOpen(ls)
	coroutineOpen(ls)
	ioOpen(ls)
	stringOpen(ls)
	tableOpen(ls)
	mathOpen(ls)
	osOpen(ls)
	debugOpen(ls)
	channelOpen(ls) //*/
}

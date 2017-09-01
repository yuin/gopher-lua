package goldump

import (
	"github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/dump/gob"
	"testing"
)

const maxMemory = 40

var lua_scripts []string = []string{
	`
local function test_dump(a, b)
  return a + b
end
local new_func = loadstring(string.dump(test_dump))
assert(new_func(1,2) == test_dump(1,2))
`,
}

func testDumpScriptDir(t *testing.T, tests []string) {
	opt := lua.Options{
		RegistrySize:  1024 * 20,
		CallStackSize: 1024,
		DumpCodec:     gobdump.NewCodec(),
	}

	for _, script := range tests {
		LD := lua.NewState(opt)
		LD.SetMx(maxMemory)

		if fn, err := LD.LoadString(script); err != nil {
			t.Error(err)
		} else {
			LD.Push(fn)
			buf, err := LD.Options.DumpCodec.Encode(fn.Proto)
			if err != nil {
				t.Error(err)
			}

			L := lua.NewState(opt)
			L.SetMx(maxMemory)
			if err := L.DoString(string(buf)); err != nil {
				t.Error(err)
			}
			L.Close()
		}

		LD.Close()
	}
}

func TestDump(t *testing.T) {
	testDumpScriptDir(t, lua_scripts)
}

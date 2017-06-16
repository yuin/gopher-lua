package lua

import (
	"bytes"
	"fmt"
	"github.com/yuin/gopher-lua/parse"
	"os"
	"testing"
)

const maxMemory = 40

var gluaTests []string = []string{
	"base.lua",
	"coroutine.lua",
	"db.lua",
	"issues.lua",
	"os.lua",
	"table.lua",
	"vm.lua",
	"math.lua",
	"strings.lua",
}

var luaTests []string = []string{
	"attrib.lua",
	"calls.lua",
	"closure.lua",
	"constructs.lua",
	"events.lua",
	"literals.lua",
	"locals.lua",
	"math.lua",
	"sort.lua",
	"strings.lua",
	"vararg.lua",
	"pm.lua",
	"files.lua",
}

func testScriptCompile(t *testing.T, script string) {
	file, err := os.Open(script)
	if err != nil {
		t.Fatal(err)
		return
	}
	chunk, err2 := parse.Parse(file, script)
	if err2 != nil {
		t.Fatal(err2)
		return
	}
	parse.Dump(chunk)
	proto, err3 := Compile(chunk, script)
	if err3 != nil {
		t.Fatal(err3)
		return
	}
	proto.String()
}

func testScriptDir(t *testing.T, tests []string, directory string) {
	if err := os.Chdir(directory); err != nil {
		t.Error(err)
	}
	defer os.Chdir("..")
	for _, script := range tests {
		fmt.Printf("testing %s/%s\n", directory, script)
		testScriptCompile(t, script)
		L := NewState(Options{
			RegistrySize:  1024 * 20,
			CallStackSize: 1024,
		})
		L.SetMx(maxMemory)
		if err := L.DoFile(script); err != nil {
			t.Error(err)
		}
		L.Close()
	}
}

func testDumpScriptDir(t *testing.T, tests []string, directory string) {
	if err := os.Chdir(directory); err != nil {
		t.Error(err)
	}
	defer os.Chdir("..")
	for _, script := range tests {
		fmt.Printf("testing %s/%s\n", directory, script)
		LD := NewState(Options{
			RegistrySize:  1024 * 20,
			CallStackSize: 1024,
		})
		LD.SetMx(maxMemory)

		buf := new(bytes.Buffer)

		if fn, err := LD.LoadFile(script); err != nil {
			t.Error(err)
		} else {
			LD.Push(fn)
			LD.Dump(buf)
		}

		L := NewState(Options{
			RegistrySize:  1024 * 20,
			CallStackSize: 1024,
		})
		L.SetMx(maxMemory)
		if err := L.DoString(buf.String()); err != nil {
			t.Error(err)
		}
		L.Close()

		LD.Close()
	}
}


func TestGlua(t *testing.T) {
	defer os.Unsetenv("_____GLUATEST______")
	testScriptDir(t, gluaTests, "_glua-tests")
}

func TestLua(t *testing.T) {
	testScriptDir(t, luaTests, "_lua5.1-tests")
}

func TestDumpGlua(t *testing.T) {
	defer os.Unsetenv("_____GLUATEST______")
	testDumpScriptDir(t, gluaTests, "_glua-tests")
}

func TestDumpLua(t *testing.T) {
	testDumpScriptDir(t, luaTests, "_lua5.1-tests")
}

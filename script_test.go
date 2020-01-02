package lua

import (
	"fmt"
	"github.com/yuin/gopher-lua/parse"
	"math"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
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
	nop := func(s string) {}
	nop(proto.String())
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
			RegistrySize:        1024 * 20,
			CallStackSize:       1024,
			IncludeGoStackTrace: true,
		})
		L.SetMx(maxMemory)
		if err := L.DoFile(script); err != nil {
			t.Error(err)
		}
		L.Close()
	}
}

var numActiveUserDatas int32 = 0

type finalizerStub struct{ x byte }

func allocFinalizerUserData(L *LState) int {
	ud := L.NewUserData()
	atomic.AddInt32(&numActiveUserDatas, 1)
	a := finalizerStub{}
	ud.Value = &a
	runtime.SetFinalizer(&a, func(aa *finalizerStub) {
		atomic.AddInt32(&numActiveUserDatas, -1)
	})
	L.Push(ud)
	return 1
}

func sleep(L *LState) int {
	time.Sleep(time.Duration(L.CheckInt(1)) * time.Millisecond)
	return 0
}

func countFinalizers(L *LState) int {
	L.Push(LNumber(numActiveUserDatas))
	return 1
}

// TestLocalVarFree verifies that tables and user user datas which are no longer referenced by the lua script are
// correctly gc-ed. There was a bug in gopher lua where local vars were not being gc-ed in all circumstances.
func TestLocalVarFree(t *testing.T) {
	s := `
		function Test(a, b, c)
			local a = { v = allocFinalizer() }
			local b = { v = allocFinalizer() }
			return a
		end
		Test(1,2,3)
		for i = 1, 100 do
			collectgarbage()
			if countFinalizers() == 0 then
				return
			end
			sleep(100)
		end
		error("user datas not finalized after 100 gcs")
`
	L := NewState()
	L.SetGlobal("allocFinalizer", L.NewFunction(allocFinalizerUserData))
	L.SetGlobal("sleep", L.NewFunction(sleep))
	L.SetGlobal("countFinalizers", L.NewFunction(countFinalizers))
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestGlua(t *testing.T) {
	testScriptDir(t, gluaTests, "_glua-tests")
}

func TestLua(t *testing.T) {
	testScriptDir(t, luaTests, "_lua5.1-tests")
}

// TestTableAllocTracking will test the basic functionality of table tracking, the count of tables and their keys.
func TestTableAllocTracking(t *testing.T) {
	s := `
		a = {}
		for i = 1, 100 do
			a[i] = {}
		end
`
	l1 := NewState(Options{MaxTables: 102})
	defer l1.Close()
	if err := l1.DoString(s); err != nil {
		t.Error(err)
	}
	// should be 101 tables created, but a temp table is created by the DoString call, so 102 tables max
	// if we collect gc a few times, this temp table should be collected leaving us with just the 101 tables from the
	// script.
	for i := 0; i < 5 && l1.GetTableAllocInfo().GetTableCount() != 101; i++ {
		runtime.GC()
	}
	remaining := l1.GetTableAllocInfo().GetTableCount()
	if remaining != 101 {
		t.Error(fmt.Sprintf("expected 101 tables to remain, found %d", remaining))
	}

	// verify that with a small quota we hit a quota error
	l2 := NewState(Options{MaxTables: 100})
	defer l2.Close()
	if err := l2.DoString(s); err == nil {
		t.Error("expected table count quota error")
	}

	// verify that if we have a sufficiently high key limit, we don't have problems
	l3 := NewState(Options{MaxTotalTableKeys: 101})
	defer l3.Close()
	if err := l3.DoString(s); err != nil {
		t.Error(err)
	}

	// verify that if we limit the number of table keys we hit a quota error
	l4 := NewState(Options{MaxTotalTableKeys: 100})
	defer l4.Close()
	if err := l4.DoString(s); err == nil {
		t.Error("expected table key quota error")
	}
}

// TestTableAllocTrackingWithGC call functions which create table garbage, but invoke the gc to collect them. Total
// table count should remain low.
// It's not necessary to explicitly call the gc to keep the table count low, but calling it explicitly is the only way
// to ensure it is reliably called within the unit test, as other wise it is up to the go runtime when to call it.
func TestTableAllocTrackingWithGC(t *testing.T) {
	s := `
		function Test()
			local a = {}
		end
		for i = 1, 1000 do
			Test()
			collectgarbage()
		end
`
	l1 := NewState(Options{MaxTables: 50})
	defer l1.Close()
	if err := l1.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestTableTrackingWithNilKeys(t *testing.T) {
	createKeys := `
		a = {}
		-- set 100 keys, a mix of int and string keys
		for i = 1,50 do
			a[i] = 1
			a[tostring(i)] = 1
		end
`
	clearKeys := `
		-- clear them all again
		for i = 50,1,-1 do
			-- note, to free the key, we must use table.remove(), setting the array indexed key to nil does not work
			-- see notes
			table.remove(a,i)
			a[tostring(i)] = nil
		end
`
	// create an LState with some key tracking, but no explict limit
	l := NewState(Options{MaxTotalTableKeys: math.MaxInt32})
	defer l.Close()
	if err := l.DoString(createKeys); err != nil {
		t.Error(err)
	}
	var count int32
	count = l.GetTableAllocInfo().GetTableKeyCount()
	if count < 100 {
		t.Error(fmt.Sprintf("Expected at least 100 keys to be set, found %d", count))
	}
	if err := l.DoString(clearKeys); err != nil {
		t.Error(err)
	}
	// call the GC to get rid of the temp tables from the call to DoString
	for i := 0; i < 5 && l.GetTableAllocInfo().GetTableCount() != 1; i++ {
		runtime.GC()
	}
	count = l.GetTableAllocInfo().GetTableKeyCount()
	if count > 0 {
		t.Error(fmt.Sprintf("Expected no keys to be in use, found %d", count))
	}
}

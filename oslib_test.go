package lua

import "testing"

func TestOpenOsAllowList(t *testing.T) {
	L := NewState(Options{SkipOpenLibs: true})

	for _, pair := range []struct {
		n string
		f LGFunction
	}{
		{LoadLibName, OpenPackage}, // Must be first
		{OsLibName, OpenOsAllowlist("time")},
	} {
		if err := L.CallByParam(P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, LString(pair.n)); err != nil {
			t.Fatalf("unable to load libs: %v", err)
		}
	}

	s := `
		os.time()
	`
	// Shoult NOT error out
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}

	s = `
		os.execute('echo "hello"')
	`
	// Should error out
	if err := L.DoString(s); err == nil {
		t.Errorf("able to run blocked function")
	}
}

package lua

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompileUpvalueLimit(t *testing.T) {
	tests := []struct {
		upvalues  int
		wantError bool
	}{
		{254, false},
		{255, false},
		{256, true},
		{300, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.upvalues), func(t *testing.T) {
			// Split locals between two enclosing functions so the leaf closure
			// reaches the upvalue limit without reaching the local-variable limit.
			var source strings.Builder
			source.WriteString("local function outer()\n")
			for i := 0; i < tt.upvalues/2; i++ {
				fmt.Fprintf(&source, "local v%d = 1\n", i)
			}
			source.WriteString("return function()\n")
			for i := tt.upvalues / 2; i < tt.upvalues; i++ {
				fmt.Fprintf(&source, "local v%d = 1\n", i)
			}
			source.WriteString("return function()\nreturn ")
			for i := 0; i < tt.upvalues; i++ {
				if i > 0 {
					source.WriteString(" + ")
				}
				fmt.Fprintf(&source, "v%d", i)
			}
			source.WriteString("\nend\nend\nend\nreturn outer()()()\n")

			L := NewState()
			defer L.Close()
			fn, err := L.LoadString(source.String())
			if tt.wantError {
				if err == nil {
					t.Fatal("LoadString accepted too many upvalues")
				}
				apiErr, ok := err.(*ApiError)
				if !ok || apiErr.Type != ApiErrorSyntax {
					t.Fatalf("LoadString error = %v (%T), want ApiErrorSyntax", err, err)
				}
				if !strings.Contains(err.Error(), "too many upvalues") {
					t.Fatalf("LoadString error = %q, want too many upvalues", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadString failed: %v", err)
			}
			if err := L.CallByParam(P{Fn: fn, NRet: 1, Protect: true}); err != nil {
				t.Fatalf("closure execution failed: %v", err)
			}
			if got := L.Get(-1); got != LNumber(tt.upvalues) {
				t.Fatalf("closure returned %v, want %d", got, tt.upvalues)
			}
		})
	}
}

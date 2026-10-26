package lua

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/gopher-lua/parse"
)

func TestCompileRegisterLimitLocations(t *testing.T) {
	locals := func(count int, initializer string) string {
		var source strings.Builder
		for i := 0; i < count; i++ {
			fmt.Fprintf(&source, "local v%d%s\n", i, initializer)
		}
		return source.String()
	}
	call := func(arguments int) string {
		return "consume(" + strings.TrimSuffix(strings.Repeat("1,", arguments), ",") + ")\n"
	}
	const mainPrefix = "-- main chunk\n"
	const nestedPrefix = "-- nested function\n\n\n\nlocal function oversized()\n"
	const localError = "too many local variables"
	const registerError = "register overflow(too many local variables)"
	locals195 := mainPrefix + locals(195, " = 1")
	locals200 := mainPrefix + locals(200, " = 1")
	multilineCall := "consume(\n" + strings.Repeat("1,\n", 19) + "1\n)\n"

	tests := []struct {
		name        string
		source      string
		wantLine    int
		wantMessage string
	}{
		{
			name:        "initialized_locals_in_main",
			source:      mainPrefix + locals(210, " = 1"),
			wantLine:    202,
			wantMessage: localError,
		},
		{
			name:        "initialized_locals_in_nested_function",
			source:      nestedPrefix + locals(210, " = 1") + "end\n",
			wantLine:    206,
			wantMessage: localError,
		},
		{
			name:        "uninitialized_locals_in_main",
			source:      mainPrefix + locals(210, ""),
			wantLine:    202,
			wantMessage: localError,
		},
		{
			name:        "uninitialized_locals_in_nested_function",
			source:      nestedPrefix + locals(210, "") + "end\n",
			wantLine:    206,
			wantMessage: localError,
		},
		{
			name:        "multiline_initializer_uses_declaration_start",
			source:      locals200 + "local overflow =\n1\n",
			wantLine:    202,
			wantMessage: localError,
		},
		{
			name:        "local_function_declaration",
			source:      locals200 + "local function overflow()\nend\n",
			wantLine:    202,
			wantMessage: localError,
		},
		{
			name:        "call_arguments_on_one_line",
			source:      locals195 + call(20),
			wantLine:    197,
			wantMessage: registerError,
		},
		{
			name:   "multiline_call_uses_first_overflowing_argument",
			source: locals195 + multilineCall,
			// The function uses register 195; the fifth argument uses register
			// 200, the first beyond the 200-register limit, on source line 202.
			wantLine:    202,
			wantMessage: registerError,
		},
		{
			name:        "later_larger_call_keeps_first_overflow_location",
			source:      locals195 + call(20) + "\n" + call(40),
			wantLine:    197,
			wantMessage: registerError,
		},
		{
			name:   "accepted_200_locals",
			source: locals200 + "return v199\n",
		},
		{
			name:   "accepted_call_below_register_limit",
			source: locals195 + call(4),
		},
		{
			name: "accepted_locals_reused_after_scope_exit",
			source: "do\n" + locals(200, " = 1") + "end\n" +
				"do\n" + locals(200, " = 1") + "end\nreturn 1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.name + ".lua"
			chunk, err := parse.Parse(strings.NewReader(tt.source), name)
			if err != nil {
				t.Fatalf("Parse failed before compilation: %v", err)
			}
			proto, err := Compile(chunk, name)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("Compile rejected an accepted register count: %v", err)
				}
				if proto == nil {
					t.Fatal("Compile returned no function prototype")
				}
				return
			}
			if err == nil {
				t.Fatal("Compile accepted an overflowing register count")
			}
			compileErr, ok := err.(*CompileError)
			if !ok {
				t.Fatalf("Compile error = %v (%T), want *CompileError", err, err)
			}
			if compileErr.Message != tt.wantMessage {
				t.Errorf("Compile error message = %q, want %q", compileErr.Message, tt.wantMessage)
			}
			if compileErr.Line != tt.wantLine {
				t.Errorf("Compile error line = %d, want %d", compileErr.Line, tt.wantLine)
			}
		})
	}
}

package lua

import (
	"context"
	"testing"
	"time"
)

func TestGenericForDeadBranch(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []LValue
	}{
		{
			name:   "return after loop",
			script: `for _, v in ipairs({1}) do if false then local unused end end return 42`,
			want:   []LValue{LNumber(42)},
		},
		{
			name:   "accumulate after branch",
			script: `local n = 0 for _, v in ipairs({1, 2, 3}) do if false then local unused end n = n + v end return n`,
			want:   []LValue{LNumber(6)},
		},
		{
			name:   "preceding numeric loop",
			script: `for i = 1, 1 do end for _, v in ipairs({1}) do if false then local unused end end return 42`,
			want:   []LValue{LNumber(42)},
		},
		{
			name:   "preserve outer locals",
			script: `local a, b = 17, 23 for _, v in ipairs({1}) do if false then local unused end end return a, b`,
			want:   []LValue{LNumber(17), LNumber(23)},
		},
		{
			name:   "dead break",
			script: `local n = 0 for _, v in ipairs({1, 2, 3}) do if false then break end n = n + v end return n`,
			want:   []LValue{LNumber(6)},
		},
		{
			name:   "dead nested loop",
			script: `for _, v in ipairs({1}) do if false then for k in pairs({}) do end end end return 42`,
			want:   []LValue{LNumber(42)},
		},
		{
			name:   "nil condition",
			script: `local n = 0 for _, v in ipairs({1, 2, 3}) do if nil then local unused end n = n + v end return n`,
			want:   []LValue{LNumber(6)},
		},
		{
			name:   "runtime false condition",
			script: `local n = 0 for _, v in ipairs({1, 2, 3}) do if v == 9 then local unused end n = n + v end return n`,
			want:   []LValue{LNumber(6)},
		},
		{
			name:   "numeric loop",
			script: `local n = 0 for v = 1, 3 do if false then local unused end n = n + v end return n`,
			want:   []LValue{LNumber(6)},
		},
		{
			name:   "nested conditions",
			script: `local n = 0 for _, v in ipairs({1, 2, 3}) do if true then if false then local unused end n = n + v end end return n`,
			want:   []LValue{LNumber(6)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			L := NewState()
			defer L.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			L.SetContext(ctx)
			if err := L.DoString(tt.script); err != nil {
				t.Fatal(err)
			}
			if got := L.GetTop(); got != len(tt.want) {
				t.Fatalf("returned %d values, want %d", got, len(tt.want))
			}
			for i, want := range tt.want {
				if got := L.Get(i + 1); got != want {
					t.Errorf("return value %d = %v, want %v", i+1, got, want)
				}
			}
		})
	}
}

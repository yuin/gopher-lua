package lua

import (
	"context"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	L := NewState()
	defer L.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	doneCh := make(chan int, 1)

	go func() {
		defer close(doneCh)
		errorIfScriptFail(t, L, `f, err = io.popen('cat', 'w'); assert(not err, err); f:write("hello"); f:close()`)
	}()

	select {
	case <-doneCh:
		//	Success
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			t.Error(err)
		}
	}
}

func TestReader(t *testing.T) {
	L := NewState()
	defer L.Close()

	t.Run("should pass", func(t *testing.T) {
		errorIfScriptFail(t, L, `
			f, err = io.popen('echo "foo"', 'r')
			assert(not err, err)
			data, err = f:read('*a')
			assert(not err, err)
			rc = f:close()
			assert(rc == 0, tostring(rc))
			assert(data == "foo\n", data)
		`)
	})

	t.Run("should fail", func(t *testing.T) {
		errorIfScriptFail(t, L, `
			f, err = io.popen('false')
			assert(not err, err)
			rc = f:close()
			assert(rc ~= 0, tostring(rc))
		`)
	})
}

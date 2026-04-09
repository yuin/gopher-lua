package lua

import (
	"testing"
)

// correctly gc-ed. There was a bug in gopher lua where local vars were not being gc-ed in all circumstances.
func TestOsWrite(t *testing.T) {
	s := `
		local function write(filename, content)
		local f = assert(io.open(filename, "w"))
		  f:write(content)
		  assert(f:close())
		end

		local filename = os.tmpname()
		write(filename, "abc")
		write(filename, "d")
		local f = assert(io.open(filename, "r"))
		local content = f:read("*all"):gsub("%s+", "")
		f:close()
		os.remove(filename)
		local expected = "d"
		if content ~= expected then
			error(string.format("Invalid content: Expecting \"%s\", got \"%s\"", expected, content))
		end
`
	L := NewState()
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestOsDateFmt(t *testing.T) {
	s := "return os.date('!weekday=%w|%a|%A, month=%b|%B, year=%y, time=%I:%M|%H:%M:%S|%X, date=%Y-%m-%d|%x', 1136214245)"
	L := NewState()
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	} else {
		ret := L.Get(-1)
		expected := LString("weekday=1|mon|Monday, month=Jan|January, year=06, time=03:04|15:04:05|15:04:05, date=2006-01-02|01/02/06")
		errorIfNotEqual(t, expected, ret)
	}
}

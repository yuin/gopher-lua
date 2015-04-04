package lua

import (
	"io/ioutil"
	"os"
	"strings"
	"time"
)

var startedAt time.Time

func init() {
	startedAt = time.Now()
}

func getIntField(L *LState, tb *LTable, key string, v int) int {
	ret := tb.RawGetH(LString(key))
	if ln, ok := ret.(LNumber); ok {
		return int(ln)
	}
	return v
}

func getBoolField(L *LState, tb *LTable, key string, v bool) bool {
	ret := tb.RawGetH(LString(key))
	if lb, ok := ret.(LBool); ok {
		return bool(lb)
	}
	return v
}

func osOpen(L *LState) {
	L.RegisterModule("os", osFuncs)
}

var osFuncs = map[string]LGFunction{
	"clock":     osClock,
	"difftime":  osDiffTime,
	"execute":   osExecute,
	"exit":      osExit,
	"date":      osDate,
	"getenv":    osGetEnv,
	"remove":    osRemove,
	"rename":    osRename,
	"setenv":    osSetEnv,
	"setlocale": osSetLocale,
	"time":      osTime,
	"tmpname":   osTmpname,
}

func osClock(L *LState) int {
	L.Push(LNumber(float64(time.Now().Sub(startedAt)) / float64(time.Second)))
	return 1
}

func osDiffTime(L *LState) int {
	L.Push(LNumber(L.CheckInt64(1) - L.CheckInt64(2)))
	return 1
}

func osExecute(L *LState) int {
	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	cmd, args := popenArgs(L.CheckString(1))
	process, err := os.StartProcess(cmd, args, &procAttr)
	if err != nil {
		L.Push(LNumber(1))
		return 1
	}

	_, err = process.Wait()
	if err != nil {
		L.Push(LNumber(1))
		return 1
	}
	L.Push(LNumber(0))
	return 0
}

func osExit(L *LState) int {
	os.Exit(L.OptInt(1, 0))
	return 1
}

func osDate(L *LState) int {
	t := time.Now()
	cfmt := "%c"
	if L.GetTop() >= 1 {
		cfmt = L.CheckString(1)
		if strings.HasPrefix(cfmt, "!") {
			t = time.Now().UTC()
			cfmt = strings.TrimLeft(cfmt, "!")
		}
		if L.GetTop() >= 2 {
			t = time.Unix(L.CheckInt64(2), 0)
		}
		if strings.HasPrefix(cfmt, "*t") {
			ret := L.NewTable()
			ret.RawSetH(LString("year"), LNumber(t.Year()))
			ret.RawSetH(LString("month"), LNumber(t.Month()))
			ret.RawSetH(LString("day"), LNumber(t.Day()))
			ret.RawSetH(LString("hour"), LNumber(t.Hour()))
			ret.RawSetH(LString("min"), LNumber(t.Minute()))
			ret.RawSetH(LString("sec"), LNumber(t.Second()))
			ret.RawSetH(LString("wday"), LNumber(t.Weekday()))
			// TODO yday & dst
			ret.RawSetH(LString("yday"), LNumber(0))
			ret.RawSetH(LString("isdst"), LFalse)
			L.Push(ret)
			return 1
		}
	}
	L.Push(LString(strftime(t, cfmt)))
	return 1
}

func osGetEnv(L *LState) int {
	v := os.Getenv(L.CheckString(1))
	if len(v) == 0 {
		L.Push(LNil)
	} else {
		L.Push(LString(v))
	}
	return 1
}

func osRemove(L *LState) int {
	err := os.Remove(L.CheckString(1))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

func osRename(L *LState) int {
	err := os.Rename(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

func osSetLocale(L *LState) int {
	// setlocale is not supported
	L.Push(LFalse)
	return 1
}

func osSetEnv(L *LState) int {
	err := os.Setenv(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

func osTime(L *LState) int {
	if L.GetTop() == 0 {
		L.Push(LNumber(time.Now().Unix()))
	} else {
		tbl := L.CheckTable(1)
		sec := getIntField(L, tbl, "sec", 0)
		min := getIntField(L, tbl, "min", 0)
		hour := getIntField(L, tbl, "hour", 12)
		day := getIntField(L, tbl, "day", -1)
		month := getIntField(L, tbl, "month", -1)
		year := getIntField(L, tbl, "year", -1)
		isdst := getBoolField(L, tbl, "isdst", false)
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
		// TODO dst
		if false {
			print(isdst)
		}
		L.Push(LNumber(t.Unix()))
	}
	return 1
}

func osTmpname(L *LState) int {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		L.RaiseError("unable to generate a unique filename")
	}
	file.Close()
	os.Remove(file.Name()) // ignore errors
	L.Push(LString(file.Name()))
	return 1
}

//

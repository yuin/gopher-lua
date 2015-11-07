package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
	"os"
	"runtime/pprof"
	"strings"
)

func main() {
	os.Exit(mainAux())
}

func mainAux() int {
	var opt_e, opt_l, opt_p string
	var opt_i, opt_v, opt_dt, opt_dc bool
	var opt_m int
	flag.StringVar(&opt_e, "e", "", "")
	flag.StringVar(&opt_l, "l", "", "")
	flag.StringVar(&opt_p, "p", "", "")
	flag.IntVar(&opt_m, "mx", 0, "")
	flag.BoolVar(&opt_i, "i", false, "")
	flag.BoolVar(&opt_v, "v", false, "")
	flag.BoolVar(&opt_dt, "dt", false, "")
	flag.BoolVar(&opt_dc, "dc", false, "")
	flag.Usage = func() {
		fmt.Println(`Usage: glua [options] [script [args]].
Available options are:
  -e stat  execute string 'stat'
  -l name  require library 'name'
  -mx MB   memory limit(default: unlimited)
  -dt      dump AST trees
  -dc      dump VM codes
  -i       enter interactive mode after executing 'script'
  -p file  write cpu profiles to the file
  -v       show version information
`)
	}
	flag.Parse()
	if len(opt_p) != 0 {
		f, err := os.Create(opt_p)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if len(opt_e) == 0 && !opt_i && !opt_v && flag.NArg() == 0 {
		opt_i = true
	}

	status := 0

	L := lua.NewState()
	defer L.Close()
	if opt_m > 0 {
		L.SetMx(opt_m)
	}

	if opt_v || opt_i {
		fmt.Println(lua.PackageCopyRight)
	}

	if len(opt_l) > 0 {
		if err := L.DoFile(opt_l); err != nil {
			fmt.Println(err.Error())
		}
	}

	if nargs := flag.NArg(); nargs > 0 {
		script := flag.Arg(0)
		argtb := L.NewTable()
		for i := 1; i < nargs; i++ {
			L.RawSet(argtb, lua.LNumber(i), lua.LString(flag.Arg(i)))
		}
		L.SetGlobal("arg", argtb)
		if opt_dt || opt_dc {
			file, err := os.Open(script)
			if err != nil {
				fmt.Println(err.Error())
				return 1
			}
			chunk, err2 := parse.Parse(file, script)
			if err2 != nil {
				fmt.Println(err2.Error())
				return 1
			}
			if opt_dt {
				fmt.Println(parse.Dump(chunk))
			}
			if opt_dc {
				proto, err3 := lua.Compile(chunk, script)
				if err3 != nil {
					fmt.Println(err3.Error())
					return 1
				}
				fmt.Println(proto.String())
			}
		}
		if err := L.DoFile(script); err != nil {
			fmt.Println(err.Error())
			status = 1
		}
	}

	if len(opt_e) > 0 {
		if err := L.DoString(opt_e); err != nil {
			fmt.Println(err.Error())
			status = 1
		}
	}

	if opt_i {
		doREPL(L)
	}
	return status
}

// do read/eval/print/loop
func doREPL(L *lua.LState) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		str, ok := loadline(reader, L)
		if !ok {
			break
		}
		if err := L.DoString(str); err != nil {
			fmt.Println(err.Error())
		}
	}
}

func incomplete(err error) bool {
	if strings.Index(err.Error(), "EOF") != -1 {
		return true
	}
	return false
}

func loadline(reader *bufio.Reader, L *lua.LState) (string, bool) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", false
	}

	// try add return
	if _, err := L.LoadString("return " + line); err == nil { // syntax ok
		return line, true
	} else { // syntax error
		return multiline(line, reader, L)
	}
}

func multiline(ml string, reader *bufio.Reader, L *lua.LState) (string, bool) {
	for {
		// try it
		if _, err := L.LoadString(ml); err == nil { // syntax ok
			return ml, true
		} else if !incomplete(err) { // syntax error
			return ml, true
		}

		fmt.Print(">> ")
		if line, err := reader.ReadString('\n'); err != nil {
			return "", false
		} else {
			ml = ml + "\n" + line
		}
	}
}

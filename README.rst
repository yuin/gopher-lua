===============================================================================
GopherLua: VM and compiler for Lua in Go.
===============================================================================
GopherLua is a Lua5.1 VM and compiler written in Go. GopherLua has a same goal
with Lua: **Be a scripting language with extensible semantics** . It provides a
Go APIs that allow you to easily embed a scripting language to your Go host 
programs.

----------------------------------------------------------------
Installation
----------------------------------------------------------------

.. code-block:: bash
   
   go get github.com/yuin/gopher-lua

----------------------------------------------------------------
Usage
----------------------------------------------------------------
GopherLua APIs perform in much the same way as Lua, **but the stack is used only 
for passing arguments and receiving returned values.**

Import a package.

.. code-block:: go
   
   import (
       "github.com/yuin/gopher-lua"
   )

Run scripts in the VM.

.. code-block:: go
   
   L := lua.NewState()
   defer L.Close()
   if err := L.DoString(`print("hello")`); err != nil {
       panic(err)
   }

.. code-block:: go

   L := lua.NewState()
   defer L.Close()
   if err := L.DoFile("hello.lua"); err != nil {
       panic(err)
   }

Refer to `Lua Reference Manual <http://www.lua.org/manual/5.1/>`_ and `Go doc <http://godoc.org/github.com/yuin/gopher-lua>`_ for further information.

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Data model
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
All data in a GopherLua program is a ``LValue`` . ``LValue`` is an interface 
type that has following methods.

- ``String() string``
- ``Type() LValueType``


Objects implement a LValue interface are

================ ========================= ================== =======================
 Type name        Go type                   Type() value       Constants
================ ========================= ================== =======================
 ``LNilType``      (constants)              ``LTNil``          ``LNil``
 ``LBool``         (constants)              ``LTBool``         ``LTrue``, ``LFalse``
 ``LNumber``        float64                 ``LTNumber``       ``-``
 ``LString``        string                  ``LTString``       ``-``
 ``LFunction``      struct pointer          ``LTFunction``     ``-``
 ``LUserData``      struct pointer          ``LTUserData``     ``-``
 ``LState``         struct pointer          ``LTThread``       ``-``
 ``LTable``         struct pointer          ``LTTable``        ``-``
================ ========================= ================== =======================

You can test an object type in Go way(type assertion) or using a ``Type()`` value.

.. code-block:: go

   lv := L.Get(-1) // get the value at the top of the stack
   if str, ok := lv.(lua.LString); ok {
       // lv is LString
       fmt.Println(string(str))
   }
   if lv.Type() != lua.LTString {
       panic("string required.")
   }

.. code-block:: go

   lv := L.Get(-1) // get the value at the top of the stack
   if tbl, ok := lv.(*lua.LTable); ok {
       // lv is LTable
       fmt.Println(L.ObjLen(tbl))
   }

Note that ``LBool`` , ``LNumber`` , ``LString`` is not a pointer.

To test ``LNilType`` and ``LBool``, You **must** use pre-defined constants.

.. code-block:: go

   lv := L.Get(-1) // get the value at the top of the stack
   
   if lv == LTrue { // correct
   }
   
   if bl, ok == lv.(lua.LBool); ok && bool(bl) { // wrong
   }

In Lua, both ``nil`` and ``false`` make a condition false. ``LVIsFalse`` and ``LVAsBool`` implement this specification.

.. code-block:: go

   lv := L.Get(-1) // get the value at the top of the stack
   if LVIsFalse(lv) { // lv is nil or false
   }
   
   if LVAsBool(lv) { // lv is neither nil nor false
   }

Objects that based on go structs(``LFunction``. ``LUserData``, ``LTable``)
have some public methods and fields. You can use these methods and fields for 
performance and debugging, but there are some limitations.

- Metatable does not work.
- No error handlings.

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
API
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Refer to `Lua Reference Manual <http://www.lua.org/manual/5.1/>`_ and `Go doc(LState methods) <http://godoc.org/github.com/yuin/gopher-lua>`_ for further information.

+++++++++++++++++++++++++++++++++++++++++
Calling Go from Lua
+++++++++++++++++++++++++++++++++++++++++

.. code-block:: go

   func Double(L lua.LState) int {
       lv := L.ToInt(1)             /* get argument */
       L.Push(lua.LNumber(lv * 2)) /* push result */
       return 1                     /* number of results */
   }
   
   func main() {
       L := lua.NewState()
       defer L.Close()
       L.SetGlobal("double", L.NewFunction(Double)) /* Original lua_setglobal uses stack... */
   }

.. code-block:: lua

   print(double(20)) -- > "40"

Any function registered with GopherLua is a ``lua.LGFunction``, defined in ``value.go``

.. code-block:: go

   type LGFunction func(*LState) int

Working with coroutines.

.. code-block:: go

   co := L.NewThread() /* create a new thread */
   fn := L.GetGlobal("coro").(*lua.LFunction) /* get function from lua */
   for {
       st, err, values := L.Resume(co, fn)
       if st == lua.ResumeError {
           println("yield break(error)")
           println(err.Error())
           break
       }
    
       for i, lv := range values {
           fmt.Printf("%v : %v\n", i, lv)
       }
    
       if st == lua.ResumeOK {
           println("yield break(ok)")
           break
       }
   }

+++++++++++++++++++++++++++++++++++++++++
Calling Lua from Go
+++++++++++++++++++++++++++++++++++++++++

.. code-block:: go

   L := lua.NewState()
   defer L.Close()
   if err := L.DoFile("double.lua"); err != nil {
       panic(err)
   }
   if err := L.CallByParam(lua.P{
       Fn: L.GetGlobal("double"),
       NRet: 1,
       Protect: true,
       }, lua.LNumber(10)); err != nil {
       panic(err)
   }
   ret := L.Get(-1) // returned value
   L.Pop(1)  // remove received value

If ``Protect`` is false, GopherLua will panic instead of returning an ``error`` value.


----------------------------------------------------------------
Differences between Lua and GopherLua
----------------------------------------------------------------
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Pattern match
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

- GopherLua uses the regexp package to implement the pattern match.
    - The Pattern match only works for utf8 strings.
    - The regexp package does not support back-references.
    - The regexp package does not support position-captures.

GopherLua has an option to use the Go regexp syntax as a pattern match format.

.. code-block:: go

   L := lua.NewState()
   defer L.Close()
   L.LuaRegex = false

.. code-block:: lua

   print(string.gsub("abc $!?", [[a(\w+)]], "${1}")) --> bc $!?

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Unsupported functions
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

- ``string.dump`` 
- ``os.setlocale``
- ``collectgarbage``
- ``lua_Debug.namewhat``
- debug hooks

~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Miscellaneous notes
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

- ``file:setvbuf`` does not support a line bufferring.

----------------------------------------------------------------
Standalone interpreter
----------------------------------------------------------------
Lua has an interpreter called ``lua`` . GopherLua has an interpreter called ``glua`` .

.. code-block:: bash

   go get github.com/yuin/gopher-lua/cmd/glua

``glua`` has same options as ``lua`` .

----------------------------------------------------------------
License
----------------------------------------------------------------
MIT

----------------------------------------------------------------
Author
----------------------------------------------------------------
Yusuke Inuzuka

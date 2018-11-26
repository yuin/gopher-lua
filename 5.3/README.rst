===============================================================================
GopherLua for Lua5.3 branch
===============================================================================
This branch is a pre-release version of GopherLua that supports Lua5.3 specs.

Tasks
=====================
- [x] lexer and parser
    - Now GopherLua can parse goto statements, new operators(e.g. ``>>`` ``//``)
- compiler
    - [x] goto statements: almost done(more testing needed)
    - [ ] new operators
    - [ ] integers
    - [ ] environment ( ``_ENV`` )
- vm
    - [ ] new operators
    - [ ] new integers
    - [ ] environment ( ``_ENV`` )
    - [ ] continuation ( ``luaK_Context`` )
- API
    - [ ] remove LIA_GLOBALSINDEX
    - [ ] removed functions: luaL_typeerror, lua_cpcall, lua_equal, lua_lessthan
    - [ ] renamed functions: lua_objlen to lua_rawlen
    - [ ] lua_compare
    - [ ] lua_load now takes a mode argument
    - [ ] lua_resume now takes a from argument
- libraries
    - [ ] removed functions : module, setfenv, getfenv, math.log10, loadstring, table.maxn, math.atan2, math.cosh, math.sinh, math.tanh, math.pow, math.frexp,  math.ldexp
    - [ ] renamed functions : unpack to table.unpack, package.loaders to package.searchers
    - [ ] os.execute now returns true when command terminates successfully and nil plus error information otherwise. 
    - [ ] io.read do not have a starting '*' anymore


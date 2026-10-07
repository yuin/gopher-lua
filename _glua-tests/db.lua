-- debug lib tests
-- debug stuff are  partially implemented; hooks are not supported.

local function f1()
end
local env = {}
local mt = {}
debug.setfenv(f1, env)
assert(debug.getfenv(f1) == env)
debug.setmetatable(f1, mt)
assert(debug.getmetatable(f1) == mt)

local function f2()
  local info = debug.getinfo(1, "Slunf")
  assert(info.currentline == 14)
  assert(info.linedefined == 13)
  assert(info.func == f2)
  assert(info.lastlinedefined == 25)
  assert(info.nups == 1)
  assert(info.name == "f2")
  assert(info.what == "Lua")
  assert(info.source == "@db.lua")
  assert(info.short_src == "db.lua")
  assert(debug.getinfo(2, "S").short_src == "db.lua")
end
f2()

local function f3()
end
local info = debug.getinfo(f3)
assert(info.currentline == -1)
assert(info.linedefined == 28)
assert(info.func == f3)
assert(info.lastlinedefined == 29)
assert(info.nups == 0)
assert(info.name == nil)
assert(info.what == "Lua")
assert(info.source == "@db.lua")
assert(info.short_src == "db.lua")

-- source and short_src as in Lua 5.1 (#547)
local s = debug.getinfo(print, "S")
assert(s.source == "=[C]" and s.short_src == "[C]")
s = loadstring("return debug.getinfo(1, 'S')", "=custom")()
assert(s.source == "=custom" and s.short_src == "custom")
s = loadstring("return debug.getinfo(1, 'S')", "@virtual.lua")()
assert(s.source == "@virtual.lua" and s.short_src == "virtual.lua")
s = loadstring("return debug.getinfo(1, 'S')", "chunk\nsecond line")()
assert(s.short_src == '[string "chunk..."]')

local function f4()
  local a,b,c = 1,2,3
  local function f5()
    local name, value = debug.getlocal(2, 2)
    assert(debug.getlocal(2, 10) == nil)
    assert(name == "b")
    assert(value == 2)
    name = debug.setlocal(2, 2, 10)
    assert(debug.setlocal(2, 10, 10) == nil)
    assert(name == "b")

    local d = a
    local e = c

    local tb = debug.traceback("--msg--")
    assert(string.find(tb, "\\-\\-msg\\-\\-"))
    assert(string.find(tb, "in.*f5"))
    assert(string.find(tb, "in.*f4"))
  end
  f5()
  local name, value = debug.getupvalue(f5, 1)
  assert(debug.getupvalue(f5, 10) == nil)
  assert(name == "a")
  assert(value == 1)
  name = debug.setupvalue(f5, 1, 11)
  assert(debug.setupvalue(f5, 10, 11) == nil)
  assert(name == "a")
  assert(a == 11)

  assert(b == 10) -- changed by debug.setlocal in f4
end
f4()

local ok, msg = pcall(function()
  debug.getlocal(10, 1)
end)
assert(not ok and string.find(msg, "level out of range"))

local ok, msg = pcall(function()
  debug.setlocal(10, 1, 1)
end)
assert(not ok and string.find(msg, "level out of range"))

assert(debug.getinfo(100) == nil)
assert(debug.getinfo(1, "a") == nil)

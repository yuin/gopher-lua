co = coroutine.wrap(function()
  co()
end)

local ok, msg = pcall(function()
  co()
end)
assert(not ok and string.find(msg, "can not resume a running thread"))

co = coroutine.wrap(function()
  return 1
end)
assert(co() == 1)
local ok, msg = pcall(function()
  co()
end)
assert(not ok and string.find(msg, "can not resume a dead thread"))

-- yield across a Go call boundary (pcall, a Go function calling Lua) is an
-- error, as in Lua 5.1; it used to crash with a nil pointer dereference (#545)
local co = coroutine.create(function()
  local ok, err = pcall(function() coroutine.yield(1) end)
  return ok, err
end)
local ok, a, b = coroutine.resume(co)
assert(ok and a == false and string.find(b, "attempt to yield across metamethod/C-call boundary", 1, true))
assert(coroutine.status(co) == "dead")

co = coroutine.create(function()
  return pcall(table.sort, {3, 1, 2}, function(x, y) coroutine.yield(); return x < y end)
end)
ok, a, b = coroutine.resume(co)
assert(ok and a == false and string.find(b, "attempt to yield across", 1, true))

co = coroutine.create(function()
  pcall(function() coroutine.yield() end)
  coroutine.yield("again")
  return "done"
end)
assert(select(2, coroutine.resume(co)) == "again")
assert(select(2, coroutine.resume(co)) == "done")

local gen = coroutine.wrap(function() coroutine.yield(1); return 2 end)
assert(gen() == 1 and gen() == 2)

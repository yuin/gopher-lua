local ok, msg = pcall(function()
  dofile("notexist")
end)
assert(not ok and string.find(msg, ".*notexist.*"))

local ok, msg = pcall(function()
  assert(getfenv(2) == _G)
end)
assert(ok)

local i = 1
local fn = assert(load(function()
  local tbl = {"return ", "1", "+", "1"}
  local v = tbl[i]
  i = i + 1
  return v
end))
assert(fn() == 2)

local fn, msg = load(function()
    return {}
end)
assert(not fn and string.find(msg, "must return a string"))

local i = 1
local fn, msg = load(function()
  if i == 1 then
      i = i + 1
      return "returna"
  end
end)
assert(not fn and string.find(string.lower(msg), "eof"))

local ok, a, b = xpcall(function()
     return "a", "b"
  end, 
  function(err)
     assert(nil)
  end)
assert(ok and a == "a" and b == "b")

local ok, a, b = xpcall(function()
     error("error!")
  end, 
  function(err)
     return err .. "!", "b"
  end)
assert(not ok and string.find(a, "error!!") and b == nil)

-- numbers convert to text with the reference Lua 5.1 "%.14g" rendering
assert(tostring(100) == "100")
assert(tostring(0.1) == "0.1")
assert(tostring(0.1 + 0.2) == "0.3")
assert(tostring(1e-10) == "1e-10")
assert(tostring(1e15) == "1e+15")
assert(tostring(2^53) == "9.007199254741e+15")
assert(tostring(123456789012345678) == "1.2345678901235e+17")
assert(tostring(1/0) == "inf")
assert(tostring(-1/0) == "-inf")
local nan = 0/0
local s1, s2 = tostring(nan), tostring(-nan)
assert(s1 == "nan" or s1 == "-nan")
assert(s2 == "nan" or s2 == "-nan")
assert(s1 ~= s2)

-- negative zero renders with its sign
local zero = 0.0
local negzero = -zero
assert(tostring(zero) == "0")
assert(tostring(negzero) == "-0")
assert(tostring(negzero * 1) == "-0")

-- the same rendering applies to concatenation and table.concat
assert((1e15 .. "") == "1e+15")
assert((2^53 .. "x") == "9.007199254741e+15x")
assert((1.5 .. "") == "1.5")
assert(("" .. negzero) == "-0")
assert(table.concat({1e15, 2^53}, ",") == "1e+15,9.007199254741e+15")

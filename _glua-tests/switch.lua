-- tests for the 'switch' statement and 'continue' (gopher-lua extension)

local function errmsg(code, m)
  local st, msg = loadstring(code)
  assert(not st and string.find(msg, m), msg)
end

-- the basic form, with fallthrough and else
local function run(v)
  local out = {}
  local function add(s) out[#out + 1] = s end
  switch v do
    case "a" then
      add("A")
      break
    case "b", "c" then
      add("BC")
    case "d" then
      add("D")
      break
    else
      add("other")
  end
  return table.concat(out, ",")
end
assert(run("a") == "A")
assert(run("b") == "BC,D")
assert(run("c") == "BC,D")
assert(run("d") == "D")
assert(run("zzz") == "other")

-- else is never reached by falling off the last case
local function last(v)
  local out = {}
  switch v do
    case 1 then out[#out + 1] = "one"
    case 2 then out[#out + 1] = "two"
    else out[#out + 1] = "else"
  end
  return table.concat(out, ",")
end
assert(last(1) == "one,two")
assert(last(2) == "two")
assert(last(3) == "else")

-- the value is evaluated exactly once
do
  local calls = 0
  local function v() calls = calls + 1; return 2 end
  local hit
  switch v() do
    case 1 then hit = "one"; break
    case 2 then hit = "two"; break
    case 3 then hit = "three"; break
  end
  assert(hit == "two" and calls == 1)
end

-- case expressions are evaluated lazily and in order
do
  local log = {}
  local function e(x) log[#log + 1] = x; return x end
  switch 2 do
    case e(1), e(2), e(3) then log[#log + 1] = "body"; break
    case e(4) then log[#log + 1] = "no"; break
  end
  assert(table.concat(log, ",") == "1,2,body")
end

-- no match and no else does nothing; an empty switch is allowed
do
  local x = 0
  switch 5 do
    case 1 then x = 1
    case 2 then x = 2
  end
  assert(x == 0)
  switch 5 do end
end

-- plain equality, no coercion
local function kind(v)
  switch v do
    case nil then return "nil"
    case false then return "false"
    case true then return "true"
    case 0 then return "zero"
    case "0" then return "string zero"
    else return "other"
  end
end
assert(kind(nil) == "nil")
assert(kind(false) == "false")
assert(kind(true) == "true")
assert(kind(0) == "zero")
assert(kind("0") == "string zero")
assert(kind(1) == "other")
assert(kind({}) == "other")

-- varargs as the value
local function vs(...)
  switch ... do
    case 1 then return "one"
    else return "other"
  end
end
assert(vs(1, 2) == "one")
assert(vs(2, 1) == "other")

-- more case constants than fit into an RK operand
do
  local parts = {"local v = ... switch v do "}
  for i = 1, 300 do
    parts[#parts + 1] = string.format("case 'k%d' then return %d ", i, i)
  end
  parts[#parts + 1] = "else return -1 end"
  local f = assert(loadstring(table.concat(parts)))
  assert(f("k1") == 1 and f("k300") == 300 and f("nope") == -1)
end

-- nested switches
local function nested(a, b)
  local r = {}
  switch a do
    case 1 then
      switch b do
        case 1 then r[#r + 1] = "11"; break
        case 2 then r[#r + 1] = "12"
      end
      r[#r + 1] = "after-inner"
      break
    case 2 then r[#r + 1] = "2"
  end
  return table.concat(r, ",")
end
assert(nested(1, 1) == "11,after-inner")
assert(nested(1, 2) == "12,after-inner")
assert(nested(2, 1) == "2")
assert(nested(3, 3) == "")

-- every case body is its own scope
do
  local function f(v)
    switch v do
      case 1 then local x = "one"; return x
      case 2 then local x = "two"; return x
    end
  end
  assert(f(1) == "one" and f(2) == "two")

  local seen = "unset"
  switch 1 do
    case 1 then local y = 10
    case 2 then seen = y
  end
  assert(seen == nil)
end

-- break leaves only the switch, not the enclosing loop
do
  local n = 0
  for i = 1, 5 do
    switch i do
      case 3 then break
    end
    n = n + 1
  end
  assert(n == 5)
end

-- break is allowed in a switch outside of any loop
assert(loadstring("switch 1 do case 1 then break end"))
errmsg("break", "no loop to break")

-- closures created in a case body survive 'break'
do
  local fns = {}
  for i = 1, 3 do
    switch i do
      case 1, 2, 3 then
        local captured = i * 10
        fns[#fns + 1] = function() return captured end
        break
    end
  end
  assert(fns[1]() == 10 and fns[2]() == 20 and fns[3]() == 30)
end

-- ... and 'break' out of a loop from a nested block
do
  local f
  for i = 1, 3 do
    if i == 2 then
      local c = "kept"
      f = function() return c end
      break
    end
  end
  local clobber = {}
  local a, b, c, d = 1, 2, 3, 4
  assert(f() == "kept")
end

-- goto out of a switch
do
  local r = {}
  for i = 1, 3 do
    switch i do
      case 2 then goto done
      else r[#r + 1] = i
    end
  end
  ::done::
  assert(table.concat(r, ",") == "1")
end

-- 'switch', 'case' and 'continue' are reserved words
assert(not loadstring("local switch = 1"))
assert(not loadstring("local case = 1"))
assert(not loadstring("local continue = 1"))

-- continue: while
do
  local i, sum = 0, 0
  while i < 6 do
    i = i + 1
    if i % 2 == 0 then continue end
    sum = sum + i
  end
  assert(sum == 1 + 3 + 5)
end

-- continue: numeric for
do
  local sum = 0
  for i = 1, 10 do
    if i % 3 ~= 0 then continue end
    sum = sum + i
  end
  assert(sum == 3 + 6 + 9)
end

-- continue: generic for
do
  local out = {}
  for k, v in ipairs({10, 20, 30, 40}) do
    if v == 20 or v == 30 then continue end
    out[#out + 1] = v
  end
  assert(table.concat(out, ",") == "10,40")
end

-- continue: repeat-until jumps to the condition
do
  local i, sum = 0, 0
  repeat
    i = i + 1
    if i == 2 then continue end
    sum = sum + i
  until i >= 4
  assert(sum == 1 + 3 + 4)

  local n = 0
  repeat
    local step = 1
    n = n + step
    if n == 2 then continue end
  until n >= 4
  assert(n == 4)
end

-- continue must not skip a local that the 'until' condition can see
errmsg([[
  local x = 1
  repeat
    if x then continue end
    local late = 1
  until late
]], "local 'late'")

-- continue inside a switch targets the enclosing loop
do
  local out = {}
  for i = 1, 5 do
    switch i do
      case 2, 4 then continue
      case 3 then out[#out + 1] = "three"; break
    end
    out[#out + 1] = i
  end
  assert(table.concat(out, ",") == "1,three,3,5")
end

-- closures survive 'continue'
do
  local fns = {}
  for i = 1, 4 do
    local j = i * 10
    fns[#fns + 1] = function() return j end
    if i % 2 == 0 then continue end
    fns[#fns + 1] = function() return -j end
  end
  assert(#fns == 6)
  assert(fns[1]() == 10 and fns[2]() == -10 and fns[3]() == 20)
  assert(fns[4]() == 30 and fns[5]() == -30 and fns[6]() == 40)

  local fns2 = {}
  for i = 1, 3 do
    if i > 0 then
      local c = i
      fns2[i] = function() return c end
      continue
    end
  end
  assert(fns2[1]() == 1 and fns2[2]() == 2 and fns2[3]() == 3)
end

-- continue needs a loop of the same function
errmsg("continue", "no loop to continue")
errmsg("if true then continue end", "no loop to continue")
errmsg("switch 1 do case 1 then continue end", "no loop to continue")
errmsg("for i = 1, 3 do local f = function() continue end end", "no loop to continue")

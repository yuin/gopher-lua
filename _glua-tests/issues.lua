
-- issue #10
local function inspect(options)
    options = options or {}
    return type(options)
end
assert(inspect(nil) == "table")

local function inspect(options)
    options = options or setmetatable({}, {__mode = "test"})
    return type(options)
end
assert(inspect(nil) == "table")

-- issue #16
local ok, msg = pcall(function()
  local a = {}
  a[nil] = 1
end)
assert(not ok and string.find(msg, "table index is nil", 1, true))

-- issue #19
local tbl = {1,2,3,4,5}
assert(#tbl == 5)
assert(table.remove(tbl) == 5)
assert(#tbl == 4)
assert(table.remove(tbl, 3) == 3)
assert(#tbl == 3)

-- issue #24
local tbl = {string.find('hello.world', '.', 0)}
assert(tbl[1] == 1 and tbl[2] == 1)
assert(string.sub('hello.world', 0, 2) == "he")

-- issue 33
local a,b
a = function ()
  pcall(function()
  end)
  coroutine.yield("a")
  return b()
end

b = function ()
  return "b"
end

local co = coroutine.create(a)
assert(select(2, coroutine.resume(co)) == "a")
assert(select(2, coroutine.resume(co)) == "b")
assert(coroutine.status(co) == "dead")

-- issue 37
function test(a, b, c)
    b = b or string.format("b%s", a)
    c = c or string.format("c%s", a)
    assert(a == "test")
    assert(b == "btest")
    assert(c == "ctest")
end
test("test")

-- issue 39
assert(string.match("あいうえお", ".*あ.*") == "あいうえお")
assert(string.match("あいうえお", "あいうえお") == "あいうえお")

-- issue 47
assert(string.gsub("A\nA", ".", "A") == "AAA")
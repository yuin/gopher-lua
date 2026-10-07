
local ok, msg = pcall(function()
  string.dump()
end)
assert(not ok and string.find(msg, "GopherLua does not support the string.dump"))
assert(string.find("","aaa") == nil)
assert(string.gsub("hello world", "(%w+)", "%1 %1 %c") == "hello hello %c world world %c")

local ret1, ret2, ret3, ret4 = string.find("aaa bbb", "(%w+())")
assert(ret1 == 1)
assert(ret2 == 3)
assert(ret3 == "aaa")
assert(ret4 == 4)

-- issue #543: string.match returns nil when there is no match
assert(select("#", ("abc"):match("(%d+)")) == 1)
assert(("abc"):match("(%d+)") == nil)
assert(tonumber(("abc"):match("(%d+)")) == nil)
assert(select("#", string.match("abc", "%d")) == 1)

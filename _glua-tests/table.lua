local a = {}
assert(table.maxn(a) == 0)
a["key"] = 1
assert(table.maxn(a) == 0)
table.insert(a, 10)
table.insert(a, 3, 10)
assert(table.maxn(a) == 3)

local ok, msg = pcall(function()
  table.insert(a)
end)
assert(not ok and string.find(msg, "wrong number of arguments"))

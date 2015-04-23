
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

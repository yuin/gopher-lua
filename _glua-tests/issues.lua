
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


-- bug #10
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
